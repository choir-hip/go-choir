# Staging answers 502 "failed to resolve publication" for a story on the live front page (2026-11-19)

Problem. `GET https://choir.news/api/platform/publications/resolve?route=<route>` answered HTTP 502 `failed to resolve publication` for a story that the front page (`/api/platform/publications/front`)
lists. The iPhone reader shows "Could not open this page. HTTP 502: failed to resolve publication" for it; the signed-in tour (`proof-real.sh`, read drive) failed three runs in a row (13:25Z, 13:30Z,
13:36Z) at the step that scrolls the reader of the second story.

Evidence. `real` run logs `integ3-real11`, `-real12`: `[reader-message] "Could not open this page. HTTP 502: failed to resolve publication"` in `drive-read.txt`; a plain `curl` of the same route
returned 502 while the first story's route returned 200. The app made one request and showed the server's answer; no app code path differs between the two stories.

Belief. A server-side defect in the publication resolver for some routes (a story whose underlying document cannot be resolved), not an app defect. Not diagnosed on the server; no server code was read.

Remaining error. Which routes fail and why is unknown; whether the paper should list a story the resolver cannot open is a server question. The app's reader says so in words (it does not spin), which is
the right behaviour for the phone. The tour works around it: `proof-real.sh` curls each top-band story's resolve route and taps only those that answer 200, notes when none do, and fails loudly when the
survey shows no `paper-story-` card at all (the paper page not drawing). The Android shared reader (`reader.ts`) answers the same case with "Could not load this story".

## Cause and fix (2026-11-22)

Status: fix built in lane `proxy-resolve-size`, reviewed once, not landed and not deployed. The cause is inferred, not proven, until the size log line is seen on staging.

Cause (read from code and the proxy journal, not from the story's bytes). `HandlePlatformPublicationResolve` read corpusd's answer through `io.LimitReader(resp.Body, 1<<20)` and
then decoded it. A bundle over 1 MiB was cut mid-document, `json.Unmarshal` failed with `decode corpusd response: unexpected end of JSON input` (8 lines in the proxy journal, 07:22Z to
07:25Z on 2026-11-22), and the handler wrote the 502. 59 of the first 60 front stories resolve with bodies of 45 to 361 KB; the failing story's bundle size was not measured. The same
1 MiB limit sat at three more sites (the proposal post, the owned-publication read, the publish post).

What makes a bundle large (the review's reading of the code; an earlier draft of this record said revisions). `nav` is four integers, so revision count does not grow the bundle. Size is
the current version's text carried several times: `artifact.content`, `body_doc`, `render_model`, `source_entities`, `payload.text`, `manifest`, `resolved_blocks`, `retrieval.spans`,
`transclusions` and `checks`. The failing story (62 revisions, 42 versions) is probably long or heavily transcluded, not old. The size log line will show it.

Fix (orange). Resolve relays corpusd's 2xx body to the client without buffering (same type both sides, no re-shaping; JSON content type unchanged), so no size bound can truncate it. It
uses a relay client whose total timeout is the server's write budget (`server.WriteTimeoutFromEnv`, 120 s), because the stream's time includes the reader's drain and the shared 30 s corpusd
client would cut a slow phone's download. A non-2xx answer, and a 2xx with an empty body, are decided before anything is written (404 stays 404; the others 502, logged). A relay that
breaks after the header is logged and aborts the connection (a chunked 200 with a short body would look complete), so the client sees a transport error. A resolve of 1 MiB or more logs
`resolve bundle size: route= bytes=`. The sites that must decode, and `getPlatformJSON` (front, entity, growth, related, expand), read through `readPlatformBody`: a bound of 64 MiB
(about 180 times the largest bundle seen, 64 times the old limit; it guards the proxy's memory, not a story's size), refused at once when Content-Length is known to exceed it, and past
it an explicit `corpusd response too large: N bytes upstream, bound B` error, logged. Tests: `internal/proxy/platform_resolve_size_test.go`.

Remaining error. Duplication of the current version inside one bundle, and a story that grows long, make resolve large for the phone; it should later be trimmed or paged (the payload once,
history on request), more so as `docs/design/first-design-2026-11-22.md` makes stories longer-lived and more recursive. Until then a very large bundle is slow, not refused. The same 1 MiB class
remains at `platform_expand.go:392` (`paperCall`, the paper service), not touched here. Not yet proven on staging.
