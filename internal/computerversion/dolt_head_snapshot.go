package computerversion

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
)

// DoltHeadSnapshot records one local non-production Dolt repository/database head
// that was produced out of band. It is a fixture observation: it does not open a
// production Dolt server, query corpusd, or prove live platform objectgraph state.
type DoltHeadSnapshot struct {
	RepoRoot           string               `json:"repo_root"`
	Database           string               `json:"database"`
	CommitHash         string               `json:"commit_hash"`
	ObjectGraph        *ObjectGraphSnapshot `json:"object_graph,omitempty"`
	Derivation         string               `json:"derivation,omitempty"`
	ContainsProduction bool                 `json:"contains_production"`
}

type doltHeadPayload struct {
	Database        string `json:"database"`
	CommitHash      string `json:"commit_hash"`
	ObjectGraphHead string `json:"object_graph_head,omitempty"`
	ObjectCount     int    `json:"object_count,omitempty"`
	EdgeCount       int    `json:"edge_count,omitempty"`
	Derivation      string `json:"derivation,omitempty"`
}

// ObservationSet emits one dolt_head observation for a declared local fixture
// repo head. Equality over this observation means the declared commit head and
// optional objectgraph head match; it does not imply live Dolt/corpusd parity.
func (s DoltHeadSnapshot) ObservationSet(name string, version ComputerVersion) (ObservationSet, error) {
	if !version.Valid() {
		return ObservationSet{}, fmt.Errorf("dolt head snapshot: invalid computer version")
	}
	value, err := s.CanonicalHead()
	if err != nil {
		return ObservationSet{}, err
	}
	if strings.TrimSpace(name) == "" {
		name = "dolt-head-snapshot"
	}
	database := strings.TrimSpace(s.Database)
	return ObservationSet{
		Name:     name,
		Version:  version,
		Required: []ObservationKind{ObservationDoltHead},
		Observations: []Observation{{
			Kind:  ObservationDoltHead,
			Key:   "dolt:" + database + ":head",
			Value: value,
		}},
	}, nil
}

func (s DoltHeadSnapshot) Validate() error {
	_, err := s.CanonicalHead()
	return err
}

func (s DoltHeadSnapshot) CanonicalHead() (string, error) {
	repoRoot := strings.TrimSpace(s.RepoRoot)
	if repoRoot == "" {
		return "", fmt.Errorf("dolt head snapshot: repo root is required")
	}
	if _, err := filepath.Abs(repoRoot); err != nil {
		return "", fmt.Errorf("dolt head snapshot: repo root: %w", err)
	}
	database := strings.TrimSpace(s.Database)
	if database == "" {
		return "", fmt.Errorf("dolt head snapshot: database is required")
	}
	commitHash := strings.TrimSpace(s.CommitHash)
	if commitHash == "" {
		return "", fmt.Errorf("dolt head snapshot: commit hash is required")
	}
	if s.ContainsProduction {
		return "", fmt.Errorf("dolt head snapshot: production state is not admissible")
	}
	payload := doltHeadPayload{Database: database, CommitHash: commitHash, Derivation: strings.TrimSpace(s.Derivation)}
	if s.ObjectGraph != nil {
		objectGraphHead, err := s.ObjectGraph.CanonicalHead()
		if err != nil {
			return "", fmt.Errorf("dolt head snapshot: object graph: %w", err)
		}
		var decoded struct {
			Head        string `json:"head"`
			ObjectCount int    `json:"object_count"`
			EdgeCount   int    `json:"edge_count"`
		}
		if err := json.Unmarshal([]byte(objectGraphHead), &decoded); err != nil {
			return "", fmt.Errorf("dolt head snapshot: decode object graph head: %w", err)
		}
		payload.ObjectGraphHead = decoded.Head
		payload.ObjectCount = decoded.ObjectCount
		payload.EdgeCount = decoded.EdgeCount
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("dolt head snapshot: encode head: %w", err)
	}
	return string(data), nil
}
