package computerversion

import (
	"fmt"
	"sort"
)

// EquivalenceStatus classifies the outcome of an equivalence check.
type EquivalenceStatus string

const (
	// EquivalenceEquivalent means the compared observation sets match under the
	// declared scope and both manifests support that scope.
	EquivalenceEquivalent EquivalenceStatus = "equivalent"
	// EquivalenceNotEquivalent means the checker found a concrete mismatch.
	EquivalenceNotEquivalent EquivalenceStatus = "not_equivalent"
	// EquivalenceNarrowed means at least one materializer cannot support the
	// declared observation scope, so the claim must be narrowed instead of passed.
	EquivalenceNarrowed EquivalenceStatus = "narrowed"
)

// Difference records one concrete mismatch between two observation sets.
type Difference struct {
	Kind   ObservationKind `json:"kind,omitempty"`
	Key    string          `json:"key,omitempty"`
	Left   string          `json:"left,omitempty"`
	Right  string          `json:"right,omitempty"`
	Reason string          `json:"reason"`
}

// EquivalenceResult is the complete claim outcome. Empty Differences and
// Unsupported are meaningful only when Status is equivalent.
type EquivalenceResult struct {
	Status      EquivalenceStatus       `json:"status"`
	Differences []Difference            `json:"differences,omitempty"`
	Unsupported []UnsupportedCapability `json:"unsupported,omitempty"`
}

// Equivalent reports whether the result authorizes an equivalence claim.
func (r EquivalenceResult) Equivalent() bool {
	return r.Status == EquivalenceEquivalent && len(r.Differences) == 0 && len(r.Unsupported) == 0
}

// EquivalenceCheck is the checker contract for substrate-independent
// observation comparison.
type EquivalenceCheck interface {
	CheckRealizations(left, right Realization) EquivalenceResult
	CheckObservationSets(left, right ObservationSet) EquivalenceResult
}

// EquivalenceChecker compares observations under declared capability scope.
// It is pure and deterministic: same inputs produce the same result.
type EquivalenceChecker struct{}

var _ EquivalenceCheck = EquivalenceChecker{}

// CheckRealizations compares two substrate realizations. The realizations must
// name the same ComputerVersion, their observation sets must also name that
// version, and both capability manifests must support every required observation
// kind. Unsupported capability narrows the claim; concrete observation mismatch
// fails it.
func (EquivalenceChecker) CheckRealizations(left, right Realization) EquivalenceResult {
	if !left.Version.Valid() {
		return notEquivalent(Difference{Reason: "left realization has invalid computer version"})
	}
	if !right.Version.Valid() {
		return notEquivalent(Difference{Reason: "right realization has invalid computer version"})
	}
	if left.Version != right.Version {
		return notEquivalent(Difference{
			Reason: "realizations name different computer versions",
			Left:   formatVersion(left.Version),
			Right:  formatVersion(right.Version),
		})
	}
	if left.Observations.Version != left.Version {
		return notEquivalent(Difference{Reason: "left observation set version does not match realization version"})
	}
	if right.Observations.Version != right.Version {
		return notEquivalent(Difference{Reason: "right observation set version does not match realization version"})
	}

	required := mergeKinds(left.Observations.RequiredKinds(), right.Observations.RequiredKinds())
	unsupported := append([]UnsupportedCapability{}, left.Capabilities.MissingRequired(required)...)
	unsupported = append(unsupported, right.Capabilities.MissingRequired(required)...)
	if len(unsupported) > 0 {
		return EquivalenceResult{Status: EquivalenceNarrowed, Unsupported: unsupported}
	}

	return EquivalenceChecker{}.CheckObservationSets(left.Observations, right.Observations)
}

// CheckObservationSets compares two observation sets without consulting
// capability manifests. Call CheckRealizations when a claim needs capability
// scoping.
func (EquivalenceChecker) CheckObservationSets(left, right ObservationSet) EquivalenceResult {
	if !left.Version.Valid() {
		return notEquivalent(Difference{Reason: "left observation set has invalid computer version"})
	}
	if !right.Version.Valid() {
		return notEquivalent(Difference{Reason: "right observation set has invalid computer version"})
	}
	if left.Version != right.Version {
		return notEquivalent(Difference{
			Reason: "observation sets name different computer versions",
			Left:   formatVersion(left.Version),
			Right:  formatVersion(right.Version),
		})
	}

	leftMap, leftErr := observationMap(left.Observations)
	if leftErr != nil {
		return notEquivalent(Difference{Reason: "invalid left observations: " + leftErr.Error()})
	}
	rightMap, rightErr := observationMap(right.Observations)
	if rightErr != nil {
		return notEquivalent(Difference{Reason: "invalid right observations: " + rightErr.Error()})
	}

	keys := make([]string, 0, len(leftMap)+len(rightMap))
	seen := make(map[string]struct{}, len(leftMap)+len(rightMap))
	for key := range leftMap {
		seen[key] = struct{}{}
		keys = append(keys, key)
	}
	for key := range rightMap {
		if _, ok := seen[key]; ok {
			continue
		}
		keys = append(keys, key)
	}
	sort.Strings(keys)

	diffs := make([]Difference, 0)
	for _, key := range keys {
		leftObservation, leftOK := leftMap[key]
		rightObservation, rightOK := rightMap[key]
		switch {
		case !leftOK:
			diffs = append(diffs, Difference{
				Kind:   rightObservation.Kind,
				Key:    rightObservation.Key,
				Right:  rightObservation.Value,
				Reason: "observation missing from left set",
			})
		case !rightOK:
			diffs = append(diffs, Difference{
				Kind:   leftObservation.Kind,
				Key:    leftObservation.Key,
				Left:   leftObservation.Value,
				Reason: "observation missing from right set",
			})
		case leftObservation.Value != rightObservation.Value:
			diffs = append(diffs, Difference{
				Kind:   leftObservation.Kind,
				Key:    leftObservation.Key,
				Left:   leftObservation.Value,
				Right:  rightObservation.Value,
				Reason: "observation values differ",
			})
		}
	}
	if len(diffs) > 0 {
		return EquivalenceResult{Status: EquivalenceNotEquivalent, Differences: diffs}
	}
	return EquivalenceResult{Status: EquivalenceEquivalent}
}

func notEquivalent(diff Difference) EquivalenceResult {
	return EquivalenceResult{Status: EquivalenceNotEquivalent, Differences: []Difference{diff}}
}

func formatVersion(version ComputerVersion) string {
	return fmt.Sprintf("%s@%s", version.CodeRef, version.ArtifactProgramRef)
}

func mergeKinds(left, right []ObservationKind) []ObservationKind {
	seen := make(map[ObservationKind]struct{}, len(left)+len(right))
	out := make([]ObservationKind, 0, len(left)+len(right))
	for _, kind := range left {
		if _, ok := seen[kind]; ok {
			continue
		}
		seen[kind] = struct{}{}
		out = append(out, kind)
	}
	for _, kind := range right {
		if _, ok := seen[kind]; ok {
			continue
		}
		seen[kind] = struct{}{}
		out = append(out, kind)
	}
	return out
}

func observationMap(observations []Observation) (map[string]Observation, error) {
	out := make(map[string]Observation, len(observations))
	for _, observation := range observations {
		if !observation.Valid() {
			return nil, fmt.Errorf("invalid observation kind=%q key=%q", observation.Kind, observation.Key)
		}
		key := observationKey(observation)
		if _, exists := out[key]; exists {
			return nil, fmt.Errorf("duplicate observation kind=%q key=%q", observation.Kind, observation.Key)
		}
		out[key] = observation
	}
	return out, nil
}

func observationKey(observation Observation) string {
	return string(observation.Kind) + "\x00" + observation.Key
}
