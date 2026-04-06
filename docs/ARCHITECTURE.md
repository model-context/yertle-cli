# Yertle CLI Architecture

This document describes the implementation that exists in this repository today. It is complementary to [DESIGN.md](/Users/almiller/Applications/yertle-repos/yertle-cli/docs/DESIGN.md), which is more product- and roadmap-oriented.

## Overview

The CLI is organized around a small number of packages with clear responsibilities:

- `cmd/`: command definitions, command dispatch, and terminal presentation
- `api/`: HTTP transport, endpoint wrappers, and API response structs
- `config/`: persistent config, auth state, flag/env resolution, and short-ID cache
- `output/`: generic table, CSV, and JSON rendering
- `tui/`: reserved for future interactive views

The main execution path is:

1. [main.go](/Users/almiller/Applications/yertle-repos/yertle-cli/main.go) calls `cmd.Execute()`.
2. [cmd/root.go](/Users/almiller/Applications/yertle-repos/yertle-cli/cmd/root.go) loads config, resolves global flags, constructs the API client and ID cache, and stores them in an `AppContext`.
3. Each subcommand pulls the shared `AppContext` from the Cobra command context with `GetAppContext`.
4. Commands call into `api.Client`, then render either structured output or custom terminal views.

## Command Layer

The root command defines global behavior:

- Global flags: `--org`, `--format`, `--api-url`, `--no-color`
- Shared runtime state in `AppContext`
- Shared auth guard via `ensureAuth`
- Shared nullable-int formatting via `formatOptionalInt`
- A custom expanded help template instead of Cobra's default grouped help

Command routing is intentionally shallow:

- `auth`, `config`, `orgs`, and `nodes` behave as both top-level commands and command groups
- `orgs [org-id]` dispatches to list-or-show behavior based on whether an argument is present
- `nodes [id]` does the same
- `tree [org-id]` accepts an optional positional org override
- `canvas <node-id>` and `monitor` are standalone subcommands

This keeps the UX compact while still allowing subcommands such as `auth login` and `auth status`.

## Runtime Context

[cmd/root.go](/Users/almiller/Applications/yertle-repos/yertle-cli/cmd/root.go) is the repository's central integration point.

During `PersistentPreRunE`, it:

1. Loads persisted config from `~/.yertle/config.json`
2. Resolves API URL and org scope using flag-over-env-over-config precedence
3. Creates an `api.Client`
4. Enables token refresh when a refresh token is available
5. Loads the short-ID cache from `~/.yertle/id-cache.json`
6. Expands cached short org IDs into full org IDs when possible
7. Attaches the resulting `AppContext` to the Cobra context

Every command executes after that initialization, so the repo avoids repeating setup code in individual command files.

## API Layer

[api/client.go](/Users/almiller/Applications/yertle-repos/yertle-cli/api/client.go) is deliberately minimal:

- `NewClient(baseURL, token)` creates a client with a 30-second timeout
- `do()` builds JSON requests, sets headers, executes the request, and unmarshals the response
- Non-2xx responses become `APIError`
- If an error is a `401`, `doWithRefresh()` will try `POST /auth/refresh` once and then retry the original request

Current endpoint wrappers are grouped by concern:

- [api/auth.go](/Users/almiller/Applications/yertle-repos/yertle-cli/api/auth.go): `SignIn`, `RefreshToken`, `VerifyToken`
- [api/orgs.go](/Users/almiller/Applications/yertle-repos/yertle-cli/api/orgs.go): `GetOrganizations`, `GetOrganization`
- [api/nodes.go](/Users/almiller/Applications/yertle-repos/yertle-cli/api/nodes.go): `GetNodes`, `GetAllNodes`, `GetHierarchy`, `GetAllHierarchy`, `GetCompleteNode`

The API layer is strongly typed but intentionally close to the wire format. Most display-specific transformation happens in `cmd/`.

## Config And Local State

[config/config.go](/Users/almiller/Applications/yertle-repos/yertle-cli/config/config.go) stores only two things:

- The API base URL
- Auth credentials and expiry metadata

Notable behaviors:

- Missing config file is treated as normal and produces a default config
- Default API target is `http://localhost:8000`
- Auth is considered active when an access token exists
- Expiry checks are local and time-based

[config/cache.go](/Users/almiller/Applications/yertle-repos/yertle-cli/config/cache.go) implements short-ID resolution:

- `ShortID()` strips dashes and keeps the first 8 characters
- `Put()` stores node or org metadata keyed by short ID
- `Resolve()` returns full IDs and cached org IDs for short inputs
- Missing or corrupt cache data is treated as non-fatal

This cache is important to the CLI experience because commands like `nodes show` and `canvas` often need an org ID to build API paths, and the cache is how the tool recovers that context from a short node ID.

## Output Strategy

[output/format.go](/Users/almiller/Applications/yertle-repos/yertle-cli/output/format.go) centralizes generic output rendering:

- `RenderTable` calculates widths, prints colored headers by default, and writes aligned rows
- `RenderJSON` writes indented JSON
- `RenderCSV` writes plain CSV with command-defined column extraction

Commands choose their render path based on the global `--format` flag. This keeps most list-style commands consistent without introducing heavy abstractions.

## Command-Specific Behavior

### `auth`

- `auth` defaults to status mode
- `auth login` reads email from stdin and password with hidden terminal input
- Successful login persists access token, refresh token, expiry, and email

### `orgs`

- `orgs` fetches `/orgs`, renders name/role/member/node counts, and seeds the org cache
- `orgs <org-id>` fetches one org and prints a structured detail view unless `--format json` is used

### `nodes`

- `nodes` paginates in batches of 50 until the API reports all nodes have been fetched
- `nodes <id>` resolves a short ID via cache, infers the org when possible, then fetches `/tree/main/complete`
- Detail rendering is intentionally richer than the wire format: tags are sorted, related nodes are aligned, and ingress/egress are printed separately

### `tree`

- Builds an in-memory tree from flat hierarchy entries
- Groups entries by organization for multi-org output
- Uses a three-pass render: collect lines, compute max width, print aligned IDs

The current tree implementation depends on `HierarchyEntry.Path` and `IsDirectory` rather than recursively fetching nodes.

### `canvas`

- Reuses the complete-node endpoint already used by `nodes show`
- Reads child-node positions from `visual_properties`
- Quantizes raw coordinates into a coarse row/column grid
- Draws only simple left-to-right adjacent arrows and same-column adjacent vertical arrows directly
- Emits all remaining connections as a textual fallback section

This approach keeps the renderer deterministic and simple, but it is not a full graph-layout engine.

### `monitor`

- Present in the command surface, but not implemented beyond a placeholder message

## Error Handling Philosophy

The code favors explicit user-facing errors at command boundaries and lenient handling for local convenience state:

- Authentication is checked early with `ensureAuth`
- Config-load failures stop execution
- Cache-save failures are treated as non-critical in commands
- Corrupt cache data is discarded rather than failing the command
- Missing org context for node-detail commands produces a direct remediation hint: use `--org` or populate the cache first

## Known Gaps

A few architectural limitations are visible in the current implementation:

- No automated test suite yet
- No interactive TUI implementation despite the future-facing package layout
- No search, diff, trace, or watch commands yet
- Canvas rendering handles only the simplest connection geometries visually
- The CLI stores auth and cache state locally but has no explicit logout command

Those gaps align with the roadmap and placeholder code already present in the repository.
