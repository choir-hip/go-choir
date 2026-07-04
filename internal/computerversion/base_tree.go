package computerversion

import (
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/yusefmosiah/go-choir/internal/base/model"
	basetree "github.com/yusefmosiah/go-choir/internal/base/tree"
)

// BaseTreeObservationSet converts Choir Base's typed tree snapshot into a
// file-manifest ObservationSet. It is the first current-state-shaped durable
// slice for this mission: item identity, path/location, deletion state, and
// blob/content refs are compared as typed observations instead of as opaque
// data.img bytes.
func BaseTreeObservationSet(name string, version ComputerVersion, tree basetree.Tree) (ObservationSet, error) {
	if !version.Valid() {
		return ObservationSet{}, fmt.Errorf("base tree observation set: invalid computer version")
	}

	ids := make([]model.ItemID, 0, len(tree.Items))
	for id := range tree.Items {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })

	observations := make([]Observation, 0, len(ids))
	for _, id := range ids {
		item := tree.Items[id]
		observation, err := baseTreeObservation(id, item, tree.Versions[id])
		if err != nil {
			return ObservationSet{}, err
		}
		observations = append(observations, observation)
	}

	return ObservationSet{
		Name:         name,
		Version:      version,
		Required:     []ObservationKind{ObservationFileManifest},
		Observations: observations,
	}, nil
}

type baseTreeEntry struct {
	ItemID           string `json:"item_id"`
	OwnerID          string `json:"owner_id,omitempty"`
	ParentItemID     string `json:"parent_item_id,omitempty"`
	Name             string `json:"name"`
	Kind             string `json:"kind"`
	Deleted          bool   `json:"deleted"`
	DeletedAt        string `json:"deleted_at,omitempty"`
	VersionID        string `json:"version_id,omitempty"`
	BlobRef          string `json:"blob_ref,omitempty"`
	ContentHash      string `json:"content_hash,omitempty"`
	ManifestJSON     string `json:"manifest_json,omitempty"`
	ProvenanceJSON   string `json:"provenance_json,omitempty"`
	CreatedByDevice  string `json:"created_by_device,omitempty"`
	CreatedBySubject string `json:"created_by_subject,omitempty"`
}

func baseTreeObservation(id model.ItemID, item model.Item, version model.Version) (Observation, error) {
	if id != item.ItemID {
		return Observation{}, fmt.Errorf("base tree observation: map key %q does not match item id %q", id, item.ItemID)
	}
	if !item.ItemID.Valid() {
		return Observation{}, fmt.Errorf("base tree observation: invalid item id %q", item.ItemID)
	}
	if !item.Kind.Valid() {
		return Observation{}, fmt.Errorf("base tree observation: invalid kind for item %q", item.ItemID)
	}
	if !item.CurrentVersion.Valid() {
		return Observation{}, fmt.Errorf("base tree observation: invalid current version for item %q", item.ItemID)
	}

	entry := baseTreeEntry{
		ItemID:       string(item.ItemID),
		OwnerID:      item.OwnerID,
		ParentItemID: string(item.ParentItemID),
		Name:         item.Name,
		Kind:         string(item.Kind),
	}
	if item.DeletedAt != nil || item.CurrentVersion == "" {
		entry.Deleted = true
		if item.DeletedAt != nil {
			entry.DeletedAt = item.DeletedAt.UTC().Format(time.RFC3339Nano)
		}
		return encodeBaseTreeEntry(item.ItemID, entry)
	}

	if version.VersionID == "" {
		return Observation{}, fmt.Errorf("base tree observation: live item %q has no version", item.ItemID)
	}
	if version.ItemID != item.ItemID {
		return Observation{}, fmt.Errorf("base tree observation: version item %q does not match item %q", version.ItemID, item.ItemID)
	}
	if version.VersionID != item.CurrentVersion {
		return Observation{}, fmt.Errorf("base tree observation: version %q does not match current version %q for item %q", version.VersionID, item.CurrentVersion, item.ItemID)
	}
	if !version.Valid() {
		return Observation{}, fmt.Errorf("base tree observation: invalid version %q for item %q", version.VersionID, item.ItemID)
	}
	if item.Kind == model.KindFile && version.BlobRef == "" {
		return Observation{}, fmt.Errorf("base tree observation: file item %q has empty blob ref", item.ItemID)
	}
	if item.Kind == model.KindFolder && version.BlobRef != "" {
		return Observation{}, fmt.Errorf("base tree observation: folder item %q has blob ref %q", item.ItemID, version.BlobRef)
	}

	entry.VersionID = string(version.VersionID)
	entry.BlobRef = string(version.BlobRef)
	entry.ContentHash = version.ContentHash
	entry.ManifestJSON = version.ManifestJSON
	entry.ProvenanceJSON = version.ProvenanceJSON
	entry.CreatedByDevice = version.CreatedByDevice
	entry.CreatedBySubject = version.CreatedBySubject
	return encodeBaseTreeEntry(item.ItemID, entry)
}

func encodeBaseTreeEntry(itemID model.ItemID, entry baseTreeEntry) (Observation, error) {
	encoded, err := json.Marshal(entry)
	if err != nil {
		return Observation{}, fmt.Errorf("base tree observation: encode item %q: %w", itemID, err)
	}
	return Observation{Kind: ObservationFileManifest, Key: string(itemID), Value: string(encoded)}, nil
}
