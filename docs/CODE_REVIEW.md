# CLI Code Review — Action Items

Review conducted on 2026-04-03 after Phase 1 completion.

## Fix Now

- [x] **No pagination** — Added `fetchAllNodes()` helper that pages through all results in a loop.
- [x] **Cache errors silently swallowed** — `cache.Save()` now returns `error`. `LoadCache()` handles corrupt JSON by returning empty cache. Callers use `_ = cache.Save()` to explicitly ignore (non-critical).
- [x] **Unsafe type assertion in `GetAppContext`** — Added nil check with descriptive panic message.
- [x] **Repeated `*int` formatting** — Extracted `formatOptionalInt()` helper in `root.go`, used across all column definitions.
- [x] **Repeated auth check** — Extracted `ensureAuth()` helper in `root.go`, used by `orgs list`, `nodes list`, `nodes show`, `tree`.

## Address Later

| Issue | Why it can wait |
|-------|-----------------|
| Token auto-refresh on 401 | Users can re-run `auth login`. Refresh adds complexity for MVP. |
| Cache unbounded growth | Keyed by 8-char prefix. Real-world: hundreds of entries. Revisit if needed. |
| API client interface for testing | No tests yet. Add interface when we write tests. |
| Password not zeroed from memory | CLI exits immediately after login. Not a long-running service. |
| Package-level doc comments | Nice to have, not blocking. |
