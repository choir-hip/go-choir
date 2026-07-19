package updater

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
)

// RestartRequestManager asks a fixed root-owned systemd path unit to restart
// Choir. The updater never receives access to PID 1 or a general service
// manager socket.
type RestartRequestManager struct {
	Path        string
	PrepareURL  string
	HandoffPath string
	Client      *http.Client
}

func (m RestartRequestManager) Restart(ctx context.Context) error {
	path := filepath.Clean(strings.TrimSpace(m.Path))
	if !filepath.IsAbs(path) || filepath.Base(path) != "restart" {
		return fmt.Errorf("updater: invalid restart request path")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	handoffPath := filepath.Clean(strings.TrimSpace(m.HandoffPath))
	if m.PrepareURL != "" {
		if !filepath.IsAbs(handoffPath) || filepath.Base(handoffPath) != "restart-capability" {
			return fmt.Errorf("updater: invalid restart handoff path")
		}
		prepareURL, err := url.Parse(m.PrepareURL)
		if err != nil || prepareURL.Scheme != "http" || prepareURL.User != nil || prepareURL.RawQuery != "" || prepareURL.Fragment != "" ||
			prepareURL.Path != "/internal/self-development/restart-handoff" {
			return fmt.Errorf("updater: restart preparation must use the fixed loopback endpoint")
		}
		address := net.ParseIP(prepareURL.Hostname())
		if address == nil || !address.IsLoopback() {
			return fmt.Errorf("updater: restart preparation must use loopback")
		}
		if err := os.Remove(handoffPath); err != nil && !os.IsNotExist(err) {
			return err
		}
		request, requestErr := http.NewRequestWithContext(ctx, http.MethodPost, prepareURL.String(), nil)
		if requestErr != nil {
			return requestErr
		}
		request.Header.Set("X-Internal-Updater", "true")
		client := m.Client
		if client == nil {
			client = &http.Client{Timeout: 10 * time.Second}
		}
		clientCopy := *client
		clientCopy.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
		response, requestErr := clientCopy.Do(request)
		if requestErr != nil {
			return fmt.Errorf("updater: prepare restart credential: %w", requestErr)
		}
		_ = response.Body.Close()
		if response.StatusCode != http.StatusNoContent {
			return fmt.Errorf("updater: prepare restart credential status %d", response.StatusCode)
		}
	}
	file, err := os.CreateTemp(filepath.Dir(path), ".restart-")
	if err != nil {
		return fmt.Errorf("updater: create restart request: %w", err)
	}
	temporary := file.Name()
	defer os.Remove(temporary)
	if _, err := file.WriteString("restart\n"); err != nil {
		_ = file.Close()
		return err
	}
	if err := file.Chmod(0o600); err != nil {
		_ = file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	if err := os.Rename(temporary, path); err != nil {
		return fmt.Errorf("updater: publish restart request: %w", err)
	}
	return nil
}

func (m RestartRequestManager) CleanupRestartHandoff() error {
	path := filepath.Clean(strings.TrimSpace(m.HandoffPath))
	if m.PrepareURL == "" {
		return nil
	}
	if !filepath.IsAbs(path) || filepath.Base(path) != "restart-capability" {
		return fmt.Errorf("updater: invalid restart handoff path")
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

type HTTPHealthProber struct {
	URL      string
	Client   *http.Client
	Attempts int
	Interval time.Duration
}

func (p HTTPHealthProber) Probe(ctx context.Context, releaseDigest string, manifest ReleaseManifest) ([]string, error) {
	if !computerevent.IsSHA256(releaseDigest) || strings.TrimSpace(p.URL) == "" {
		return nil, fmt.Errorf("updater: invalid health probe contract")
	}
	client := p.Client
	if client == nil {
		client = &http.Client{Timeout: 5 * time.Second}
	}
	attempts := p.Attempts
	if attempts <= 0 {
		attempts = 30
	}
	interval := p.Interval
	if interval <= 0 {
		interval = time.Second
	}
	var lastErr error
	for attempt := 0; attempt < attempts; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(interval):
			}
		}
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, p.URL, nil)
		if err != nil {
			return nil, err
		}
		response, err := client.Do(request)
		if err != nil {
			lastErr = err
			continue
		}
		var health struct {
			Status                string `json:"status"`
			SelfDevelopmentMarker string `json:"self_development_marker"`
			EventSchemaVersion    uint64 `json:"event_schema_version"`
			ReducerVersion        uint64 `json:"reducer_version"`
			ReleaseDigest         string `json:"release_digest"`
		}
		decodeErr := json.NewDecoder(response.Body).Decode(&health)
		_ = response.Body.Close()
		if response.StatusCode != http.StatusOK || decodeErr != nil {
			lastErr = fmt.Errorf("health status=%d decode=%v", response.StatusCode, decodeErr)
			continue
		}
		if health.Status == "" || health.Status == "failed" || health.SelfDevelopmentMarker != manifest.Marker || health.EventSchemaVersion != manifest.EventSchemaVersion || health.ReducerVersion != manifest.ReducerVersion || health.ReleaseDigest != releaseDigest {
			lastErr = fmt.Errorf("health identity mismatch")
			continue
		}
		canonical, err := computerevent.CanonicalJSON(health)
		if err != nil {
			return nil, err
		}
		return []string{computerevent.DigestBytes(canonical)}, nil
	}
	return nil, fmt.Errorf("updater: health probe failed: %w", lastErr)
}
