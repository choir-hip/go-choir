# choir.news serves every response uncompressed (2026-10-16)

Status: open. Found at 22:56 UTC while measuring article opens for the
owner ("opening an article is slow… How can we optimize resource usage?
Be ambitious with optimizations").

## Evidence

Asked with `Accept-Encoding: gzip, br, zstd`, the edge answers with no
`Content-Encoding` on any route:

| Response | On the wire | gzip -6 locally |
|---|---|---|
| Story bundle, the front's largest (`/api/platform/publications/resolve`) | 466,887 B | 101,066 B |
| App script `/assets/index-CdlNYlKx.js` | 270,569 B | not measured |
| `/assets/TextureEditor-bm6ls-ov.js` (loaded on the first story open) | 127,768 B | not measured |
| `/assets/index-CjKbJs93.css` | 77,351 B | not measured |

A browser on choir.news (headless, desktop and iPhone 13 size, after
575cb3ef5) shows a story 0.2–0.35 s after the click. 0.12–0.16 s of
that is the bundle request, and the bundle is up to 456 KB. The server time
is now small (see article-open-slow-store-b-scans-2026-10-16.md), so on a
phone the bytes are what is left: JSON that is 4.6 times larger than it
needs to be, and about 475 KB of script and CSS on a first visit.

## Mechanism

`nix/node-b.nix` configures Caddy 2.11.4 for `choir.news` with
`reverse_proxy` handles only. There is no `encode` directive, and the Go
proxy behind it (127.0.0.1:8082) does not compress either.

## Constraints on a fix

- Live updates are Server-Sent Events (`text/event-stream`: the Texture
  document stream, trajectory events, observations). A compressor that
  buffers them would stall the live view. The gateway's NDJSON stream is
  not on the public edge (`/provider/*` is refused).
- WebSocket upgrades must pass untouched.

## Fix direction (orange, edge config)

Caddy `encode zstd gzip` on the `choir.news` site, with an explicit
`match` on the content types that benefit (JSON, HTML, JavaScript, CSS,
SVG), so `text/event-stream` is never encoded whatever Caddy's default
list does. Acceptance: on choir.news the story bundle and the app
scripts come back with `Content-Encoding`, and an SSE route does not.
