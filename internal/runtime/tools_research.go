package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/types"
)

type webSearchClient interface {
	Search(ctx context.Context, query string, maxResults int) (*webSearchResponse, error)
}

type webSearchResponse struct {
	Query     string           `json:"query"`
	Provider  string           `json:"provider"`
	Providers []string         `json:"providers,omitempty"`
	Attempts  []map[string]any `json:"attempts,omitempty"`
	Results   []map[string]any `json:"results"`
}

func RegisterResearchTools(registry *ToolRegistry, searchClient webSearchClient, httpClient *http.Client, rt *Runtime) error {
	for _, tool := range []Tool{
		newWebSearchTool(searchClient, rt),
		newFetchURLTool(httpClient),
		newImportURLContentTool(rt),
	} {
		if err := registry.Register(tool); err != nil {
			return err
		}
	}
	return nil
}

func newImportURLContentTool(rt *Runtime) Tool {
	type args struct {
		URL   string `json:"url"`
		Query string `json:"query,omitempty"`
	}
	return Tool{
		Name:        "import_url_content",
		Description: "Fetch a URL into the shared content substrate, extracting readable text and provenance for later VText ingestion or display apps.",
		Parameters: jsonSchemaObject(map[string]any{
			"url":   map[string]any{"type": "string"},
			"query": map[string]any{"type": "string"},
		}, []string{"url"}, false),
		Func: func(ctx context.Context, raw json.RawMessage) (string, error) {
			if rt == nil {
				return "", fmt.Errorf("runtime not configured")
			}
			var in args
			if err := json.Unmarshal(raw, &in); err != nil {
				return "", fmt.Errorf("decode import_url_content args: %w", err)
			}
			ownerID := stringFromToolContext(ctx, toolCtxOwnerID)
			if ownerID == "" {
				return "", fmt.Errorf("import_url_content missing owner context")
			}
			item, err := rt.ImportURLContent(ctx, ownerID, strings.TrimSpace(in.URL), strings.TrimSpace(in.Query))
			if err != nil {
				return "", err
			}
			return toolResultJSON(map[string]any{
				"content_id":    item.ContentID,
				"source_type":   item.SourceType,
				"media_type":    item.MediaType,
				"app_hint":      item.AppHint,
				"title":         item.Title,
				"source_url":    item.SourceURL,
				"canonical_url": item.CanonicalURL,
				"content_hash":  item.ContentHash,
				"text_chars":    len(item.TextContent),
				"provenance":    item.Provenance,
			})
		},
	}
}

func newWebSearchTool(searchClient webSearchClient, rt *Runtime) Tool {
	type args struct {
		Query      string `json:"query"`
		MaxResults int    `json:"max_results,omitempty"`
	}
	return Tool{
		Name:        "web_search",
		Description: "Search the web using the configured multi-provider search client. Researcher cadence: for a broad first pass, call one web_search, then submit_research_findings on the next model turn before any additional search-only turn; deeper searches can run after or alongside that checkpoint.",
		Parameters: jsonSchemaObject(map[string]any{
			"query":       map[string]any{"type": "string"},
			"max_results": map[string]any{"type": "integer", "minimum": 1, "maximum": 50},
		}, []string{"query"}, false),
		Func: func(ctx context.Context, raw json.RawMessage) (string, error) {
			var in args
			if err := json.Unmarshal(raw, &in); err != nil {
				return "", fmt.Errorf("decode web_search args: %w", err)
			}
			if searchClient == nil {
				return "", fmt.Errorf("search client not configured")
			}
			if strings.TrimSpace(in.Query) == "" {
				return "", fmt.Errorf("query must not be empty")
			}
			resp, err := searchClient.Search(ctx, strings.TrimSpace(in.Query), in.MaxResults)
			if err != nil {
				return "", err
			}
			full := map[string]any{
				"query":     resp.Query,
				"provider":  resp.Provider,
				"providers": resp.Providers,
				"attempts":  resp.Attempts,
				"results":   resp.Results,
			}
			model, metadata := compactWebSearchProjection(full, resp, shouldRequireResearchFindingsAfterSearch(ctx, rt))
			return toolProjectionResultJSON(model, full, metadata)
		},
	}
}

func shouldRequireResearchFindingsAfterSearch(ctx context.Context, rt *Runtime) bool {
	if stringFromToolContext(ctx, toolCtxProfile) != AgentProfileResearcher {
		return false
	}
	if rt == nil || rt.store == nil {
		return false
	}
	runID := stringFromToolContext(ctx, toolCtxRunID)
	if runID == "" {
		return false
	}
	events, err := rt.store.ListEvents(ctx, runID, 200)
	if err != nil {
		return false
	}
	if researchRunHasSuccessfulTool(events, "submit_research_findings") {
		return false
	}
	return !researchRunHasSuccessfulTool(events, "web_search")
}

func researchRunHasSuccessfulTool(events []types.EventRecord, toolName string) bool {
	for _, ev := range events {
		if ev.Kind != types.EventToolResult {
			continue
		}
		var payload map[string]any
		if err := json.Unmarshal(ev.Payload, &payload); err != nil {
			continue
		}
		tool, _ := payload["tool"].(string)
		isError, _ := payload["is_error"].(bool)
		if tool == toolName && !isError {
			return true
		}
	}
	return false
}

func newFetchURLTool(httpClient *http.Client) Tool {
	type args struct {
		URL      string `json:"url"`
		MaxChars int    `json:"max_chars,omitempty"`
	}
	return Tool{
		Name:        "fetch_url",
		Description: "Fetch a URL and return response metadata plus a truncated content excerpt.",
		Parameters: jsonSchemaObject(map[string]any{
			"url":       map[string]any{"type": "string"},
			"max_chars": map[string]any{"type": "integer", "minimum": 1},
		}, []string{"url"}, false),
		Func: func(ctx context.Context, raw json.RawMessage) (string, error) {
			var in args
			if err := json.Unmarshal(raw, &in); err != nil {
				return "", fmt.Errorf("decode fetch_url args: %w", err)
			}
			target := strings.TrimSpace(in.URL)
			if target == "" {
				return "", fmt.Errorf("url must not be empty")
			}
			client := httpClient
			if client == nil {
				client = &http.Client{Timeout: 30 * time.Second}
			}
			req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
			if err != nil {
				return "", err
			}
			resp, err := client.Do(req)
			if err != nil {
				return "", err
			}
			defer func() { _ = resp.Body.Close() }()

			data, err := io.ReadAll(io.LimitReader(resp.Body, 256*1024))
			if err != nil {
				return "", err
			}
			maxChars := in.MaxChars
			if maxChars <= 0 {
				maxChars = 12000
			}
			content := strings.TrimSpace(string(data))
			if len(content) > maxChars {
				content = content[:maxChars]
			}
			full := map[string]any{
				"url":            target,
				"status_code":    resp.StatusCode,
				"content_type":   resp.Header.Get("Content-Type"),
				"content_length": len(data),
				"content":        content,
			}
			model, metadata := compactFetchURLProjection(full, content)
			return toolProjectionResultJSON(model, full, metadata)
		},
	}
}

func compactWebSearchProjection(full map[string]any, resp *webSearchResponse, requireFindingsCheckpoint bool) (map[string]any, map[string]any) {
	const maxVisibleResults = 8
	const maxSnippetChars = 700
	visibleResults := make([]map[string]any, 0, researchMinInt(len(resp.Results), maxVisibleResults))
	for idx, result := range resp.Results {
		if idx >= maxVisibleResults {
			break
		}
		visible := map[string]any{
			"rank":     idx + 1,
			"title":    stringValue(result["title"]),
			"url":      stringValue(result["url"]),
			"snippet":  truncateString(stringValue(result["snippet"]), maxSnippetChars),
			"provider": stringValue(result["provider"]),
		}
		if published := stringValue(result["published_at"]); published != "" {
			visible["published_at"] = published
		}
		if score, ok := result["score"]; ok {
			visible["score"] = score
		}
		visibleResults = append(visibleResults, visible)
	}
	attempts := make([]map[string]any, 0, len(resp.Attempts))
	degraded := false
	for _, attempt := range resp.Attempts {
		compact := map[string]any{
			"provider":   attempt["provider"],
			"status":     attempt["status"],
			"latency_ms": attempt["latency_ms"],
			"results":    attempt["results"],
		}
		if status := stringValue(attempt["status"]); status != "" && status != "success" {
			degraded = true
			if errText := stringValue(attempt["error"]); errText != "" {
				compact["error"] = truncateString(errText, 160)
			}
		}
		attempts = append(attempts, compact)
	}
	model := map[string]any{
		"query":                 resp.Query,
		"provider":              resp.Provider,
		"providers":             resp.Providers,
		"result_count":          len(resp.Results),
		"visible_result_count":  len(visibleResults),
		"omitted_result_count":  researchMaxInt(0, len(resp.Results)-len(visibleResults)),
		"results":               visibleResults,
		"attempts":              attempts,
		"full_evidence":         "stored in Trace tool.result full_output/full_output_sha256",
		"projection_policy":     "top bounded result cards with compact snippets",
		"provider_health_owner": "gateway",
	}
	if requireFindingsCheckpoint {
		model["next_required_tool"] = "submit_research_findings"
		model["next_instruction"] = "Submit concise first findings from this search result before any additional search-only turn. Include 2-4 grounded facts, notes, questions, or a precise blocker; evidence entries may be omitted until richer evidence is ready."
	}
	if degraded {
		model["gateway_status"] = "one or more providers failed or were unavailable; gateway returned available evidence and preserved provider details in Trace"
	}
	metadata := map[string]any{
		"type":                 "web_search",
		"full_result_count":    len(resp.Results),
		"visible_result_count": len(visibleResults),
		"full_output_bytes":    len(researchMustJSON(full)),
		"model_output_bytes":   len(researchMustJSON(model)),
	}
	return model, metadata
}

func compactFetchURLProjection(full map[string]any, content string) (map[string]any, map[string]any) {
	const maxContentChars = 4000
	visibleContent := truncateString(content, maxContentChars)
	model := map[string]any{
		"url":               full["url"],
		"status_code":       full["status_code"],
		"content_type":      full["content_type"],
		"content_length":    full["content_length"],
		"content_chars":     len(content),
		"visible_chars":     len(visibleContent),
		"omitted_chars":     researchMaxInt(0, len(content)-len(visibleContent)),
		"content":           visibleContent,
		"full_evidence":     "stored in Trace tool.result full_output/full_output_sha256",
		"projection_policy": "bounded excerpt; fetch/read deeper only when needed",
	}
	metadata := map[string]any{
		"type":               "fetch_url",
		"full_content_chars": len(content),
		"visible_chars":      len(visibleContent),
		"full_output_bytes":  len(researchMustJSON(full)),
		"model_output_bytes": len(researchMustJSON(model)),
	}
	return model, metadata
}

func stringValue(value any) string {
	switch v := value.(type) {
	case string:
		return v
	default:
		return strings.TrimSpace(fmt.Sprint(v))
	}
}

func truncateString(value string, maxChars int) string {
	value = strings.TrimSpace(value)
	if maxChars <= 0 || len(value) <= maxChars {
		return value
	}
	return strings.TrimSpace(value[:maxChars]) + "..."
}

func researchMustJSON(value any) string {
	out, err := json.Marshal(value)
	if err != nil {
		return ""
	}
	return string(out)
}

func researchMinInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func researchMaxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
