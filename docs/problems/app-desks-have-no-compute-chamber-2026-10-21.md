# App Desks Have No Compute Chamber: The Only Capsule Is an Engineering Capsule

Date: 2026-10-21. Found while scoping SDC ARCH-09 (station
[choir-launch-fp10-arch-09-2026-10-21.md](../definitions/choir-launch-fp10-arch-09-2026-10-21.md))
from code on `main` at d1c60ad87. Nothing was run on a guest. Status: open;
design in [app-desk-compute-capsule-2026-10-21.md](../design/app-desk-compute-capsule-2026-10-21.md).
Mutation class of any fix: **red** (guest desk confinement, capsule spawn).

## The problem in one sentence

Every capsule a computer can open is an engineering capsule bound to an
engineering assignment, so an app desk that needs a heavy, artifact-producing
job (a slides or PDF export, a video render, an attachment conversion, a
viewport capture) has no chamber of its own: it can only borrow
self-development authority and the computer's one engineering slot, or run
the work in its own desk worker with no containment.

## Evidence (code)

- `capsule.SpawnSpec` (`internal/capsule/types.go`) has only engineering
  inputs: `SourceArtifactRef` and `ExpectedSubjectDigest` (the platform
  source tree), `VerifierBundleDir`, `AdoptionDir`, `CarriedPatchDir`,
  `Egress`, `ClaudeDelegate`. There is no kind field.
- The one production caller of `Spawn` is the engineering assignment saga
  (`internal/agentcore/engineering_assignment_runtime.go:457`), which spawns
  with `WorkingDir: "/workspace/platform"` and the preflighted source
  subject, after reclaiming every superseded assignment capsule (one live
  engineering capsule per computer, ARCH-10).
- `capsule.RoleVerbSets` (`internal/capsule/roles.go`) names three roles,
  management, engineering and research; no app desk holds a capsule verb.
- Visual capture (`internal/visualcapture`, `internal/capsule/visual_evidence*.go`,
  `origin/feat/visual-evidence-2026-10-20`) runs as a capsule job inside an
  engineering capsule and lands in the frozen self-development bundle: the
  only artifact path out of a capsule is the effect bundle.
- The email desk design names a "conversion capsule" for attachments and a
  capsule PDF render for outbound mail
  (`docs/reports/agentic-email-desk-2026-10-14.md` §6.4, §6.5, phase E5,
  "a malformed PDF crashes only its capsule"); nothing implements it.
- FP2 batch 2 (`lane/b3-desk-boundary-2`, not on main) adds `internal/doctool`,
  a sandbox for pdftotext, pdfinfo and pandoc over untrusted bytes. It returns
  text through a pipe; it is not a chamber for jobs that write files, need a
  browser or run for minutes.

## Consequence

Nothing is broken today: no app desk exists in production (ARCH-03 is in
review, off by default) and no app agent renders. The gap blocks Slides
export, E5 attachments and Texture PDF rendering, and invites two wrong
fixes: granting an app desk the engineering role (self-development authority
for a render), or exec-ing a converter in the desk worker (untrusted bytes
parsed next to the desk's handles).

## What a fix must preserve

- Engineering stays the only capsule-bound mutator of the platform (I26);
  a compute capsule mounts no source tree and freezes no effect bundle.
- General process execution stays absent by default (I19): the app desk
  names a package-declared recipe; it never authors argv.
- A crash closes an open compute job as interrupted and never resumes it.
- Memory per doctrine (`memory.high` = requested, `memory.max` = 2x),
  charged to the capsules aggregate.
