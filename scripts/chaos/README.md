# Chaos drills (d@e only)

Adversarial drills for the test computer d@e
(`computer-59c902c5d46caecc0a180a8557164e72`) on staging Node B. They are the
acceptance drill for `tape.group_commit` (telemetry rides the next
acknowledged write, grouped acknowledged writes, run-memory messages in a
crash-lossy window of at most 10 s;
`docs/design/tape-group-commit-2026-10-31.md` on `lane/tape-group-commit`)
before it widens to the paper. Mutation class: yellow (proof tooling).

```
scripts/chaos/run --list
scripts/chaos/run s1-guest-crash               # dry-run (default): read-only Node B probes, plan, checks
scripts/chaos/run s1-guest-crash --offline     # dry-run with no ssh
scripts/chaos/run s1-guest-crash --execute     # inject the fault
scripts/chaos/selftest.sh                      # the harness's own guards, with fakes
```

Each run writes `scripts/chaos/results/<scenario>-<run id>/artifact.json`
(`choir.chaos_run.v1`): scenario, timestamps, target (computer, vm, pid),
the fault as planned and as injected, the undos run, pre-flight and every
check with expected and actual, writes attempted and acknowledged, and pass.
`results/` is gitignored.

## Contract under drill

- Every acknowledged write survives: file PUTs read back their nonce; Texture
  creates read back, and a replay of their `client_request_id` answers the
  same `doc_id` (no duplicate); a prompt-bar replay of its `command_id`
  answers the same submission.
- After a crash only queued telemetry and run-memory *messages* may be
  missing, and the crash-closed run's `loop.cancelled` event carries the
  "may be missing" note. Open runs end `cancelled` with error
  `interrupted_by_restart`; none is left active or started again.
- A planned restart consumes its marker once (one `restart kind=planned`
  line, no crash boot, no "treating boot as a crash", no drain failure).
- Replay completeness of d@e is `equivalent`.

## Scenarios

| | fault | APIs |
|---|---|---|
| s1-guest-crash | `kill -KILL` of d@e's firecracker pid, re-mapped on Node B in the same script | product API with d@e's key: `/api/files/*`, `/api/texture/lifecycle-documents`, `/api/texture/documents/{id}`, `/api/prompt-bar`, `/api/runs`, `/api/runs/{id}`, `/api/runs/{id}/events`, `/api/compute/status`, `/api/runtime/observability`, `/api/computer-settings`, `/api/computers/{id}/self-development/replay-completeness`; Node B: ownerships.json, /proc, the VM's console.log |
| s2-corpusd-crash | dead-man `systemd-run --on-active` restart, then `systemctl kill -s KILL go-choir-corpusd` | as s1, plus PostgreSQL `computer_event_append_receipts` (gap, distinct commitments, `c1:` compact rows), corpusd `127.0.0.1:8086/internal/computers/events/replay` (expanded receipts, contiguous sequences) and `/health` (`tape.segment_fallbacks`, `tape.reader.segment_error_fallbacks`), d@e's newest tape segment |
| s3-planned-restart | `POST 127.0.0.1:8083/internal/vmctl/deploy-refresh` (the marker-backed refresh; `/internal/vmctl/refresh` is markerless and not used) | as s1; undo `DELETE <guest>/internal/runtime/planned-restart` (no token) when no planned boot came |

## Safety

- Target is d@e only; the paper, the owner's computer (and VM
  `candidate-fleet-e15c...`) and the delegate are refused by id, VM name or
  hex anywhere in a value (exit 3).
- A VM is used only when exactly one ownerships.json row maps d@e to it, no
  other row names it, and exactly one firecracker process runs `--id <vm>`;
  the mapping is checked again on Node B in the script that kills.
- Pre-flight (exit 4) refuses unless the paper is active and its `/health`
  answers 200, host MemAvailable >= 8 GB, `/var/lib` free >= 60 GB, the CI
  gate lock is free and no `node-b-ci-deploy`/`prebuild` runs, and neither
  kill switch is set.
- Kill switch: `~/.choir-chaos-stop` locally (checked every second of every
  wait) or `/run/choir-chaos-stop` on Node B (checked in the fault script).
  The undos run on abort.
- Every fault pushes its undo first; the EXIT/INT/TERM trap runs them.
- The internal-service token goes only to loopback host ports
  (`scripts/lib/choir_internal.sh`), never to a guest. d@e's key is read from
  `~/Dev/go-choir/.env` (`CHOIR_TEST_D_E_API_KEY`) into a variable and given to
  curl by descriptor; never printed or on a command line.
- Node B reads that walk data (psql, segment read) run under `systemd-run`
  with `MemoryMax=512M`.

## Known limits

- d@e is class `interactive`: vmctl defers a planned refresh while runs run
  (only `critical_protected`/`public_platform` get `allow_busy`), so s3's load
  is file PUTs and a deferral is waited out. The paper's busy refresh path
  (`allow_busy`) is not drilled here.
- Cursor, `cell_fate` and compaction run-memory entries are not driven
  directly; they are covered only through replay completeness.
- Boot kind is read from the VM's console log on Node B (SSH-shaped).

## Deferred (owner: happy path first)

4. Guest-host network cut (nft rule scoped to d@e's tap, removed in a trap).
5. Maintenance under memory pressure (growth meter and snapshot beside a
   capped memory hog; the paper's RSS and health unaffected).
6. Hibernate and wake mid-write (layered image drives reattach).
