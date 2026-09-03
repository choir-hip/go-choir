# Problem Receipt: Producer-Report Settlement API Unreachable Through Proxy Computer Binding

- Date: 2026-09-03
- Mutation class of this receipt: green (documentation); the repair it directs is red (proxy routing + owner-authority surface)
- Status: documented before any code fix (problem-documentation-first)
- Discovered by: deployed acceptance for Definition 1 criterion 4 on staging `2bf93be7`

## The Problem

The new store-layer settlement surface added for Definition 1 —
`GET /api/computers/<id>/lifecycle/producer-reports` and
`POST /api/computers/<id>/lifecycle/settle-producer-reports` — is not
reachable through the product path. The proxy's `/api/computers/` routers
either treat `lifecycle/<action>` as a fixed vocabulary (status/start/stop/
restart/refresh/recover, routed to corpusd control) or forward generic
computer paths to the guest WITHOUT the `X-Authenticated-Computer` header.
The guest's `handleSelfDevelopmentRoute` requires that exact binding
(`authenticated computer binding required`), so every product-path settlement
attempt fails with 403.

Observed: `curl https://choir.news/api/computers/<id>/lifecycle/producer-reports`
with an owner-wide API key →
`{"error":"authenticated computer binding required"}` (guest handler,
`internal/agentcore/api_self_development.go:105`).

## Root Cause

The settlement endpoints were added only at the guest runtime layer
(`internal/agentcore/api_producer_reports.go`) and the CLI, without adding
the matching proxy computer-binding route. The proxy is the only component
that can mint the `X-Authenticated-Computer` binding from API-key auth.

## Repair Direction

1. Add proxy routing for the two paths that (a) resolves the computer target
   through the existing ownership authority (`requireAPIKeyComputerTarget`
   for API keys), (b) forwards to the guest with `X-Authenticated-User` and
   `X-Authenticated-Computer` set, and (c) gates API-key access by scope
   (read: `computer:self_development:read`; settle: `computer:lifecycle` —
   settlement tombstones are lifecycle/CAS store writes under owner
   authority, never Texture revisions and never Super consumption).
2. Keep the guest-side handlers unchanged; they already enforce the binding.
3. Land via the full Landing Loop (commit, CI, staging deploy, deployed
   acceptance retry of criterion 4).

## Evidence

- Deploy receipt: `/var/lib/go-choir/deploy-receipt.json` target_commit
  `2bf93be7b9eb6436dbdc2a8c14b43b9f853eec22` (CI run 33711967877, deploy job green).
- 403 reproduction: 2026-09-03 ~04:20Z against `choir.news`.
