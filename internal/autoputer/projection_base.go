package autoputer

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/projectionbase"
)

// materializeProjectionBaseIfNeeded installs the required base before the
// local embedded Dolt/SQLite store is opened. The contract is explicit:
//
//   - non-empty store: nothing to do, the resume path owns recovery;
//   - no canonical chain on the platform (head 404): explicit new-computer
//     bootstrap, genesis is allowed and no base is required;
//   - canonical chain exists: a verified base is REQUIRED. Any missing,
//     foreign, corrupt, non-ancestor, or incompatible base refuses loudly.
//     There is no silent genesis fallback: a failed install is a fatal boot
//     refusal, never a fresh store.
//
// The previous succeeds-with-nothing deferral (missing capability, HTTP or
// decode failure returning materialized=false without an error) is deleted:
// every required-base failure is now a typed error the caller must not survive.
func materializeProjectionBaseIfNeeded(ctx context.Context, storePath, computerID, platformURL string, capability func(context.Context) (string, error)) (bool, error) {
	storePath = filepath.Clean(storePath)
	markerName := filepath.Base(storePath)
	storeDir := filepath.Dir(storePath)
	computerID = strings.TrimSpace(computerID)
	platformURL = strings.TrimRight(strings.TrimSpace(platformURL), "/")
	if storeDir == "" || storeDir == "." || markerName == "" || markerName == "." || markerName == "/" || computerID == "" || platformURL == "" || capability == nil {
		return false, nil
	}

	if !isStoreEmpty(storeDir) {
		return false, nil
	}

	head, err := queryCanonicalHead(ctx, platformURL, computerID, capability)
	if err != nil {
		return false, err
	}
	if head == nil || head.Sequence == 0 {
		log.Printf("autoputer: no canonical chain for %s; explicit new-computer bootstrap without base", computerID)
		return false, nil
	}

	source := projectionbase.NewHTTPSource(platformURL, projectionbase.CapabilityFunc(capability))
	descriptor, err := projectionbase.InstallVerifiedBase(ctx, source, storeDir, markerName, computerID, head.CanonicalEventHead, head.Sequence)
	if err != nil {
		return false, fmt.Errorf("autoputer: required projection base refused: %w", err)
	}
	log.Printf("autoputer: ProjectionBase installed at sequence %d (base %s) for target %d", descriptor.Sequence, descriptor.BlobSHA256, head.Sequence)
	return true, nil
}

// queryCanonicalHead reads the platform canonical head. A missing head is the
// explicit bootstrap signal (nil, nil); any other failure is loud because
// boot cannot distinguish a new computer from a broken recovery without it.
func queryCanonicalHead(ctx context.Context, platformURL, computerID string, capability func(context.Context) (string, error)) (*computerevent.Head, error) {
	token, err := capability(ctx)
	if err != nil || strings.TrimSpace(token) == "" {
		return nil, fmt.Errorf("autoputer: platform capability unavailable: %w", err)
	}
	query := url.Values{"computer_id": {computerID}}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, platformURL+"/internal/computers/events/head?"+query.Encode(), nil)
	if err != nil {
		return nil, fmt.Errorf("autoputer: canonical head request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(token))
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("autoputer: canonical head fetch: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("autoputer: canonical head status %d", resp.StatusCode)
	}
	var head computerevent.Head
	if err := json.NewDecoder(resp.Body).Decode(&head); err != nil {
		return nil, fmt.Errorf("autoputer: decode canonical head: %w", err)
	}
	return &head, nil
}

func isStoreEmpty(dir string) bool {
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) == 0 {
		return true
	}
	for _, e := range entries {
		name := e.Name()
		if strings.HasPrefix(name, ".") && name != ".dolt" {
			continue
		}
		if name == ".dolt" || strings.HasSuffix(name, ".db") || strings.HasSuffix(name, ".sqlite") {
			return false
		}
		if !e.IsDir() {
			return false
		}
	}
	return true
}
