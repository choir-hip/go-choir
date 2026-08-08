// Package agentprofile defines canonical agent profile identifiers,
// normalization, capabilities, and spawn/message policy.
package agentprofile

import "strings"

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
	switch Canonical(profile) {
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
		// retrieval so it can recover its own compacted context.
		return Policy{
			Profile:               Texture,
			AllowMemoryTools:      true,
			AllowCoAgentTools:     true,
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
		// CoSuper is a delegated assignment role. Its static affordances are
		// intentionally limited to inspection, evidence, model diagnostics, and
		// reporting results. Capsule-local tools require a separate capability-
		// bound registry and are not implied by this host profile policy.
		return Policy{
			Profile:                   CoSuper,
			AllowReadOnlyFiles:        true,
			AllowEvidenceTools:        true,
			AllowModelDiagnosticTools: true,
			AllowedMessageTargets:     []string{Super, Texture},
		}
	case Super:
		return Policy{
			Profile:                   Super,
			AllowReadOnlyFiles:        true,
			AllowResearchTools:        true,
			AllowEvidenceTools:        true,
			AllowMemoryTools:          true,
			AllowModelDiagnosticTools: true,
			AllowCoAgentTools:         true,
			AllowedSpawnTargets:       []string{Researcher, CoSuper},
			AllowedMessageTargets:     []string{Texture, Researcher, CoSuper},
		}
	default:
		return Policy{Profile: strings.TrimSpace(profile)}
	}
}

// Canonical normalizes a profile name and its accepted aliases.
func Canonical(profile string) string {
	profile = strings.TrimSpace(profile)
	normalized := strings.ToLower(strings.ReplaceAll(profile, "_", "-"))
	switch normalized {
	case "researcher", "researchers", "research", "research-agent", "web-research", "web-researcher":
		return Researcher
	case "cosuper", "co-super", "coagent", "co-agent":
		return CoSuper
	case "texture", "texture-agent", "document-agent":
		return Texture
	case "processor", "news-processor", "source-processor", "universal-wire-processor":
		return Processor
	case "reconciler", "news-reconciler", "story-reconciler", "corpus-reconciler", "universal-wire-reconciler":
		return Reconciler
	case "email", "email-agent", "email-appagent", "mail", "mail-agent":
		return Email
	case Super:
		return Super
	case Conductor:
		return Conductor
	default:
		return normalized
	}
}

// IsTexture reports whether profile resolves to the Texture profile.
func IsTexture(profile string) bool {
	return Canonical(profile) == Texture
}

// CanSpawn reports whether callerProfile may spawn targetProfile.
func CanSpawn(callerProfile, targetProfile string) bool {
	policy := PolicyFor(callerProfile)
	targetProfile = Canonical(targetProfile)
	for _, allowed := range policy.AllowedSpawnTargets {
		if targetProfile == Canonical(allowed) {
			return true
		}
	}
	return false
}

// CanMessage reports whether callerProfile may address targetProfile.
func CanMessage(callerProfile, targetProfile string) bool {
	policy := PolicyFor(callerProfile)
	targetProfile = Canonical(targetProfile)
	for _, allowed := range policy.AllowedMessageTargets {
		if targetProfile == Canonical(allowed) {
			return true
		}
	}
	return false
}
