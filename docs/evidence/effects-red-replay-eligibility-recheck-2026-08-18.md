# Effects replay eligibility recheck — 2026-08-18

**Boundary:** verify the already-authorized residue-import result. This is a
read-only staging proof, not whole-computer replay acceptance, checkpoint,
restore, retry, promotion, qualified consensus, or effect authorization.
Effects remain **OFF**.

**Parent:** `docs/definitions/choir-supervised-self-development-effects-2026-08-11.md`

## Probe

Owner-authorized replay completeness was rerun against the retained staging
computer after the recorded residue import:

```text
CHOIR_HOST=https://choir.news CHOIR_TIMEOUT=900s \
  go run ./cmd/choir computer replay-completeness \
  --computer computer-03335285269bdba4f94377e56879f9e6 --timeout 10m
```

The command completed successfully at `2026-08-18T22:59:20.117161546Z` and
returned schema version `3`, not an HTTP or transport error.

Observed replay state:

- computer: `computer-03335285269bdba4f94377e56879f9e6`;
- live and replay sequence: `3342`;
- canonical event head: `0a2362bf34b5e1d06559baea407fb564751390b45f10cfeba57779265e188980`
  on both sides;
- desired/effective event heads and state commitments matched on both sides;
- `run_memory`: `1083` live, `1083` replay, `0` live-only, `0` replay-only,
  `0` differing;
- result: `not_equivalent`;
- differences: `dolt:texture:content_root` and
  `dolt:texture:table:texture_document_aliases`;
- eligibility: `false`, reason
  `behavior-bearing direct-write tables are non-empty without reducers`;
- unsupported table: `texture_document_aliases`;
- probe digest:
  `e4ae0933498e1fa17e44f3ff7ffcee3da43051b31c9e30422982247f36c098c9`.

The returned live version code reference was
`d7f268ddf3fe1339758cf7b9911ff156e9a5f54fb8c188df8ad6123e42e65dae`; this
receipt does not infer a platform deploy identity from that guest-reported
value. The earlier deployed platform identity and residue-import receipt remain
in the parent Definition and run-memory repair receipt.

## Disposition

This recheck confirms the existing alias-authority blocker; it does not create a
new repair authorization. The exact run-memory contract remains narrowly
accepted (`1083/1083` with no row differences), while whole-computer replay is
still ineligible because `texture_document_aliases` is non-empty and has no
reducer-backed authority. `content_root` also differs and is not independently
explained by this probe.

No product state, event chain, VM-local projection, checkpoint, candidate,
bundle, operation, mode, effect, or mail state was changed by the read-only
probe. Do not append another residue snapshot, SQL-empty or SQL-reverse the
alias table, change the manifest class, bind a checkpoint, rematerialize,
restore, retry self-development, self-promote, invoke qualified consensus, or
send mail while eligibility is false.

**Mutation class:** green documentation/evidence only.

**Rollback:** revert this documentation-only receipt; no product-state or
event-chain rollback exists.

**Heresy delta:** discovered — none beyond the already documented alias
authority blocker; introduced — none; repaired — none.

**Conjecture delta:** the current same-computer replay result continues to
support the narrow run-memory repair and leaves whole-computer restore
unproven until owner-ratified alias authority and a reconstructible projection
exist.
