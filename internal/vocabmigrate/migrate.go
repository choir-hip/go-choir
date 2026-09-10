package vocabmigrate

// Frozen V1→V2 row migration core for mission-2 (decode item, second half).
//
// Pure functions only: no SQL, no store, no network. The serving fence
// (appender replay deposits, base rebuild in scratch, rematerialize flip,
// boot dispatch, RecoverPrepared paths) calls these appliers and refuses
// when unknowns are non-empty. Forward migration ships only with the proven
// inverse exercised here: INV-CANON (canonical V1 representative, proven
// semantically compatible with old source) and INV-PROV (exact restoration
// from retained per-row source-token provenance where many-to-one collapse
// would otherwise lose the original token).
//
// Mapping source: docs/evidence/choir-rlm-v2-mapping-2026-09-10.md §3.
// Frozen protocol (owner, trusted-core, PrivacyClass owner) is never mapped:
// appliers skip it and report it separately. Unknown tokens are reported,
// never silently passed through and never elevated.

import (
	"fmt"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/types"
)

// forwardV1ToV2 is the single frozen map. Every V1 desk spelling known to any
// per-function acceptor table appears here exactly once.
var forwardV1ToV2 = map[string]string{
	// Management desk (Canonical V1 {super}).
	"super": "management",
	// Engineering desk (Canonical V1 cosuper/co-super/coagent/co-agent,
	// NormalizeRole extras co_super/cosuper_coding/co-super-coding,
	// V1 spawn synonym engineering).
	"co-super": "engineering", "cosuper": "engineering",
	"coagent": "engineering", "co-agent": "engineering",
	"co_super": "engineering", "cosuper_coding": "engineering",
	"co-super-coding": "engineering", "engineering": "engineering",
}

// Desk tokens that survive the rename unchanged.
var staysLive = map[string]bool{
	"texture": true, "conductor": true, "processor": true,
	"reconciler": true, "email": true, "verifier": true,
	"verifier-multimodal": true, "verifier_multimodal": true,
}

// Frozen protocol values: never desk vocabulary, never migrated.
var frozenProtocol = map[string]bool{
	"owner": true, "trusted-core": true,
}

func init() {
	// Research desk (kept out of the literal so the engineering/research
	// boundary stays review-visible, one desk per block).
	for _, tok := range []string{"researcher", "researchers", "research",
		"research-agent", "web-research", "web-researcher"} {
		forwardV1ToV2[tok] = "research"
	}
}

// ForwardV1ToV2 maps one frozen V1 desk token to its V2 live name.
func ForwardV1ToV2(token string) (string, bool) {
	key := strings.TrimSpace(strings.ToLower(token))
	if v2, ok := forwardV1ToV2[key]; ok {
		return v2, true
	}
	// Underscore spellings resolve through their hyphen forms, mirroring
	// the Canonical `_`→`-` normalization that precedes every V1 branch.
	if v2, ok := forwardV1ToV2[strings.ReplaceAll(key, "_", "-")]; ok {
		return v2, true
	}
	if staysLive[key] {
		return key, true
	}
	return "", false
}

// IsFrozenProtocol reports whether a token is frozen non-desk vocabulary
// (owner, trusted-core) that migration must skip, never map.
func IsFrozenProtocol(token string) bool {
	return frozenProtocol[strings.TrimSpace(strings.ToLower(token))]
}

// InverseV2ToV1Canonical maps a V2 live name back to its canonical V1
// representative (INV-CANON). Many-to-one spellings need INV-PROV instead;
// see ProvenanceLog.
func InverseV2ToV1Canonical(v2 string) (string, bool) {
	switch strings.TrimSpace(strings.ToLower(v2)) {
	case "management":
		return "super", true
	case "engineering":
		return "co-super", true
	case "research":
		return "researcher", true
	case "texture", "conductor", "processor", "reconciler", "email",
		"verifier", "verifier-multimodal", "verifier_multimodal":
		return strings.TrimSpace(strings.ToLower(v2)), true
	default:
		return "", false
	}
}

// ProvenanceLog retains per-row source tokens where forward migration
// collapses many V1 spellings onto one V2 name. INV-PROV restores the exact
// original; rows without a log entry fall back to INV-CANON.
type ProvenanceLog struct {
	entries map[string]string // field key -> exact V1 token
}

// Record retains the exact V1 token for field key when it is not the
// canonical representative for its V2 name.
func (l *ProvenanceLog) Record(key, v1, v2 string) {
	canon, ok := InverseV2ToV1Canonical(v2)
	if !ok || canon == strings.TrimSpace(strings.ToLower(v1)) {
		return
	}
	if l.entries == nil {
		l.entries = map[string]string{}
	}
	l.entries[key] = v1
}

// InverseExact restores the exact V1 token for a field key, falling back to
// the canonical representative when no provenance was retained.
func (l *ProvenanceLog) InverseExact(key, v2 string) (string, bool) {
	if l != nil && l.entries != nil {
		if v1, ok := l.entries[key]; ok {
			return v1, true
		}
	}
	return InverseV2ToV1Canonical(v2)
}

// Outcome records what one applier did. Unknowns block the fence; Skipped
// names frozen-protocol values left in place by design.
type Outcome struct {
	Changed  bool
	Skipped  []string
	Unknowns []string
}

func (o *Outcome) mapToken(key, token string, log *ProvenanceLog) string {
	if strings.TrimSpace(token) == "" {
		return token
	}
	if IsFrozenProtocol(token) {
		o.Skipped = append(o.Skipped, key+"="+token)
		return token
	}
	v2, ok := ForwardV1ToV2(token)
	if !ok {
		o.Unknowns = append(o.Unknowns, key+"="+token)
		return token
	}
	if v2 != token {
		o.Changed = true
		if log != nil {
			log.Record(key, token, v2)
		}
	}
	return v2
}

// MigrateAgentID rewrites desk-bearing agent-ID prefixes under the frozen
// map: super:→management:, co-super:/cosuper:→engineering:, researcher:→
// research:. work:/run:/texture:/conductor: and bare command identities are
// untouched. It reports (possibly unchanged) ID plus outcome.
func MigrateAgentID(id string, log *ProvenanceLog) (string, Outcome) {
	var out Outcome
	for _, prefix := range []string{"super:", "co-super:", "cosuper:", "researcher:"} {
		if strings.HasPrefix(id, prefix) {
			v1tok := strings.TrimSuffix(prefix, ":")
			v2, ok := ForwardV1ToV2(v1tok)
			if !ok {
				out.Unknowns = append(out.Unknowns, "agent_id="+id)
				return id, out
			}
			if log != nil {
				log.Record("agent_id", v1tok, v2)
			}
			out.Changed = true
			return v2 + ":" + strings.TrimPrefix(id, prefix), out
		}
	}
	return id, out
}

// MigrateRunRecord migrates RunRecord AgentProfile/AgentRole. Digest note:
// run rows whose digest covers these fields take a V2-domain successor
// digest at the fence; history rows are never rewritten.
func MigrateRunRecord(rec *types.RunRecord, log *ProvenanceLog) Outcome {
	var out Outcome
	rec.AgentProfile = out.mapToken("agent_profile", rec.AgentProfile, log)
	rec.AgentRole = out.mapToken("agent_role", rec.AgentRole, log)
	return out
}

// MigrateAgentRecord migrates AgentRecord Profile/Role.
func MigrateAgentRecord(rec *types.AgentRecord, log *ProvenanceLog) Outcome {
	var out Outcome
	rec.Profile = out.mapToken("profile", rec.Profile, log)
	rec.Role = out.mapToken("role", rec.Role, log)
	return out
}

// MigrateWorkItemRecord migrates WorkItemRecord AuthorityProfile.
func MigrateWorkItemRecord(work *types.WorkItemRecord, log *ProvenanceLog) Outcome {
	var out Outcome
	work.AuthorityProfile = out.mapToken("authority_profile", work.AuthorityProfile, log)
	return out
}

// MigrateChannelMessage migrates ChannelMessage Role. Content bodies are
// out of scope (model-authored prose); addresses migrate via MigrateAgentID
// at the fence where join predicates must stay intact.
func MigrateChannelMessage(msg *types.ChannelMessage, log *ProvenanceLog) Outcome {
	var out Outcome
	msg.Role = out.mapToken("role", msg.Role, log)
	return out
}

// MigrateInboxDelivery migrates InboxDelivery Role.
func MigrateInboxDelivery(msg *types.InboxDelivery, log *ProvenanceLog) Outcome {
	var out Outcome
	msg.Role = out.mapToken("role", msg.Role, log)
	return out
}

// MigrateGrantAttestation migrates CoSuperGrantPolicyAttestation Role.
// Digest note: the live V2 attestation takes a V2-domain successor digest
// with the mapping receipt linking the immutable V1 predecessor.
func MigrateGrantAttestation(att *types.CoSuperGrantPolicyAttestation, log *ProvenanceLog) Outcome {
	var out Outcome
	att.Role = out.mapToken("role", att.Role, log)
	return out
}

// Metadata role keys migrated wherever run metadata carries them.
var metadataRoleKeys = []string{"agent_profile", "agent_role", "requested_by_profile"}

// MigrateMetadata migrates desk-bearing role values inside a run metadata
// map in place.
func MigrateMetadata(metadata map[string]any, log *ProvenanceLog) Outcome {
	var out Outcome
	if metadata == nil {
		return out
	}
	for _, key := range metadataRoleKeys {
		raw, ok := metadata[key].(string)
		if !ok || strings.TrimSpace(raw) == "" {
			continue
		}
		migrated := out.mapToken(key, raw, log)
		if migrated != raw {
			metadata[key] = migrated
		}
	}
	return out
}

// RevertRowV1 restores one canonical V1 representative for a V2 token
// (INV-CANON). Callers needing exact spellings use ProvenanceLog.InverseExact.
func RevertRowV1(v2 string) (string, error) {
	v1, ok := InverseV2ToV1Canonical(v2)
	if !ok {
		return "", fmt.Errorf("vocabmigrate: no V1 inverse for %q", v2)
	}
	return v1, nil
}
