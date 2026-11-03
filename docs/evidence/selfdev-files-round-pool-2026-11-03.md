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

- Full live set (scratch pool/live-final3, 06:04-06:08Z; every scene passed, after the 429 window cleared):
  seven presets at 1100x750 and 390px in aero, noir and salmon with no horizontal scroll. Every preset applies.
  The PDF preview renders its first page. More defects: the list has no column headers (kind, size and date run
  together on one line); PDF and video grid tiles are generic icons; a typed search clips its own field and
  collapses the breadcrumbs to "H... / V..."; at 390px the breadcrumbs clip to "H... / D..." and rows lose size
  and date.
- Round 2 revision written 06:10Z (scratch pool/revise-r2.md): nine defects, package-only scope with "import these
  from app-sdk once it has them; do not edit app-sdk.ts", self-contained. It waits for a build slot.

### Round 2

- 06:20:10Z owner revision 3a707516 (pool/revise-r2.md: nine live defects, package-only scope, self-contained).
  06:20:31Z Texture forwarded; 06:21:05Z assignment-146c3b6c56aa805f6f60f241 opened, 06:21:06Z bound. No
  stack_base_moved and no admission refusal (the trace has none).
- 06:41:38Z Freeze opened selfdev-c4ac7712467862d5b7cd527ed6287034; 06:42:36Z the judge passed it
  (awaiting_approval). 06:43:12Z FP0 approved as the automatic stand-in (receipt
  01a1a025-c6ad-71e0-89be-4da8ac936fe6; base ef47c045...; prior effective head 8bbbb219...); 06:43:14Z applied.
- The engineer's account: kinds, column headers and sorting, breadcrumb priority, no selected preview text,
  search layout with result folders, one menu surface, no desktop sidebar close, PDF and video tiles, the phone
  row's second line; build, check-packages, a package-only diff check and DOM checks passed (its own numbers).
  The operation's patch against the base still shows app-sdk.ts, carried from round 1.
- Texture's document (revision 6a68590f) still said "the candidate is not accepted or materialized ... the owner
  should apply it" after the operation was applied at 06:43:14Z: Texture does not learn of the apply.
- Library publish 06:43Z refused: package_parent_unpublished (Problem 11): round 1 is the package's parent and
  can never publish.
- Spend round 2: $0.60, 93 calls (06:20-06:43Z; $1.84 total).
- Live real-content shots (scratch pool/live-r2, 06:44-06:48Z; all scenes passed, 390px no overflow in three
  presets): column headers with sort, Markdown/CSV/PDF/Plain text kinds, PDF first-page and video poster tiles,
  PDF and video previews, search everywhere with scope and kind controls, no desktop sidebar close, no PLACES.
  Still open: the phone toolbar shows the folder as "D..."; desktop parent crumbs truncate beside a full-width
  search field; the noir context menu still reads as separate rounded buttons; everywhere results in grid show no
  folder. Upload, trash and undo, rename, Move to, `?` and the error state are not yet seen on real files (the
  harness drives browsing only).
- Round 3 revision written 06:50Z (scratch pool/revise-r3.md); waits for a build slot.

### Round 3

- 07:07:50Z refresh to 29d47c215; stack_base_moved (base 2795b8ee6881, op selfdev-c4ac7712). 07:01Z
  standing_consensus armed (generation 6).
- 07:14:30Z owner revision edea4107 (pool/revise-r3.md). 07:14:46Z Texture forwarded; management's Cast was
  refused before an assignment opened: "The current computer is on release 29d47c215 while the existing stack is
  based on 2795b8ee; its patch is not in the booted base and touches frontend/src/lib/app-sdk.ts, so the platform
  requires a root-desk rebase that is unavailable in this run" (07:15:41Z). Problem 10.
- Status at 07:25Z: blocked on the platform. Spend $1.92 total (288 calls).

#### Cycle time (UTC, minutes)

| Round | Sent | Sent to Cast | Cast to capsule start | Build (start to Freeze) | Freeze to judge done | Approve wait | Apply | Outcome |
|---|---|---|---|---|---|---|---|---|
| 1 | 04:37:47 | 0.4 | 0.0 | 47.2 | 2.3 | 0.4 | 0.03 | applied |
| 2 | 06:20:10 | 0.9 | 0.02 | 20.5 | 1.0 | 0.6 | 0.03 | applied |
| 3 | 07:14:30 | refused at admission (stack_base_moved) | - | - | - | - | - | blocked |

Live shots plus next request (apply to next revision sent): round 1, 05:28:08 to 06:20:10 = 52.0 min (first live run 05:40Z; full-preset runs blocked by email sign-in 429s until 06:04Z; then a build-slot wait).
