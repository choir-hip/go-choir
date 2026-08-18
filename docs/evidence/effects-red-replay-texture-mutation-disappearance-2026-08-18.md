# Effects replay Texture mutation non-resolution — 2026-08-18

**Boundary:** diagnose. Not implement. Not checkpoint. Not restore. Not promote.
No live send.

**Parent:** `docs/definitions/choir-supervised-self-development-effects-2026-08-11.md`

## Problem documented first

The replay-only legacy Texture repair in source commit
`750a145fcdd9663517b776fde1ce9c83e5bd7f5b` landed through CI and Node B. Staging
`/health` reports proxy/guest commit `750a145f`. The retained same computer was
owner-refreshed from realization epoch 310 to 311 with receipt
`01a015c7-3e75-77aa-8924-a2e3c06136c6`.

The fresh owner-authorized replay-completeness proof still fails at the same
semantic transition:

```text
go run ./cmd/choir computer replay-completeness \
  --computer computer-03335285269bdba4f94377e56879f9e6 --timeout 10m

http 500: replay completeness: reconstruct event chain: computer event appender:
replay finalize sequence 3222: computer event projection: op 0: computer event
projection mismatch: Texture mutation disappeared
```

The command ran for 137.04 seconds. Replay eligibility remains false. No
candidate, bundle, checkpoint, restore, self-development retry, promotion,
qualified-consensus transition, or effect is authorized.

## Belief update

The earlier diagnosis that empty-`computer_id` legacy Texture residue omission
was sufficient is not established. The source repair now imports that legacy
scope and provides a replay-only compatibility finalizer, but the deployed
reconstruction still cannot materialize the Texture mutation at sequence 3222.
The repair shape is therefore incomplete, or the sequence-3222 mutation follows
a different event/payload/projection path than the repaired legacy scope. This
receipt does not choose between those hypotheses.

The next probe is source- and event-specific: inspect the canonical event and
payload at sequence 3222, identify the exact Texture mutation identity and
expected predecessor, and reproduce the disappearance in a focused replay
fixture before proposing another repair. Do not patch or retry from this
receipt alone.

**Heresy delta:** discovered — the deployed legacy-scope replay repair does not
close the sequence-3222 Texture mutation disappearance; introduced — none;
repaired — none by this receipt.

**Rollback:** docs-only problem receipt; no product-state rollback. If the
source repair must be reverted later, use `revert 750a145fcdd9663517b776fde1ce9c83e5bd7f5b` only through a separately authorized
problem-first repair decision.
