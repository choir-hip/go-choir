# choir.news serves every response uncompressed (2026-10-16)

Status: fixed and deployed (869420ad3, 1a0eef8de; staging on 85a9f5742 at 23:27).
Found at 22:56 UTC while measuring article opens for the
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

## Regression introduced by the fix (869420ad3, found 23:25)

Deployed at 23:15, the edge compresses: on the wire the story bundle is
55 KB (zstd) instead of 467 KB, the app script 91 KB instead of 271 KB,
the stylesheet 14 KB instead of 77 KB. The deployed spec passes 3 of 3.

But on staging an SSE stream that has not sent an event yet now sends **no
response headers** when the client advertises compression (every browser
does). On the paper's Texture document stream, with
`Accept-Encoding: identity` the headers came at 0.96 s; with
`gzip, br, zstd` none came in 25 s. Caddy's encoder holds the headers
until the first body write, so it can choose an encoding, and it ignores
the upstream's early flush. The content-type match is only checked once
there is a body. The local harness (Caddy 2.11.7, HTTP/1.1) did not show
this; Node B runs 2.11.4 over HTTP/2.

Impact: `frontend/src/lib/lifecycle.js` waits for the EventSource `open`
before it loads the trajectory snapshot, so a lifecycle view of an idle
trajectory hangs until its next event. The Texture document stream's
`open` is late in the same way.

Fix direction: skip encoding for any request that asks for a stream
(`Accept: text/event-stream`, which EventSource always sends). The
encoder then never wraps a stream response at all.

## Verified on staging (23:28)

- The Texture document stream and the trajectory stream both send
  headers in 0.10 s with `Accept-Encoding: gzip, br, zstd`, uncompressed
  (`text/event-stream`).
- Authenticated JSON comes back gzip. `/autopaper`, the story bundle and
  the hashed assets come back compressed.
- The deployed spec passes 3 of 3.
