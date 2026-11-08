# Install review as a desk act (r1, 2026-11-08)

Owner ruling, 2026-11-08: "System1 models outside desks are fine, but not
llms." The install review seats (MA7.5) are the last LLM call the computer
runtime makes outside any desk: `runInstallReview` calls a DeepSeek seat (and a
Devin seat that is not wired) through the tool-loop provider inside the
install request. This design moves that judgment onto management's desk, the
pattern the desk shots design set (docs/design/desk-shots-and-review-2026-11-07.md).

## 1. What stays

The gate's contract does not change. An install whose ceiling widens records
only with a sealed accepting verdict for exactly its release, tree patch,
manifest and behaviour surface (`installreview.Gate`, `VerifySeal`). The
mechanical checks (`installreview.Mechanical`), the risk table
(`PlanFor`: I4 refused for a Library release; mechanical only for a strong
publisher at I0 to I1), the guest-core signer and the boot migration's
deferral (`install_review_deferred`) all stay. The verdict record keeps its
shape; its seat records name the desk that judged.

## 2. What changes

### 2.1 Owner and update installs

**The owed review and the pending install.** When the gate finds no verdict
and the plan needs one, it no longer runs seats. Under `packageInstallMu` it
writes, once per subject digest, a row in a new store table
`package_install_pending`: the subject digest, the release record bytes, the
tree patch (bounded by `pkgrelease.MaxBlobBytes`), the action and the
`installChange` (trigger, forced, listing), the requesting owner and the
time. In the same step it commits a precommitment record
`install-review-owed:<release>:<subject digest>` addressed to management,
binding the subject and the plan. The install answers 202 with reason
`install_review_owed` and the owed record id. The consumers of that answer are
the proxy's install route and the Library's install button, which show
"management is reviewing this release; it installs when management accepts".
A second request for the same subject answers the same 202; a request for
different bytes of the same package opens its own review.

**Owner-visible state.** The installs API lists a pending row as state
`pending_review`, and a closed one as `review_rejected`,
`review_timed_out`, `review_interrupted` or `superseded`, with its reason.

**The clock.** An armed continuation, the `verify_owed` pattern, re-enters
each owed review: a notice to management when committed, at most two more,
and at 45 minutes the obligation closes `review_timed_out`. A timeout seals
nothing: no verdict is written, and a later attempt of the same subject opens
a fresh owed review. Only management's own rejection is sealed.

**Restart.** A crash boot closes every pending row `review_interrupted` and
its owed record, by a named closer beside `closeOpenObligationsAtBoot` (the
leases pattern). A planned restart with its marker re-arms the continuation.

**Completion.** An accepting sealed verdict re-runs the pending row's install
from its stored request, in a detached context
(`context.WithoutCancel`), off management's cell: the verb returns when the
verdict is sealed and the re-run is queued. The re-run's authority is the
sealed verdict plus the owner's original request, recorded on the row; the
route's owner check is not repeated, and the design says so. The re-run takes
`packageInstallMu` itself. If the package's current install changed since the
row was written (a newer release recorded), the row closes `superseded` and
installs nothing. A rejecting verdict closes the row `review_rejected` with
management's reason.

**Forced platform updates.** A forced update that widens a ceiling now waits
on management like any other install, up to the deadline. Platform yanks to a
fallback narrow the ceiling and need no review. Named for the owner as a
change to break-glass semantics.

### 2.2 Package self-development operations

The gate fires at the pre-publish check (`package_operation.go`, the checked
module before publish). There, a needed review does not fail the operation.
The operation pends, as a candidate does while its shots are owed: the owed
record binds the operation's release id (stable for the life of the
operation) and the subject. Each reconcile pass re-enters the check, which is
the clock for this path (notices and the 45 minute close, which fails the
operation "install review not judged in time"). An accepting sealed verdict
lets the next pass continue the operation through publish, install and
`recordPackageApplied`; a rejection fails it with management's reason. No
pending row is written for this path: the operation is the pending act.

### 2.3 The verbs

Management only, by a new privileged grant `assignment.install_review` added
to `soleHolderGrants`, held by management's manifest. The verbs check
`Holds(callingDesk)` as the shot verbs do.

- `choir.InstallPacket(release)` returns the packet the seats read: subject,
  risk, added ceiling entries, flow sentences, mechanical checks, manifest
  JSON and source truncated at 120 KiB. It refuses when no review is owed for
  that release.
- `choir.InstallVerdict(release, pWithinCeiling float64, accept bool,
  findings []map[string]any, reason string)`. `pWithinCeiling` is
  management's answer to the standard question the seats answered ("this
  package stays inside its ceiling"), so the seat record's precommitment is
  management's real prediction, scored like any other. The runtime builds the
  seat record (seat `management`, model from the run's served-model record),
  the verdict with `VerdictRecord` (its children hold the one seat record),
  seals and appends both, and closes the owed record. Findings are plain maps
  (keys `concern`, `evidence`, `blocking`; unknown keys refused by name). A
  blocking finding makes the verdict a rejection. A refusal stamps
  `RecordIntentError`; a replay of the same verdict answers the sealed record;
  a different second verdict is refused.

## 3. Deletions

`installreview/seat.go` (ProviderSeat, Roster, ParseSeatVerdict, the seat
prompt, the Devin and DeepSeek seat constants), `Runtime.reviewSeats`, the
`installReviewSeats` test seam and the seat timeout. `PlanFor` keeps the risk
row but its output is "review needed or not", not a seat list: one desk
verdict at every reviewable level. This is a deliberate reduction at I3 for a
thin publisher (two seats before): the independence the second seat bought
came from a second model family, and management's chain already fails over
across families; the publisher's trust row still gates the act.

## 4. Failure modes

- FM1 management never answers: the continuation (owner path) or the reconcile
  pass (self-development path) closes it at 45 minutes; nothing is sealed.
- FM2 a verdict for different bytes: the seat and verdict bind the subject
  digest; `Gate` ignores a verdict for another subject.
- FM3 a desk other than management calls the verbs: the grant is sole-holder
  and privileged; the verbs check `Holds`.
- FM4 management reviews its own package self-development install: accepted,
  as in the shots design; the swap is revertible and the owner's act is the
  revert.
- FM5 the source exceeds 120 KiB: the packet says truncated.
- FM6 a double verdict: identical replays answer the sealed record, a
  different one is refused.
- FM7 a verdict lands after a newer release of the package installed: the
  re-run closes the row `superseded` and installs nothing.
- FM8 a crash between the sealed verdict and the re-run: the boot closer
  closes the row interrupted; the sealed verdict stays, so the owner's next
  install of the same bytes passes the gate at once.
- FM9 a verdict sealed between a concurrent request's gate read and its owed
  commit: the owed commit re-reads the gate under the mutex and does not open
  a review a verdict already answers.
- FM10 boot migration: unchanged; it defers and never opens a review.
- FM11 the pending table grows: rows are closed by verdict, timeout, crash or
  supersession, and closed rows older than 30 days are pruned.

## 5. Slices and proof

One slice, red, tier 2 (install gate, store schema, new sole-holder grant,
desk protocol change, deletion of the seat path). The trust draft's 7.2
table is amended to one desk verdict at every reviewable level. Security epoch and desk protocol major
bump. Proof on d@e: install a fixture package that widens the ceiling at I2;
see the owed record, management's InstallPacket and InstallVerdict(accept) in
its run, the sealed verdict, and the install recorded without a second owner
act; then a fixture with a planted flow to an outbound sink, management
rejects, the install stays refused with its reason. Rollback: revert the
commit; the gate refuses widened installs with no seat (no install review
available) until the seats return.

## 6. Review record

- r0, Devin, SEND BACK, five blockers, all adopted in r1: the pending install
  is a durable row with the full request (2.1); the self-development path
  pends on the operation, not a pending row (2.2); the clock is an armed
  continuation or the reconcile pass (2.1, 2.2); the verdict carries
  management's prediction so the seat record's precommitment is real (2.3);
  a timeout seals nothing (2.1). Non-blocking notes adopted: FM7 restated
  (the re-run's authority is the sealed verdict and the recorded request),
  the re-run detached from the cell, forced updates named, the grant made
  sole-holder, supersession on a newer install, owner-visible states, the 202
  contract's consumers named, the 7.2 table amended.
