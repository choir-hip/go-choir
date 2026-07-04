package computerversion

import (
	"context"
	"fmt"
	"strings"
)

// ProductFixtureRoot names the smallest non-production product-shaped root this
// mission can observe without touching deployed services or VM lifecycle code.
// It deliberately carries explicit paths and refs; callers must provision the
// fixture out of band and this type only reads/serializes evidence.
type ProductFixtureRoot struct {
	Version     ComputerVersion       `json:"version"`
	Base        BaseCurrentStatePaths `json:"base"`
	VM          VMManagerScopedPath   `json:"vm"`
	Promotion   PromotionCertificate  `json:"promotion"`
	ObjectGraph *ObjectGraphSnapshot  `json:"object_graph,omitempty"`
	DoltHead    *DoltHeadSnapshot     `json:"dolt_head,omitempty"`
}

// ObservationSet opens the Base fixture root read-only, serializes the scoped
// vmmanager manifest, serializes the local promotion certificate, optionally
// serializes local typed objectgraph and Dolt head snapshots, and bundles all
// evidence under one ComputerVersion. The result remains fixture evidence: it
// does not prove live production state, route mutation, rollback execution,
// corpusd/platform Dolt state, or VM lifecycle behavior.
func (r ProductFixtureRoot) ObservationSet(ctx context.Context, name string) (ObservationSet, error) {
	if err := ctx.Err(); err != nil {
		return ObservationSet{}, err
	}
	if !r.Version.Valid() {
		return ObservationSet{}, fmt.Errorf("product fixture root: invalid computer version")
	}
	if strings.TrimSpace(name) == "" {
		name = "product-fixture-root"
	}
	if r.Promotion.Candidate != r.Version {
		return ObservationSet{}, fmt.Errorf("product fixture root: promotion candidate does not match fixture version")
	}

	baseSource, err := OpenBaseCurrentStateSource(r.Base)
	if err != nil {
		return ObservationSet{}, err
	}
	defer baseSource.Close()

	baseSet, err := baseSource.ObservationSet(ctx, name+":base", r.Version)
	if err != nil {
		return ObservationSet{}, err
	}
	vmSet, err := r.VM.ObservationSet(name+":vm", r.Version)
	if err != nil {
		return ObservationSet{}, err
	}
	promotionSet, err := r.Promotion.ObservationSet(name + ":promotion")
	if err != nil {
		return ObservationSet{}, err
	}
	return r.combineObservationSets(name, r.Version, baseSet, vmSet, promotionSet)
}

func (r ProductFixtureRoot) combineObservationSets(name string, version ComputerVersion, sets ...ObservationSet) (ObservationSet, error) {
	if r.ObjectGraph != nil {
		objectGraphSet, err := r.ObjectGraph.ObservationSet(name+":objectgraph", version)
		if err != nil {
			return ObservationSet{}, err
		}
		sets = append(sets, objectGraphSet)
	}
	if r.DoltHead != nil {
		doltHeadSet, err := r.DoltHead.ObservationSet(name+":dolt", version)
		if err != nil {
			return ObservationSet{}, err
		}
		sets = append(sets, doltHeadSet)
	}
	return CombineObservationSets(name, version, sets...)
}
