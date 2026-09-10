// Package agentprofile defines canonical agent profile identifiers,
// normalization, capabilities, and spawn/message policy.
package agentprofile

import (
	"fmt"
	"strings"
)

const (
	Conductor  = "conductor"
	Super      = "management"
	CoSuper    = "engineering"
	Researcher = "research"
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
// It fails closed on unknown profiles: no default policy is synthesized.
func PolicyFor(profile string) (Policy, error) {
	canonical, err := Canonical(profile)
	if err != nil {
		return Policy{}, err
	}
	switch canonical {
	case Conductor:
		return Policy{
			Profile:             Conductor,
			AllowCoAgentTools:   true,
			AllowedSpawnTargets: []string{Texture},
		}, nil
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
		}, nil
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
		}, nil
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
		}, nil
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
		}, nil
	case Email:
		return Policy{Profile: Email}, nil
	case CoSuper:
		// CoSuper has no static tool authority. The assignment runtime constructs a
		// fresh per-run registry from the exact capsule-local closed set plus
		// update_coagent. Message policy allows reports to Super; executability of
		// those packets is sender-authorized at Super, not granted by packet.kind.
		return Policy{Profile: CoSuper, AllowedMessageTargets: []string{Super}}, nil
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
		}, nil
	default:
		return Policy{Profile: strings.TrimSpace(profile)}, nil
	}
}

// UnknownProfileError reports a profile token outside the V2 live vocabulary.
// Fail-closed: unknown live values return the empty string with this typed
// error, never the input token. Callers that proceed only on a non-empty
// canonical fail closed automatically; bare `!=` comparisons must check the
// error explicitly (a non-empty unknown token with a checked error is a
// writer-purity failure).
type UnknownProfileError struct {
	Input string
}

func (e UnknownProfileError) Error() string {
	return fmt.Sprintf("agentprofile: unknown profile %q", e.Input)
}

// Canonical resolves exactly the V2 live vocabulary (mapping §2 identity
// map, zero alias branches, fail-closed default). V1 aliases refuse here;
// history decodes through the frozen V1 decoder (computerevent package),
// never through this function. The verifier roles have no agentprofile
// constants (modelpolicy owns them) and resolve as themselves.
func Canonical(profile string) (string, error) {
	normalized := strings.ToLower(strings.TrimSpace(profile))
	switch normalized {
	case Super, CoSuper, Researcher, Texture, Conductor, Processor, Reconciler, Email,
		"verifier", "verifier-multimodal":
		return normalized, nil
	default:
		return "", UnknownProfileError{Input: profile}
	}
}

// IsTexture reports whether profile resolves to the Texture profile.
func IsTexture(profile string) bool {
	canonical, _ := Canonical(profile)
	return canonical == Texture
}

// CanSpawn reports whether callerProfile may spawn targetProfile, failing
// closed on unknown profiles on either side.
func CanSpawn(callerProfile, targetProfile string) (bool, error) {
	policy, err := PolicyFor(callerProfile)
	if err != nil {
		return false, err
	}
	target, err := Canonical(targetProfile)
	if err != nil {
		return false, err
	}
	for _, allowed := range policy.AllowedSpawnTargets {
		canonicalAllowed, err := Canonical(allowed)
		if err != nil {
			return false, err
		}
		if target == canonicalAllowed {
			return true, nil
		}
	}
	return false, nil
}

// CanMessage reports whether callerProfile may address targetProfile,
// failing closed on unknown profiles on either side.
func CanMessage(callerProfile, targetProfile string) (bool, error) {
	policy, err := PolicyFor(callerProfile)
	if err != nil {
		return false, err
	}
	target, err := Canonical(targetProfile)
	if err != nil {
		return false, err
	}
	for _, allowed := range policy.AllowedMessageTargets {
		canonicalAllowed, err := Canonical(allowed)
		if err != nil {
			return false, err
		}
		if target == canonicalAllowed {
			return true, nil
		}
	}
	return false, nil
}
