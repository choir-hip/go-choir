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
