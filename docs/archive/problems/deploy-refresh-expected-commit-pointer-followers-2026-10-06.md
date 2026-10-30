# Deploy refresh verifier rejects pointer-following guests on a new deploy

## Symptom

Deploy run 37414528508 (2026-10-06, commit `1b1d9b7e`) failed at
"Refreshing active interactive computers onto deployed VM boot contract":

```
vm-48bc09812f3fce06d715f6c7f8e2db9a: expecting served commit
12d3adc0d70d110a2bea60abc786ce2908419dcb after refresh
Refreshed vm-48bc09812f3fce06d715f6c7f8e2db9a
Timed out waiting for vm-48bc09812f3fce06d715f6c7f8e2db9a to serve
autoputer runtime 12d3adc0d70d110a2bea60abc786ce2908419dcb;
observed 1b1d9b7e1a1095de99d88671c2902f56dcfb56c0
```

The guest correctly served the *new* deployed commit; the verifier waited
for the *old* one and failed the run.

## Root cause

`.github/workflows/ci.yml` (~line 1401) derives the post-refresh identity
expectation from the guest's **pre-refresh** served commit:

```bash
refresh_expected_commit=$(curl ... ${refresh_computer_url}/health | jq -r '.build.commit')
if [ -z "$refresh_expected_commit" ]; then
  refresh_expected_commit="$autoputer_runtime_commit"
fi
```

The comment above it reasons about layered guests: a refreshed layered
computer keeps serving its retained app-layer release, so the expectation
is the retained release, not the host pointer commit. That is correct for
guests on a retained release.

The missing case: a guest that follows the host autoputer pointer (no
retained release, or its retained release is being advanced by this same
deploy). After the refresh swaps the guest onto the deployed boot
contract, such a guest serves `DEPLOY_COMMIT` — the deploy's own intent.
Waiting for the pre-refresh commit then always times out, and the run
dies after services and guest are already healthy on the new commit.

## Why this matters

This is a deploy-gate false negative, not a product failure: the deploy
succeeded (services healthy, guest serving `1b1d9b7e`) while CI reported
failure and recorded a deploy-failure evidence file. Repeated hits make
"incomplete deployment" receipts meaningless — the opposite of what the
evidence file is for.

## Fix direction

`wait_for_autoputer_commit` should accept either valid post-refresh
identity:

- the guest's retained release commit (`refresh_expected_commit`, when
  the guest is layered and keeps it), or
- the deployed runtime commit (`autoputer_runtime_commit`, when the guest
  follows the host pointer).

Both are deterministic; the check stays fail-closed on anything else.

Mutation class: orange (CI deploy verification only; no product behavior
change). Rollback: git revert.
