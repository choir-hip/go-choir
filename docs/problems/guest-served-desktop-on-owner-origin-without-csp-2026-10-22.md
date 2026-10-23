# The signed-in Desktop is served from the guest on the owner's origin, with no CSP

Status: open. This record documents the problem first; this commit has no
fix. Mutation class of a fix: red (auth/session surface, edge routing).

Found 2026-10-22 by reading code during the provider vault design. The
risk below is an inference from code, not an observed exploit.

## What the code does

- For a signed-in user, the proxy reverse-proxies the Desktop, Texture,
  apps, Settings and assets from that user's guest computer
  (`internal/proxy/computer_surface.go:12-30`). Caddy sends every path
  that is not `/auth`, `/api`, `/health`, `/provider` or `/internal` to
  the proxy (`nix/node-b.nix:750-785`). So guest-served HTML and script
  run on the same origin as `/auth/*` and `/api/*`.
- The access cookie `choir_access` is HttpOnly, SameSite=Lax and Path `/`
  (`internal/auth/handlers.go:336-355`). Script on that origin cannot read
  it, but every same-origin request it makes carries it.
- No Content-Security-Policy, X-Frame-Options or HSTS header is set on the
  app shell. Caddy sets none (`nix/node-b.nix:729-785`), the proxy sets
  only Cache-Control (`computer_surface.go`), and `frontend/index.html`
  has no CSP meta tag. CSP exists only for mail HTML in a sandboxed iframe
  (`frontend/src/lib/MailApp.svelte:1258`, `:1952`) and for runtime assets
  (`internal/agentcore/api_assets.go:60-66`).
- The frontend has `{@html}` sinks, and their sanitization was not traced:
  `BrowserApp.svelte:289`, `ContentViewer.svelte:160`,
  `SlidesApp.svelte:458` and `TextureEditor.svelte:2013`.

## Impact (inference)

Per-computer frontend (`C15`/`I25`) means a computer's own UI can change
through self-development. Script that a guest serves, or an XSS in a sink
above, runs with the owner's session. Within the 5-minute access-cookie
lifetime it can call any route the session can, owner acts included,
because those take a cookie with no step-up
(`docs/problems/no-step-up-and-no-platform-owner-2026-10-22.md`).

## Consequence for the vault design

The admin panel must not live on this origin. The design puts it on a
host-served origin of its own, with a strict CSP
(`docs/provider-vault-and-model-policy-design-2026-10-22.md` §5.1).
