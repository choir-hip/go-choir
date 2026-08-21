# Staging Evidence: Capsule Shell Job-Control Recurrence

- Date: 2026-08-21
- Mutation class: Red
- Computer: `computer-03335285269bdba4f94377e56879f9e6`
- Guest realization epoch: 353
- Host deployment observed before the run: `013575843477f8bb8aa4476b2d046b6906f626c8`
- CoSuper: `run:assignment-d7b8e04b-4445-55a8-9531-f76b00aed668`
- Capsule: `capsule-cc2290ba-f910-56ad-bdf8-7aecb88a939e`
- Provider: `chatgpt/gpt-5.6-luna`

## Observation

After the capsule lower-directory permission repair, a fresh assigned CoSuper
reached the capsule through the healthy ChatGPT path. The capsule still could
not author the candidate:

```text
capsule_exec failed during Bash initialization (getpgrp error)
capsule_write_file returned permission denied for /workspace/platform and its subpaths
No operation artifact or frozen bundle was produced.
```

The public CoSuper result also retained `llm_provider=chatgpt` and
`llm_model=gpt-5.6-luna`, so this is not an OAuth or provider-selection failure.

## Source comparison

The prior shell repair `651d86bc` preferred `sh -c` for non-interactive capsule
execution. Commit `7064beb6` later changed the broker back to prefer Bash, while
keeping `SysProcAttr{Setpgid:false}`. The live recurrence proves that disabling
process-group creation does not prevent the Bash initialization path from
calling `getpgrp` inside the capsule PID namespace.

Current source evidence:

- `cmd/capsule-broker/main.go`: Bash is selected before `sh`.
- `docs/evidence/effects-red-capsule-broker-job-control-2026-08-19.md`: the
  `sh -c` path previously ran without the `getpgrp` error.
- Fresh run `d7b8e04b...`: Bash initialization failed again.

This receipt documents the problem before the next broker repair. Effects stay
OFF; no candidate artifact, bundle, proposal, promotion, or live state write
exists.

## Required next diagnosis/repair

Restore the non-interactive `sh -c` preference for capsule execution, retain
Bash only as a fallback when `sh` is unavailable, and verify the actual
networkless capsule path with `capsule_exec` plus a write under
`/workspace/platform`. Do not weaken namespace, broker, Landlock, or capability
isolation. The permission-denied result must be rechecked after the shell path
is repaired; it may be a secondary report from the same failed command batch.

Rollback: revert the broker repair through `origin/main` and CI/deploy. The
pre-A checkpoint `99949fe2` remains untouched.
