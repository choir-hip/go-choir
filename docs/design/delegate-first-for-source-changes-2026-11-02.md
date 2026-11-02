# Delegate first for source changes (2026-11-02)

Mutation class: orange. Rollback: revert the commit.

Owner, 2026-11-02: "With delegate, should strongly go to Claude but some
things don't need it. For example some uses of Eng desk aren't development per
se but just making an api call or two. Note that this isn't something we have
explored much yet but will eventually."

## Gap

On b@c the base-rebase capsule (assignment-f755b8aefccfe857a5c3f1db, capsule
capsule-6abee1d6-46ee-5ff4-b587-16fb34a8b08a) ran in the claude network mode
with "claude delegate ready on /run/capsule/delegate.sock", and the engineering
desk (Synthetic DeepSeek-V4.1-Flash) made 24 capsule_go_eval calls and never
invoked the delegate. The 2026-10-26 Reader round did the same. The prompt said
the capsule "may hand code-writing" to the delegate; "may" was not read as a
default. This is a hypothesis about wording from the cell ledger (no delegate
start in any cell source), not a finding about the model's intent.

## The rule

In the claude-mode block of the engineering overlay
(`internal/runtimeprompts/overlays/rlm_engineering_runtime.yaml`, "Delegate
first:"), next to the delegate's own description:

- Work that edits source (writes or changes files in the platform or package
  tree) is handed to the delegate: the engineer writes the brief, calls
  `choir-delegate`, reviews and verifies the result, and iterates through the
  delegate (resume_session). The engineer edits source itself only for a
  trivial fixup after a delegate pass.
- Work that does not change source (an API call or two, a read, a lookup,
  running a check) stays with the engineer and does not call the delegate.
- The invocation surface (start, wait, what each returns) is stated in the
  same block.

Where the delegate is unavailable the overlay is unchanged.

## What is measured

`EngineeringAssignmentReport.DelegateUse` (`delegate_use` on the report, and on
each report in `choir lifecycle capsule-evidence`): `delegate_available`,
`source_patch` (a freeze committed in the attempt), `delegate_calls` (cells
that start a run), `delegate_succeeded` (distinct run ids whose printed status
was state done, subtype success) and `delegate_model`. It is derived by the
host from the run's cells (`internal/agentcore/engineering_delegate_use.go`),
excluded from the report command digest, and nil on a computer without the
delegate. A patch with zero calls while the delegate was available logs once:
`engineering: source patch without delegate`.

The counts are best effort. choir.Exec runs inside the capsule and leaves no
host-side argv, so the host reads the cell source and the cell's printed
output. A tally is in memory for the attempt (a crash closes the work anyway).

## Deliberately not done

- No hard refusal: a Freeze with no delegate run is not blocked. First learn
  how often the strong default is followed.
- No model choice per Cast yet: which model does which kind of work (including
  non-development uses of the engineering desk) is future work.
- No change for computers without the delegate.
