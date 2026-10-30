# Re-engineering studies, 2026-10-12

Six read-only design studies run during the owner's line-by-line review
of the master report
([../master-report-protocol-research-autopaper-2026-10-11.md](../master-report-protocol-research-autopaper-2026-10-11.md)).
Each was written by a research agent at `a401c878` without changing the
repo. Their evidence classes are marked inside (verified from code or
probe, from a doc, or inference).

**Where a study's proposal differs from the owner's decisions, the
decisions win:** [../../re-engineering-decisions-2026-10-12.md](../../re-engineering-decisions-2026-10-12.md).
Known differences: study C's end-of-turn reminder and 60-minute "park and
continue" (owner: no reminder; the 60-minute limit is a prototyping safety
constraint, not design), study D's "move the research cap into the
research job" (owner: no research job; the cap is deleted), and study E's
stage letters (the decision record uses the original B/A/C/D labels).

| Study | Subject |
|---|---|
| [A-async-search-yaegi.md](A-async-search-yaegi.md) | How Go cells run in yaegi; async search with arrivals in REPL state; the parallel-call cross-wire bug |
| [B-work-items-as-records.md](B-work-items-as-records.md) | Every use of work items; work and status as a fold over precommitment records; P2d2 as part of that migration |
| [C-research-desk-redesign.md](../../archive/reeng-studies-2026-10-12/C-research-desk-redesign.md) | Why research stops early; runtime rules shaping it; a simple research desk; hypothesis-shaped Casts |
| [D-texture-reengineering.md](../../archive/reeng-studies-2026-10-12/D-texture-reengineering.md) | Where Texture's turn time goes; 26 prompt/affordance mismatches; a one-verb write interface |
| [E-unified-document-model.md](E-unified-document-model.md) | Captures in Texture's block model; content-addressed ids; transclusion; memoized citation checks; per-version permalinks |
| [G-affordance-audit-runtime-pass.md](../../archive/reeng-studies-2026-10-12/G-affordance-audit-runtime-pass.md) | Prompt vs affordance audit for all desks; 52 runtime-owned decisions; Assign, Jev, path B and self-development |

`probes/` holds study A's yaegi probe tests as `.go.txt` (not compiled).
To rerun one, copy it into `internal/yaegikernel/` as `zz_*_test.go` (or
use `go test -overlay`) and run `go test ./internal/yaegikernel -run TestZZ`;
delete it afterwards.
