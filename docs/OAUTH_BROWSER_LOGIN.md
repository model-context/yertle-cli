# CLI OAuth Browser Login

> **Status:** Planned — depends on the Cognito OAuth infrastructure being deployed first (see `yertle-mcp/docs/MCP_DEV_IMPLEMENTATION.md`).

## Overview

Replace the current email/password prompt in `yertle auth login` with a browser-based OAuth 2.1 login flow. This is the same pattern used by `gh auth login`, `aws sso login`, and `gcloud auth login`.

**Before (current):**
```
$ yertle auth login
Email: albert@example.com
Password: ********
Logged in as albert@example.com
```

**After:**
```
$ yertle auth login
Opening browser to authenticate...
If browser doesn't open, visit: https://auth.yertle.com/oauth2/authorize?...

Waiting for authentication... done
Logged in as albert@example.com
```

## Shared Infrastructure

This feature reuses OAuth infrastructure built for the MCP server deployment — no new AWS resources needed.

| Resource | Details |
|----------|---------|
| Cognito hosted UI | `auth.albertcmiller.com` (dev) / `auth.yertle.com` (prod) |
| App client | `OAuthUserPoolClient` — shared with MCP server |
| Callback URL | `http://localhost:9876/callback` (already registered on the client) |
| OAuth flow | Authorization code + PKCE (S256) |
| Scopes | `openid`, `profile`, `email` |

## Browser Login Flow

```
CLI                           Browser                        Cognito
 │                              │                              │
 │  1. Generate PKCE verifier   │                              │
 │     + code challenge         │                              │
 │                              │                              │
 │  2. Start localhost:9876     │                              │
 │     HTTP server              │                              │
 │                              │                              │
 │  3. Open browser ──────────► │                              │
 │     /oauth2/authorize?       │                              │
 │     client_id=...&           │  4. User logs in ──────────► │
 │     code_challenge=...&      │                              │
 │     redirect_uri=            │  5. Redirect ◄────────────── │
 │     localhost:9876/callback  │     ?code=AUTH_CODE           │
 │                              │                              │
 │  6. Receive callback ◄───── │                              │
 │     with auth code           │                              │
 │                              │                              │
 │  7. Exchange code + verifier ──────────────────────────────► │
 │     POST /oauth2/token                                      │
 │                                                             │
 │  8. Receive tokens ◄─────────────────────────────────────── │
 │     {access_token, id_token, refresh_token}                 │
 │                                                             │
 │  9. Store in ~/.yertle/config.json                           │
 │     (same format as today)                                  │
```

## Go Implementation

### Files to modify

**`cmd/auth.go`** — Replace `loginCmd` logic:
- Generate PKCE code verifier (32 random bytes, base64url) and challenge (SHA256 of verifier, base64url)
- Build Cognito authorize URL with params: `client_id`, `response_type=code`, `scope`, `redirect_uri`, `code_challenge`, `code_challenge_method=S256`, `state`
- Start temporary HTTP server on `localhost:9876`
- Open browser via `open` (macOS) / `xdg-open` (Linux) / `start` (Windows)
- Wait for callback with auth code (with timeout, e.g., 2 minutes)
- Exchange code for tokens via `POST /oauth2/token`
- Store tokens in config (same `AuthConfig` struct, same file)
- Shut down temporary server

**`api/auth.go`** — Add token exchange function:
```go
func (c *Client) ExchangeCode(code, codeVerifier, redirectURI string) (*TokenResponse, error)
// POST to Cognito /oauth2/token endpoint (not the Flow API)
```

**`config/config.go`** — Add Cognito OAuth config fields:
```go
type OAuthConfig struct {
    CognitoDomain string `json:"cognito_domain"` // e.g., "auth.yertle.com"
    ClientID      string `json:"client_id"`       // OAuthUserPoolClient ID
}
```

These could also come from a hardcoded default or `--cognito-domain` flag, depending on how we want to handle dev vs. prod.

**`cmd/auth.go`** — Keep `yertle auth status` as-is (token format unchanged).

### What stays the same

- **Token storage**: `~/.yertle/config.json` with `AuthConfig` struct (access_token, refresh_token, expires_at, email)
- **Token refresh**: `POST /auth/refresh` on 401 via `doWithRefresh()` in `api/client.go`
- **Bearer header**: `Authorization: Bearer <token>` on every request
- **File permissions**: 0700 dir, 0600 file

### Optional: keep password login as fallback

For headless/CI environments where no browser is available:
```
$ yertle auth login --manual
Email: albert@example.com
Password: ********
```

This keeps the current `POST /auth/signin` path working alongside the new browser flow.

## Dependencies

- Cognito `UserPoolDomain` deployed (hosted UI available)
- `OAuthUserPoolClient` deployed with `http://localhost:9876/callback` in `CallbackURLs`
- Both are part of the MCP server deployment — see `yertle-mcp/docs/MCP_DEV_IMPLEMENTATION.md`, Step 1

## Benefits

- No passwords typed into terminals
- MFA-ready — handled entirely in the browser by Cognito
- SSO-ready — if Google/GitHub identity providers are added to Cognito, the CLI gets them for free
- Consistent with industry standard CLI auth patterns
