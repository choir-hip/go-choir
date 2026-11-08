# Install review as a desk act (r0, 2026-11-08)

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

**The owed review.** When the gate finds no verdict and the plan needs one,
it no longer runs seats. It commits once per subject a record
`install-review-owed:<release>:<subject digest>` addressed to management (a
precommitment record, kind `install_review_owed`, binding the subject, the
plan and the requesting act), and refuses the install with 202-shaped
`install_review_owed`: "management is reviewing this release; it installs
when management accepts it". The refusal carries the owed record's id.

**The pending install.** The requesting act (the owner's install request, or
a package self-development operation's install) is recorded on the owed
record: owner, computer, release, track. Only one pending install per subject.

**The verbs** (management only, granted by a new privileged grant
`assignment.install_review`, held by management's manifest; the verbs find
their desk through the registry as the shot verbs do):

- `choir.InstallPacket(release)` returns the packet the seats read today:
  subject, risk, added ceiling entries, the flow sentences, the mechanical
  checks, the manifest JSON and the source, truncated at 120 KiB. It refuses
  when no review is owed for that release.
- `choir.InstallVerdict(release, accept bool, blocking bool, findings
  []map[string]any, reason string)` records management's answer. The
  runtime builds the seat record (seat `management`, model = the model that
  served the calling turn, taken from the run's served-model record, not
  from the cell), builds the verdict with `VerdictRecord`, seals it and
  appends both. Findings are plain maps (keys `concern`, `evidence`,
  `blocking`; unknown keys refused by name), the lesson of the ShotFinding
  defect. A refusal stamps `RecordIntentError`; a replay of the same verdict
  for the same subject answers the sealed record.

**The install completes from the verdict.** An accepting sealed verdict
re-runs the pending install for exactly that subject, under
`packageInstallMu`, as the requesting actor. A rejecting verdict closes the
pending install refused with management's reason, visible on the install
record. The owner can install again; a new subject (new bytes) opens a new
review, the same subject reads the sealed rejection.

**Reminders and deadline.** As shots owed: a notice to management when the
record is committed, at most two more, and the owed review closes rejected
"not reviewed in time" 45 minutes after it was committed. The pending install
closes with it.

**Restart.** The owed record is an obligation. A crash boot closes it and its
pending install with "interrupted by a restart" (CLAUDE.md restart rule); a
planned restart with its marker leaves it open. The owner installs again.

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

- FM1 management never answers: the deadline closes it, the install stays
  refused, the reason is visible.
- FM2 a verdict for different bytes: the seat and verdict bind the subject
  digest; `Gate` ignores a verdict for another subject.
- FM3 a desk other than management calls the verbs: the grant is privileged
  and one-holder; the verbs refuse without it.
- FM4 management accepts its own package self-development install: the same
  desk froze and now reviews. Accepted as in the shots design: the swap is
  revertible, and the owner's act is the revert; named in the residuals.
- FM5 the source exceeds 120 KiB: the packet says truncated, as today.
- FM6 double verdict: the second is refused unless identical (replay).
- FM7 the pending install's actor lost authority meanwhile (an owner key
  revoked): the re-run is the owner's original act, checked again at re-run
  by the same install path; a refusal there closes the pending install.
- FM8 boot migration: unchanged, it defers and never opens an owed review.

## 5. Slices and proof

One slice, red, tier 2 (install gate, new privileged grant, desk protocol
change, deletion of the seat path). Security epoch and desk protocol major
bump. Proof on d@e: install a fixture package that widens the ceiling at I2;
see the owed record, management's InstallPacket and InstallVerdict(accept) in
its run, the sealed verdict, and the install recorded without a second owner
act; then a fixture with a planted flow to an outbound sink, management
rejects, the install stays refused with its reason. Rollback: revert the
commit; the gate refuses widened installs with no seat (no install review
available) until the seats return.
