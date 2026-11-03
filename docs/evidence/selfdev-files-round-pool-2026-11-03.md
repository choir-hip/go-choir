# Self-development Files (reinvented) round, pool arm (2026-11-03)

Mutation class of this doc: green (evidence). Station:
[choir-selfdev-apps-feature-complete-2026-11-03](../definitions/choir-selfdev-apps-feature-complete-2026-11-03.md).
Spec: [apps-feature-complete-spec-2026-11-03](../design/apps-feature-complete-spec-2026-11-03.md)
("Rules every app shares", "Files (reinvented)", "UI design") and the
[design bar](../design/app-design-bar-2026-10-26.md). Orchestrator: lane C (FP0 on the owner's delegation).

- Computer: computer-4d38420cf85ac490a36baa0ef5d7a67a (staging https://choir.news)
- Engineering: pool:default; texture and management pool:default
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
- 04:27:37Z `source.projection` set true (empty stack, before any round); projected tree holds the spec.
  04:37:39Z `choir computer refresh`.

### Round 1

- 04:37:47Z request (v2 text) on Texture doc b1ad3952-5ab4-54e0-80ba-94d312f88385, trajectory
  5dc6a6ff-41d6-5ee6-9025-151c57bb5eff.
