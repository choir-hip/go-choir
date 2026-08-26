package autoputer

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/projectionbase"
)

type watermarkResponse struct {
	WatermarkSequence uint64 `json:"watermark_sequence"`
	BaseRef           string `json:"base_ref"`
}

// materializeProjectionBaseIfNeeded inspects storeDir and, if empty, queries
// the platform for a verified ProjectionBase watermark and unpacks it before
// the local embedded Dolt/SQLite store is opened.
func materializeProjectionBaseIfNeeded(ctx context.Context, storeDir, computerID, platformURL string, capability func(context.Context) (string, error)) (bool, error) {
	storeDir = filepath.Clean(storeDir)
	computerID = strings.TrimSpace(computerID)
	platformURL = strings.TrimRight(strings.TrimSpace(platformURL), "/")
	if storeDir == "" || computerID == "" || platformURL == "" || capability == nil {
		return false, nil
	}

	if !isStoreEmpty(storeDir) {
		return false, nil
	}

	token, err := capability(ctx)
	if err != nil || strings.TrimSpace(token) == "" {
		return false, nil
	}

	watermarkURL := fmt.Sprintf("%s/internal/computers/files/watermark?computer_id=%s", platformURL, url.QueryEscape(computerID))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, watermarkURL, nil)
	if err != nil {
		return false, nil
	}
	req.Header.Set("Authorization", "Bearer "+token)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return false, nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return false, nil
	}

	var wm watermarkResponse
	if err := json.NewDecoder(resp.Body).Decode(&wm); err != nil {
		return false, nil
	}

	baseRef := strings.TrimSpace(wm.BaseRef)
	if wm.WatermarkSequence == 0 || baseRef == "" {
		return false, nil
	}

	// Fetch the ProjectionBase blob from the payload artifact endpoint
	artifactRef := "artifact:sha256:" + baseRef
	payloadURL := fmt.Sprintf("%s/internal/computers/events/payload?computer_id=%s&artifact_ref=%s", platformURL, url.QueryEscape(computerID), url.QueryEscape(artifactRef))
	payloadReq, err := http.NewRequestWithContext(ctx, http.MethodGet, payloadURL, nil)
	if err != nil {
		return false, nil
	}
	payloadReq.Header.Set("Authorization", "Bearer "+token)

	payloadResp, err := client.Do(payloadReq)
	if err != nil {
		return false, nil
	}
	defer payloadResp.Body.Close()

	if payloadResp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("projection base: payload fetch status %d", payloadResp.StatusCode)
	}

	tmpFile, err := os.CreateTemp(storeDir, ".projection-base-download-*")
	if err != nil {
		return false, fmt.Errorf("projection base: create temp download file: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer func() {
		_ = tmpFile.Close()
		_ = os.Remove(tmpPath)
	}()

	hasher := sha256.New()
	writer := io.MultiWriter(tmpFile, hasher)
	if _, err := io.Copy(writer, payloadResp.Body); err != nil {
		return false, fmt.Errorf("projection base: write payload: %w", err)
	}
	if err := tmpFile.Close(); err != nil {
		return false, fmt.Errorf("projection base: close temp download: %w", err)
	}

	digest := hex.EncodeToString(hasher.Sum(nil))
	if digest != baseRef {
		return false, fmt.Errorf("projection base: blob digest mismatch: got %s, want %s", digest, baseRef)
	}

	stagingDir := filepath.Join(storeDir, ".projection-base-staging")
	_ = os.RemoveAll(stagingDir)
	if err := os.MkdirAll(stagingDir, 0o755); err != nil {
		return false, fmt.Errorf("projection base: create staging dir: %w", err)
	}
	defer os.RemoveAll(stagingDir)

	if err := projectionbase.Unpack(tmpPath, stagingDir); err != nil {
		return false, fmt.Errorf("projection base: unpack blob: %w", err)
	}

	// Move unpacked contents into storeDir
	entries, err := os.ReadDir(stagingDir)
	if err != nil {
		return false, fmt.Errorf("projection base: read staging dir: %w", err)
	}
	for _, entry := range entries {
		src := filepath.Join(stagingDir, entry.Name())
		dst := filepath.Join(storeDir, entry.Name())
		if err := os.Rename(src, dst); err != nil {
			return false, fmt.Errorf("projection base: install entry %s: %w", entry.Name(), err)
		}
	}

	// Fsync storeDir
	if dir, err := os.Open(storeDir); err == nil {
		_ = dir.Sync()
		_ = dir.Close()
	}

	log.Printf("autoputer: ProjectionBase materialized at sequence %d (base %s)", wm.WatermarkSequence, baseRef)
	return true, nil
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
