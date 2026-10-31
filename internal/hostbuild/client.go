package hostbuild

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
)

// Route is the corpusd host build route. POST submits; GET Route/<job>
// reads a job.
const Route = "/internal/computers/selfdev-builds"

// JSONDoer is the guest's authenticated corpusd client
// (computerevent.HTTPClient.DoJSON).
type JSONDoer interface {
	DoJSON(ctx context.Context, method, path string, body, response any) (int, error)
}

// Client is the guest side of the host build route.
type Client struct {
	Doer JSONDoer
}

// Submit sends the request; the answer is the job (new or existing).
func (c Client) Submit(ctx context.Context, req Request) (Job, int, error) {
	if c.Doer == nil {
		return Job{}, 0, fmt.Errorf("host build: no corpusd client")
	}
	var job Job
	status, err := c.Doer.DoJSON(ctx, http.MethodPost, Route, req, &job)
	return job, status, err
}

// Status reads a job this computer owns.
func (c Client) Status(ctx context.Context, computerID, jobID string) (Job, int, error) {
	if c.Doer == nil {
		return Job{}, 0, fmt.Errorf("host build: no corpusd client")
	}
	if !computerevent.IsSHA256(jobID) || strings.TrimSpace(computerID) == "" {
		return Job{}, 0, fmt.Errorf("host build: computer and job id are required")
	}
	var job Job
	status, err := c.Doer.DoJSON(ctx, http.MethodGet, Route+"/"+jobID+"?"+url.Values{"computer_id": []string{computerID}}.Encode(), nil, &job)
	return job, status, err
}

// ConfigFromEnv reads the corpusd host build configuration. The service is
// enabled only when CORPUSD_HOST_BUILD_SOURCE_DIR names the host's source
// checkout; unset, Go-touching self-development applies fail with
// host_builder_unconfigured and change nothing.
func ConfigFromEnv(getenv func(string) string) (Config, bool) {
	get := func(key, fallback string) string {
		if v := strings.TrimSpace(getenv(key)); v != "" {
			return v
		}
		return fallback
	}
	source := get("CORPUSD_HOST_BUILD_SOURCE_DIR", "")
	if source == "" {
		return Config{}, false
	}
	cfg := Config{
		SourceDir:             source,
		StateDir:              get("CORPUSD_HOST_BUILD_STATE_DIR", "/var/lib/go-choir/host-build"),
		BaseManifestPath:      get("CORPUSD_HOST_BUILD_BASE_MANIFEST", "/var/lib/go-choir/guest/guest-image-manifest"),
		BaseStoreDiskPath:     get("CORPUSD_HOST_BUILD_BASE_STOREDISK", "/var/lib/go-choir/guest/storedisk.erofs"),
		Installable:           get("CORPUSD_HOST_BUILD_INSTALLABLE", ".#autoputer"),
		BaseStoreDiskBasePath: get("CORPUSD_HOST_BUILD_BASE_STOREDISK_BASE", ""),
	}
	if d, err := time.ParseDuration(get("CORPUSD_HOST_BUILD_TIMEOUT", "")); err == nil {
		cfg.BuildTimeout = d
	}
	if d, err := time.ParseDuration(get("CORPUSD_HOST_BUILD_QUEUE_TIMEOUT", "")); err == nil {
		cfg.QueueTimeout = d
	}
	if n, err := strconv.ParseUint(get("CORPUSD_HOST_BUILD_MEMORY_FLOOR_BYTES", ""), 10, 64); err == nil {
		cfg.MemoryFloorBytes = n
	}
	if n, err := strconv.Atoi(get("CORPUSD_HOST_BUILD_MAX_PER_COMPUTER_PER_DAY", "")); err == nil && n > 0 {
		cfg.MaxBuildsPerComputerPerDay = n
	}
	return cfg, true
}
