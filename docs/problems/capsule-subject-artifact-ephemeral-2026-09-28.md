# Capsule subject artifacts live on tmpfs — `capsule-subject:` refs die with every realization

Date: 2026-09-28
Status: **problem documented; repair pending consensus adjudication.**
Mutation class: **red** (capsule executor artifact persistence, guest image
layout — verification subject authority).
Clustering note: third defect this session in the engineering-verification
chain (after `engineering-verification-chain-dead-2026-09-28.md`). The two
reconcile bugs are symptom-adjacent; this one is substrate — durable refs to
ephemeral bytes. Fixes to all three must land together before the
verification chain is exercised on a real op.

## Observed on staging (deployed `765f35a4`, the reconcile fix)

M11 episode probe `M11_SELFDEV_EPISODE_1790628310490`,
computer `computer-94b04ad5c86c4b687b47c4ea8d779594`, operation
`selfdev-ffbdd629334eeb6bb12aac56407f4c0f`:

Post-deploy boot sweep (guest autoputer `765f35a4`, VM
`vm-64ed4fd1d83b1da6e6b9e9a53123ed2b`, epoch 12773, 22:57:42Z):

```text
textureowner: boot engineering desk reconcile engineering:97fce7b4-…:
  engineering desk reconcile: open verification:
  preflight immutable assignment subject: capsule candidate artifact is unavailable or corrupt
```

The repaired chain ran end-to-end — verification-only reconcile found the
completed implementation, resolved its report via canonical ID, minted the
deterministic verification assignment identity, and reached the opener —
then failed because the candidate artifact is gone.

## Defect: durable ref → ephemeral store

`internal/capsule/executor.go`:

- `PersistGrantedCandidate` (executor.go:1225) copies the frozen capsule
  subject tree into `e.stateDir/subjects/<digest>/workspace/platform` and
  returns `ArtifactRef = "capsule-subject:sha256:<digest>"`. That ref is
  committed into the durable report (`CandidateArtifactRef`) and the
  minted `co_super_subject_candidate` object.
- `subjectArtifactPath` (executor.go:98) resolves the ref back to
  `stateDir/subjects/<digest>` at verification-open time.

But the guest image sets `CHOIR_CAPSULE_STATE_DIR = "/run/choir/capsules"`
(`nix/autoputer-vm.nix:727`) — tmpfs. Every VM hibernate→resume boots a
*fresh disposable realization* (epoch advances, `data.img` reattached at
`/mnt/persistent`, `/run` cleared). The persisted subject tree is deleted
with it. The durable `capsule-subject:` ref outlives the bytes it names —
verification can never re-derive its subject.

Same fate for `stateDir/receipts/{fate,execution,granted}` — the files
`restartRecastReportRef` and `ResolveGrantedExecutionReceipts` need on
post-restart paths. Any restart-recast after a hibernate has the same
failure shape.

## Empirical timeline

- 20:45 op started; impl capsule froze subject → `PersistGrantedCandidate`
  wrote `/run/choir/capsules/subjects/c8ab9176…`.
- 21:00 impl completed; report committed with `candidate_id` +
  `candidate_subject_digest sha256:c8ab9176…`; op → `frozen`.
- 21:19 desk supervision landed revision v1 (appagent head).
- 21:54 VM hibernated (`stopped_by: pressure`); `/run` gone.
- 22:57 resume → fresh realization → boot reconcile reached the opener →
  `capsule candidate artifact is unavailable or corrupt`.

## Design space (under consensus adjudication)

1. Move `subjects/` + `receipts/` onto `/mnt/persistent` (new artifactDir
   knob on `capsule.Executor`; capsule scratch dirs stay on tmpfs).
2. Move all of `CHOIR_CAPSULE_STATE_DIR` to `/mnt/persistent` + an
   init-time scrub of stale live-capsule dirs (executor currently never
   reads stale dirs — `e.capsules` is in-memory).
3. Upload the frozen subject tree to the corpusd CAS / event tape at
   freeze time; `subjectArtifactPath` fetches on miss. Candidate bytes
   become durably replicated platform evidence instead of guest-local
   disk.

Open sub-question: the opener error should probably become a terminal
assignment fate / op `degraded` when the artifact is *provably* gone,
rather than retrying forever on every boot.

## Salvageability of the wedged op

The tree bytes for `sha256:c8ab9176…` were deleted at the first
hibernation. Unless a copy exists on `data.img`, `persist/`, or the
corpusd CAS, the op is permanently unverifiable and needs a terminal
disposition (op→failed with residue note) or a repair op — pending panel
evidence.

Refs: `docs/problems/engineering-verification-chain-dead-2026-09-28.md`
(reconcile fix, commit `765f35a4`), `vm-restart-no-rebind-8085-2026-09-25.md`,
`vmctl-idle-sweep-hibernates-busy-guest-2026-09-28.md` (pressure hibernate
mid-work is compounding this class).
Mission: `docs/definitions/choir-selfdev-gate-2026-09-27.md` (M11).
