# Effects product-path forward — start/decision reach the guest

**Boundary:** execute (route map 10 prep, red). Not live proof. Not a live send. Effects remain OFF.
**Parent:** `docs/definitions/choir-supervised-self-development-effects-2026-08-11.md`
**Parent HEAD:** `387fbfaa` (docs pin of staging 3141b90b after orange rehearsal)
**Mutation class:** red (proxy product path)

## What landed

The guest already had start/decision/receipts APIs, including `qualified_consensus` consume. The proxy catch-all returned `self-development effects are disabled` before those routes could reach the computer.

This slice forwards the product paths that live proof needs:

- `POST /api/computers/{id}/self-development/operations` (`computer:self_development:propose`)
- `GET .../operations/{op}` and `.../receipts` (`:read`)
- `POST .../operations/{op}/decision` (`:approve`)
- `GET .../kernel-capabilities` (`:read`)
- `POST .../rollbacks` (`:rollback`)

Guest fail-closed behavior is unchanged: mode `off` still refuses proposal (`current signed mode does not authorize proposal`). Genesis remains 409 at the proxy. Owner gates `external-owner:` / `accept_once` / `awaiting_approval` were not deleted. Outbox `Armed` remains false.

## What did not change

- No mail was sent.
- Mode on the retained computer was not set.
- Restore was not rematerialized.
- Staging was 3141b90b at dispatch time; this commit must deploy before live proof.

## Ceremony

- **Conjecture delta:** Live proof uses the existing guest self-development API through the public product path; the proxy is a scoped forwarder, not a second effects authority.
- **Protected surfaces:** genesis stays disabled; mode `off` still refuses at the guest; trusted-outbox remains unarmed.
- **Admissible evidence:** `go test ./internal/proxy -run TestSelfDevelopment|TestPublicGenesis|TestBootstrapChainDoesNotTouchSelfDevGenesis`.
- **Rollback:** revert this commit. Effects-OFF at guest and genesis 409 remain.
- **Heresy delta:** `repaired` the public catch-all that made orange rehearsal unreachable on staging. `preserved` genesis refusal and mode-off proposal refusal.

## Residual

Red/live proof remains unpaid until this commit is deployed and a staging trajectory actually starts, promotes, restores, and receipts the acceptance email. Do not treat proxy forwarding as deployed acceptance.

## Next

Wait for staging deploy of this commit. Then red rehearsal on the retained computer without a live send. Do not rematerialize. Do not delete owner gates.
