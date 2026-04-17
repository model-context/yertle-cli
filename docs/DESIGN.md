# Yertle CLI — Design Document

A command-line interface for interacting with the Yertle/Flow platform. Provides both static scriptable commands and rich interactive TUI views for exploring organizations, nodes, and live system health.

---

## Goals

1. **Scriptable first** — Static commands print clean output, work with pipes and shell scripts
2. **Rich when needed** — Interactive TUI views for exploration and monitoring
3. **Expandable** — Adding a new command is one file in `cmd/`, optionally one model in `tui/`
4. **Single binary** — No runtime dependencies, cross-platform distribution
5. **API consumer** — The CLI is a thin client over the existing FastAPI backend

---

## Tech Stack

| Component | Library | Purpose |
|-----------|---------|---------|
| CLI framework | [cobra](https://github.com/spf13/cobra) | Subcommand routing, flags, help text |
| TUI framework | [bubbletea](https://github.com/charmbracelet/bubbletea) | Elm-architecture TUI for interactive views |
| Styling | [lipgloss](https://github.com/charmbracelet/lipgloss) | Terminal colors, borders, padding |
| Components | [bubbles](https://github.com/charmbracelet/bubbles) | Pre-built tables, spinners, viewports, text inputs |
| Config | Built-in `encoding/json` | Auth tokens, API base URL, user preferences |
| HTTP | Built-in `net/http` | API calls to the FastAPI backend |
| Password input | [golang.org/x/term](https://pkg.go.dev/golang.org/x/term) | Hidden password prompt for `auth login` |

### Why Go

- Single binary — no Python/Node runtime needed on the user's machine
- Cross-compile for macOS, Linux, Windows from one codebase
- The Charm TUI ecosystem is the most mature option for terminal UIs
- Clean boundary from the Python backend — the CLI is a consumer, not an extension

### Why Cobra + Bubbletea (not just one)

Not every command needs a TUI. The CLI has two interaction modes:

- **Static commands** (`orgs list`, `nodes list`) — print output and exit, scriptable, pipeable
- **Interactive views** (`tree`, `monitor`) — launch a bubbletea program with live rendering and key events

Cobra handles the outer shell (routing, flags, help). Each subcommand decides whether to print a table or launch a bubbletea model.

---

## Command Structure

```
yertle
├── about                        # What Yertle is, data model, and CLI workflow guide
│
├── auth
│   ├── login                    # Authenticate with backend, store JWT token
│   └── status                   # Show current user, token expiry, API target
│
├── config
│   ├── set-org <org>            # Set default organization (saves to config file)
│   └── show                     # Show current configuration
│
├── orgs
│   └── list                     # List organizations the user belongs to
│
├── nodes
│   ├── list                     # List nodes (uses default org, or all orgs if unset)
│   └── show <id>               # Full node details: children, parents, connections, tags
│
├── tree                         # Containment hierarchy with short IDs
│
├── canvas <id>                  # ASCII architecture diagram of a node's children
│
└── monitor                      # [Future] Interactive: htop-like live health dashboard
```

### Global Flags

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--org` | `-o` | from config | Organization ID or name to operate on |
| `--format` | `-f` | `table` | Output format: `table`, `json`, `csv` |
| `--api-url` | | from config | Override the API base URL |
| `--no-color` | | `false` | Disable terminal colors |

---

## Command Details

### `yertle auth login`

Authenticate and store the JWT token locally.

```
$ yertle auth login
Email: user@example.com
Password: ********
Authenticated as user@example.com
Token stored in ~/.yertle/config.json
```

- Calls `POST /auth/signin` on the backend
- Stores access token, refresh token, and expiry in `~/.yertle/config.json`
- Password input is hidden via `x/term.ReadPassword`

### `yertle auth status`

```
$ yertle auth status
User:     user@example.com
API:      https://api.yertle.com
Token:    valid (expires in 47m)
Org:      my-org (default)
```

### `yertle config set-org <org>`

Set a default organization so `--org` isn't needed on every command.

```
$ yertle config set-org my-org
Default org set to "my-org"
Config saved to ~/.yertle/config.json
```

Most users work within a single org, so this avoids repetitive flag passing. Use `all` to query across all organizations.

### `yertle config show`

```
$ yertle config show
Config file:  ~/.yertle/config.json
API URL:      http://localhost:8000
Default org:  my-org
Logged in as: user@example.com
```

### `yertle orgs list`

List organizations the authenticated user belongs to.

```
$ yertle orgs list
NAME          ROLE     MEMBERS  NODES  ID
my-org        owner    5        12     ed411977-...
acme-corp     editor   12       34     a83bc012-...
```

- Static output, supports `--format json` and `--format csv`
- API: `GET /orgs`

### `yertle nodes list`

List nodes. Uses the default org from config, or queries all orgs if no default is set. Fetches all pages automatically.

```
$ yertle nodes list
TITLE              CHILDREN  PARENTS  DESCENDANTS  ANCESTORS  ORG       ID
Production API     3         1        8            2          ed411977  d29d9d78
Auth Service       0         1        0            3          ed411977  f1a2b3c4
Frontend           2         0        4            0          ed411977  e5f6a7b8
```

- `--org` flag overrides the default org for a single invocation
- When org is unset or `all`, queries `GET /orgs/all/nodes`
- When org is set, queries `GET /orgs/{org_id}/nodes`
- Supports `--format json` and `--format csv`
- All IDs shown as 8-char short IDs (full UUIDs available via `--format json`)

### `yertle nodes show <id>`

Show full details for a node including children, parents, connections, tags, and directories.

```
$ yertle nodes show d29d9d78
Production API
────────────────────────────────────────
  Description:  Core API serving all client applications
  Node ID:      d29d9d78
  Org ID:       ed411977

  Tags:
    - env: prod
    - team: backend

  Directories:
    - /services

  2 Parents:
    - Platform       86b72aeb
    - Infrastructure a83bc012

  3 Children:
    - Auth Service       f1a2b3c4
    - User Service       c6e5d4f3
    - Payment Service    725d259d

  2 Connections:
    - Auth Service → User Service (authenticates)
    - Payment Service → Stripe Gateway (calls)
```

- Accepts short IDs (resolved via cache) or full UUIDs
- Org auto-resolved from cache — no `--org` needed for previously seen nodes
- `--format json` outputs raw `/complete` endpoint response
- API: `GET /orgs/{org_id}/nodes/{node_id}/tree/main/complete`

### `yertle tree`

Containment hierarchy of all nodes as a tree with short IDs. Defaults to all orgs.

```
$ yertle tree
AWS (9f65231f)
  Root                               86b72aeb
  ├── EC2                            db9a8ecd
  │   ├── Orchestrator               e45301e5
  │   └── Placement Service          725d259d
  └── S3                             3e49e32d

Test org (2ceec156)
  Root                               2800d6d2
```

- Short IDs right-aligned in columns for clean scanning
- `--org` flag scopes to a single org (no org header printed)
- `--format json` outputs raw hierarchy entries
- API: `GET /orgs/all/hierarchy` or `GET /orgs/{org_id}/hierarchy`

### `yertle canvas <id>`

ASCII architecture diagram showing a node's children and their connections.

```
$ yertle canvas db9a8ecd
EC2
────────────────────────────────────────

                    ┌──────────────┐    ┌────────────────────┐    ┌────────────────────┐
                    │  IAM AuthZ   │    │ Placement Service  │    │ Host Manager/Nitro │
                    └──────────────┘    └────────────────────┘    └────────────────────┘

┌────────────────────┐    ┌──────────────┐    ┌────────────────────┐    ┌────────────────────┐
│ EC2 API Frontend   │───→│ Orchestrator │    │ Capacity Manager   │    │ Network Controller │
└────────────────────┘    └──────────────┘    └────────────────────┘    └────────────────────┘
                                │
                                ↓
                          ┌────────────────────┐    ┌────────────────────────┐
                          │ AMI/Image Service  │    │ Metadata Service(IMDS) │
                          └────────────────────┘    └────────────────────────┘

  Other connections:
    - Orchestrator → Network Controller (allocate ENIs)
    - EC2 API Frontend → IAM AuthZ (auth check)
```

- Positions boxes using `visual_properties` from the `/complete` endpoint
- Quantizes X/Y positions into a grid to match the UI layout
- Draws horizontal arrows between directly adjacent connected nodes
- Draws vertical arrows between same-column nodes in adjacent rows
- Complex connections (diagonal, cross-row) listed as text below the diagram
- `--format json` outputs raw `/complete` data
- API: `GET /orgs/{org_id}/nodes/{node_id}/tree/main/complete`

### `yertle about`

Prints product context, data model, and workflow guide. Useful for AI agents discovering the CLI.

### `yertle monitor`

**[Future]** Interactive htop-like live dashboard showing real-time health status of nodes. Requires backend health/monitoring endpoints.

```
$ yertle monitor --org my-org
┌─ System Health ─────────────────────────────────────────────┐
│ NODE                STATUS    LATENCY   UPTIME    ALERTS    │
│ Production API      ● UP      45ms      99.98%    0         │
│ Auth Service        ● UP      32ms      99.99%    0         │
│ Payment Service     ▲ WARN    890ms     99.2%     2         │
│ Frontend CDN        ● UP      12ms      100%      0         │
│ CI/CD Pipeline      ○ DOWN    —         94.1%     5         │
├─────────────────────────────────────────────────────────────┤
│ Refreshed 3s ago   5 nodes   3 healthy  1 warn  1 down     │
└─────────────────────────────────────────────────────────────┘

↑/↓ navigate  enter details  r refresh  f filter  q quit
```

**Behavior:**
- Polls health endpoints on a configurable interval (default 10s)
- Color-coded status indicators
- Drill into a node for detailed metrics
- API: TBD — requires health/monitoring endpoints on the backend

---

## Architecture

```
cli/
├── main.go                  # Entry point — calls cmd.Execute()
├── cmd/                     # Cobra command definitions (thin wiring layer)
│   ├── root.go              # Root command, global flags, AppContext, helpers (ensureAuth, formatOptionalInt)
│   ├── about.go             # about — product context for agents and humans
│   ├── auth.go              # auth login, auth status
│   ├── canvas.go            # canvas <id> — ASCII architecture diagram
│   ├── config.go            # config set-org, config show
│   ├── orgs.go              # orgs list
│   ├── nodes.go             # nodes list, nodes show
│   ├── tree.go              # tree — containment hierarchy with short IDs
│   └── monitor.go           # Placeholder (future)
├── api/                     # HTTP client for the FastAPI backend
│   ├── client.go            # Base client: do(), Get(), Post(), auth header injection
│   ├── auth.go              # SignIn(), RefreshToken(), VerifyToken()
│   ├── orgs.go              # GetOrganizations()
│   └── nodes.go             # GetNodes(), GetAllNodes(), GetHierarchy(), GetCompleteNode()
├── output/                  # Domain-agnostic output formatting
│   └── format.go            # RenderTable, RenderJSON, RenderCSV
├── config/                  # Configuration, auth tokens, and ID cache
│   ├── config.go            # Load, Save, Resolve, IsAuthenticated, IsTokenExpired
│   └── cache.go             # Short ID cache: LoadCache, Save, Put, Resolve, ShortID
├── tui/                     # Bubbletea interactive models (future)
│   └── tree/
│       └── model.go         # Package stub
├── go.mod
└── go.sum
```

### Layer rules

| Layer | Imports | Never imports |
|-------|---------|---------------|
| `config/` | stdlib only | anything in this project |
| `api/` | stdlib only | config, cmd, output, tui |
| `output/` | lipgloss, stdlib | config, api, cmd |
| `cmd/` | config, api, output | tui (until Phase 2) |
| `tui/` | bubbletea, lipgloss, bubbles, api | cmd, config |

Token is passed as a string to `api.NewClient()` — the api package never reads config directly.

### Adding a new command

1. Add a file in `cmd/` that registers a cobra subcommand
2. If it needs API data, add methods to `api/`
3. If it's interactive, add a bubbletea model in `tui/`
4. Register the command in `cmd/root.go` init()

### AppContext pattern

`cmd/root.go` defines an `AppContext` struct that is built in `PersistentPreRunE` and stored in the cobra command context. Every subcommand retrieves it via `GetAppContext(cmd)`:

```go
type AppContext struct {
    Config  *config.Config
    Cache   *config.IDCache
    Client  *api.Client
    Format  string   // "table", "json", "csv"
    NoColor bool
    OrgID   string   // resolved org (flag > env > config > "")
}
```

This keeps subcommands thin — they don't need to load config or construct clients.

Shared helpers in `root.go`:
- `ensureAuth(appCtx)` — returns error if not logged in
- `formatOptionalInt(v *int)` — formats `*int` as string or `"-"`

---

## Configuration

Stored at `~/.yertle/config.json`:

```json
{
  "api_url": "http://localhost:8000",
  "auth": {
    "access_token": "eyJ...",
    "refresh_token": "eyJ...",
    "expires_at": "2026-04-03T15:30:00Z",
    "email": "user@example.com"
  },
  "defaults": {
    "org": "my-org"
  }
}
```

- **`api_url`** — backend API base URL, defaults to `http://localhost:8000`
- **`auth`** — JWT tokens from `auth login`, omitted when not logged in
- **`defaults.org`** — default organization for commands that need one; when unset, commands query across all orgs

### Precedence

```
CLI flag (--org)  >  Environment variable (YERTLE_ORG)  >  Config file (defaults.org)  >  all orgs
```

Same pattern for API URL: `--api-url` > `YERTLE_API_URL` > config > `http://localhost:8000`.

### Config management

- `yertle config set-org <org>` — writes `defaults.org` to config file
- `yertle config show` — prints current config state
- Config file is created automatically on first `auth login` or `config set-org`
- Directory `~/.yertle/` is created with 0700 permissions, file with 0600

---

## Short ID Cache

Stored at `~/.yertle/id-cache.json`. Maps 8-character short IDs to full UUIDs with org context.

```json
{
  "entries": {
    "d29d9d78": {"full_id": "d29d9d78-1234-...", "org_id": "ed411977-...", "name": "Platform", "type": "node"},
    "ed411977": {"full_id": "ed411977-5678-...", "name": "my-org", "type": "org"}
  }
}
```

**Population:** Every data-fetching command (`tree`, `orgs list`, `nodes list`) auto-populates the cache as a side effect. Cache saves are non-critical (errors ignored).

**Resolution:** When a command receives an ID argument:
1. If it contains `-` or is 32+ chars → full UUID, use directly
2. Otherwise → look up in cache, return full UUID + org_id
3. If not found → pass through as-is (let the API error if invalid)

**Benefit:** Users can run `yertle tree` to browse, then `yertle nodes show d29d9d78` without needing `--org` or full UUIDs.

---

## Authentication Flow

```
yertle auth login
  → Prompt for email + password (password hidden)
  → POST /auth/signin
  → Receive access_token + refresh_token + expiresIn
  → Compute expires_at = now + expiresIn
  → Store in ~/.yertle/config.json
  → All subsequent commands read token from config
```

The API client (`api/client.go`) injects the `Authorization: Bearer <token>` header on every request when a token is present. Unauthenticated endpoints (like `/auth/signin`) work without a token.

---

## Output Formats

Static commands support multiple output formats via `--format`:

| Format | Use case | Example |
|--------|----------|---------|
| `table` (default) | Human-readable terminal output | Aligned columns with lipgloss-styled headers |
| `json` | Scripting, piping to `jq` | Indented JSON |
| `csv` | Spreadsheet export | Comma-separated with header row |

```bash
# Pipe to jq for scripting
yertle nodes list --format json | jq '.[].title'

# Export to CSV
yertle nodes list --format csv > nodes.csv
```

The `output/format.go` package is domain-agnostic. Each `cmd/` file defines its own columns using closures that type-assert rows:

```go
columns := []output.Column{
    {Header: "NAME", Value: func(r any) string { return r.(api.Organization).Name }},
    {Header: "ROLE", Value: func(r any) string { return r.(api.Organization).Role }},
}
```

Interactive commands (`tree`, `monitor`) always render TUI output and ignore `--format`.

---

## API Mapping

How CLI commands map to backend endpoints:

| Command | Backend Endpoint | Auth Required |
|---------|-----------------|---------------|
| `about` | None (local) | No |
| `auth login` | `POST /auth/signin` | No |
| `auth status` | Local token inspection | No |
| `orgs list` | `GET /orgs` | Yes |
| `nodes list` (with org) | `GET /orgs/{org_id}/nodes` | Yes |
| `nodes list` (all orgs) | `GET /orgs/all/nodes` | Yes |
| `nodes show` | `GET /orgs/{org_id}/nodes/{id}/tree/main/complete` | Yes |
| `tree` (single org) | `GET /orgs/{org_id}/hierarchy` | Yes |
| `tree` (all orgs) | `GET /orgs/all/hierarchy` | Yes |
| `canvas` | `GET /orgs/{org_id}/nodes/{id}/tree/main/complete` | Yes |
| `monitor` | TBD | Yes |

---

## Phased Rollout

### Phase 1 — Foundation ✅

- [x] Project scaffolding: `go.mod`, `main.go`, cobra root command
- [x] Config management: `~/.yertle/config.json` with precedence resolution
- [x] Default org support: `config set-org`, fallback to all orgs
- [x] API client: base HTTP client with auth header injection
- [x] `auth login` / `auth status`
- [x] `orgs list` — table output with short IDs
- [x] `nodes list` — paginated, with descendants/ancestors columns
- [x] Output formatting: table, json, csv via `--format` flag

### Phase 2 — Navigation & Inspection ✅

- [x] `tree` — containment hierarchy with short IDs, right-aligned columns
- [x] `tree` — multi-org support with org headers showing short org IDs
- [x] `nodes show` — full node details with aligned sections
- [x] `canvas` — ASCII architecture diagram from visual_properties
- [x] `about` — product context for agents and humans
- [x] Short ID cache (`~/.yertle/id-cache.json`) with auto-population
- [x] Cache-based ID + org resolution across all commands
- [x] Backend: `GET /orgs/{org_id}/hierarchy` and `GET /orgs/all/hierarchy` endpoints
- [x] Code review fixes: pagination, error handling, shared helpers

### Phase 3 — Search & History (next)

See `ROADMAP.md` for the full feature roadmap. Next priorities:
- `yertle nodes search` — free-text and tag-based search
- `yertle diff` — recent commits on a node
- Health status per node + `yertle monitor`
