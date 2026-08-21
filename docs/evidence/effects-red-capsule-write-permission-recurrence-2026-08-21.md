# Staging Evidence: Capsule Write Permission Recurrence

- Date: 2026-08-21
- Mutation class: Red
- Computer: `computer-03335285269bdba4f94377e56879f9e6`
- Guest realization epoch: 352
- Host deployment observed before the run: `d6af08212f91599d6fd583eded7dcaba61a0b506`
- Provider path: ChatGPT OAuth, `chatgpt/gpt-5.6-luna`

## Observation

After the persistent-Super recovery and model-policy repairs were deployed and
the retained computer was refreshed, a fresh assigned CoSuper reached the
networkless capsule using the ChatGPT provider:

- Super: `d4ebe99d-0c4b-4b17-b116-5ed4d159c710`
- CoSuper: `run:assignment-340f441d-3d2f-586f-985d-edca04c00d5c`
- Assignment: `assignment-340f441d-3d2f-586f-985d-edca04c00d5c`
- Capsule: `capsule-0d4a99de-ecf2-584c-9dda-229403156e9c`
- Operation: `selfdev-8dcdd2c5e7841addb24b0c7991f09a5c`
- CoSuper state: `completed`
- CoSuper metadata: `llm_provider=chatgpt`, `llm_model=gpt-5.6-luna`

The run result was:

```text
Blocked: the capsule rejected artifact creation in `/workspace/platform` with `permission denied`. No candidate artifact was produced, so no freeze or self-development proposal could be completed.
```

This is live proof that the model-routing/OAuth path is repaired. It is also a
new capsule execution blocker: candidate authorship still cannot be accepted.

## Prior substrate repairs and recurrence boundary

Earlier receipts documented and repaired lower-directory modes, toolchain
mounting, broker shell lookup, and upperdir ownership:

- `effects-red-capsule-toolchain-and-overlay-write-2026-08-18.md`
- `effects-red-capsule-overlay-ownership-and-shell-job-control-2026-08-20.md`

The recurrence is narrower than the original DeepSeek/permission failures:
ChatGPT inference succeeds, the assigned capsule is created, but artifact
creation under `/workspace/platform` is still denied. No source repair is
claimed by this receipt. Effects remain OFF; no candidate bundle, freeze,
proposal, promotion, or live state write exists.

## Required next diagnosis

Determine the exact filesystem object and UID/mode boundary rejecting artifact
creation inside the refreshed capsule. Use capsule-bound product diagnostics
and existing execution receipts; do not SQL-empty the retained computer, use
SSH as an operational substitute, or weaken capsule isolation. Document the
specific failing object and authority before the repair commit.

Rollback remains the normal git revert/deploy path. The pre-A checkpoint
`99949fe2` remains untouched.
