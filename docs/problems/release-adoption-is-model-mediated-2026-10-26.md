# Release Adoption Is Done by Models, Not by a Job

Date: 2026-10-26. Found by the SR9 investigation (read-only). Problem
first; no fix in this commit. Status: open; fixed by Library step S5
(docs/design/library-package-propagation-2026-10-26.md, section 6.4,
failure mode L7).

Adopting a release is a mechanical act: apply a verified patch and check the
result. Today it is a request that two language-model desks must carry out.

## What happens

`adoptRelease` (internal/agentcore/release_adoption.go:116-219) runs seven
checks that need no model: the record hashes to its id, it is published, the
patch hashes to the record, no secret is added, and the patch applies to the
adopter's tree (`PatchedTree`, :185). Then it stops being
deterministic. It stages the patch (:194) and submits a prompt to the
adopter's desks (`rt.promptSubmitter`, :199), built by `adoptionRequestText`
(:336-351). The prompt asks the root desk to cast to management, management
to cast to engineering, and engineering to run `git apply`, run the check,
build, and Freeze. Every one of those hops is a model choice.

## Evidence

- adopt-1418 (docs/problems/adopt-1418-adopter-desks-finish-without-an-operation-2026-10-15.md):
  the request was accepted (201), the apply and the checks succeeded in the
  capsule (14:31:52, 14:31:57), and the run still ended with no operation.
  An engineering assignment failed in the overlay copy step, and Texture
  closed the request without a second attempt. The adoption was never
  applied, though the patch applied cleanly.
- The prompt itself tells the model what to do in a sentence, "Cast this
  request to management verbatim" (release_adoption.go:349). If a desk
  rewrites or skips it, there is no check that it did the adoption it was
  asked for.

## Why it matters

- The outcome depends on model judgment on a path with a known answer.
- A failure leaves no typed reason. The adoption record shows "requested",
  and the owner cannot tell a conflict from a desk that gave up.
- Every adoption pays the cost of three desk turns and a capsule for what
  `git apply` does in a second.

Fix: Library step S5. A deterministic adoption job: the same checks, then a
three-way tree check, a place into the adopter's source, the gate, and a
typed outcome. The job makes zero model calls. Conflicts go to engineering
as a named case, not as the default path.
