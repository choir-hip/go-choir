# Prompt-bar demo: "make a Minesweeper game" never reaches engineering on a fresh computer (2026-10-10)

Owner direction (16:40Z): demonstrate self-development from the prompt bar —
"make a Minesweeper game" — with live Texture supervision and screenshots of
every stage; then an arXiv replication that research must find unprompted.
Harness: `scripts/demo_selfdev_prompt_bar.mjs` on a fresh test computer
(`scripts/qa_test_computer.mjs`, account `demo-computer-2026-10-10@example.com`,
computer-e9c7dcfed4a1289fd2572256f0de3972, propose_only armed through the CLI).

## Timeline (V0 → V1)

- V0 16:51:10.47Z owner prompt. The conductor run completed at 16:51:11.07
  (0.6 s) and opened a Texture document; the window showed "Writing first
  draft…". (An earlier "8 seconds" was the harness's own screenshot wait.)
- 16:51–16:55 the Texture run generated a complete single-file Minesweeper
  (about 14.8 KB of HTML and JavaScript) inside its first cell.
- 16:55:24 that cell staged an `ApplyTexture` with the document and a
  control `open_persistent_super` (execution_request: write the HTML to a
  file and verify it headlessly). The reduce failed:
  `apply atomic Texture lifecycle turn: Texture controls[0] load exact target: record not found`.
- 16:55:31–16:56:13 four more cells, including `println("ping ok")` and
  `choir.Help(...)`, failed with the same error.
- 16:56:44 a cell applied the document without the control and committed.
  V1 = 16:57:03, 5 min 53 s after V0. No management, engineering or
  research run started; no capsule; no candidate.

## Failures

F1. On a fresh computer Texture cannot open the persistent management desk.
`textureTurnControls` (internal/textureowner/texture_turn_runtime.go) looks
up `management:<owner>` and fails when the agent record does not exist. The
HTTP opener (`issueManagementOpenControl`) calls
`EnsurePersistentManagementAgent` first; the Texture turn path never does.
Owner computers registered the agent long ago, so it never showed there.
Every new computer is affected: Texture can never delegate.

F2. (hypothesis) After a cell's reduce fails, later cells that stage nothing
still fail with the same `tray-1` error. `Tray.Drain` says undrained trays
die with their cell, so something re-stages or replays the failed intent
(cell results report `"reuse":"preserve"`). Needs a kernel-level test.

F3. V1 took 5 min 53 s because Texture's first turn wrote the whole program
before acknowledging the request. Owner (17:05Z): V1 is Texture showing it
understands the request; V2 is the first substantive worker result; later
revisions revise the ideas (hypotheses, what is built, what is next, what
the evidence says), not a log of actions or a code dump. Building a program
is engineering's work, transcluded into the document as a diff.

F4. The approval surface is missing: no GUI control and no route lists a
computer's self-development operations; a freeze-opened candidate can only
be found by deriving its id from the assignment id.

## Fix order

F1 now (connect the existing ensure). Then rerun the demo and read the
trace again (F2, F3). Conductor latency is 0.6 s; the owner's Jev-classifier
conductor experiment is separate.
