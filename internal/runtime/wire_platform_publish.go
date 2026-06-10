package runtime

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/types"
	"github.com/yusefmosiah/go-choir/internal/wirepublish"
)

type wirePlatformPublishRequest struct {
	DocID         string          `json:"doc_id"`
	RevisionID    string          `json:"revision_id"`
	Title         string          `json:"title,omitempty"`
	Content       string          `json:"content,omitempty"`
	Citations     json.RawMessage `json:"citations,omitempty"`
	Metadata      json.RawMessage `json:"metadata,omitempty"`
	RunID         string          `json:"run_id,omitempty"`
	RequestIntent string          `json:"request_intent,omitempty"`
}

func (rt *Runtime) publishWireArticleToPlatform(ctx context.Context, doc types.Document, rev types.Revision, rec *types.RunRecord) (*wirepublish.PublishVTextResponse, error) {
	if rt == nil {
		return nil, fmt.Errorf("runtime unavailable")
	}
	if rt.wirePlatformPublisher != nil {
		return rt.wirePlatformPublisher(ctx, doc, rev, rec)
	}
	wireURL := strings.TrimRight(strings.TrimSpace(rt.cfg.WirePublishURL), "/")
	if wireURL == "" {
		wireURL = fallbackWirePublishURLFromEnv()
	}
	if wireURL != "" {
		return rt.postWirePublishProxy(ctx, wireURL, doc, rev, rec)
	}
	platformdURL := strings.TrimSpace(rt.cfg.PlatformdURL)
	if platformdURL != "" {
		req := wirepublish.BuildAutonomousPublishRequest(doc, rev, rec, rev.Metadata)
		return wirepublish.PostPlatformPublication(ctx, nil, platformdURL, req)
	}
	return nil, fmt.Errorf("wire publish is not configured")
}


func fallbackWirePublishURLFromEnv() string {
	if url := fallbackWirePublishURLFromBases([]string{
		os.Getenv("RUNTIME_VMCTL_URL"),
		os.Getenv("PROXY_VMCTL_URL"),
		os.Getenv("RUNTIME_GATEWAY_URL"),
		os.Getenv("RUNTIME_MAILD_URL"),
	}); url != "" {
		return url
	}
	data, err := os.ReadFile("/proc/cmdline")
	if err != nil {
		return ""
	}
	var bases []string
	for _, field := range strings.Fields(string(data)) {
		switch {
		case strings.HasPrefix(field, "choir.wire_publish_url="):
			return strings.TrimPrefix(field, "choir.wire_publish_url=")
		case strings.HasPrefix(field, "choir.vmctl_url="):
			bases = append(bases, strings.TrimPrefix(field, "choir.vmctl_url="))
		case strings.HasPrefix(field, "choir.gateway_url="):
			bases = append(bases, strings.TrimPrefix(field, "choir.gateway_url="))
		case strings.HasPrefix(field, "choir.maild_url="):
			bases = append(bases, strings.TrimPrefix(field, "choir.maild_url="))
		}
	}
	return fallbackWirePublishURLFromBases(bases)
}

func fallbackWirePublishURLFromBases(bases []string) string {
	for _, raw := range bases {
		base := strings.TrimRight(strings.TrimSpace(raw), "/")
		if base == "" {
			continue
		}
		for _, suffix := range []string{":8083", ":8084", ":8087"} {
			if strings.HasSuffix(base, suffix) {
				return strings.TrimSuffix(base, suffix) + ":8082"
			}
		}
	}
	return ""
}

func (rt *Runtime) postWirePublishProxy(ctx context.Context, wireURL string, doc types.Document, rev types.Revision, rec *types.RunRecord) (*wirepublish.PublishVTextResponse, error) {
	payload := wirePlatformPublishRequest{
		DocID:      doc.DocID,
		RevisionID: rev.RevisionID,
		Title:      doc.Title,
		Content:    rev.Content,
		Citations:  rev.Citations,
		Metadata:   rev.Metadata,
	}
	if rec != nil {
		payload.RunID = strings.TrimSpace(rec.RunID)
		payload.RequestIntent = metadataStringValue(rec.Metadata, "request_intent")
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal wire publish request: %w", err)
	}
	target := wireURL + "/internal/wire/platform/publications/vtext"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, target, bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("build wire publish request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Internal-Caller", "true")
	req.Header.Set("X-Authenticated-User", universalWirePlatformOwnerID())
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("call wire publish proxy: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("read wire publish response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var apiErr struct {
			Error string `json:"error"`
		}
		_ = json.Unmarshal(body, &apiErr)
		if strings.TrimSpace(apiErr.Error) == "" {
			apiErr.Error = strings.TrimSpace(string(body))
		}
		if apiErr.Error == "" {
			apiErr.Error = fmt.Sprintf("wire publish status %d", resp.StatusCode)
		}
		return nil, fmt.Errorf("%s", apiErr.Error)
	}
	var out wirepublish.PublishVTextResponse
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, fmt.Errorf("decode wire publish response: %w", err)
	}
	return &out, nil
}

func (rt *Runtime) persistWirePlatformPublicationRef(ctx context.Context, ownerID string, rev types.Revision, pub *wirepublish.PublishVTextResponse) error {
	if rt == nil || rt.store == nil || pub == nil {
		return fmt.Errorf("persist wire publication ref: unavailable")
	}
	ref := map[string]any{
		"publication_id":         pub.PublicationID,
		"publication_version_id": pub.PublicationVersionID,
		"route_path":             pub.RoutePath,
		"source_revision_hash":   pub.SourceRevisionHash,
		"requested_by":           wirepublish.RequestedByWirePolicy,
		"publication_kind":       wirepublish.PublicationKind,
	}
	return rt.store.PatchRevisionMetadata(ctx, ownerID, rev.RevisionID, map[string]any{
		"platformd_publication_ref": ref,
		"platformd_route_path":      pub.RoutePath,
	})
}
