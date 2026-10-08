package projectionbase

import (
	"context"
	"fmt"

	"github.com/yusefmosiah/go-choir/internal/computerversion"
	"github.com/yusefmosiah/go-choir/internal/selfdevprotocol"
)

// witnessForWorkspace extracts the VM-local content witness for a store
// workspace using the production extraction configuration. The witness ignores
// operational write timestamps so a seeded base and a genesis replay of the
// same chain agree.
func witnessForWorkspace(ctx context.Context, computerID, canonicalHead, workspacePath string) (selfdevprotocol.VMLocalContentWitness, error) {
	version := computerversion.ComputerVersion{
		CodeRef:            computerversion.CodeRef("runtime:" + computerID),
		ArtifactProgramRef: computerversion.ArtifactProgramRef("event-chain:" + canonicalHead),
	}
	extractor := computerversion.DoltStateExtractor{
		WorkspacePath: workspacePath,
		Database:      "texture",
		IgnoredContentColumns: map[string]map[string]struct{}{
			"computer_event_index": {
				"prepared_at":  {},
				"finalized_at": {},
			},
			"computer_event_projection_heads": {
				"updated_at": {},
			},
		},
	}
	extracted, err := extractor.Extract(ctx, computerversion.ExtractRequest{
		Name:    "projection-base-extraction",
		Version: version,
	})
	if err != nil {
		return selfdevprotocol.VMLocalContentWitness{}, fmt.Errorf("projection base: extract content witness: %w", err)
	}
	witness, err := selfdevprotocol.WitnessFromObservationSets(extracted, extracted, computerversion.EquivalenceChecker{}.CheckObservationSets(extracted, extracted))
	if err != nil {
		return selfdevprotocol.VMLocalContentWitness{}, fmt.Errorf("projection base: compute content witness: %w", err)
	}
	return witness, nil
}
