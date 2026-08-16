# Effects supervision wiring — joinable identities without payload schema change

**Boundary:** execute (route map 8, green). Not rehearsal. Not a live send. Effects remain OFF.
**Parent:** `docs/definitions/choir-supervised-self-development-effects-2026-08-11.md`
**Parent HEAD:** `3141b90b` (trusted-outbox dispatch, Armed=false)
**Mutation class:** green (Texture collation/metadata + tests; packet payload schema unchanged)

## What landed

The upward packet already existed. This slice populates the layers the Definition already named: revision metadata and typed citations. No observation subsystem was added. The `CoagentSourcePacketPayload` schema was not changed.

Concretely:

1. Joinable identities ride existing `packet.sources` typed URIs: `operation:<id>`, `capsule_bundle:<digest>`, `receipt:<id>`, `event_head:<digest>`. Source kind stays an existing vocabulary member (`capsule_bundle`). A new source kind `operation` is still refused.
2. Texture collation mints typed source entities from those URIs. Runtime copies them into durable revision metadata keys `self_development_operation_id`, `self_development_bundle_digest`, `self_development_receipt_id`, and `self_development_event_head`. Available-source prompt context is still stripped from persisted revision metadata. Prose is not scraped.
3. Texture's production registry still omits generic `update_coagent`. `AllowCoAgentTools` remains false. Texture keeps `patch_texture` / `rewrite_texture` / `record_texture_decision`. Super holds `update_coagent` and `report_to_texture`. Assigned CoSuper holds `update_coagent`. This is the CTS-safe answer to the registry-gap unknown: do not register the generic resolver on Texture.

## What did not change

- `external-owner:` and `accept_once` remain.
- `awaiting_approval` remains.
- Mode `off` remains the default. Outbox `Armed` remains false.
- No mail was sent. Restore was not rematerialized.
- OwnerRecovery remains inadmissible.
- Deployed staging is still `4ac90583`. This receipt is source confirmation on current main, not deployed acceptance.

## Ceremony

- **Conjecture delta:** Supervision can join a Texture revision to an exact operation, bundle, receipt, and head without putting those identifiers in owner-facing prose and without widening the packet schema.
- **Protected surfaces:** decision binding, event chain, materializer, checkpoint/route, updater, vmctl, auth/session, gateway, deploy, and Texture canonical write authority were not mutated. Collation and durable metadata keys are the named supervision surface.
- **Admissible evidence:** `go test ./internal/textureowner ./internal/agentcore -run TestSelfDevelopmentJoin|TestTextureProductionRegistryOmitsGenericUpdateCoagent|TestUpdateCoagentAcceptsJoinableIdentitySourcesWithoutSchemaChange|TestUpdateCoagentRejectsUnknownJoinableSourceKind|TestDefaultProfileRegistriesExactAuthorityContract`.
- **Rollback:** revert this commit. Owner gates and effects-OFF are unchanged.
- **Heresy delta:** `introduced` typed URI join keys and durable metadata carry-forward. `repaired` the open check that Texture omitted generic `update_coagent` by pinning that omission as the production contract.

## Residual

Rehearsal, live solitaire proof, irreversible send, and completion cutover remain unpaid. A later deploy must re-observe the Texture registry on staging; source main already omits the generic tool.

## Next

Route map 9: rehearsal. Do not send live mail. Do not rematerialize. Do not delete owner gates.
