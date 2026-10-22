---
definition_version: 4

# Station FP10, ARCH-09 (P2, red): a compute capsule kind for app desks.
# Spine: docs/definitions/choir-launch-first-pass-metamission-2026-10-21.md
# (FP10). Item: docs/security-density-architecture-checklist-2026-10-21.md
# ARCH-09. Problem: docs/problems/app-desks-have-no-compute-chamber-2026-10-21.md.
# Design: docs/design/app-desk-compute-capsule-2026-10-21.md.
readiness: built

review:
  reviewer: none yet; FP0's bunch panel reviews the frozen candidate
  frozen_ref: none
  verdict: none
  evidence_ref: none

start:
  captured_at: 2026-10-21T23:30:00Z
  source:
    canonical_ref: main d1c60ad87 (lane branch lane/fp10-arch09-app-capsule starts here)
    deploy_identity: none of this station is deployed
  observed:
    - Only engineering capsules exist; capsule.SpawnSpec has engineering inputs only and one caller (the engineering assignment saga).
    - Visual capture runs as a job in an engineering capsule and leaves only through the frozen effect bundle.
    - App desks (ARCH-03, lane/fp4-arch03-app-desk) and FP2's capsule memory policy and doctool sandbox (lane/b3-desk-boundary-2) are in review, not on main; both edit internal/capsule, internal/deskmanifest and internal/yaegikernel.
    - FP8 (lane/fp8-image-jailer) edits vmctl, vmmanager and the guest image.

finish:
  deliver: >-
    The design of the compute capsule kind (ontology, authority, lifecycle,
    resources, containment, failure modes, slices and their lane order), and
    its first slice S0: the pure contract a later spawn path calls, in a new
    package that touches no file another lane edits. Nothing calls it.
  artifact: >-
    docs/design/app-desk-compute-capsule-2026-10-21.md; internal/computecapsule
    (Validate with typed refusals, Argv rendering from a package recipe,
    RefuseEngineeringInputs, CheckOutputs and the output manifest digest).
  acceptance:
    - action: GOMAXPROCS=2 GOFLAGS=-p=2 go test ./internal/computecapsule
      proves: design §6 failure modes C1 to C12 at the pure layer
      evidence_class: local test
  rollback: git revert of the lane's commits; nothing imports internal/computecapsule.
  landing:
    required: true
    environment: staging
    required_receipts: [pushed_commit, ci, deploy, environment_identity]

boundaries:
  mutation_class: red item; S0 is a new pure package with no caller (no runtime behavior change); green for docs
  must_preserve:
    - Engineering stays the only capsule-bound mutator of the platform (I26); a compute capsule mounts no source and freezes no effect bundle.
    - General process execution stays absent by default (I19); argv comes from a package recipe, never from the model.
    - A crash closes an open compute job as interrupted; no boot-time resumer.
    - No file edited that lane/fp4-arch03-app-desk, lane/b3-desk-boundary-2 or lane/fp8-image-jailer edits.
  excluded:
    - SpawnSpec kind, executor and Freeze refusals (S1, after FP2 lands); compute request store and runtime (S2); desk verb and manifest recipes (S3, after ARCH-03 lands); the first recipe and its guest-image tool (S4, after FP8).
  protected_surfaces: [guest desk confinement (design only in S0)]

now:
  status: checkpoint_incomplete
  slice: >-
    hand-off to FP0. Built: 60bbb0f8d (station file, problem record),
    867f183a9 (design), 2fb7637db (S0: internal/computecapsule, tests
    first). Local evidence: GOMAXPROCS=2 GOFLAGS=-p=2 go test
    ./internal/computecapsule passes (60 subtests); go vet clean.
  source_ref: d1c60ad87
  deploy_identity: none
  candidate:
    id: fp10-arch09-s0
    ref: 2fb7637db
    lands_after: nothing (a new package; no file shared with any lane)
    deployed_proof: >-
      none beyond CI: nothing imports internal/computecapsule, so a deploy
      changes no runtime behavior; CI's go test of the package is the receipt
  residuals:
    - S1 (SpawnSpec.Kind, executor and Freeze refusals, Landlock roots for /in and /out) waits for FP2 batch 2 to land; it edits internal/capsule.
    - S2 (compute request store and runtime, boot closure as interrupted, pool cap, CHOIR_COMPUTE_CAPSULES off by default) follows S1.
    - S3 (recipes in the app manifest, the choir.Compute verb) waits for ARCH-03 to land.
    - S4 (the first recipe's tool in the guest image, deployed proof) waits for FP8.
    - MaxMemoryMiB (2048) is a stand-in until S3 reads the requesting desk's app floor; the desk-to-package binding is checked in S3 (appdesk.PackageOrigin), not in S0.
    - Failure modes C13-C19 are untested until their slices.
  open_owner_questions: >-
    design §8 defaults stand unless ruled: compute capsules are interrupted
    by an always-busy computer's planned refresh; the first recipe is a PDF
    render of a Texture revision; one live compute capsule per computer
---

# FP10 ARCH-09: a compute capsule kind for app desks

The item is large and crosses three lanes in review, so this station writes
the design and builds only the contract package nobody else touches.
