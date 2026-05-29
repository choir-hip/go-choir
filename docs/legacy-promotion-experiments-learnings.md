# Legacy Patchset Promotion Experiments: Learnings

This note preserves the useful lessons from the old candidate-world patchset
promotion experiments while removing their implementation and proof artifacts
from the active codebase.

## Why This Was Pruned

The old path was:

```text
worker/candidate worktree -> export_patchset -> promotion_candidates queue
-> /api/promotions owner review -> internal patchset import/verify/promote
```

That path was valuable as a bootstrap experiment, but it is no longer the
current product path. It trained the repo to treat patch files, queue rows, and
host-git integration branches as the success object. The current direction is
typed source movement through `AppChangePackage`, recipient `AppAdoption`,
verifier evidence, owner review, promotion, rollback, and run acceptance.

Keeping both paths active creates destructive gravity:

- tests can pass against an obsolete success route;
- docs imply `/api/promotions` or `export_patchset` are still valid proof;
- agents spend attention reconciling two incompatible promotion objects;
- Campaign Compiler work inherits dated mechanics instead of compiling toward
  durable multi-computer product objects.

## What The Experiments Proved

The failed path still produced real invariants worth carrying forward:

- Foreground/canonical state must stay stable while background candidates
  mutate.
- Candidate work must identify owner, source run, candidate run or computer,
  base ref, candidate ref, evidence, verifier contracts, and rollback refs.
- Promotion must be serialized. A candidate cannot overwrite foreground-tail
  work that arrived while the candidate was running.
- A worker must not verify itself. Verification is evidence-scoped and may use
  tools, but it needs independence from the producer path.
- Owner review is necessary but not sufficient. Review authorizes a verified
  transition; it does not replace verification.
- Evidence must be durable refs, not only prose. Trace, VText, source lineage,
  package manifests, verifier results, screenshots, videos, and build ids need
  typed attachment points.
- Browser/media proof must observe the product path. Internal/test endpoints
  and manually seeded success records create false confidence.
- Duplicate delegation and duplicate candidate production are common in live
  multi-agent runs. Objective fingerprints and artifact digests help, but
  semantic dedupe belongs in the campaign/work-order layer.
- Package mobility beats alternate-account login choreography. The recipient
  computer should rebuild, verify, adopt, promote, roll back, and roll forward
  from typed packages, not from copied sessions or host branches.
- A platform deploy is not proof of user-computer promotion.

## What Replaces It

For current and future Choir-in-Choir work, use:

- `AppChangePackage` as the source package object;
- `AppAdoption` as the recipient-computer candidate and transition object;
- source lineage records for active refs, candidate refs, artifact digests, and
  rollback state;
- product APIs such as `/api/app-change-packages/*`, `/api/computers/*/adoptions`,
  `/api/adoptions/*`, `/api/trace/*`, `/api/vtext/*`, and
  `/api/run-acceptances/*`;
- Campaign Compiler work orders and evidence packets to coordinate many
  computers, runtimes, agents, users, and time horizons.

The replacement object is not a patchset queue. It is a compiled campaign that
can publish typed packages, route them to candidate computers, verify them,
request owner attention, promote or roll back, and feed learnings back into the
next campaign cycle.

## Negative Rules

- Do not restore `/api/promotions` as a compatibility success path.
- Do not use `export_patchset` as acceptance evidence for Choir-in-Choir.
- Do not add new tests whose only proof is a queued patchset candidate.
- Do not treat old dated promotion proof docs as active specifications.
- Do not bypass AppChangePackage/adoption with host-git branch promotion unless
  the mission is explicitly a platform config or deploy operation.

## Campaign Compiler Implication

Campaign Compiler should internalize these lessons as compile-time constraints:

- compile user intent into work orders with explicit authority, candidate
  mutation scope, verifier contracts, evidence packet requirements, rollback
  refs, and owner-attention gates;
- invoke cognitive transforms as action-changing review operations, not as
  decorative prose;
- widen from one simple candidate mutation to divergent computers only after the
  product path proves package publication, recipient adoption, promotion,
  rollback, and reentry;
- preserve healthy redundancy in canonical docs while pruning experiment
  artifacts once their learnings have been consolidated.

