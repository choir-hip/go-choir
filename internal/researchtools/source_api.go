package researchtools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/yusefmosiah/go-choir/internal/toolregistry"
)

// sourceAPIToolMaxChars bounds what choir.SourceAPI returns into a cell: a
// keyed API can answer megabytes (a long FRED series), and the desk's
// context is the scarce thing.
const sourceAPIToolMaxChars = 48000

// newSourceAPITool is choir.SourceAPI: one named endpoint of one keyed
// source API (FRED, Congress.gov, CourtListener, YouTube, NewsAPI.ai)
// through the host gateway, which adds the key; this computer never holds
// it (docs/problems/keyed-source-apis-2026-10-20.md). It reads; it writes
// nothing.
func newSourceAPITool(deps Dependencies) toolregistry.Tool {
	type args struct {
		Provider string         `json:"provider"`
		Endpoint string         `json:"endpoint"`
		Params   map[string]any `json:"params,omitempty"`
	}
	return toolregistry.Tool{Name: "source_api",
		Description: "Call one named endpoint of a keyed source API (fred, congress, courtlistener, youtube, newsapi_ai) through the host gateway; returns the provider's JSON answer.",
		Parameters: toolregistry.JSONSchemaObject(map[string]any{
			"provider": map[string]any{"type": "string"},
			"endpoint": map[string]any{"type": "string"},
			"params":   map[string]any{"type": "object"},
		}, []string{"provider", "endpoint"}, false),
		Func: func(ctx context.Context, raw json.RawMessage) (string, error) {
			var in args
			if err := json.Unmarshal(raw, &in); err != nil {
				return "", fmt.Errorf("decode source_api args: %w", err)
			}
			if deps.SourceAPI == nil {
				return "", fmt.Errorf("source_api: the gateway source api is not available on this computer")
			}
			answer, err := deps.SourceAPI.SourceAPI(ctx, strings.TrimSpace(in.Provider), strings.TrimSpace(in.Endpoint), in.Params)
			if err != nil {
				return "", err
			}
			if len(answer) <= sourceAPIToolMaxChars {
				return string(answer), nil
			}
			return cutSourceAPIAnswer(answer)
		}}
}

// cutSourceAPIAnswer keeps the envelope's fields and the first part of the
// provider's answer as text.
func cutSourceAPIAnswer(answer json.RawMessage) (string, error) {
	var env struct {
		Provider       string          `json:"provider"`
		Endpoint       string          `json:"endpoint"`
		UpstreamStatus int             `json:"upstream_status"`
		Body           json.RawMessage `json:"body"`
		Text           string          `json:"text"`
	}
	_ = json.Unmarshal(answer, &env)
	text := env.Text
	if len(env.Body) > 0 {
		text = string(env.Body)
	}
	if text == "" {
		text = string(answer)
	}
	limit := sourceAPIToolMaxChars - 1024
	if len(text) > limit {
		text = text[:limit]
		for len(text) > 0 && !utf8.ValidString(text) {
			text = text[:len(text)-1]
		}
	}
	return toolregistry.ResultJSON(map[string]any{
		"provider":        env.Provider,
		"endpoint":        env.Endpoint,
		"upstream_status": env.UpstreamStatus,
		"truncated":       true,
		"text":            text,
		"hint":            "the answer was cut; narrow the request (limit, dates, fewer ids) to read it whole",
	})
}
