---
definition_version: 4

# Station FP4, ARCH-03 red slice: an app package carries its app desk.
# Spine: docs/definitions/choir-launch-first-pass-metamission-2026-10-21.md
# (FP4). Design: docs/design/app-package-carries-app-desk-2026-10-21.md.
# Base: FP2's desk manifest (ARCH-02, lane/b3-desk-boundary-b1fix 27d39057d).
readiness: drafted

review:
  reviewer: none yet; FP0's bunch panel reviews the frozen candidate
  frozen_ref: none
  verdict: none
  evidence_ref: none

start:
  captured_at: 2026-10-21T22:00:00Z
  source:
    canonical_ref: 27d39057d (FP2 batch 1 fix; lane branch lane/fp4-arch03-app-desk starts here)
    deploy_identity: none of FP2 or this slice is deployed
  observed:
    - deskmanifest.NewAppDesk and RegisterApp exist (FP2); nothing calls RegisterApp.
    - RegisterApp mutates only the calling process's registry; a desk worker is re-executed and never learns an app desk (docs/problems/app-desk-registration-does-not-cross-worker-execve-2026-10-21.md). Fails closed.
    - The package manifest (internal/apppackage) is on the paused S7-0 branch, not on main; there are no install events on any tape yet.
    - ARCH-04's per-kind writer (objectgraph KindRegistration.Writer, desk:<name>) is on lane/b10-selfdev, not on this base.

finish:
  deliver: >-
    The pure half of install and the worker half of activation: a desk
    manifest an app package embeds is validated with typed refusals, bound by
    digest, folded from install and disable events into one derived desk set
    (desks, kinds with their one writer, retired kinds), and an app desk the
    daemon has derived reaches its own desk worker, re-validated there. Off by
    default: CHOIR_APP_DESKS unset activates nothing.
  artifact: >-
    internal/appdesk (Validate, Digest, Derive, Activate, PackageOrigin,
    WriterFor); deskmanifest.RetireApp and an idempotent re-register; the
    worker carry (CHOIR_DESK_SESSION_APP_DESK) and its worker-side
    re-validation.
  acceptance:
    - action: GOMAXPROCS=2 GOFLAGS=-p=2 go test ./internal/appdesk ./internal/deskmanifest
      proves: design §5 failure modes 1 to 6 and 9 at the pure layer
      evidence_class: local test
    - action: GOMAXPROCS=2 GOFLAGS=-p=2 go test ./internal/yaegikernel -run 'TestAppDesk'
      proves: the carry renders only for an app role, and a worker refuses a carried row that is missing, mismatched or wider than the app set
      evidence_class: local test
  rollback: git revert of the lane's commits; with CHOIR_APP_DESKS unset no code path registers an app desk.
  landing:
    required: true
    environment: staging
    required_receipts: [pushed_commit, ci, deploy, environment_identity]

boundaries:
  mutation_class: red (desk worker spawn environment, run authority); green for docs
  must_preserve:
    - FP2's: no desk's authority widens; an unknown desk fails closed; the daemon decides a frame's authority from the worker it spawned.
    - The desk set is derived from install events, never written to an editable table (design §2.3).
    - Off by default; no boot-time resumer; no scan of og_objects.
  excluded:
    - Reading install events from the tape and the S7-0 package manifest (paused branch); the write-gate overlay (needs ARCH-04 landed); egress grants (NET-10); owner approval UI.
  protected_surfaces: [desk worker spawn, run authority at the daemon]

now:
  status: checkpoint_incomplete
  slice: >-
    hand-off to FP0. Built: 71fd9ffa5 (docs, problem record) and 6d1cffed5
    (internal/appdesk; deskmanifest RetireApp, AppSpec, Equal; the worker
    carry). Local evidence: go test ./internal/appdesk ./internal/deskmanifest
    ./internal/yaegikernel pass (GOMAXPROCS=2); go vet clean.
  source_ref: 27d39057d
  deploy_identity: none
  candidate:
    id: fp4-arch03-app-desk
    lands_after: FP2 (this branch is based on lane/b3-desk-boundary-b1fix)
    deployed_proof: >-
      none needed beyond FP2's: with CHOIR_APP_DESKS unset no app desk is
      registered and sessionEnv is unchanged, so a deployed system desk turn
      (FP2 step 5, texture-acceptance-deployed) is the proof of no regression
  residuals:
    - Activate has no caller: install events need S7-0's package manifest and event kind on main.
    - The write-gate overlay of derived kinds (writer desk:<pkg>, retired kinds unwritable) needs ARCH-04 landed.
    - App desks have empty MessageTargets and Addresses, so Cast and Report reach no desk yet; routing needs the addressing field.
    - Upgrade and reinstall after disable are refused (design §5.10 re-approval path not built); failure modes 7, 8, 10 untested.
    - Egress is refused outright until NET-10's owner grants.
  open_owner_questions: design §7 defaults stand (embed, choir.app.<pkg>.<kind> namespace, Texture stays unpackaged); "podcast" is an S7 reserved name, so the L3-03 podcast app agent needs S7-0's legacy rule
---

# FP4 ARCH-03: an app package carries its app desk

The smallest red slice of the design that does not guess at an unlanded
schema: the pure fold the install path will call, and the worker carry the
reviewer found missing. Wiring the fold to the tape waits for S7-0.
