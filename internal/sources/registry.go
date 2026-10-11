package sources

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"strings"
)

// registryJSON is the source list Autopaper polls (formerly
// configs/sources.json). It is embedded so a guest computer carries it.
//
//go:embed registry.json
var registryJSON []byte

// DefaultRegistry returns the embedded source registry.
func DefaultRegistry() (Registry, error) {
	var reg Registry
	if err := json.Unmarshal(registryJSON, &reg); err != nil {
		return Registry{}, fmt.Errorf("decode embedded source registry: %w", err)
	}
	return reg, nil
}

// Lookup returns the source with the given id.
func (r Registry) Lookup(id string) (Source, bool) {
	id = strings.TrimSpace(id)
	for _, source := range r.Sources {
		if source.ID == id {
			return source, true
		}
	}
	return Source{}, false
}
