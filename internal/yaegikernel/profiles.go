package yaegikernel

import (
	"fmt"
	"strings"
	"time"
)

const (
	ProfileCoSuper    = "cosuper"
	ProfileResearcher = "researcher"
)

// ProfileDefinition defines the capabilities, package allowances, and permissions
// for an actor activation profile.
type ProfileDefinition struct {
	Name            string         `json:"name"`
	AllowedActions  []BrokerAction `json:"allowed_actions"`
	AllowedPackages []string       `json:"allowed_packages"`
	EffectsEnabled  bool           `json:"effects_enabled"`
	MaxExecutionTTL time.Duration  `json:"max_execution_ttl"`
}

// ProfileRegistry maintains the authoritative profile configurations.
type ProfileRegistry struct {
	profiles map[string]ProfileDefinition
}

// NewDefaultProfileRegistry creates a registry with CoSuper and Researcher profiles.
func NewDefaultProfileRegistry() *ProfileRegistry {
	r := &ProfileRegistry{
		profiles: make(map[string]ProfileDefinition),
	}

	// 1. CoSuper Profile: effects-capable authoring, Bash + Go execution, but external effects stay OFF
	r.profiles[ProfileCoSuper] = ProfileDefinition{
		Name: ProfileCoSuper,
		AllowedActions: []BrokerAction{
			ActionExec,
			ActionReadFile,
			ActionWriteFile,
			ActionAssign,
			ActionMessage,
		},
		AllowedPackages: []string{
			"fmt", "strings", "bytes", "time", "math", "math/big", "math/rand/v2",
			"sort", "strconv", "unicode", "unicode/utf8", "encoding/json",
			"encoding/base64", "encoding/hex", "errors", "regexp",
		},
		EffectsEnabled:  false, // Effects stay OFF: no Accept, Materialize, Checkpoint, Route
		MaxExecutionTTL: 10 * time.Minute,
	}

	// 2. Restricted Researcher Profile: read-only, no Bash, no file writes, narrow modules
	r.profiles[ProfileResearcher] = ProfileDefinition{
		Name: ProfileResearcher,
		AllowedActions: []BrokerAction{
			ActionReadFile,
			ActionMessage,
		},
		AllowedPackages: []string{
			"fmt", "strings", "bytes", "time", "math", "math/big", "math/rand/v2",
			"sort", "strconv", "encoding/json", "encoding/base64", "errors",
		},
		EffectsEnabled:  false,
		MaxExecutionTTL: 5 * time.Minute,
	}

	return r
}

// GetProfile retrieves a profile definition by name.
func (r *ProfileRegistry) GetProfile(name string) (ProfileDefinition, error) {
	name = strings.TrimSpace(strings.ToLower(name))
	if r == nil || r.profiles == nil {
		return ProfileDefinition{}, fmt.Errorf("profile registry: uninitialized")
	}
	p, ok := r.profiles[name]
	if !ok {
		return ProfileDefinition{}, fmt.Errorf("profile registry: unknown profile %q", name)
	}
	return p, nil
}

// AuthorizeAction checks whether the named profile is permitted to execute the requested action.
func (r *ProfileRegistry) AuthorizeAction(profileName string, action BrokerAction) error {
	p, err := r.GetProfile(profileName)
	if err != nil {
		return err
	}
	for _, a := range p.AllowedActions {
		if a == action {
			return nil
		}
	}
	return fmt.Errorf("profile %q is not authorized to perform action %q", profileName, action)
}
