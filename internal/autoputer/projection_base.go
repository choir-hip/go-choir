package autoputer

import (
	"context"
	"encoding/json"
	"errors"
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
	choirstore "github.com/yusefmosiah/go-choir/internal/store"
)

// materializeProjectionBaseIfNeeded installs or rebases onto a verified
// ProjectionBase before reconstruct. The contract:
//
//   - empty store + no canonical chain: explicit new-computer genesis
//   - chain exists: a verified base is REQUIRED and the remaining tail
//     (start, H] must be ≤ MaxRecoveryTailEvents
//   - local < W: staged rebase (sibling install + quarantine/swap)
//   - local ≥ W and tail in bound: resume from the retained head
//
// A non-empty store is never skipped. Silent genesis fallback is deleted.
func materializeProjectionBaseIfNeeded(ctx context.Context, storePath, computerID, platformURL string, capability func(context.Context) (string, error), live *choirstore.Store) (bool, error) {
	storePath = filepath.Clean(storePath)
	markerName := filepath.Base(storePath)
	storeDir := filepath.Dir(storePath)
	computerID = strings.TrimSpace(computerID)
	platformURL = strings.TrimRight(strings.TrimSpace(platformURL), "/")
	if storeDir == "" || storeDir == "." || markerName == "" || markerName == "." || markerName == "/" || computerID == "" || platformURL == "" || capability == nil {
		return false, nil
	}
	sweepStagingArtifacts(storeDir)


	empty := isStoreEmpty(storeDir)
	var localSeq uint64
	if !empty {
		if live != nil {
			head, err := live.Head(ctx, computerID)
			if err != nil {
				return false, fmt.Errorf("%w: retained store head: %v", projectionbase.ErrBaseRefused, err)
			}
			if head != nil {
				localSeq = head.Sequence
			}
		} else {
			seq, err := projectionbase.PeekLocalSequence(ctx, storePath, computerID)
			if err != nil {
				return false, err
			}
			localSeq = seq
		}
	}

	head, err := queryCanonicalHead(ctx, platformURL, computerID, capability)
	if err != nil {
		return false, err
	}
	chainExists := head != nil && head.Sequence > 0
	var targetHead string
	var targetSeq uint64
	if chainExists {
		targetHead = head.CanonicalEventHead
		targetSeq = head.Sequence
	}

	source := projectionbase.NewHTTPSource(platformURL, projectionbase.CapabilityFunc(capability))
	var watermarkSeq uint64
	if chainExists {
		seq, _, wmErr := source.Watermark(ctx, computerID)
		if wmErr != nil && !errors.Is(wmErr, projectionbase.ErrBaseRefused) {
			return false, wmErr
		}
		if wmErr == nil {
			watermarkSeq = seq
		}
	}

	plan, err := projectionbase.PlanRecovery(empty, localSeq, chainExists, watermarkSeq, targetSeq)
	if err != nil {
		return false, err
	}
	switch plan.Action {
	case projectionbase.RecoveryGenesis, projectionbase.RecoveryResume:
		log.Printf("autoputer: projection recovery %s for %s (local=%d W=%d H=%d tail=%d)", plan.Action, computerID, localSeq, watermarkSeq, targetSeq, plan.TailEvents)
		return false, nil
	case projectionbase.RecoveryInstall:
		descriptor, err := projectionbase.InstallVerifiedBase(ctx, source, storeDir, markerName, computerID, targetHead, targetSeq)
		if err != nil {
			return false, fmt.Errorf("autoputer: required projection base refused: %w", err)
		}
		log.Printf("autoputer: ProjectionBase installed at sequence %d (base %s) for target %d", descriptor.Sequence, descriptor.BlobSHA256, targetSeq)
		return true, nil
	case projectionbase.RecoveryRebase:
		if live != nil {
			if err := live.Close(); err != nil {
				return false, fmt.Errorf("autoputer: close retained store before rebase: %w", err)
			}
		}
		descriptor, err := projectionbase.RebaseRetainedStore(ctx, source, storePath, computerID, targetHead, targetSeq)
		if err != nil {
			if live != nil {
				_ = live.Reopen(storePath)
			}
			return false, fmt.Errorf("autoputer: required projection rebase refused: %w", err)
		}
		if live != nil {
			if err := live.Reopen(storePath); err != nil {
				return false, fmt.Errorf("autoputer: reopen rebased store: %w", err)
			}
		}
		log.Printf("autoputer: ProjectionBase rebased retained store from %d onto W=%d (base %s) for target %d", localSeq, descriptor.Sequence, descriptor.BlobSHA256, targetSeq)
		return true, nil
	default:
		return false, fmt.Errorf("%w: unknown recovery action %s", projectionbase.ErrBaseRefused, plan.Action)
	}
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
		if strings.HasPrefix(name, "restore-staging-") || strings.HasPrefix(name, "restore-quarantine-") {
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

func sweepStagingArtifacts(dir string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		name := e.Name()
		if strings.HasPrefix(name, "restore-staging-") || strings.HasPrefix(name, ".base-download-") || strings.HasPrefix(name, ".base-staging") {
			_ = os.RemoveAll(filepath.Join(dir, name))
		}
	}
}
