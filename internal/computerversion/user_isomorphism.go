package computerversion

import "fmt"

// UserSemantic names one user-visible durable semantic that an isomorphism
// claim may cover. These are scoped claims, not a full-computer guarantee.
type UserSemantic string

const (
	UserSemanticFilePath              UserSemantic = "file_path"
	UserSemanticFileContent           UserSemantic = "file_content"
	UserSemanticDeletionState         UserSemantic = "deletion_state"
	UserSemanticFileProvenance        UserSemantic = "file_provenance"
	UserSemanticLiveProcessContinuity UserSemantic = "live_process_continuity"
)

// Valid reports whether the semantic is known by this package.
func (s UserSemantic) Valid() bool {
	switch s {
	case UserSemanticFilePath,
		UserSemanticFileContent,
		UserSemanticDeletionState,
		UserSemanticFileProvenance,
		UserSemanticLiveProcessContinuity:
		return true
	default:
		return false
	}
}

// UnsupportedUserSemantic records a semantic the current proof cannot claim.
type UnsupportedUserSemantic struct {
	Semantic UserSemantic `json:"semantic"`
	Reason   string       `json:"reason"`
}

// UserIsomorphismScope declares exactly which user-visible semantics a proof is
// allowed to claim for a set of observation kinds.
type UserIsomorphismScope struct {
	Name                 string                    `json:"name"`
	ObservationKinds     []ObservationKind         `json:"observation_kinds"`
	RequiredSemantics    []UserSemantic            `json:"required_semantics"`
	CoveredSemantics     []UserSemantic            `json:"covered_semantics"`
	UnsupportedSemantics []UnsupportedUserSemantic `json:"unsupported_semantics,omitempty"`
}

// UserIsomorphismStatus classifies a scoped user-isomorphism proof.
type UserIsomorphismStatus string

const (
	UserIsomorphismEquivalent    UserIsomorphismStatus = "user_isomorphic"
	UserIsomorphismNotEquivalent UserIsomorphismStatus = "not_user_isomorphic"
	UserIsomorphismNarrowed      UserIsomorphismStatus = "narrowed"
)

// UserIsomorphismResult records the scoped user-isomorphism outcome.
type UserIsomorphismResult struct {
	Status      UserIsomorphismStatus     `json:"status"`
	Differences []Difference              `json:"differences,omitempty"`
	Unsupported []UnsupportedUserSemantic `json:"unsupported,omitempty"`
}

// UserIsomorphic reports whether the scoped user-isomorphism claim passed.
func (r UserIsomorphismResult) UserIsomorphic() bool {
	return r.Status == UserIsomorphismEquivalent && len(r.Differences) == 0 && len(r.Unsupported) == 0
}

// UserIsomorphismChecker proves scoped user-visible equivalence on top of the
// lower-level observation equivalence checker.
type UserIsomorphismChecker struct {
	Equivalence EquivalenceCheck
}

// CheckRealizations checks observation equivalence and then verifies that every
// requested user semantic is explicitly covered by the declared scope. Unsupported
// or unclaimed semantics narrow the claim instead of passing.
func (c UserIsomorphismChecker) CheckRealizations(left, right Realization, scope UserIsomorphismScope) UserIsomorphismResult {
	if err := validateUserIsomorphismScope(scope); err != nil {
		return UserIsomorphismResult{Status: UserIsomorphismNarrowed, Unsupported: []UnsupportedUserSemantic{{Reason: err.Error()}}}
	}
	for _, unsupported := range scope.UnsupportedSemantics {
		for _, required := range scope.RequiredSemantics {
			if unsupported.Semantic == required {
				return UserIsomorphismResult{Status: UserIsomorphismNarrowed, Unsupported: []UnsupportedUserSemantic{unsupported}}
			}
		}
	}
	covered := make(map[UserSemantic]struct{}, len(scope.CoveredSemantics))
	for _, semantic := range scope.CoveredSemantics {
		covered[semantic] = struct{}{}
	}
	for _, required := range scope.RequiredSemantics {
		if _, ok := covered[required]; !ok {
			return UserIsomorphismResult{Status: UserIsomorphismNarrowed, Unsupported: []UnsupportedUserSemantic{{Semantic: required, Reason: "semantic not covered by scope"}}}
		}
	}
	if missing := missingObservationKinds(scope.ObservationKinds, left.Observations.RequiredKinds(), right.Observations.RequiredKinds()); len(missing) > 0 {
		return UserIsomorphismResult{Status: UserIsomorphismNarrowed, Unsupported: []UnsupportedUserSemantic{{Reason: fmt.Sprintf("observation kind %q not present in both realizations", missing[0])}}}
	}

	checker := c.Equivalence
	if checker == nil {
		checker = EquivalenceChecker{}
	}
	equivalence := checker.CheckRealizations(left, right)
	switch equivalence.Status {
	case EquivalenceEquivalent:
		return UserIsomorphismResult{Status: UserIsomorphismEquivalent}
	case EquivalenceNarrowed:
		unsupported := make([]UnsupportedUserSemantic, 0, len(equivalence.Unsupported))
		for _, capability := range equivalence.Unsupported {
			unsupported = append(unsupported, UnsupportedUserSemantic{Reason: fmt.Sprintf("observation capability %q unsupported: %s", capability.Kind, capability.Reason)})
		}
		return UserIsomorphismResult{Status: UserIsomorphismNarrowed, Unsupported: unsupported}
	default:
		return UserIsomorphismResult{Status: UserIsomorphismNotEquivalent, Differences: equivalence.Differences}
	}
}

func validateUserIsomorphismScope(scope UserIsomorphismScope) error {
	if len(scope.ObservationKinds) == 0 {
		return fmt.Errorf("user isomorphism scope has no observation kinds")
	}
	if len(scope.RequiredSemantics) == 0 {
		return fmt.Errorf("user isomorphism scope has no required semantics")
	}
	for _, kind := range scope.ObservationKinds {
		if !kind.Valid() {
			return fmt.Errorf("invalid observation kind %q", kind)
		}
	}
	for _, semantic := range scope.RequiredSemantics {
		if !semantic.Valid() {
			return fmt.Errorf("invalid required semantic %q", semantic)
		}
	}
	for _, semantic := range scope.CoveredSemantics {
		if !semantic.Valid() {
			return fmt.Errorf("invalid covered semantic %q", semantic)
		}
	}
	for _, unsupported := range scope.UnsupportedSemantics {
		if !unsupported.Semantic.Valid() {
			return fmt.Errorf("invalid unsupported semantic %q", unsupported.Semantic)
		}
	}
	return nil
}

func missingObservationKinds(required, left, right []ObservationKind) []ObservationKind {
	leftSet := make(map[ObservationKind]struct{}, len(left))
	for _, kind := range left {
		leftSet[kind] = struct{}{}
	}
	rightSet := make(map[ObservationKind]struct{}, len(right))
	for _, kind := range right {
		rightSet[kind] = struct{}{}
	}
	missing := make([]ObservationKind, 0)
	for _, kind := range required {
		if _, ok := leftSet[kind]; !ok {
			missing = append(missing, kind)
			continue
		}
		if _, ok := rightSet[kind]; !ok {
			missing = append(missing, kind)
		}
	}
	return missing
}
