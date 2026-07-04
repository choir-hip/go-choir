package computerversion

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/yusefmosiah/go-choir/internal/base/blob"
	"github.com/yusefmosiah/go-choir/internal/base/journal"
	"github.com/yusefmosiah/go-choir/internal/base/model"
)

// BaseCurrentStateObservationSet composes the current Base journal-derived file
// manifest with the filesystem blob-store integrity slice referenced by that
// manifest. It remains scoped to Base observations; it does not inspect VM
// images or claim full-computer equivalence.

// BaseCurrentStateCapabilityManifest declares the observation scope for the
// current Base journal/blob slice. It is intentionally narrower than a full
// computer: only file-manifest and blob-set observations are in scope.
func BaseCurrentStateCapabilityManifest(materializer, substrate string) CapabilityManifest {
	return CapabilityManifest{
		Materializer: materializer,
		Substrate:    substrate,
		Supported:    []ObservationKind{ObservationBlobSet, ObservationFileManifest},
	}
}
func BaseCurrentStateObservationSet(ctx context.Context, name string, version ComputerVersion, jr journal.Journal, blobs *blob.Store) (ObservationSet, error) {
	journalSet, err := (BaseJournalExtractor{Journal: jr}).Extract(ctx, ExtractRequest{Name: name + ":journal", Version: version})
	if err != nil {
		return ObservationSet{}, err
	}
	refs, err := blobRefsFromFileManifestObservations(journalSet.Observations)
	if err != nil {
		return ObservationSet{}, err
	}
	blobSet, err := BaseBlobStoreObservationSet(ctx, name+":blobs", version, blobs, refs)
	if err != nil {
		return ObservationSet{}, err
	}
	observations := make([]Observation, 0, len(journalSet.Observations)+len(blobSet.Observations))
	observations = append(observations, journalSet.Observations...)
	observations = append(observations, blobSet.Observations...)
	sort.Slice(observations, func(i, j int) bool {
		if observations[i].Kind != observations[j].Kind {
			return observations[i].Kind < observations[j].Kind
		}
		return observations[i].Key < observations[j].Key
	})
	return ObservationSet{Name: name, Version: version, Observations: observations}, nil
}

func blobRefsFromFileManifestObservations(observations []Observation) ([]model.BlobRef, error) {
	seen := make(map[model.BlobRef]struct{})
	refs := make([]model.BlobRef, 0)
	for _, observation := range observations {
		if observation.Kind != ObservationFileManifest {
			continue
		}
		var entry struct {
			BlobRef string `json:"blob_ref"`
		}
		if err := json.Unmarshal([]byte(observation.Value), &entry); err != nil {
			return nil, fmt.Errorf("base current state observation: decode file manifest %s: %w", observation.Key, err)
		}
		if entry.BlobRef == "" {
			continue
		}
		ref := model.BlobRef(entry.BlobRef)
		if !ref.Valid() {
			return nil, fmt.Errorf("base current state observation: invalid blob ref %q for %s", ref, observation.Key)
		}
		if _, ok := seen[ref]; ok {
			continue
		}
		seen[ref] = struct{}{}
		refs = append(refs, ref)
	}
	if len(refs) == 0 {
		return nil, fmt.Errorf("base current state observation: no blob refs in file manifest observations")
	}
	sort.Slice(refs, func(i, j int) bool { return refs[i] < refs[j] })
	return refs, nil
}
