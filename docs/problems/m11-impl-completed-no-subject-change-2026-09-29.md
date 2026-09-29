# M11 probe run 2026-09-29: implementation completed with zero subject mutations

Date: 2026-09-29
Status: **observed on staging (deployed 7953b840); under adjudication — is the
"completed-without-change -> op failed" path the right verdict, or should an
unchanged implementation be refused/adjudicated differently?**
Mutation class (of any fix): orange/red.

## Observed

`scripts/m11_selfdev_episode_probe.mjs` run 2026-09-29 ~14:32–14:55 UTC against
staging `7953b840` (fresh owner `3522f702-…`, computer
`computer-bccb8b4cdf837b1ce83a1c57d2b66593`, trajectory
`trajectory-cccd30e4e888c99823551cd49a7a0104`, op
`selfdev-a39fc89620bcd577cf739fc431bf9ef5`).

- 14:32 op started; desk opened impl assignment
  `assignment-3ef4362b-4baf-56e9-bdb0-8e602410d97f`, bound
  `capsule-c18e9144-4acd-550f-aa1b-0fa188e1ef49`, run
  `run:assignment-3ef4362b…` executed ~21 min (27+ tool iterations, all
  exit 0).
- 14:54 impl reported `completed`; report commits with
  `observed_subject_digest == binding.subject_digest`
  (`sha256:5e8cc1fb…e071`) — zero mutations recorded. No candidate object
  minted (correctly: `changed` gate requires a digest delta).
- Desk reconcile on the frozen op found no `candidate_id` and failed the op
  (`engineering_desk.go`: "implementation completed without a candidate
  artifact for verification") — this is the edge-3-gated path working as
  designed.
- Probe verdict: `blocked` at `awaiting_approval` — never reached
  approve/apply/falsify/restore.

## Question open

The substrate behaved correctly at every step; the failure is that the
engineering model produced a `completed` report with no filesystem change
under `/workspace/platform`. Two interpretations:

1. **Legit verdict** — the model genuinely made no change (prompt too
   abstract: "minimal reversible self-development evidence change"). Then
   op→`failed` is correct and the fix is probe-side: make the objective
   concrete enough that a compliant impl cannot return empty.
2. **Lost-write wedge** — the model wrote outside the subject tree (e.g.
   `/tmp`, doc surface) or its writes were dropped before freeze; the report
   then *lies* (completed with nothing). Evidence needed: capsule command
   payloads for the 27 iterations (bound commands are digest-only in the
   evidence route; stdout refs exist).

Non-blocking for substrate repair acceptance: the wedge class this mission
closed (missing trigger edges) is distinct — every obligation fired its
recovery authority. The M11 acceptance leg remains open pending a rerun.

## Next

Re-run the probe; if `no-change` repeats deterministically, tighten the probe
prompt (concrete file edit) rather than softening the desk fail verdict —
"completed with no mutation must not pass verification" is a correct gate.
