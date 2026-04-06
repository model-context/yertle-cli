# Yertle CLI

The Yertle CLI is a Go command-line client for exploring the Yertle platform. It gives engineers and AI agents a navigable view of software systems built around organizations, nodes, containment hierarchies, connections, tags, and directories.

This repository is the CLI implementation. It is intentionally thin: Cobra handles command routing, a small API client talks to the backend over HTTP, and commands render either scriptable output (`table`, `json`, `csv`) or terminal-native views such as the ASCII tree and canvas.

## Current Scope

Implemented today:

- `auth` and `auth login` for API authentication
- `config` for showing active CLI configuration
- `orgs` for listing organizations or inspecting one org
- `nodes` for listing nodes or inspecting one node in detail
- `tree` for rendering containment hierarchies
- `canvas` for rendering an ASCII architecture sketch from child-node layout data
- `about` for product and workflow context

Planned but not implemented:

- `monitor` currently prints a placeholder message
- `tui/tree` is a stub for a future interactive view

## Build

The module currently targets Go `1.26.1` in [go.mod](/Users/almiller/Applications/yertle-repos/yertle-cli/go.mod).

```bash
go build -o yertle .
```

For local runs inside restricted environments, it can help to override `GOCACHE`:

```bash
GOCACHE=/tmp/yertle-go-cache go run . --help
```

## Configuration

The CLI reads and writes local state under `~/.yertle/`.

- Config file: `~/.yertle/config.json`
- Short-ID cache: `~/.yertle/id-cache.json`

Config behavior from the current code:

- Default API URL is `http://localhost:8000`
- `YERTLE_API_URL` overrides the config value
- `--api-url` overrides both config and environment
- `YERTLE_ORG` sets the default org scope for a command
- `--org` overrides `YERTLE_ORG`

Authentication data is stored in the config file after `yertle auth login`. The CLI also supports automatic access-token refresh when a refresh token is present.

## Common Workflow

1. Authenticate:

```bash
yertle auth login
```

2. Confirm the active target:

```bash
yertle auth status
yertle config
```

3. Populate the short-ID cache and inspect the model:

```bash
yertle orgs
yertle tree
yertle nodes
```

4. Drill into a specific object:

```bash
yertle orgs <org-id>
yertle nodes <node-id>
yertle canvas <node-id>
```

Short IDs are the first 8 characters of an ID with dashes removed. Commands such as `tree`, `orgs`, and `nodes` populate the cache so later commands can resolve short IDs back to full IDs.

## Output Modes

Static commands support:

- `--format table`
- `--format json`
- `--format csv`

Examples:

```bash
yertle orgs --format json
yertle nodes --org <org-id> --format csv
yertle tree --format json
```

`canvas` and the default `tree` view are optimized for terminal reading. `canvas --format json` falls back to raw API data.

## Command Notes

### `yertle orgs`

- With no argument, lists organizations available to the authenticated user
- With one argument, shows details for a specific organization
- Successful list calls populate the org portion of the short-ID cache

### `yertle nodes`

- With no argument, pages through all nodes for the active org scope
- With one argument, fetches the node's complete view, including parents, children, connections, tags, directories, ingress, and egress
- Node detail lookup needs an org. If you do not pass `--org`, the CLI tries to recover the org from the local short-ID cache

### `yertle tree`

- Defaults to all orgs when no org scope is set
- Groups hierarchy output by organization when multiple orgs are returned
- Uses box-drawing characters and right-aligned short IDs for scanability

### `yertle canvas`

- Fetches the same complete node payload used by `nodes show`
- Uses `visual_properties.position_x` and `position_y` to place child nodes onto a coarse terminal grid
- Draws only simple adjacent horizontal and vertical connections directly; other edges are listed underneath the diagram

## Repository Layout

- [main.go](/Users/almiller/Applications/yertle-repos/yertle-cli/main.go): program entrypoint
- [cmd/](/Users/almiller/Applications/yertle-repos/yertle-cli/cmd): Cobra commands and presentation logic
- [api/](/Users/almiller/Applications/yertle-repos/yertle-cli/api): HTTP client and response types
- [config/](/Users/almiller/Applications/yertle-repos/yertle-cli/config): config loading, saving, env/flag resolution, ID cache
- [output/](/Users/almiller/Applications/yertle-repos/yertle-cli/output): shared renderers for table, JSON, and CSV output
- [tui/](/Users/almiller/Applications/yertle-repos/yertle-cli/tui): placeholder location for future interactive UIs
- [docs/](/Users/almiller/Applications/yertle-repos/yertle-cli/docs): design, roadmap, demo, review, and architecture notes

## Additional Documentation

- [Architecture](/Users/almiller/Applications/yertle-repos/yertle-cli/docs/ARCHITECTURE.md)
- [Design](/Users/almiller/Applications/yertle-repos/yertle-cli/docs/DESIGN.md)
- [Roadmap](/Users/almiller/Applications/yertle-repos/yertle-cli/docs/ROADMAP.md)
- [Code Review Notes](/Users/almiller/Applications/yertle-repos/yertle-cli/docs/CODE_REVIEW.md)
- [Demo Notes](/Users/almiller/Applications/yertle-repos/yertle-cli/docs/DEMO.md)
