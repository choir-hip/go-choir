package vocabmigrate

// Serving-fence verification for mission-2 (decode item, second half).
//
// The fence NEVER migrates: it verifies that rows about to serve live
// authority speak the active vocabulary, and refuses otherwise. Forward
// migration runs only inside the drill and cutover flows, which invoke the
// fence afterward as the guarantee. Pre-cutover (active v1) the fence passes
// every frozen V1 token (including aliases), stays-live profiles, and frozen
// protocol, and refuses V2-only names and unknowns. Post-cutover (active v2)
// it passes exactly the V2 live set plus frozen protocol and refuses
// everything else — including V1 aliases, so no unmigrated row can hold
// authority after the cutover.
//
// Wire points (each verifies before the guarded transition commits):
//   - appender RecoverPrepared (replayed prepared events);
//   - rematerialize staged-to-live flip;
//   - base rebuild publish (inside scratch, before publish).
// Boot-dispatch sweeps are explicitly deferred: they iterate the same row
// classes the fence already covers at deposit time, and a full vocabulary
// sweep on every boot is a cutover-time cost decision, not a step-5 gate.

import (
	"fmt"
	"strings"
)

// V1 membership is decided by ForwardV1ToV2 itself (frozen map plus
// stays-live identity plus underscore folding), so the fence and the
// migrator can never disagree: one function is the single authority.

// v2LiveSet is the frozen V2 vocabulary used while replaying historical
// records. Processor and reconciler are retained only for that replay path.
// management_task is management's delegated intake task (S2a): every role
// agentprofile registers must be here, or the post-cutover write guard
// refuses its agent object
// (docs/archive/problems/s2a-management-tasks-refused-by-vocab-fence-2026-10-14.md).
var v2LiveSet = map[string]bool{
	"management": true, "engineering": true, "research": true,
	"texture": true, "conductor": true, "processor": true,
	"reconciler": true, "email": true, "verifier": true,
	"verifier_multimodal": true, "management_task": true,
}

// Field is one role-bearing value presented to the fence.
type Field struct {
	// Key names the carrier for refusal messages (e.g. "run.agent_profile").
	Key string
	// Value is the token to check. Empty values are skipped (typed-missing
	// is the activation item's concern, not the fence's).
	Value string
}

// AuthorityPathSeparator joins the distinct desk roles of a trajectory into
// a run acceptance's authority_profile ("texture > research"). The writer
// (agentcore acceptanceAuthorityProfile) and the fence split on this one
// constant.
const AuthorityPathSeparator = " > "

// AuthorityPathFields presents an authority path to the fence one role per
// field, so every part must be a live role and a retired role inside a path
// is still refused. A path with an empty part is presented whole, which the
// fence refuses: the fence skips empty values, so forwarding an empty part
// would pass " >  > "
// (docs/problems/run-acceptance-refused-by-vocab-fence-on-multi-desk-trajectory-2026-10-22.md).
func AuthorityPathFields(key, path string) []Field {
	if strings.TrimSpace(path) == "" {
		return []Field{{Key: key, Value: path}}
	}
	parts := strings.Split(path, AuthorityPathSeparator)
	fields := make([]Field, 0, len(parts))
	for _, part := range parts {
		if strings.TrimSpace(part) == "" {
			return []Field{{Key: key, Value: path}}
		}
		fields = append(fields, Field{Key: key, Value: part})
	}
	return fields
}

// VerifyServingVocabulary refuses when any field falls outside the active
// vocabulary. Frozen-protocol values pass under both vocabularies. The
// returned error names the first offending field.
func VerifyServingVocabulary(active string, fields ...Field) error {
	active = strings.TrimSpace(strings.ToLower(active))
	if active != VocabularyV1 && active != VocabularyV2 {
		return fmt.Errorf("vocabfence: unknown active vocabulary %q", active)
	}
	for _, f := range fields {
		value := strings.TrimSpace(strings.ToLower(f.Value))
		if value == "" {
			continue
		}
		if IsFrozenProtocol(value) {
			continue
		}
		var ok bool
		if active == VocabularyV1 {
			_, ok = ForwardV1ToV2(value)
		} else {
			ok = v2LiveSet[value]
		}
		if !ok {
			return fmt.Errorf("vocabfence: %s=%q not in %s vocabulary", f.Key, f.Value, active)
		}
	}
	return nil
}

// EventActorFields projects an event's role-bearing envelope for the fence.
// Authority refs are a different namespace and are never fenced as roles.
func EventActorFields(eventActorProfile string) []Field {
	return []Field{
		{Key: "event.actor_profile", Value: eventActorProfile},
	}
}
