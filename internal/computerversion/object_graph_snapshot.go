package computerversion

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
)

// ObjectGraphSnapshot is a fixture-level typed objectgraph slice. It records
// local non-production objectgraph objects and edges in their native
// objectgraph types, then reduces them to one deterministic head observation.
// It does not read corpusd, Dolt, deployed routes, or production state.
type ObjectGraphSnapshot struct {
	Objects []objectgraph.Object `json:"objects"`
	Edges   []objectgraph.Edge   `json:"edges,omitempty"`
}

type objectGraphHeadPayload struct {
	Head        string                    `json:"head"`
	ObjectCount int                       `json:"object_count"`
	EdgeCount   int                       `json:"edge_count"`
	Objects     []objectGraphObjectDigest `json:"objects"`
	Edges       []objectGraphEdgeDigest   `json:"edges,omitempty"`
}

type objectGraphObjectDigest struct {
	CanonicalID  string                 `json:"canonical_id"`
	ObjectKind   objectgraph.ObjectKind `json:"object_kind"`
	OwnerID      string                 `json:"owner_id"`
	ComputerID   string                 `json:"computer_id,omitempty"`
	VersionID    string                 `json:"version_id,omitempty"`
	ContentHash  string                 `json:"content_hash"`
	Tombstone    bool                   `json:"tombstone,omitempty"`
	SupersededBy string                 `json:"superseded_by,omitempty"`
}

type objectGraphEdgeDigest struct {
	EdgeID    string               `json:"edge_id"`
	FromID    string               `json:"from_id"`
	ToID      string               `json:"to_id"`
	Kind      objectgraph.EdgeKind `json:"kind"`
	Tombstone bool                 `json:"tombstone,omitempty"`
}

// ObservationSet validates the typed objectgraph snapshot and emits a single
// object_graph_head observation. Passing equality means this fixture-level graph
// head matches; it does not imply live platform Dolt/corpusd state equivalence.
func (s ObjectGraphSnapshot) ObservationSet(name string, version ComputerVersion) (ObservationSet, error) {
	if !version.Valid() {
		return ObservationSet{}, fmt.Errorf("object graph snapshot: invalid computer version")
	}
	value, err := s.CanonicalHead()
	if err != nil {
		return ObservationSet{}, err
	}
	if strings.TrimSpace(name) == "" {
		name = "object-graph-snapshot"
	}
	return ObservationSet{
		Name:     name,
		Version:  version,
		Required: []ObservationKind{ObservationObjectGraphHead},
		Observations: []Observation{{
			Kind:  ObservationObjectGraphHead,
			Key:   "objectgraph:head",
			Value: value,
		}},
	}, nil
}

// Validate checks the snapshot without returning the canonical head payload.
func (s ObjectGraphSnapshot) Validate() error {
	_, err := s.CanonicalHead()
	return err
}

// CanonicalHead returns a deterministic JSON payload whose head is computed from
// objectgraph identity/content digests and edge identity digests.
func (s ObjectGraphSnapshot) CanonicalHead() (string, error) {
	objects, edges, err := s.canonicalDigests()
	if err != nil {
		return "", err
	}
	headInput, err := json.Marshal(struct {
		Objects []objectGraphObjectDigest `json:"objects"`
		Edges   []objectGraphEdgeDigest   `json:"edges,omitempty"`
	}{Objects: objects, Edges: edges})
	if err != nil {
		return "", fmt.Errorf("object graph snapshot: encode head input: %w", err)
	}
	payload := objectGraphHeadPayload{
		Head:        objectgraph.SHA256(headInput),
		ObjectCount: len(objects),
		EdgeCount:   len(edges),
		Objects:     objects,
		Edges:       edges,
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("object graph snapshot: encode head: %w", err)
	}
	return string(data), nil
}

func (s ObjectGraphSnapshot) canonicalDigests() ([]objectGraphObjectDigest, []objectGraphEdgeDigest, error) {
	if len(s.Objects) == 0 {
		return nil, nil, fmt.Errorf("object graph snapshot: at least one object is required")
	}
	objects := make([]objectGraphObjectDigest, 0, len(s.Objects))
	knownObjects := make(map[string]struct{}, len(s.Objects))
	for _, obj := range s.Objects {
		if strings.TrimSpace(obj.CanonicalID) == "" {
			return nil, nil, fmt.Errorf("object graph snapshot: object canonical_id is required")
		}
		if _, exists := knownObjects[obj.CanonicalID]; exists {
			return nil, nil, fmt.Errorf("object graph snapshot: duplicate object %s", obj.CanonicalID)
		}
		kind, ownerID, _, err := objectgraph.ParseCanonicalID(obj.CanonicalID)
		if err != nil {
			return nil, nil, fmt.Errorf("object graph snapshot: object %s: %w", obj.CanonicalID, err)
		}
		if obj.ObjectKind != kind {
			return nil, nil, fmt.Errorf("object graph snapshot: object %s kind mismatch", obj.CanonicalID)
		}
		if strings.TrimSpace(obj.OwnerID) == "" {
			return nil, nil, fmt.Errorf("object graph snapshot: object %s owner_id is required", obj.CanonicalID)
		}
		if obj.OwnerID != ownerID {
			return nil, nil, fmt.Errorf("object graph snapshot: object %s owner mismatch", obj.CanonicalID)
		}
		wantHash := objectgraph.ContentHash(obj.ObjectKind, obj.Body, obj.Metadata)
		if obj.ContentHash != wantHash {
			return nil, nil, fmt.Errorf("object graph snapshot: object %s content hash mismatch", obj.CanonicalID)
		}
		knownObjects[obj.CanonicalID] = struct{}{}
		objects = append(objects, objectGraphObjectDigest{
			CanonicalID:  obj.CanonicalID,
			ObjectKind:   obj.ObjectKind,
			OwnerID:      obj.OwnerID,
			ComputerID:   strings.TrimSpace(obj.ComputerID),
			VersionID:    strings.TrimSpace(obj.VersionID),
			ContentHash:  obj.ContentHash,
			Tombstone:    obj.Tombstone,
			SupersededBy: strings.TrimSpace(obj.SupersededBy),
		})
	}
	edges := make([]objectGraphEdgeDigest, 0, len(s.Edges))
	seenEdges := make(map[string]struct{}, len(s.Edges))
	for _, edge := range s.Edges {
		if strings.TrimSpace(edge.EdgeID) == "" {
			return nil, nil, fmt.Errorf("object graph snapshot: edge_id is required")
		}
		if _, exists := seenEdges[edge.EdgeID]; exists {
			return nil, nil, fmt.Errorf("object graph snapshot: duplicate edge %s", edge.EdgeID)
		}
		if _, ok := knownObjects[edge.FromID]; !ok {
			return nil, nil, fmt.Errorf("object graph snapshot: edge %s references missing from object %s", edge.EdgeID, edge.FromID)
		}
		if _, ok := knownObjects[edge.ToID]; !ok {
			return nil, nil, fmt.Errorf("object graph snapshot: edge %s references missing to object %s", edge.EdgeID, edge.ToID)
		}
		wantID, err := objectgraph.BuildEdgeID(edge.FromID, edge.ToID, edge.Kind, edge.Metadata)
		if err != nil {
			return nil, nil, fmt.Errorf("object graph snapshot: edge %s: %w", edge.EdgeID, err)
		}
		if edge.EdgeID != wantID {
			return nil, nil, fmt.Errorf("object graph snapshot: edge %s id mismatch", edge.EdgeID)
		}
		seenEdges[edge.EdgeID] = struct{}{}
		edges = append(edges, objectGraphEdgeDigest{
			EdgeID:    edge.EdgeID,
			FromID:    edge.FromID,
			ToID:      edge.ToID,
			Kind:      edge.Kind,
			Tombstone: edge.Tombstone,
		})
	}
	sort.Slice(objects, func(i, j int) bool { return objects[i].CanonicalID < objects[j].CanonicalID })
	sort.Slice(edges, func(i, j int) bool { return edges[i].EdgeID < edges[j].EdgeID })
	return objects, edges, nil
}
