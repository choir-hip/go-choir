package vocabmigrate

// Per-family frozen decoders for the stratum-B durable vocabulary (R5a;
// desk-rlm-rectification plan §4, §11.2). The profile-token frozen tables
// live in migrate.go and computerevent/decode.go; this file freezes the
// remaining families BEFORE any rename ships, so bytes written under the
// V1 vocabulary decode identically forever: the table IS the decoder —
// membership resolves the token to its live semantic value and nothing
// outside the table is frozen vocabulary.
//
// Decode-boundary routing rule: a frozen decoder is routed where a
// persisted token drives interpretation — kind dispatch, schema
// validation, digest mint, identity derivation, profile comparison.
// Storage-opaque reads (a kind string carried but never dispatched on,
// a column name, a JSON tag consumed structurally by encoding/json) are
// frozen in the manifest tables here and enforced by the coverage test,
// not routed per-read: routing every opaque read would churn hot paths
// for zero fold benefit. That distinction is the falsifier guard —
// decoders without interpretation sites would be dead code.
//
// Freeze date 2026-09-26, post-K kernel, pre-R5b. R5b (the rename) is
// deferred indefinitely (plan §11.4): these tables never grow — new
// vocabulary is NEW vocabulary, not a table entry.

import (
	"strings"

	"github.com/yusefmosiah/go-choir/internal/types"
)

// ---------------------------------------------------------------------------
// Family: lifecycle command kinds (types/lifecycle.go constants; the durable
// spellings in choir.lifecycle_command objects and lifecycle receipts).
// ---------------------------------------------------------------------------

// frozenLifecycleCommandKinds is every lifecycle command kind string in the
// durable vocabulary at freeze. V1-spelled members are marked in the comment;
// the frozen set covers all of them so decode never mints a silent unknown.
var frozenLifecycleCommandKinds = map[types.LifecycleCommandKind]bool{
	types.LifecyclePrepareCancelTrajectory:          true,
	types.LifecycleStart:                            true,
	types.LifecycleOpenWork:                         true,
	types.LifecycleAmendWork:                        true,
	types.LifecycleRecordRefs:                       true,
	types.LifecycleQueueUpdate:                      true,
	types.LifecycleApplyUpdate:                      true,
	types.LifecycleCommitArtifactHead:               true,
	types.LifecycleReplaceActivation:                true,
	types.LifecycleSettleWork:                       true,
	types.LifecycleRefuseWork:                       true,
	types.LifecycleSettleTrajectory:                 true,
	types.LifecycleCancelTrajectory:                 true,
	types.LifecycleArchiveArtifact:                  true,
	types.LifecycleApplyTextureTurn:                 true,
	types.LifecycleIssueControl:                     true,
	types.LifecycleCommitAct:                        true, // RN3 record-native mint 2026-10-02
	types.LifecycleBindControlDelivery:              true,
	types.LifecycleFailControlActivation:            true,
	types.LifecycleOpenEngineeringAssignment:        true, // V1 "open_co_super_assignment"
	types.LifecycleBindEngineeringAssignment:        true, // V1 "bind_co_super_assignment"
	types.LifecycleRecordEngineeringAssignment:      true, // V1 "record_co_super_assignment"
	types.LifecycleCancelEngineeringAssignment:      true, // V1 "cancel_co_super_assignment"
	types.LifecycleSetEngineeringCapsuleDisposition: true, // V1 "set_co_super_capsule_disposition"
	types.LifecycleSettleProducerReports:            true,
	types.LifecycleReconcileUpdateDelivery:          true,
	types.LifecycleTerminalizeRun:                   true,
	types.LifecycleReactivateRun:                    true,
}

// frozenLifecycleEventKinds is every lifecycle event kind string in the
// durable vocabulary at freeze.
var frozenLifecycleEventKinds = map[types.LifecycleEventKind]bool{
	types.LifecycleUpdateLate:                       true,
	types.LifecycleTrajectoryStarted:                true,
	types.LifecycleWorkOpened:                       true,
	types.LifecycleWorkAmended:                      true,
	types.LifecycleRefsRecorded:                     true,
	types.LifecycleUpdateQueued:                     true,
	types.LifecycleActivationReplaced:               true,
	types.LifecycleUpdateApplied:                    true,
	types.LifecycleArtifactHeadAdvanced:             true,
	types.LifecycleWorkSettled:                      true,
	types.LifecycleUpdateRejected:                   true,
	types.LifecycleWorkRefused:                      true,
	types.LifecycleTrajectorySettled:                true,
	types.LifecycleTrajectoryCancelled:              true,
	types.LifecycleTrajectoryCancellationRequested:  true,
	types.LifecycleArtifactArchived:                 true,
	types.LifecycleTextureTurnCommitted:             true,
	types.LifecycleControlQueued:                    true,
	types.LifecycleControlDelivered:                 true,
	types.LifecycleControlActivationFailed:          true,
	types.LifecycleUpdateDelivered:                  true,
	types.LifecycleTextureActivationFailed:          true,
	types.LifecycleEngineeringAssignmentOpened:      true, // V1 "co_super_assignment_opened"
	types.LifecycleEngineeringAssignmentBound:       true, // V1 "co_super_assignment_bound"
	types.LifecycleEngineeringAssignmentReported:    true, // V1 "co_super_assignment_reported"
	types.LifecycleEngineeringAssignmentCancelled:   true, // V1 "co_super_assignment_cancelled"
	types.LifecycleEngineeringCapsuleDispositionSet: true, // V1 "co_super_capsule_disposition_set"
	types.LifecycleRunTerminalized:                  true,
	types.LifecycleRunReactivated:                   true,
	// Tombstone: retired by M1's owner-input cutover (2026-09-23). Pre-M1
	// computers' frozen tapes still carry it; the decode boundary must accept
	// it so boot reconcile on those computers does not fail closed.
	types.LifecycleOwnerInstructionQueued: true,
}

// DecodeLifecycleCommandKind interprets a persisted lifecycle command kind
// under the frozen table: known kinds resolve to themselves (byte-identical,
// frozen vocabulary); anything else reports not-frozen so the interpretation
// boundary fails loudly instead of minting silent semantics.
func DecodeLifecycleCommandKind(kind types.LifecycleCommandKind) (types.LifecycleCommandKind, bool) {
	_, ok := frozenLifecycleCommandKinds[kind]
	return kind, ok
}

// DecodeLifecycleEventKind is the event-kind half of the same frozen table.
func DecodeLifecycleEventKind(kind types.LifecycleEventKind) (types.LifecycleEventKind, bool) {
	_, ok := frozenLifecycleEventKinds[kind]
	return kind, ok
}

// ---------------------------------------------------------------------------
// Family: object-graph object/edge kinds (og_objects.kind; the value set is
// frozen — all are V1-spelled *co_super* kinds plus the neutral set).
// ---------------------------------------------------------------------------

// frozenOGObjectKinds is every objectgraph kind string in the durable
// vocabulary at freeze. Write-side constants and identity-formula entries
// must appear here (the coverage test pins the tables equal to production
// registries); reads compare by kind and a kind absent from the table is
// not interpretable as frozen vocabulary.
var frozenOGObjectKinds = map[string]bool{
	// Neutral kinds (no V1 spelling).
	"choir.actor_wake_outbox":            true,
	"choir.agent":                        true,
	"choir.agent_evidence":               true,
	"choir.artifact_blob":                true,
	"choir.artifact_manifest":            true,
	"choir.audio_recording":              true,
	"choir.autoradio_run_sheet":          true,
	"choir.browser_session":              true,
	"choir.channel_message":              true,
	"choir.coagent_mailbox":              true,
	"choir.commitment_record":            true,
	"choir.consent_record":               true,
	"choir.content_item":                 true,
	"choir.desktop_app_instance":         true,
	"choir.desktop_session":              true,
	"choir.event":                        true,
	"choir.inbox_delivery":               true,
	"choir.lifecycle_cancel_intent":      true,
	"choir.lifecycle_command":            true,
	"choir.lifecycle_event":              true,
	"choir.lifecycle_sequence":           true,
	"choir.media_item":                   true,
	"choir.podcast_subscription":         true,
	"choir.provenance_activity":          true,
	"choir.provenance_agent":             true,
	"choir.provenance_entity":            true,
	"choir.public_route":                 true,
	"choir.publication":                  true,
	"choir.publication_policy":           true,
	"choir.publication_proposal":         true,
	"choir.publication_source_entity":    true,
	"choir.publication_transclusion":     true,
	"choir.publication_version":          true,
	"choir.retrieval_manifest":           true,
	"choir.retrieval_source":             true,
	"choir.retrieval_span":               true,
	"choir.review_record":                true,
	"choir.run":                          true,
	"choir.run_acceptance":               true,
	"choir.run_continuation":             true,
	"choir.run_memory_entry":             true,
	"choir.scheduler_sequence":           true,
	"choir.source_entity":                true,
	"choir.source_ref":                   true,
	"choir.subject":                      true,
	"choir.texture_decision":             true,
	"choir.texture_document":             true,
	"choir.texture_revision":             true,
	"choir.trajectory":                   true,
	"choir.transcript":                   true,
	"choir.universal_wire_story_cluster": true,
	"choir.verifier_attestation":         true,
	"choir.web_capture":                  true,
	"choir.work_item":                    true,
	"choir.worker_update":                true,
	// V1-spelled kinds (the vocabulary this family freezes).
	"choir.co_super_assignment":        true, // V1 — engineering assignment
	"choir.co_super_assignment_report": true, // V1 — engineering report
	"choir.co_super_subject_candidate": true, // V1 — engineering candidate
	"choir.co_super_capability_claim":  true, // V1 — capability claim
	"choir.co_super_capsule_claim":     true, // V1 — capsule claim
	"choir.co_super_run_claim":         true, // V1 — run claim
}

// DecodeOGObjectKind interprets a persisted objectgraph kind under the
// frozen table.
func DecodeOGObjectKind(kind string) (string, bool) {
	_, ok := frozenOGObjectKinds[kind]
	return kind, ok
}

// ---------------------------------------------------------------------------
// Family: schema strings (durable JSON "schema" members + wire envelopes).
// ---------------------------------------------------------------------------

// frozenSchemaStrings is every schema/digest-domain string carrying V1
// vocabulary at freeze. Members are the live constants' values pinned here
// so a later rename cannot silently change what historic bytes validate
// against.
var frozenSchemaStrings = map[string]bool{
	// V1-spelled schemas and digest domains.
	"choir.co_super_assignment/v1":                true,
	"choir.co_super_assignment_command":           true,
	"choir.co_super_capsule_evidence/v1":          true,
	"choir.co_super_capsule_evidence_verifier/v1": true,
	"choir.co_super_capsule_fate_step/v1":         true,
	"choir.co_super_execution_attestation/v1":     true,
	"choir.co_super_grant_policy/v1":              true,
	"choir.co_super_grant_policy_attestation/v1":  true,
	"choir.co_super_verb_set/v1":                  true,
	"choir.lifecycle.persistent-super-report/v1":  true, // V1 retained alongside v2; both frozen
	"choir.lifecycle.persistent-super-report/v2":  true,
	`choir.co-super.opaque-capability/v1\x00`:     true, // digest domain incl. NUL marshal sep
	"choir.co-super.opaque-capability/v1":         true,
	// Neutral durable schema strings (same family — durable bytes carrying
	// these must validate under frozen vocabulary too).
	"choir.durable_work.v1":                    true,
	"choir.execution_identity.v1":              true,
	"choir.lifecycle.cancel-intent/v1":         true,
	"choir.lifecycle_injection.v1":             true,
	"choir.texture_create.v1":                  true,
	"choir.texture_observation.v1":             true,
	"choir.texture_show.v1":                    true,
	"choir.texture_source_open.v1":             true,
	"choir.news/acceptance/execution-identity": true,
}

// DecodeSchemaString interprets a persisted schema string under the frozen
// table.
func DecodeSchemaString(schema string) (string, bool) {
	_, ok := frozenSchemaStrings[schema]
	return schema, ok
}

// SchemaCapsuleEvidenceV1 is the frozen durable evidence-envelope schema the
// CLI compares against (cmd/choir routes through this instead of the
// literal).
const SchemaCapsuleEvidenceV1 = "choir.co_super_capsule_evidence/v1"

// ---------------------------------------------------------------------------
// Family: identity seeds. These mint content-addressed IDs — renaming one
// re-mints every replayed identity. Decision (R5a, plan §11.4): KEEP
// choir:co-super-assignment:v3 and all neighbors byte-for-byte forever.
// ---------------------------------------------------------------------------

const (
	// IdentitySeedCoSuperAssignmentV3 mints engineering assignment IDs.
	// KEEP-V3: pinned by the owner directive — a v4 seed with a replay
	// story was the rejected alternative (plan §4: renaming breaks replay —
	// the same revision would mint a new assignment ID).
	IdentitySeedCoSuperAssignmentV3 = "choir:co-super-assignment:v3"
	// IdentitySeedCoSuperRequestV2 mints delegated cast request IDs.
	IdentitySeedCoSuperRequestV2 = "choir:co-super-request:v2"
	// IdentitySeedCoSuperDecisionV3 mints owner decision records.
	IdentitySeedCoSuperDecisionV3 = "choir:co-super-decision:v3"
	// IdentitySeedDelegatedCastRequestV1 mints delegated-cast request IDs.
	IdentitySeedDelegatedCastRequestV1 = "choir:delegated-cast-request:v1"
	// IdentitySeedDelegatedDecisionV1 mints delegated-cast decision IDs.
	IdentitySeedDelegatedDecisionV1 = "choir:delegated-decision:v1"
	// IdentitySeedPersistentSuperReportV1 mints persistent report occurrence
	// IDs.
	IdentitySeedPersistentSuperReportV1 = "choir:persistent-super-report:v1"
)

// frozenIdentitySeeds is the complete seed family at freeze. Any new seed
// is a NEW seed; these strings never change.
var frozenIdentitySeeds = map[string]bool{
	IdentitySeedCoSuperAssignmentV3:     true,
	IdentitySeedCoSuperRequestV2:        true,
	IdentitySeedCoSuperDecisionV3:       true,
	IdentitySeedDelegatedCastRequestV1:  true,
	IdentitySeedDelegatedDecisionV1:     true,
	IdentitySeedPersistentSuperReportV1: true,
}

// IsFrozenIdentitySeed reports whether seed is frozen vocabulary.
func IsFrozenIdentitySeed(seed string) bool {
	return frozenIdentitySeeds[seed]
}

// ---------------------------------------------------------------------------
// Family manifest: durable-vocabulary identifiers that have no routed decode
// boundary (structural interpretation only) — frozen for the coverage test
// and the R5b census so nothing V1-spelled escapes the freeze.
// ---------------------------------------------------------------------------

// FrozenLifecycleJSONFields are durable JSON member names carrying V1
// spelling (encoding/json consumes them structurally; the freeze prevents a
// rename from orphaning historic payloads).
var FrozenLifecycleJSONFields = []string{
	"co_super_assignments", // types.LifecycleAssignmentSet.EngineeringAssignments tag
}

// FrozenSQLIdentifiers are SQL table/index/column names carrying V1
// spelling. They are DDL, not decoded bytes; they freeze so a rename is a
// deliberate migration, never a silent drift.
var FrozenSQLIdentifiers = []string{
	"co_super_slots",
	"idx_co_super_slots_run_id",
	"idx_co_super_slots_agent_id",
}

// ---------------------------------------------------------------------------
// Stratum C: the ONE explicit V1→V2 normalization point (plan §4).
// ---------------------------------------------------------------------------

// normalizeV2Live is the active V2 canonical acceptor set mirrored as data so
// this package never imports agentprofile (which must stay able to change).
// Processor and reconciler entries are frozen-history recognition only; they
// remain to replay V2 records, not to admit live profiles.
var normalizeV2Live = map[string]string{
	"management":          "management",
	"engineering":         "engineering",
	"research":            "research",
	"texture":             "texture",
	"conductor":           "conductor",
	"processor":           "processor",
	"reconciler":          "reconciler",
	"email":               "email",
	"verifier":            "verifier",
	"verifier_multimodal": "verifier_multimodal",
	"verifier-multimodal": "verifier_multimodal",
}

// NormalizeHistoricProfile is the single explicit normalization point for
// comparing a decoded historic ActorProfile token against live V2 constants
// (plan stratum C — the V1→V2 decode mismatch resolves here, nowhere else):
//
//   - live V2 tokens resolve to their canonical live spelling;
//   - frozen V1 desk spellings resolve through ForwardV1ToV2;
//   - frozen protocol values (owner, trusted-core) pass through —
//     they are never desk vocabulary;
//   - everything else reports unknown (fail closed).
//
// Callers comparing a decoded historic event's ActorProfile to a live
// constant MUST use this function, not raw equality — a pre-cutover event
// spelled "co-super" is byte-identical history, not an invalid actor.
func NormalizeHistoricProfile(token string) (string, bool) {
	key := strings.TrimSpace(strings.ToLower(token))
	if frozenProtocol[key] {
		return key, true
	}
	if v2, ok := normalizeV2Live[key]; ok {
		return v2, true
	}
	if v2, ok := ForwardV1ToV2(key); ok {
		return v2, true
	}
	return "", false
}
