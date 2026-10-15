package agentcore

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/release"
)

// corpusdReleaseSource reads published releases from corpusd with this
// computer's capability. It moves bytes; adoptRelease verifies them.
type corpusdReleaseSource struct {
	client *computerevent.HTTPClient
}

// NewCorpusdReleaseSource returns the host release reader for a guest.
func NewCorpusdReleaseSource(client *computerevent.HTTPClient) ReleaseSource {
	if client == nil {
		return nil
	}
	return corpusdReleaseSource{client: client}
}

func (s corpusdReleaseSource) FetchRelease(ctx context.Context, computerID, releaseID string) (json.RawMessage, error) {
	raw, status, err := s.client.FetchReleaseRecord(ctx, computerID, releaseID)
	switch {
	case err != nil:
		return nil, err
	case status == http.StatusNotFound:
		return nil, ErrReleaseNotPublished
	case status != http.StatusOK:
		return nil, fmt.Errorf("corpusd release read returned %d", status)
	}
	var view struct {
		ReleaseID string          `json:"release_id"`
		Record    json.RawMessage `json:"record"`
	}
	if err := json.Unmarshal(raw, &view); err != nil || len(view.Record) == 0 {
		return nil, fmt.Errorf("corpusd release read: malformed answer")
	}
	return view.Record, nil
}

func (s corpusdReleaseSource) FetchReleasePatch(ctx context.Context, computerID, patchSHA256 string) ([]byte, error) {
	raw, status, err := s.client.FetchReleaseBlob(ctx, computerID, patchSHA256, release.MaxPatchBytes)
	switch {
	case err != nil:
		return nil, err
	case status == http.StatusNotFound:
		return nil, ErrReleasePatchUnavailable
	case status == http.StatusConflict:
		// corpusd re-hashes on read: the stored bytes no longer hash to
		// their name.
		return nil, fmt.Errorf("%w: the host's stored patch does not hash to its name", release.ErrPatchMismatch)
	case status != http.StatusOK:
		return nil, fmt.Errorf("corpusd release blob read returned %d", status)
	}
	return raw, nil
}
