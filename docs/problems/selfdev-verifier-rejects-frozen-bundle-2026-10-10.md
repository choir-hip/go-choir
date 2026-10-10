# Self-development: the independent verifier rejects the frozen bundle (2026-10-10)

Found by the fifth Gate 2 reality rerun (M11 probe) on staging, build
5b851eed, disposable `computer-69454e0a…` (VM `vm-b4faf10b…`), operation
`selfdev-d2cf4897…`. Written 00:01Z. Problem first; cause not yet known.

## What got further

- 23:15:52 operation started. 23:39:09 the implementation assignment
  completed: one evidence file under `docs/evidence/` (a new directory
  in the capsule; the 2c68cc18 freeze fix held), release tree staged,
  **freeze succeeded first try** (bundle `3ebbc78d…`, state `frozen`).
- The report reached Texture (disposition `delivered`; the 028446a5
  binding fix held). 23:47:51 Texture took a supervision turn
  (`wait_for_evidence`).
- The host-side reconcile opened the verification assignment
  (`assignment-4a803a1d…`).
- 23:59:46 the verifier recorded `decision: fail` through the
  verification tool (`tools_capsule.go`); the operation went
  `frozen → failed`, "independent verifier rejected frozen bundle".
  The probe then stopped (5 of 16 legs).

## What the trace shows so far

The verifier wrote a program into the verification capsule's source
tree (`/workspace/platform/cmd/verifydigest/main.go`) and ran it with
`go run` to check the digest. Its reason for `fail` is not in the first
200 run events (the listing cap) and the run is still active at
00:00Z, so its report has not arrived. Hypotheses, not findings:

- H1: the verifier could not reproduce the bundle digest (its own
  digest program disagrees with the freeze's content digest algorithm).
- H2: the verifier judged the change itself (an evidence file plus a
  release tree) as not matching the request.
- H3: something in the verification capsule (network closed, toolchain,
  Go module cache) made its checks fail and it recorded fail rather than
  a blocker.

Next observation: the verifier's report summary, then the decision
cell's arguments.

## Cause (00:02Z; the verifier's report, confirmed in code)

The verifier reported a fail-closed blocker at 00:01:42Z:
`choir.InspectBundle` refuses the mounted draft ("invalid frozen bundle
draft"). `internal/yaegikernel/inspect_bundle.go` decodes the draft into
`bundleDraftMirror` with `DisallowUnknownFields`; the mirror predates
S2-f and lacks `source_patch_sha256` and `source_patch_base_commit`,
which `transaction.CapsuleEffectBundle` (and so every draft with a source
patch) carries. The mirror would also recompute the wrong content digest,
since the declared digest covers those fields. Every payload check the
verifier could run by hand passed (binding, content digest, runtime file
hashes, source patch hash, subject tree digest).

H1 confirmed in a narrower form: the in-cell inspection, not the
verifier's own program, cannot read the draft. H2 and H3 refuted. The
verifier's `fail` was the correct fail-closed call. Every self-development
change that touches platform sources has been unverifiable since S2-f
landed; the September 29 pass predates it.

Fix: add the two fields to the mirror with the same tags
(`omitempty`), so decode and digest match; pin with a test that inspects
a draft carrying both fields. Mutation class orange (verifier input
parsing; the verification decision stays with the verifier).
Rollback: git revert.
