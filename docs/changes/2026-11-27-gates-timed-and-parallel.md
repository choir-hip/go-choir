---
date: 2026-11-27
title: scripts/gates is timed and runs its checks in parallel, with a stall guard
class: yellow
---
What: `scripts/gates` times every check, prints the ten slowest and the sum, and appends one `gates` line to the land timing log. Every check runs as a background job with stdin from /dev/null, `GATES_JOBS` at a time (default the CPU count, at most 8; `GATES_JOBS=1` is the old serial order), its output in its own file; failures print in file order. `gate_tag serial` holds the two nix-evaluating checks to one at a time, after every other check has finished. The pool is counted with bash's own job table (running and stopped jobs); a killed job fails (missing status file); `GATES_STALL` (default 1800 s) ends a run in which no check finishes, naming the running or stopped checks and killing them, in the pool, the serial group and at `GATES_JOBS=1`. `scripts/check-gatefixture-imports` is one grep instead of three processes per Go file (NUL-safe, a failing find, sort or grep is exit 2). New: `scripts/selftest-gates-runner`, `scripts/selftest-check-gatefixture-imports`. The `run` lines are unchanged. A shortcut that re-ran only the documentation gates after a docs-only move of main was built, reviewed three times, and removed: each round found a path it wrongly treated as docs (a docs-like script, a `.go` under docs/, a top-level JSON that a non-docs gate reads). A move of main re-runs build and the full gates, as before; one `scripts/land.test.sh` case asserts it.

Why: a landing of scripts-only changes spent four to eight minutes before its push, almost all of it in `scripts/gates` (219 s on the first timed run, 59 s of it in one lint). With the gates at about 70 s the saving of the shortcut no longer justified its risk.

Proof: see the commit message of this change for the timed runs; `scripts/selftest-gates-runner` 50 cases (stdin EOF, killed wrapper, stopped job, hung serial check, `GATES_JOBS=1` stall, nix checks tagged serial), `scripts/selftest-check-gatefixture-imports` 18, `scripts/land.test.sh` passed. Not proved: a real landing with the new gates.

Replaced: serial checks in `scripts/gates`; the loop in `check-gatefixture-imports`.

Undo: revert the commits; or `GATES_JOBS=1` for serial gates only.
