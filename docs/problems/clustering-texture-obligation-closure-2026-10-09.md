# Clustering assessment: Texture obligations close only by model choice (2026-10-09)

AGENTS.md "Root Cause Clustering": four Texture problems were documented
today, all found by the Gate 1 Texture acceptance suite on staging. This
assessment comes before the next fix.

## The four

1. `texture-create-occurrence-deferred-never-refires` — an occurrence
   whose activation ends without disposing it is deferred, then consumed
   as terminal on re-fire because the activation's own `decide` turn
   consumed the head. Confirmed on staging with the new log lines.
2. `texture-settled-work-refuses-owner-revise` — the desk settles its own
   work at the end of a turn; the owner's next revise was refused.
   Repaired (4f9331cf: owner input reopens completed Texture work).
3. `texture-cancel-and-research-convergence` §1 — the editor's Cancel
   cancels the document's trajectory, which is terminal; the document can
   never be revised again.
4. Same doc §2 — research work items never settle, so a Texture that
   waits for research waits forever, re-spawning research.

## Common cause

Every durable obligation in the Texture loop closes only when a model
makes an explicit choice, and the defaults either never close or close
for good:

| Obligation | Closes when | Default |
|---|---|---|
| Research work item | research reports with `work_disposition: completed` **and** Texture names that update in its disposition | report packets default to `open` (`agentcore/rlm_reduce.go`); no desk module or prompt mentions `work_disposition`; unnamed packets are consumed with work `open` (`textureowner/texture_turn_runtime.go`). Result: never. |
| Texture work item | the desk passes `work_disposition: completed` on a turn | open; when the desk does settle, nothing reopened it (fixed) |
| Texture occurrence | an apply or decide turn consumes the head | a `decide` consumes it; a re-fire then drops it silently (now logged) |
| Document trajectory | settlement or cancel | owner Cancel makes it terminal; there is no turn-level cancel |

This is operational invariant O1 again (every obligation has one driver
and a recorded terminal fate) at the Texture layer: closure is delegated
to model choices the models are not told about, and the substrate offers
no runtime-derived closure.

## Is there an existing replacement, unwired?

Partly. The SL work gives the observer (obligations surface, actor tape,
consumption log lines) but not the closure rule. Work-item disposition
plumbing exists end to end (`work_disposition` on packets, inbound
dispositions on Texture turns); what is missing is a runtime rule that
uses it. Nothing found that already implements "research work completes
when its report is incorporated".

## Substrate-level options (to decide before more patches)

A. **Runtime-derived research closure.** A research report incorporated
   by a Texture turn completes the producer's work item unless the report
   explicitly says `open` (invert the default for research reports), or
   research's work completes when its run ends after a report. Removes
   the wait-forever loop without asking either model to learn a field.
B. **Cancel is turn-level for documents.** The editor's Cancel stops the
   current Texture turn and its delegated work (cancel work items, fence
   the run) and leaves the document trajectory live. Trajectory cancel
   stays for deleting a document.
C. **Texture contract invariants in the register.** Write the Texture
   invariants Gate 1 already calls for: a live document is always
   revisable by its owner; every delegated piece of work reaches a fate
   without either model's cooperation; no occurrence is consumed without
   a recorded reason.

A and B are product behavior on red surfaces and change what the owner
sees. C is documentation that A and B would enforce.

## Recommendation

Do C first (green), then A (smallest change that removes the T6 failure
class), then B. A and B each get a problem-first commit, a test pinning
the failure modes, and a deployed rerun of the Texture suite. The suite
(T5b, T6) is the acceptance for both.
