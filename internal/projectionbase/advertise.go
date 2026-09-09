package projectionbase

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// AdvertiseWatermark POSTs the published blob as the computer's advertised W.
// The platform handler remains the sole writer of computer_replay_watermarks;
// this is the offline publisher's call into that handler.
func AdvertiseWatermark(ctx context.Context, platformURL, token, computerID string, sequence uint64, baseRef string) error {
	platformURL = strings.TrimRight(strings.TrimSpace(platformURL), "/")
	token = strings.TrimSpace(token)
	computerID = strings.TrimSpace(computerID)
	baseRef = strings.TrimSpace(baseRef)
	if platformURL == "" || token == "" || computerID == "" || sequence == 0 || baseRef == "" {
		return fmt.Errorf("%w: advertise requires platform, capability, computer, sequence, and base digest", ErrBaseRefused)
	}
	body, err := json.Marshal(map[string]any{
		"computer_id":        computerID,
		"watermark_sequence": sequence,
		"base_ref":           baseRef,
	})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, platformURL+"/internal/computers/files/watermark", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%w: advertise watermark status %d", ErrBaseRefused, resp.StatusCode)
	}
	return nil
}
