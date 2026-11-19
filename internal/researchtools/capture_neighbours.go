package researchtools

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// CaptureNeighboursQuery names captures the computer holds and asks for the
// nearest other captures of each, by their stored vectors (corpusd POST
// /internal/platform/captures/neighbours). Since is a day, inclusive.
type CaptureNeighboursQuery struct {
	OwnerID    string   `json:"owner_id"`
	ComputerID string   `json:"computer_id"`
	CaptureIDs []string `json:"capture_ids"`
	Since      string   `json:"since,omitempty"`
	Limit      int      `json:"limit,omitempty"`
}

// CaptureNeighbour mirrors platform.CaptureNeighbour.
type CaptureNeighbour struct {
	CaptureID string  `json:"capture_id"`
	Score     float64 `json:"score"`
}

// CaptureNeighboursAnswer mirrors platform.CaptureNeighboursResponse.
type CaptureNeighboursAnswer struct {
	Neighbours  map[string][]CaptureNeighbour `json:"neighbours"`
	Missing     []string                      `json:"missing,omitempty"`
	Unavailable string                        `json:"unavailable,omitempty"`
	State       string                        `json:"state,omitempty"`
}

// CaptureNeighbourer is a capture sink that answers nearest-capture questions.
// A sink that does not (an older corpusd, a test fake) leaves the caller to
// proceed without the vector half.
type CaptureNeighbourer interface {
	CaptureNeighbours(ctx context.Context, q CaptureNeighboursQuery) (CaptureNeighboursAnswer, error)
}

// CaptureNeighbours asks corpusd for the nearest captures of each named one,
// in one request.
func (c *CorpusdCaptureSink) CaptureNeighbours(ctx context.Context, q CaptureNeighboursQuery) (CaptureNeighboursAnswer, error) {
	var out CaptureNeighboursAnswer
	if c == nil || c.BaseURL == "" {
		return out, fmt.Errorf("capture neighbours: corpusd URL is not configured")
	}
	raw, err := json.Marshal(q)
	if err != nil {
		return out, fmt.Errorf("capture neighbours: %w", err)
	}
	body, err := c.do(ctx, http.MethodPost, c.BaseURL+"/internal/platform/captures/neighbours", raw, 32<<20)
	if err != nil {
		return out, fmt.Errorf("capture neighbours: %w", err)
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return out, fmt.Errorf("capture neighbours: decode: %w", err)
	}
	return out, nil
}
