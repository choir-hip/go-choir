# FP4 (ARCH-10 engineering admission, ARCH-04 writer gate): deployed proof on a test computer (2026-10-22)

Run by the self-development monitor (lane/fp4-selfdev-monitor), with
`scripts/proofs/fp4-selfdev-admission.sh` (unchanged on main since
d991907e6). It ran on the test computer
`computer-71afbf1461dffabcce8eeebe558d03bd` (vm-c5c801f8…, warmness
`test`). The paper, the owner's computer and the delegate were not
targeted.

## Build identity

- FP4 landed on main from a233b7506 to a84b39bac (rebased round 2; the
  round-2 docs commit was 61447bd9e on lane/fp4-on-g).
- CI on a84b39bac succeeded. Staging has served
  `a84b39bac7ebb116c3ea894326b6640ab5f5a245` since about 09:47.
- vmctl's `deploy refresh caller=deploy` round ran 09:45:19-09:46:39:

  | vm | outcome |
  |---|---|
  | candidate-fleet-e15cb89f | refreshed |
  | vm-48bc0981 | refreshed |
  | vm-4a9afa23 (the delegate's) | deferred |
  | vm-1669 | refreshed |

  The test computer was hibernated (idle since 09:07:02), so the round
  did not list it.
- 09:49:43: owner lifecycle start on the test computer, key
  `fp4-proof-wake-20261022T0950`. Result:
  - 201, receipt `01a16304-63d7-7114-b4cb-20316160d039`;
  - epoch 13234;
  - vmctl: "snapshot resume … not used (launch_config_mismatch)", then a
    cold boot; booted at 09:49:54.
- Guest /health: ready, running_runs 0, commit a84b39bac.

## Steps (`--list`)

| step | class | text |
|---|---|---|
| 1 | ro | ARCH-04: observability objectgraph_writer_gate reports mode observe and counts |
| 2 | ro-after | ARCH-10: Cast A then Cast B; rows: A bound and live, B open and never bound |
| 3 | mut | ARCH-10: a planned refresh of the test computer while A is live |
| 4 | ro-after | ARCH-10: after boot B's row is still open and never bound |
| 5 | ro-after | ARCH-10: B's bound event follows A's terminal_at within the nudge bound |
| 6 | mut | ARCH-10 crash half: operator refresh (crash boot) of a THROWAWAY computer with B queued |
| 7 | ro-after | ARCH-10 crash half: B's row is cancelled interrupted_by_restart and was never bound |

## Step 1: PASS (09:50:02)

```
BASH_ENV= CHOIR_API_KEY=<test key> scripts/proofs/fp4-selfdev-admission.sh \
    --computer computer-71afbf1461dffabcce8eeebe558d03bd --step 1
INFO step=1 writer gate {"mode":"observe","counts":{}}
PASS step=1 objectgraph_writer_gate.mode is observe
PASS step=1 counts are reported
INFO step=- done, no failed checks
```

The counts map is empty. The guest had just cold-booted, with no object
writes yet. The step checks only that `counts` is present. A reading after
the casts of step 2 would show whether the gate counts writes.

## Steps 2-7: not run (they act on the test computer)

The script calls for:

- Steps 2, 4 and 5: two management casts on the test computer (A, then B
  while A is live), then letting A finish. These are model turns. The
  script prints them as MANUAL lines.
- Step 3: a host planned refresh (`POST /internal/vmctl/deploy-refresh`)
  while A is live. The script says a busy computer that is not
  `critical_protected` defers this refresh. 71afbf14 is class `test`, so
  step 3 needs a protected test computer to show the planned boot.
- Step 6: a crash boot (`POST /internal/vmctl/refresh`) of a THROWAWAY
  computer with B queued, which means a fresh mint. Host admission needs
  about 6.8 GiB free; 10.7 GiB was available at 09:49.

My standing instruction runs only read-only steps unless FP0's ledger
allows the acts. The ledger (last entry 08:33) does not yet. Dry runs of
steps 2, 3, 5 and 6 (`--dry-run --step N`) printed the acts above and
nothing failed.
