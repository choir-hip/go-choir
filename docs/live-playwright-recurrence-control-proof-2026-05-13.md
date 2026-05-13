# Live Playwright Recurrence Control Proof

Date: 2026-05-13
Mission: `docs/mission-choir-grand-deformation-v0.md`

## Slice

Reran the visible desktop dogfood after adding inbox idempotency, worker lease reuse, objective fingerprints, patchset digests, and continuation objective fingerprints:

```text
desktop prompt bar -> conductor -> VText -> super -> vmctl worker lease -> worker runtime -> export_patchset -> promotion queue -> VText integration
```

## Harness Repair

The first rerun failed before reaching Choir because `localhost:4173` was not serving. `start-services.sh` then failed at gateway compile time until it inherited the Homebrew ICU flags already required by Go tests.

`start-services.sh` now:

- defaults `CGO_CFLAGS`, `CGO_CXXFLAGS`, and `CGO_LDFLAGS` to `/opt/homebrew/opt/icu4c@78` when present;
- supports `CHOIR_SERVICES_FOREGROUND=1`, which keeps the service process tree alive under agent harnesses and cleans it up on Ctrl-C.

Foreground mode was verified by starting the stack, confirming listeners on `4173`, `8084`, and `8085`, sending Ctrl-C, and confirming no listeners remained on `4173`, `8081`, `8082`, `8083`, `8084`, or `8085`.

## Passing Dogfood

Command run:

```text
GO_CHOIR_RUN_BACKGROUND_WORKER_DEMO=1 npx playwright test vtext-background-worker-demo.spec.js --workers=1 --timeout=420000
```

Result: passed in 2.2 minutes.

## Recurrence Evidence

`vmctl.log` showed one worker allocation and one reuse for the repeated request:

```text
assigned worker VM vm-36cf52bfde86f6c12a2e1cd2db0a3cc1 ... worker_id=worker-8e2c97c37ef18b1b
reused worker VM vm-36cf52bfde86f6c12a2e1cd2db0a3cc1 ... worker_id=worker-8e2c97c37ef18b1b
```

The latest live source run queued one promotion candidate with both identity signals recorded:

```text
source_loop_id=b4f45042-dd3c-4aca-afea-4b6e260e2517
candidate_count=1
distinct_objective_fingerprints=1
distinct_patchset_sha256=1
```

This is the first live product-path proof that duplicate worker requests converge to one active worker lease and one promotion candidate for the source run.

## Boundary

The run still used host-process local worktree fallback, not Firecracker isolation. The recurrence controls are deterministic, not semantic: they normalize obvious objective text variance and patchset identity, but they do not yet understand broader paraphrases.

## Next Deformation

Make context pressure observable before provider limits, then run a longer Choir-in-Choir loop where continuation selection uses the recorded objective fingerprints instead of stopping after one pass.
