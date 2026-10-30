# Capsule freeze fails when the change adds a source directory (2026-10-09)

Found by the fourth Gate 2 reality rerun (M11 probe) on staging, build
733bec77, disposable computer `computer-beb952a2` (VM `vm-480d590c…`). The
session worker repair held: the capsule ran cells for 116 iterations. Then
the engineering run ended with a blocker report (22:28:18Z) instead of a
frozen candidate. Problem first; no fix in this commit. Mutation class of
the fix: orange (capsule release staging; the self-development approval
and apply path stay untouched). Rollback: git revert.

## Evidence

Engineering's blocker (trajectory update, `producer_report`, kind
`blocker`), in its own words: the freeze fails because the capsule overlay
holds directory changes under `/workspace/platform` (`docs` and
`docs/evidence`) created when it wrote the evidence file;
`StageGrantedRelease` → `emitSourcePatch` iterates the raw diff, does not
skip directory entries, and `os.ReadFile` on the lower directory fails
with "is a directory". The release tree itself was staged and verified.

Code reading confirms it (`internal/capsule/executor.go`
`emitSourcePatch`): every change under `/workspace/platform/` is read as a
file from the lower layer and the merged layer. The manifest diff
(`manifest.go` `diffManifests`) includes directories (`Mode` carries
`os.ModeDir`). A directory that already exists in the lower layer and
gained a child is a `modified` directory entry; `os.ReadFile` on it returns
EISDIR, which is not `ErrNotExist`, so the whole freeze fails. A unified
diff cannot express a directory anyway; the files inside carry it.

The September 29 pass wrote its evidence into an existing directory with
no directory mtime change, so it never hit this.

## Second finding, same run

Engineering's blocker never reached the owner's supervision surface. The
Texture occurrence for it was consumed without a turn: `invalid Texture
actor occurrence: Texture producer control binding authority mismatch`
(22:28:18Z). The report's `control_binding_id` names management's control
to engineering, which is not an update on the document's trajectory, so
the Texture validator (`texture_controller.go`, binding check over
`snapshot.Updates`) rejects it. On a self-development document Texture
supervises engineering that management admitted; the check assumes Texture
issued the control. The report stays `pending` on the trajectory. Named
residual `texture-supervision-rejects-management-bound-reports`; not
fixed here (red: Texture occurrence authority).

## Fix direction

`emitSourcePatch` skips directory entries (`change.Mode.IsDir()`), and
treats a base path that is a directory as absent. Files inside a new
directory are added with `/dev/null` as their base, as they are today.
Failure modes to pin: a new directory with a new file; an existing
directory that gains a file; a deleted directory; a directory replaced by
a file.

## The rejected report, traced (22:42Z, code reading)

The M11 engineering run is a **delegated cast**
(`engineering_assignment_runtime.go`): its binding's `ParentControlID` is
the cast's commitment record id (`CommitmentControlID`), a ledger record,
not a lifecycle update. The report packet carries that id as
`control_binding_id` (`store/engineering_assignments.go`
`buildEngineeringReturnPacket`). The Texture validator's binding check
(August, fd83ce64) looks for a lifecycle **control update** with that id
on the trajectory; the trajectory has none (its only update is the
report), so every delegated-cast report to Texture fails with "producer
control binding authority mismatch". The September 29 pass predates the
delegated-cast reports reaching Texture. Fix direction (red, Texture
occurrence authority): the binding check accepts a commitment-record cast
that names the producer agent and work item, as the lifecycle control
check does. Residual `texture-supervision-rejects-management-bound-reports`
stays open; it does not block approval (the operation state comes from
the assignment, not from Texture), but the owner never sees the report.

## Fix plan for the rejected report (22:43Z; red, ceremony)

- Conjecture delta: an Engineering report's control binding is proven by
  its stored assignment, not by a lifecycle control update. The newer
  authority check (`ValidateLifecycleProducerReportAuthority`) already
  loads the assignment and pins trajectory, agent, work item and run; it
  does not yet pin `ParentControlID` to the report's
  `control_binding_id`.
- Change: that check also requires `ParentControlID ==
  control_binding_id` for Engineering; the Texture binding scan then
  accepts zero matching lifecycle control updates for an Engineering
  producer (assignment-bound) and still rejects two or more. Research and
  Management keep the update-scan rule unchanged.
- Failure modes pinned: a delegated-cast report with its own control id
  and no lifecycle control update must be accepted; an Engineering report
  whose binding differs from its assignment's must be rejected; a
  Research report without a matching control update must still be
  rejected; two matching control updates must still be rejected.
- Protected surface: Texture occurrence authority (red). Admissible
  evidence: the M11 rerun on staging shows a Texture turn for the
  engineering report instead of "consumed without a turn". Rollback: git
  revert. Heresy delta: discovered the stale binding rule; repaired on
  staging proof only.
