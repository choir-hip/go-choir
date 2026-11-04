// Package agentprofile defines canonical agent profile identifiers,
// normalization, capabilities, and message policy.
package agentprofile

import (
	"fmt"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/deskmanifest"
)

const (
	Conductor   = "conductor"
	Management  = "management"
	Engineering = "engineering"
	Research    = "research"
	Texture     = "texture"
	Email       = "email"
	// ManagementTask is the run role of a management task (management
	// delegation S2a, design round 2 R2-1): a bounded sub-RLM of the
	// management desk with its own run, session and inbox
	// (management-task:<id>). Its run profile stays Management; the role
	// selects its narrow verb set, prompt and model. It is a role, not a
	// desk anyone addresses: IsLive is false, so no Cast, Note or message
	// names it; management reaches a task only through choir.Delegate.
	ManagementTask = "management_task"
)

// Policy is the canonical policy for an agent profile. Message routing is
// not here: any desk may send a request to any registered desk, and the
// agentcore address resolver says where it lands (modular architecture
// MA1.2, R5).
type Policy struct {
	Profile string
}

// PolicyFor returns the policy for profile.
// It fails closed on unknown profiles: no default policy is synthesized.
func PolicyFor(profile string) (Policy, error) {
	canonical, err := Canonical(profile)
	if err != nil {
		return Policy{}, err
	}
	// The verifier roles and any other canonical profile the manifest does not
	// name get a bare policy; the processor and reconciler profiles are
	// frozen history and have none.
	switch canonical {
	case "processor", "reconciler":
		return Policy{}, UnknownProfileError{Input: profile}
	}
	return Policy{Profile: canonical}, nil
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

// Canonical resolves the V2 live vocabulary and recognizes frozen processor
// and reconciler records so historical runs can be replayed. V1 aliases
// refuse here; history decodes through the frozen V1 decoder (computerevent
// package). The verifier roles have no agentprofile constants (modelpolicy
// owns them) and resolve as themselves.
func Canonical(profile string) (string, error) {
	normalized := strings.ToLower(strings.TrimSpace(profile))
	// The desk registry is the vocabulary (MA2.5, desks design F1): a name is
	// a profile when a registered desk says so. The frozen history names and
	// the verifier roles are not desks.
	if d, ok := deskmanifest.Lookup(normalized); ok && d.Profile {
		return normalized, nil
	}
	switch normalized {
	case "processor", "reconciler", "verifier":
		return normalized, nil
	case "verifier-multimodal", "verifier_multimodal":
		// One canonical spelling (the modelpolicy roles.<name> key): the
		// hyphenated form folds to verifier_multimodal.
		return "verifier_multimodal", nil
	default:
		return "", UnknownProfileError{Input: profile}
	}
}

// IsLive reports whether a profile may be used for new runtime work.
// Processor and reconciler remain Canonical solely so stored historical
// records can be decoded and replayed.
func IsLive(profile string) bool {
	canonical, err := Canonical(profile)
	if err != nil {
		return false
	}
	if d, ok := deskmanifest.Lookup(canonical); ok {
		// A run role (a management task) is a profile but not a desk anyone
		// addresses or starts work on.
		return d.Profile && d.Kind != deskmanifest.KindRole
	}
	return canonical == "verifier" || canonical == "verifier_multimodal"
}
