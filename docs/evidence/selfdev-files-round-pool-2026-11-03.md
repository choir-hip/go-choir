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
- 04:38:10Z engineering assignment-273d9f8926186553594d57ce. Engineering ran on DeepSeek-V4.1-Flash-1017 (synthetic)
  through pool:default (142 calls, $1.23); 14 gpt-6-luna calls ($0.006) were probably Texture and management.
- 05:25:24Z Freeze opened selfdev-1d0527916b182fb513039514188a874d (frozen 05:25:26Z); 12 screenshots with the
  requested names and presets; 84.5% listing share; contrast 5.38-8.37; no 390px overflow.
- 05:27:44Z judge passed (awaiting_approval); 05:28:06Z FP0 approved (stand-in for automatic apply; receipt
  01a19fe1-0611-7581-97a1-e06cd5e44359; prior effective head 3f4858c3...); 05:28:08Z applied (computer rebooted on apply).
- Live real-content screenshots (scratch pool/live-r1, aero-chrome-heart, inside the desktop window): list of
  Documents; grid with real thumbnails; PDF first page in the preview with "PDF - 46 pages", size, modified, path;
  context menu (Open, Open with, Rename, Move to, Duplicate, Download, Copy link, Star, Info, Delete); mp4 preview;
  search. Defects seen: md and csv show kind "Text"; breadcrumbs truncate to "Ho... / Docum... / Rep..." when the
  preview is open although there is room; preview title and metadata render text-selected after open; the search
  field's white box overflows its pill; context-menu rows read as separate pill buttons; a close (x) button on the
  sidebar at desktop width; uppercase "PLACES" label.
- Library publish 05:35Z refused: package_change_not_confined (the candidate also edits app-sdk.ts; 4 re-exports).
  packages.publish was off on all four arms; FP0 turned it on at 05:35Z.
- Spend round 1: $1.24, 156 calls (04:38-05:30Z).

#### Cycle time (UTC, minutes)

| Round | Sent | Sent to Cast | Cast to capsule start | Build (start to Freeze) | Freeze to judge done | Approve wait | Apply | Outcome |
|---|---|---|---|---|---|---|---|---|
| 1 | 04:37:47 | 0.4 | 0.0 | 47.2 | 2.3 | 0.4 | 0.03 | applied |
