package computerversion

import (
	"context"
	"fmt"
	"strings"
)

// ProjectionMaterializer turns an already-extracted ObservationSet into a
// Realization under a declared CapabilityManifest. It is a non-runtime
// projection materializer: useful for proving materializer/equivalence contracts
// without launching a VM or touching product state.
type ProjectionMaterializer struct {
	ID           string
	Observations ObservationSet
}

var _ Materializer = ProjectionMaterializer{}

// Materialize returns a Realization for version when the stored observations
// match that version and manifest supports the observations' required kinds.
func (m ProjectionMaterializer) Materialize(ctx context.Context, version ComputerVersion, manifest CapabilityManifest) (Realization, error) {
	if err := ctx.Err(); err != nil {
		return Realization{}, err
	}
	if !version.Valid() {
		return Realization{}, fmt.Errorf("projection materializer: invalid computer version")
	}
	if m.Observations.Version != version {
		return Realization{}, fmt.Errorf("projection materializer: observation version %s does not match requested version %s", formatVersion(m.Observations.Version), formatVersion(version))
	}
	if strings.TrimSpace(manifest.Materializer) == "" {
		return Realization{}, fmt.Errorf("projection materializer: manifest must name materializer")
	}
	if strings.TrimSpace(manifest.Substrate) == "" {
		return Realization{}, fmt.Errorf("projection materializer: manifest must name substrate")
	}
	if missing := manifest.MissingRequired(m.Observations.RequiredKinds()); len(missing) > 0 {
		return Realization{}, fmt.Errorf("projection materializer: manifest %q lacks required capability %q", manifest.Materializer, missing[0].Kind)
	}
	id := m.ID
	if strings.TrimSpace(id) == "" {
		id = manifest.Materializer
	}
	return Realization{
		ID:           id,
		Version:      version,
		Capabilities: manifest,
		Observations: m.Observations,
	}, nil
}
