# Self-development Slides (slides-next) rounds, luna arm (2026-11-03)

Mutation class of this doc: green. Request: laneC/slides-request.md (the spec's "Slides" section verbatim, package-only
scope, real decks seeded at 11:24Z in Documents/Decks/: rainfall review 2026.pptx, choir in five slides.html, Choir state
and vision.pdf). Engineering: gpt-6-luna at high reasoning (main, after problem 15). Mode standing_consensus.

## Round 1 (doc 2f40db5c-4d3b-5eb0-bf5a-189e4971a7cc, trajectory cf62dc0b-f6b9-5fa9-adaf-a58e9b2ecfb7)

- 11:50:39Z request with the work order spelled out (build is step 2 of 6). 11:50:53Z Texture cast; 11:50:56Z
  assignment-b66d7f66f40a580848c97004. selfdev-0efb6e0642fb3cecaf104815c40344f7 frozen 11:54:06Z, judge failed 11:54:26Z
  ("lacks the requested real-file screenshot iteration and twelve-shot measurements").
- 11:51:27Z and 11:54:28Z owner notes: the choir-shot dependency workaround (problem 18) and main's fallback line.
- 11:55:24Z Texture re-cast (assignment-7638d4e1fc8236926d52690e, 26 min): selfdev-c403e37ce506f6b85c3ec369d931ffc6 frozen
  12:21:41Z with twelve screenshots, build and package checks, layout and contrast measured; judge failed 12:22:26Z only
  because shots 11 and 12 were 1100x750, not 390px. First luna candidate with real screenshots: the real PPTX open with
  titles, body and speaker notes, a presenter view, an authoring panel (scratch luna/screens-s1/).
- Owner's view of those shots: shot 03 (HTML deck, noir) shows the empty desktop; filmstrip shows clipped text, not
  thumbnails; the bar's "Present" label clips; presenter-view counter invisible on the dark bar; the authoring panel takes
  a third of the width while viewing.

## Round 2 (same document)

- 12:23:14Z revision: retake 11 and 12 at 390x844. selfdev-0ef4ade2394472e9e0542874c3cf6354 frozen 12:26:59Z with only the
  two phone shots (390x844, verified); judge failed 12:27:48Z: "the screenshots are not available in this verifier
  workspace or exposed as image refs in its final report". The trajectory settled 12:28:09Z; luna then refreshed.

## Round 3 (doc defecdb9-67b0-520a-82e1-e6dd1b65fc28, trajectory 6f7d54a3-2692-5f78-9f37-b03477952e15)

- 12:29:16Z request (luna/slides-r3.md: one candidate with all twelve shots, image paths listed in the report, the five
  defects above) plus 12:29:25Z the full Slides request as a second revision (failed candidates are not carried).

## Cycle time (UTC, minutes)

| Round | Sent | Sent to Cast | Cast to capsule start | Build (start to Freeze) | Freeze to decision | Outcome |
|---|---|---|---|---|---|---|
| 1a | 11:50:39 | 0.2 | 0.1 | 3.2 | 0.3 | judge failed (no shots) |
| 1b (re-cast) | 11:54:35 | 0.8 | 0.15 | 26.1 | 0.75 | judge failed (phone shot size) |
| 2 | 12:23:14 | about 1 | - | about 3 | 0.8 | judge failed (shots not visible to judge) |
