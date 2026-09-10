# Mission-2 Decoder Matrix — 2026-09-10

Landing step 3 (acceptance item 2, first half). One row per raw-entry path
and version state. Source ref `main@25fc8ae8`. At this step V1 aliases are
still accepted live; both roots behave identically on V1. The cutover flips
the admission root to V2-only; the historic root never changes.

Roots: `H` = DecodeHistoricEvent, `HP` = DecodeHistoricDurableEvents,
`A` = DecodeAdmissionCASRequest, `PB` = DecodeProjectionBatch (format
discriminator, vocabulary-neutral), `DESC` = ParseDescriptor (descriptor
seam). Charter scope is Event/CASRequest/DurableEvent; Receipt decodes and
non-computerevent types are listed as explicitly out of scope with reasons
(mirrored in scripts/check-decode-roots.sh, which enforces this matrix in CI).

## In-scope rows

| Raw entry path | Raw bytes | Carrier / selector | Decoder | Consumer | Canonicalization boundary | Expected V1 | Expected V2 (post-cutover) |
|---|---|---|---|---|---|---|---|
| platform EventArtifactService PinEvent (`event_artifacts.go`) | canonical event bytes | tape event field (unmarked → V1) | H | pin + artifact store | frozen V1 view only; bytes verified byte-identical after | replay byte-identical, verify via frozen V1 decode | n/a (history only) |
| platform DiskEventSource EventsPage (`projectionbase/source.go`, HTTP `source_http.go` TailPage) | disk/HTTP artifact bytes | tape event field (unmarked → V1) | H / HP | rebuilder, restore, rematerialization | frozen V1 view only | replay byte-identical | n/a (history only) |
| computerevent HTTPClient EventsPage (`http_client.go`) | replay endpoint body | tape event field (unmarked → V1) | HP | runReplayPhase, Reconstruct* | frozen V1 view only | replay byte-identical | n/a (history only) |
| store Prepared / EventByIdempotency / EventByDigest (`computer_events.go`) | SQL event_json | run/lifecycle row columns (unmarked → V1) | H | replay, recovery, admission checks | frozen V1 view | rows read as V1 until forward-migrated | rows serve live only stamped v2 |
| store FinalizedDecisionForOperation (`computer_event_recovery.go`) | SQL event_json | run row (unmarked → V1) | H | recovery paths | frozen V1 view | V1 tail replays byte-identical | forward-migrated before projections commit |
| platform checkpoint decoders (`checkpoints.go` genesis + verifier) | event artifact bytes | tape event field (unmarked → V1) | H | checkpoint authority | frozen V1 view + strict authority compare (co-super/verifier literals are frozen-protocol positions) | V1 verifies | n/a (history only) |
| platform CAS append (`event_handlers.go` HandleComputerEventAppend) | HTTP request body | live write (admission) | A | appender tape-append gate | version-selected: V1 today, V2 at cutover | accepted (aliases live) | V1 names refuse; unknown refuse |
| appender resolveProjectionBatch (`appender.go` after ResolvePayloads) | payload bytes | payload envelope marker where role-bearing | PB | dry-run, decode, application | format gate only (V1/V2 projector pair, not vocabulary) | pair enforced | pair enforced |
| projectionbase Rebuilder Run + restore/rematerialization | base blob + tape tail | base descriptor vocabulary_version + unmarked tail → V1 | DESC + H | install, boot reconstruction | descriptor refuse-missing/unknown; tail frozen V1 | v1 base installs on post-cutover source | v2 base installs; genesis fallback is a mission-0 regression |
| autoputer runReplayPhase, Reconstruct/Into/ThroughTarget | staged store + tape | event envelope (interpretation only) | H | replay dispatch | frozen V1 view | byte-identical | n/a (history only) |

Negative rows: ValidateEventPins receipt lists, credential/lifecycle/mode
receipt decodes, capsule/proxy/verifier receipts — Receipt namespace, no desk
vocabulary, outside the gate. Provider message events, maild/cycle/routeledger
events, texture-watch WorkState struct, buildinfo deploy receipt — distinct
namespaces. Payload plaintext is opaque arbitrary bytes (only
`Role='projection_batch'` synthetic ref).

## Cutover flip (normative, not this slice)

Landing refusal or alias retirement while live writers still emit V1, or
serving V2 rows while writers still emit V1, is a defect. The drill
(migrate/revert/migrate ending on V1 serving rows) precedes the single
never-partially-deployed cutover: forward-migration + serving fence, then
CurrentVocabularyVersion=v2 plus V2-only writers plus alias retirement plus
fail-closed refusal plus in-scope client bundles.
