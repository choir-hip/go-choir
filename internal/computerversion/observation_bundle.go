package computerversion

import (
	"fmt"
	"sort"
	"strings"
)

// CombineObservationSets builds one fixture-level evidence package from multiple
// already-derived ObservationSets for the same ComputerVersion. It is a bundling
// boundary only: it does not widen the semantics of any member observation kind
// or turn vm_state_manifest/promotion_certificate evidence into durable
// file/blob/user-state equivalence.
func CombineObservationSets(name string, version ComputerVersion, sets ...ObservationSet) (ObservationSet, error) {
	if !version.Valid() {
		return ObservationSet{}, fmt.Errorf("observation bundle: invalid computer version")
	}
	if len(sets) == 0 {
		return ObservationSet{}, fmt.Errorf("observation bundle: at least one observation set is required")
	}
	name = strings.TrimSpace(name)
	if name == "" {
		name = "combined-observation-set"
	}

	required := make([]ObservationKind, 0)
	observationsByKey := make(map[string]Observation)
	observations := make([]Observation, 0)
	for i, set := range sets {
		if set.Version != version {
			return ObservationSet{}, fmt.Errorf("observation bundle: set %d names different computer version", i)
		}
		required = mergeKinds(required, set.RequiredKinds())
		setMap, err := observationMap(set.Observations)
		if err != nil {
			return ObservationSet{}, fmt.Errorf("observation bundle: set %d has invalid observations: %w", i, err)
		}
		keys := make([]string, 0, len(setMap))
		for key := range setMap {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			observation := setMap[key]
			if existing, ok := observationsByKey[key]; ok {
				if existing.Value != observation.Value {
					return ObservationSet{}, fmt.Errorf("observation bundle: conflicting observation kind=%q key=%q", observation.Kind, observation.Key)
				}
				continue
			}
			observationsByKey[key] = observation
			observations = append(observations, observation)
		}
	}

	sort.Slice(observations, func(i, j int) bool {
		if observations[i].Kind != observations[j].Kind {
			return observations[i].Kind < observations[j].Kind
		}
		return observations[i].Key < observations[j].Key
	})
	sort.Slice(required, func(i, j int) bool { return required[i] < required[j] })

	return ObservationSet{
		Name:         name,
		Version:      version,
		Required:     required,
		Observations: observations,
	}, nil
}
