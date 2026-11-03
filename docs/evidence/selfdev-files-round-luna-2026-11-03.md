# Self-development Files (reinvented) round, luna arm (2026-11-03)

Mutation class of this doc: green (evidence). Station:
[choir-selfdev-apps-feature-complete-2026-11-03](../definitions/choir-selfdev-apps-feature-complete-2026-11-03.md).
Spec: [apps-feature-complete-spec-2026-11-03](../design/apps-feature-complete-spec-2026-11-03.md)
("Rules every app shares", "Files (reinvented)", "UI design") and the
[design bar](../design/app-design-bar-2026-10-26.md). Orchestrator: lane C (FP0 on the owner's delegation).

- Computer: computer-2bf3cab1e3ebf3eb24a515c4603cf954 (staging https://choir.news)
- Engineering: chatgpt gpt-6-luna; texture and management pool:default
- Approver: FP0 (test arm only); publish to the Library, no git promotion

## Setup (all four arms, 02:50-02:53Z)

- Seeded a real-content corpus through `choir files put` (16 files: two
  report PDFs and Rainfall.pdf from the repo, README and two Autopaper design
  docs, a txt and a csv, three generated images and two repo screenshots, an
  m4a narration and an mp3, a 10 s mp4). No personal data.
- Set the computer setting `files.object` to true (it was false on every arm;
  PATCH moves answered 405). After it, PATCH move and move-back answered 200.
  Same on all four arms, so the arms stay identical except the engineering model.
- Spend baseline (/api/costs): 0 calls at 02:55Z.

## Rounds
- Not started yet (at most two capsule builds at once until S4). 04:07Z `self-dev mode` set to propose_only
  (generation 1), same as the other arms.
- 04:27:36Z `source.projection` set true (empty stack, before any round); the source tree was projected at
  04:27:59Z and now holds the spec. 04:33:50Z `choir computer refresh`.

### Round 1

- 04:34:13Z request (v2 text: round-1 lessons and the spec quoted) on Texture doc ac26aaa3-bab6-5c13-866d-bd3b62c1d1ce,
  trajectory 00338a51-6c74-5c46-9095-732f0e665867. 04:34:36Z management Cast; 04:34:38Z engineering
  assignment-73be7adfd13e4ae7c5ba37fc (gpt-6-luna, reasoning low).
- 04:34:54Z after 8 model iterations (all reads) the engineer called Complete("blocked") without writing or building:
  it cited an "available assignment execution window" and an unestablished environment. No ledger field shows a time
  limit; it never probed node, npm or Chromium (hypothesis: a model behaviour, not a platform limit). Texture asked
  for a narrower slice. Spend: about 0.33M input tokens, 18 calls.
- 04:40:07Z owner revision c58e3c64: no window and no narrower slice; build steps in order; freeze partial work
  with named gaps rather than none.
- 04:40:40Z FP0 cancelled the trajectory to keep at most two capsule builds at once (claude r3 and pool r1 were
  building). Luna's round is re-sent when a slot frees.
