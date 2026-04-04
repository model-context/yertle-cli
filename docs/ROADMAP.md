# Yertle CLI — Roadmap

What's been built (Phase 1) and what's next to make the CLI indispensable for engineers and AI agents debugging, navigating, and monitoring software systems.

---

## Completed — Phase 1: Foundation

- [x] `auth login` / `auth status` — JWT authentication
- [x] `config set-org` / `config show` — default org management
- [x] `orgs list` — list organizations with short IDs
- [x] `nodes list` — list nodes with children, parents, descendants, ancestors
- [x] `nodes show <id>` — full node details from `/complete` endpoint
- [x] `tree` — containment hierarchy with box-drawing characters
- [x] `about` — product context for agents and humans
- [x] Short ID cache — 8-char IDs with auto-populated `~/.yertle/id-cache.json`
- [x] Multi-format output — `--format table/json/csv` for all static commands
- [x] `GET /orgs/{org_id}/hierarchy` + `GET /orgs/all/hierarchy` backend endpoints

---

## Tier 1 — High impact, builds on what exists

### `yertle nodes search <query>`

Free-text search across node titles, descriptions, and tags. The backend already has `POST /orgs/{org_id}/nodes/search` with filters for `title_contains`, tag key-value pairs, directory paths, child/parent count ranges, and more.

**Why it matters:** An agent investigating an incident needs to find the relevant node fast — "find me the payment service" or "show me everything tagged env:prod." Search is the bridge between knowing something exists and finding it.

**Scope:**
- Positional arg for title search: `yertle nodes search "payment"`
- Flag-based tag filters: `--tag env:prod`
- Flag-based directory filter: `--dir /services`
- Uses `POST /orgs/{org_id}/nodes/search` (already exists)
- Table output with same columns as `nodes list`

### `yertle canvas <id>` — ASCII architecture diagram

Render a node's children and connections as a box-and-arrow diagram in the terminal. The data is all there from the `/complete` endpoint — child nodes have positions (`visual_properties` with `position_x`, `position_y`), and connections have `from_child_id`, `to_child_id`, labels, and edge directions.

**Why it matters:** A tree shows containment, but doesn't show how things connect. The canvas shows the actual architecture — which service calls which, how data flows. This is the view an engineer draws on a whiteboard. Rendering it in the terminal means an agent can "see" the architecture without opening a browser.

**Approach options (incremental):**
1. **Simple:** List connections as `from → to (label)` — already done in `nodes show`
2. **Medium:** Position boxes in a rough grid based on `position_x`/`position_y`, draw ASCII arrows
3. **Rich:** Interactive bubbletea canvas with scrolling and node selection

**Start with option 2.** Map the float positions to terminal columns/rows, draw boxes with `┌─┐│└─┘`, and connect them with `───→` arrows. The frontend already lays things out left-to-right, so this should map naturally.

### `yertle diff <id>` — what changed recently

Show the last N commits on a node. When debugging, the first question is often "what changed?"

**Why it matters:** An agent or engineer responding to an incident needs to correlate the problem with recent changes. "The payment service is failing — what changed in the last hour?" This answers that question from the terminal.

**Scope:**
- Default: show last 10 commits
- `--limit N` to control how many
- Each commit: hash (short), message, author, timestamp
- `--format json` for agent consumption
- Uses `GET /orgs/{org_id}/nodes/{node_id}/tree/{branch}/commits` (already exists)

---

## Tier 2 — Requires backend work

### Health status per node

Add a health/status dimension to nodes so the tree and list commands become actionable at a glance.

**Tree with health indicators:**
```
├── Payment Service  ▲ WARN   725d259d
├── Auth Service     ● UP     648935db
└── Database         ○ DOWN   a96d131f
```

**What's needed:**
- Backend: health status table or field on nodes (UP/WARN/DOWN + optional message)
- Backend: API to set/get health status (could be push-based from monitoring tools)
- CLI: `tree` and `nodes list` show status inline
- CLI: `yertle monitor` becomes the htop-like live view

**`yertle monitor` — htop-like live dashboard:**
- Polls health endpoints on a configurable interval
- Color-coded status indicators (green/yellow/red)
- Sortable by status, name, latency
- Drill into a node for detailed metrics
- Built with bubbletea for interactive TUI

### AWS metrics integration

If nodes have ARN tags, pull CloudWatch metrics and display them alongside node info.

**What it could look like:**
```
$ yertle nodes show <id> --metrics
EC2
────────────────────────────────────────
  Node ID:      db9a8ecd
  ARN:          arn:aws:ec2:us-east-1:123456789:instance/i-abc123

  Metrics (last 5m):
    CPU:        23.4%
    Network In: 1.2 MB/s
    Status:     passing (2/2 checks)
```

**What's needed:**
- CLI-side AWS SDK integration (read CloudWatch using local AWS credentials)
- Tag convention: nodes with an `ARN` tag get metrics pulled automatically
- No backend changes — this is a CLI-only feature reading directly from AWS

---

## Tier 3 — Differentiators

### `yertle trace <from> <to>` — path finding

Find the connection path between two nodes in the graph. "How does a request get from the API gateway to the database?"

**Why it matters:** Large systems have deep dependency chains. An agent debugging a latency issue needs to understand the full request path. This command walks the connection graph and shows every hop.

**Output:**
```
$ yertle trace api-gw database
API Gateway → Auth Service → User Service → Database
  1. API Gateway → Auth Service (authenticate)
  2. Auth Service → User Service (get user)
  3. User Service → Database (query)
```

**What's needed:**
- Backend: graph traversal endpoint (BFS/DFS on connections)
- Or: CLI-side traversal using multiple `/complete` calls (slower but no backend change)

### `yertle watch <id>` — live change subscription

Subscribe to changes on a node via polling. An agent running a deployment could watch for status changes in real-time.

```
$ yertle watch <id>
[14:32:01] Commit: "Update connection labels" by albert
[14:32:45] Health: UP → WARN (high latency)
[14:33:12] Commit: "Scale up replicas" by deploy-bot
[14:33:15] Health: WARN → UP
```

### Agent-optimized output mode

A `--format agent` flag that outputs structured context optimized for LLM consumption — not raw JSON, but a curated summary with the most relevant information first, relationships explained in natural language, and suggested next commands.

```
$ yertle nodes show <id> --format agent
This is "Payment Service", a node in the "AWS" organization.
It has 3 children: Stripe Gateway, Invoice Generator, and Tax Calculator.
Stripe Gateway connects to Invoice Generator (labeled "on success").
It is contained by "Backend Platform" and has no health alerts.
To inspect children: yertle nodes show <child-id>
To see recent changes: yertle diff <id>
```

---

## Priority order

For maximum usefulness to both agents and humans:

1. `yertle nodes search` — trivial to build, immediately useful
2. `yertle diff` — easy to build, critical for debugging
3. `yertle canvas` — medium effort, makes architecture tangible in the terminal
4. Health status + `yertle monitor` — needs backend design, unlocks the ops use case
5. AWS metrics — high impact for infra teams, CLI-only implementation
6. `yertle trace` — graph traversal, powerful for debugging complex systems
7. `yertle watch` / `--format agent` — polish and agent optimization
