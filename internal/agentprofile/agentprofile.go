// Package agentprofile defines canonical agent profile identifiers,
// normalization, capabilities, and spawn/message policy.
package agentprofile

import (
	"fmt"
	"strings"
)

const (
	Conductor  = "conductor"
	Super      = "super"
	CoSuper    = "co-super"
	Researcher = "researcher"
	Texture    = "texture"
	Processor  = "processor"
	Reconciler = "reconciler"
	Email      = "email"
)

// Policy is the canonical capability, spawn, and message policy for an agent profile.
type Policy struct {
	Profile                   string
	AllowReadOnlyFiles        bool
	AllowResearchTools        bool
	AllowEvidenceTools        bool
	AllowMemoryTools          bool
	AllowModelDiagnosticTools bool
	AllowCoAgentTools         bool
	AllowedSpawnTargets       []string
	AllowedMessageTargets     []string
}

// PolicyFor returns the capability, spawn, and message policy for profile.
func PolicyFor(profile string) Policy {
	canonical, _ := Canonical(profile)
	switch canonical {
	case Conductor:
		return Policy{
			Profile:             Conductor,
			AllowCoAgentTools:   true,
			AllowedSpawnTargets: []string{Texture},
		}
	case Researcher:
		return Policy{
			Profile:                   Researcher,
			AllowReadOnlyFiles:        true,
			AllowResearchTools:        true,
			AllowEvidenceTools:        true,
			AllowMemoryTools:          true,
			AllowModelDiagnosticTools: true,
			AllowCoAgentTools:         true,
			AllowedSpawnTargets:       nil,
			AllowedMessageTargets:     []string{Texture},
		}
	case Texture:
		// Texture is the artifact control plane, not an evidence gatherer. It does
		// not receive researcher-owned evidence tools (save/read/list_evidence) or
		// the verify_model_capability diagnostic by default. It keeps run-memory
		// retrieval so it can recover its own compacted context. Lifecycle target
		// and cancellation authority stays in atomic Texture controls/reducers;
		// Texture must not receive the generic model-authored cancel_agent tool.
		return Policy{
			Profile:               Texture,
			AllowMemoryTools:      true,
			AllowCoAgentTools:     false,
			AllowedSpawnTargets:   []string{Researcher},
			AllowedMessageTargets: []string{Researcher, Super},
		}
	case Processor:
		return Policy{
			Profile:                   Processor,
			AllowReadOnlyFiles:        true,
			AllowResearchTools:        true,
			AllowEvidenceTools:        true,
			AllowMemoryTools:          true,
			AllowModelDiagnosticTools: true,
			AllowCoAgentTools:         true,
			AllowedSpawnTargets:       []string{Texture},
			AllowedMessageTargets:     []string{Texture},
		}
	case Reconciler:
		return Policy{
			Profile:                   Reconciler,
			AllowReadOnlyFiles:        true,
			AllowResearchTools:        true,
			AllowEvidenceTools:        true,
			AllowMemoryTools:          true,
			AllowModelDiagnosticTools: true,
			AllowCoAgentTools:         true,
			AllowedSpawnTargets:       []string{Texture},
			AllowedMessageTargets:     []string{Texture},
		}
	case Email:
		return Policy{Profile: Email}
	case CoSuper:
		// CoSuper has no static tool authority. The assignment runtime constructs a
		// fresh per-run registry from the exact capsule-local closed set plus
		// update_coagent. Message policy allows reports to Super; executability of
		// those packets is sender-authorized at Super, not granted by packet.kind.
		return Policy{Profile: CoSuper, AllowedMessageTargets: []string{Super}}
	case Super:
		return Policy{
			Profile:                   Super,
			AllowReadOnlyFiles:        true,
			AllowResearchTools:        true,
			AllowEvidenceTools:        true,
			AllowMemoryTools:          true,
			AllowModelDiagnosticTools: true,
			AllowCoAgentTools:         true,
			AllowedSpawnTargets:       []string{Researcher},
			AllowedMessageTargets:     []string{Texture, Researcher},
		}
	default:
		return Policy{Profile: strings.TrimSpace(profile)}
	}
}

// UnknownProfileError reports a profile token outside the frozen V1 acceptor
// tables. Landing step 4 (behaviorally inert): the rejected token is still
// returned so existing callers behave exactly as before; every call site
// takes the tuple and the writer cutover (step 6) turns the report into
// fail-closed refusal. A non-empty unknown token with a checked error is a
// writer-purity failure.
type UnknownProfileError struct {
	Input string
}

func (e UnknownProfileError) Error() string {
	return fmt.Sprintf("agentprofile: unknown profile %q", e.Input)
}

// Canonical normalizes a profile name and its accepted aliases, reporting
// whether the token is a known V1 profile. Known aliases behave exactly as
// before; unknown tokens return the normalized input with a typed error.
func Canonical(profile string) (string, error) {
	profile = strings.TrimSpace(profile)
	normalized := strings.ToLower(strings.ReplaceAll(profile, "_", "-"))
	switch normalized {
	case "researcher", "researchers", "research", "research-agent", "web-research", "web-researcher":
		return Researcher, nil
	case "cosuper", "co-super", "coagent", "co-agent":
		return CoSuper, nil
	case "texture", "texture-agent", "document-agent":
		return Texture, nil
	case "processor", "news-processor", "source-processor", "universal-wire-processor":
		return Processor, nil
	case "reconciler", "news-reconciler", "story-reconciler", "corpus-reconciler", "universal-wire-reconciler":
		return Reconciler, nil
	case "email", "email-agent", "email-appagent", "mail", "mail-agent":
		return Email, nil
	case Super:
		return Super, nil
	case Conductor:
		return Conductor, nil
	default:
		return normalized, UnknownProfileError{Input: profile}
	}
}

// IsTexture reports whether profile resolves to the Texture profile.
func IsTexture(profile string) bool {
	canonical, _ := Canonical(profile)
	return canonical == Texture
}

// CanSpawn reports whether callerProfile may spawn targetProfile.
func CanSpawn(callerProfile, targetProfile string) bool {
	policy := PolicyFor(callerProfile)
	target, _ := Canonical(targetProfile)
	targetProfile = target
	for _, allowed := range policy.AllowedSpawnTargets {
		canonicalAllowed, _ := Canonical(allowed)
		if targetProfile == canonicalAllowed {
			return true
		}
	}
	return false
}

// CanMessage reports whether callerProfile may address targetProfile.
func CanMessage(callerProfile, targetProfile string) bool {
	policy := PolicyFor(callerProfile)
	target, _ := Canonical(targetProfile)
	targetProfile = target
	for _, allowed := range policy.AllowedMessageTargets {
		canonicalAllowed, _ := Canonical(allowed)
		if targetProfile == canonicalAllowed {
			return true
		}
	}
	return false
}
