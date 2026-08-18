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


## Follow-up diagnosis: the repair prerequisite was not exercised

Source inspection after the failed proof confirms the deployed repair's
compatibility branch is reachable only during replay and the empty-scope legacy
row is supplied by the owner-scoped `ImportResidueSnapshotForOwner` path. The
retained computer was refreshed and replayed directly; no owner-authorized
`computer import-residue-snapshot` receipt was produced after commit `750a145f`.
Therefore the failed read does not yet distinguish an incomplete repair from an
unexercised residue prerequisite.

The first attempted CLI invocation included an unsupported `--idempotency-key`
flag and exited before making an HTTP request; no product state changed. The
route generates its projection-batch idempotency key internally. The next safe
probe is the existing product path, not SQL mutation: append one owner- and
computer-scoped residue snapshot, capture its durable response/receipt, then
rerun replay completeness. The snapshot is current state as of now, not
fabricated history; it does not reclassify unsupported tables or authorize
checkpoint, restore, retry, promotion, qualified consensus, or effects. If the
snapshot itself fails, document that failure before changing source.

**Follow-up heresy delta:** discovered — the source repair acceptance probe
skipped its required owner-authorized residue import; introduced — none;
repaired — none yet.

## Follow-up probe: residue import exercised, mismatch persists

At `2026-08-18T17:05:41Z`, the existing owner-authorized product path was
executed on the same retained computer after correcting the CLI invocation:

```text
go run ./cmd/choir computer import-residue-snapshot \
  --computer computer-03335285269bdba4f94377e56879f9e6

{"computer_id":"computer-03335285269bdba4f94377e56879f9e6","desktops":1,
"sessions":0,"objects":413,"edges":111,"run_memory_entries":0,
"start_intents":1,"operations":1,"texture_mutations":1,
"appended":true}
```

The route generated its projection-batch idempotency key internally; the
unsupported first invocation with `--idempotency-key` had already exited before
HTTP and changed no state. The corrected invocation appended one current-state
residue snapshot (`appended: true`). The response exposes counts but no event
sequence, event digest, or signed append receipt.

The authorized replay read was then rerun against the same computer:

```text
go run ./cmd/choir computer replay-completeness \
  --computer computer-03335285269bdba4f94377e56879f9e6 --timeout 10m

http 500: replay completeness: reconstruct event chain: computer event appender:
replay finalize sequence 3222: computer event projection: op 0: computer event
projection mismatch: Texture mutation disappeared
```

The import prerequisite is therefore exercised and does not close the semantic
mismatch. Replay eligibility remains false; no checkpoint, restore, retry,
promotion, qualified-consensus transition, or effect is authorized. The next
probe is a source/local replay fixture that identifies the exact sequence-3222
mutation scope and ordering; no further live mutation or source repair follows
from this receipt until that fixture is understood.

**Follow-up heresy delta:** discovered — the existing residue import does not
make the legacy Texture mutation available at its earlier canonical sequence,
and its response does not expose a durable event identity; introduced — none;
repaired — none.
