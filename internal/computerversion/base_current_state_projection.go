package computerversion

import "context"

const (
	// BaseCurrentStateReaderMaterializer names the realization produced from the
	// Base SQLite journal/blob reader path.
	BaseCurrentStateReaderMaterializer = "base-current-state-reader"
	// BaseCurrentStateReaderSubstrate names the existing persistence substrate for
	// the narrow Base current-state observation slice.
	BaseCurrentStateReaderSubstrate = "base-sqlite-journal-blob"
	// BaseFileProjectionMaterializer names the non-Firecracker projection path used
	// to compare the same scoped Base observations without claiming VM equivalence.
	BaseFileProjectionMaterializer = "base-file-projection"
	// BaseFileProjectionSubstrate names the non-runtime projection substrate for
	// file-manifest/blob-set comparisons.
	BaseFileProjectionSubstrate = "non-firecracker-file-projection"
)

// CompareBaseCurrentStateToFileProjection materializes the extracted Base
// current-state observations and a non-Firecracker file projection under narrow
// file-manifest/blob-set capability manifests, then compares the realizations.
func CompareBaseCurrentStateToFileProjection(ctx context.Context, current, projection ObservationSet) (EquivalenceResult, error) {
	currentRealization, err := (ProjectionMaterializer{ID: BaseCurrentStateReaderMaterializer, Observations: current}).Materialize(
		ctx,
		current.Version,
		BaseCurrentStateCapabilityManifest(BaseCurrentStateReaderMaterializer, BaseCurrentStateReaderSubstrate),
	)
	if err != nil {
		return EquivalenceResult{}, err
	}
	projectionRealization, err := (ProjectionMaterializer{ID: BaseFileProjectionMaterializer, Observations: projection}).Materialize(
		ctx,
		current.Version,
		BaseCurrentStateCapabilityManifest(BaseFileProjectionMaterializer, BaseFileProjectionSubstrate),
	)
	if err != nil {
		return EquivalenceResult{}, err
	}
	return EquivalenceChecker{}.CheckRealizations(currentRealization, projectionRealization), nil
}
