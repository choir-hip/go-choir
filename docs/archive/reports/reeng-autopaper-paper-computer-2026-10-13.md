# The designated Autopaper paper computer

Designated 2026-10-13 by the orchestrator under the metamission's
authority (owner ruling: start the paper computer's schedule and publish
in public; Cut 2 autopaper_live).

- Computer: computer-1355b83bced1613eb048ee28d66688a8
- Owner: e5962310-8767-4624-87e5-a9b93a4e1db1 (a dedicated owner created
  for the paper; credential file ~/.config/choir-qa/test-computers/autopaper-paper.json,
  0600, never printed)
- Minted with scripts/qa_test_computer.mjs --label autopaper-paper on build
  761d94c5; warmness class primary (protected from the idle sweep).
- Not a tracer test computer: no tracer acts on it, and it is not stopped
  after a run. It is the one computer whose stories the public front list
  shows (corpusd CORPUSD_AUTOPAPER_OWNERS in nix/node-b.nix).

Operating rules (Cut 2 boundaries and CLAUDE.md "Restarts End Work"):
the owner act that starts Autopaper is POST /api/autopaper/sources/schedules
on this computer; a crash restart closes the schedules and every open round
and nothing resumes; Autopaper restarts only by that owner act again.
The front list is served from Store B and keeps working while this
computer is stopped.
