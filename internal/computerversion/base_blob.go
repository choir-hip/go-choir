package computerversion

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/yusefmosiah/go-choir/internal/base/blob"
	"github.com/yusefmosiah/go-choir/internal/base/model"
)

// BaseBlobStoreObservationSet converts selected refs from the existing Choir
// Base filesystem blob store into a blob-set observation slice. The caller names
// refs explicitly so this remains a scoped slice, not a full store scan.
func BaseBlobStoreObservationSet(ctx context.Context, name string, version ComputerVersion, store *blob.Store, refs []model.BlobRef) (ObservationSet, error) {
	if err := ctx.Err(); err != nil {
		return ObservationSet{}, err
	}
	if !version.Valid() {
		return ObservationSet{}, fmt.Errorf("base blob observation: invalid computer version")
	}
	if store == nil {
		return ObservationSet{}, fmt.Errorf("base blob observation: nil blob store")
	}
	if len(refs) == 0 {
		return ObservationSet{}, fmt.Errorf("base blob observation: no blob refs")
	}
	ordered := append([]model.BlobRef(nil), refs...)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i] < ordered[j] })
	seen := make(map[model.BlobRef]struct{}, len(ordered))
	observations := make([]Observation, 0, len(ordered))
	for _, ref := range ordered {
		if _, ok := seen[ref]; ok {
			return ObservationSet{}, fmt.Errorf("base blob observation: duplicate blob ref %s", ref)
		}
		seen[ref] = struct{}{}
		if err := ctx.Err(); err != nil {
			return ObservationSet{}, err
		}
		if !ref.Valid() || ref == "" {
			return ObservationSet{}, fmt.Errorf("base blob observation: invalid blob ref %q", ref)
		}
		data, err := store.Get(ref)
		if err != nil {
			return ObservationSet{}, fmt.Errorf("base blob observation: get %s: %w", ref, err)
		}
		stat, err := store.Stat(ref)
		if err != nil {
			return ObservationSet{}, fmt.Errorf("base blob observation: stat %s: %w", ref, err)
		}
		if int64(len(data)) != stat.SizeBytes {
			return ObservationSet{}, fmt.Errorf("base blob observation: size mismatch for %s", ref)
		}
		observation, err := encodeBaseBlobObservation(stat)
		if err != nil {
			return ObservationSet{}, err
		}
		observations = append(observations, observation)
	}
	return ObservationSet{Name: name, Version: version, Observations: observations}, nil
}

type baseBlobEntry struct {
	BlobRef   string `json:"blob_ref"`
	SizeBytes int64  `json:"size_bytes"`
	SHA256    string `json:"sha256"`
}

func encodeBaseBlobObservation(blob model.Blob) (Observation, error) {
	if !blob.Valid() {
		return Observation{}, fmt.Errorf("base blob observation: invalid blob metadata for %s", blob.BlobRef)
	}
	entry := baseBlobEntry{
		BlobRef:   string(blob.BlobRef),
		SizeBytes: blob.SizeBytes,
		SHA256:    blob.SHA256,
	}
	encoded, err := json.Marshal(entry)
	if err != nil {
		return Observation{}, fmt.Errorf("base blob observation: encode %s: %w", blob.BlobRef, err)
	}
	return Observation{Kind: ObservationBlobSet, Key: string(blob.BlobRef), Value: string(encoded)}, nil
}
