# Irreversible Effects Were Mistaken for a Human-Approval Boundary

**Status:** documented before repair

**Observed:** 2026-08-13

**Mutation class:** red architecture correction; the immediate receipt is documentation-only

## Problem

Choir's current high-authority documents incorrectly turn reversibility into the boundary of autonomy. They say reversible computer-local effects may auto-promote while irreversible external effects must refuse pending a separate human decision. That was an inference, not an owner-settled product principle.

The settled correction is:

> Reversible and irreversible effects may both occur inside the autonomy window. Effect-specific, predeclared multiagent consensus governs authorization. A human may be a required, optional, or absent participant according to that policy; human approval is not a universal gate. Reversibility changes recovery mechanics and the evidence threshold, not whether the computer may act autonomously.

## Evidence of Drift

Current authoritative contradictions include:

- `docs/choir-doctrine.md`: black mutations require explicit human authority; the active effects envelope says irreversible effects refuse without a separate decision.
- `docs/agent-product-doctrine.md`: the active product path is summarized as reversible-only with irreversible refusal.
- `docs/computer-ontology.md` and `docs/ACTIVE.md`: the active mission is described as a reversible envelope rather than an effect-policy envelope.
- `docs/definitions/choir-supervised-self-development-effects-2026-08-11.md`: acceptance explicitly requires irreversible-boundary refusal and fixed Super+Texture approval.
- `README.md`: the public architecture says owner approval authorizes every accepted change.

The code still reflects the older owner-gated substrate:

- `internal/agentcore/self_development_decision_binding.go` requires an `external-owner:` authority prefix.
- `internal/agentcore/api_self_development.go` emits and retries owner-authority decision events.
- `internal/platform/self_development_modes.go` exposes the transitional `accept_once` control mode.
- `internal/selfdev/operations.go` retains `awaiting_approval` vocabulary.
- `internal/computerversion/promotion_certificate.go` contains a non-runtime `OwnerApproved` observation field.
- `internal/vmctl/route_authority.go` reads historical construction receipts carrying `owner_approval_ref` and `owner_approved` fields.

These code paths are evidence of the pre-consensus implementation, not normative authority for the product design. Effects remain gated while the active Definition replaces them; simply deleting the owner check before a consensus receipt and reducer exist would weaken authority and is forbidden.

## Correct Boundary

The authority policy for an effect class must be selected before participant outputs are visible and must bind:

- the exact effect subject and state heads;
- eligible seats and independence domains;
- quorum, weighting, dissent, abstention, timeout, replacement, and recusal rules;
- admissible verifier and product evidence;
- capabilities, scope, budget, privacy, and blast-radius limits;
- whether a human seat is required, optional, or absent;
- the execution actuator, consequence receipt, and recovery or compensation plan.

For irreversible or high-consequence effects, policy requires stronger ex ante evidence, narrower subject binding, qualified independent seats, no erased dissent, and durable consequence receipts. Restore cannot undo an external send, publication, payment, or shared-ledger mutation; that fact requires prevention and compensation planning, not a categorical human gate.

## Non-Contradictions

The following remain valid when scoped precisely:

- A specific policy may require a human seat.
- Repository operators may require explicit human authorization for destructive maintenance under the coding-agent safety contract.
- Historical receipts may retain `owner_approval` field names so old evidence remains readable.
- The owner remains the constitutional source of policy and can observe, intervene, revise policy, or revoke capabilities.
- A trusted reducer and actuator enforce a consensus decision; model consensus is not itself canonical event authority.

## Repair Obligation

1. Promote the corrected boundary into Choir Doctrine and Agent Product Doctrine.
2. Reconcile the active effects Definition from fixed two-seat reversible auto-promotion plus irreversible refusal to effect-specific multiagent consensus with optional human participation.
3. Correct current product-facing summaries and the Definition supplement; preserve historical documents as historical evidence.
4. Mark hard-coded owner gates and legacy receipt vocabulary as pre-consensus residue until the active Definition replaces them atomically with policy, consensus receipts, and reducer validation.
5. Add documentation and runtime detectors that reject both errors: requiring a human universally, and accepting an effect without its predeclared consensus policy.
