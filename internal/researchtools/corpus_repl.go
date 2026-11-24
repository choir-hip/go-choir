package researchtools

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/toolregistry"
)

// The corpus as values for a desk's cell (owner, 2026-11-24: "we should
// definitely be able to have the whole corpus and new captures at a desks
// fingertips (repl)"). Four host-served reads, none a model call:
//
//	corpus_captures  a page of the computer's captures, metadata rows;
//	corpus_texts     the text of many captures at once (corpusd's /texts);
//	corpus_near      the nearest captures and published stories to a capture,
//	                 a story or a text, with the similarity;
//	corpus_edges     the edges around a capture or a published story.
//
// The page, near and edges answers are corpusd's JSON passed through
// (internal/platform/corpus_repl.go holds the scope and eligibility rules).
// Scope is always the run's own owner and computer, taken from the run
// context, never from an argument and never from a fallback. The four are
// corpusd's JSON passed through; every answer is cut by corpusd below its byte
// bound (a "cut" list or a cursor says so), and an answer that still reaches
// the bound is refused here, never read short.

// corpusReplCaller is a capture sink that can post to corpusd's corpus routes.
type corpusReplCaller interface {
	CorpusCall(ctx context.Context, route string, body any) (json.RawMessage, error)
}

// CorpusCall posts one corpus request to corpusd's /internal/platform/captures/<route>.
func (c *CorpusdCaptureSink) CorpusCall(ctx context.Context, route string, body any) (json.RawMessage, error) {
	if c == nil || c.BaseURL == "" {
		return nil, fmt.Errorf("corpus %s: corpusd URL is not configured", route)
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	// One byte past the bound is read, so an answer that reaches it is an
	// error and never a truncated document that does not decode.
	out, err := c.do(ctx, http.MethodPost, c.BaseURL+"/internal/platform/captures/"+route, raw, corpusReplResponseBytes+1)
	if err != nil {
		return nil, fmt.Errorf("corpus %s: %w", route, err)
	}
	if len(out) > corpusReplResponseBytes {
		return nil, fmt.Errorf("corpus %s: the answer is over %d bytes; corpusd cuts its answers below that, so this one is refused rather than read short", route, corpusReplResponseBytes)
	}
	return json.RawMessage(out), nil
}

const (
	// corpusReplResponseBytes bounds one answer: a desk cell's frame is 4 MiB
	// (yaegikernel.MaxFramePayload) and an answer that did not fit would be
	// lost whole. Each list is cut below this with a cursor or a count.
	corpusReplResponseBytes = 3 << 20
	// corpusTextMaxIDs is how many captures one corpus_texts call names.
	corpusTextMaxIDs = 200
)

func corpusScope(ctx context.Context, verb string) (owner, computer string, err error) {
	exec := toolregistry.ExecutionContextFrom(ctx)
	owner, computer = strings.TrimSpace(exec.OwnerID), strings.TrimSpace(exec.ComputerID)
	if owner == "" || computer == "" {
		return "", "", fmt.Errorf("%s: the run names no owner and computer", verb)
	}
	return owner, computer, nil
}

func corpusCaller(deps Dependencies, verb string) (corpusReplCaller, error) {
	c, ok := deps.Captures.(corpusReplCaller)
	if !ok || deps.Captures == nil {
		return nil, fmt.Errorf("%s: the shared capture store is not configured", verb)
	}
	return c, nil
}

func newCorpusCapturesTool(deps Dependencies) toolregistry.Tool {
	type args struct {
		Since    string `json:"since,omitempty"`
		Until    string `json:"until,omitempty"`
		SourceID string `json:"source_id,omitempty"`
		Kind     string `json:"kind,omitempty"`
		Cursor   string `json:"cursor,omitempty"`
		Limit    int    `json:"limit,omitempty"`
	}
	return toolregistry.Tool{Name: "corpus_captures",
		Description: "One page of this computer's captures as metadata rows, newest fetched first.",
		Parameters: toolregistry.JSONSchemaObject(map[string]any{
			"since": map[string]any{"type": "string"}, "until": map[string]any{"type": "string"},
			"source_id": map[string]any{"type": "string"}, "kind": map[string]any{"type": "string"},
			"cursor": map[string]any{"type": "string"}, "limit": map[string]any{"type": "integer"},
		}, nil, false),
		Func: func(ctx context.Context, raw json.RawMessage) (string, error) {
			var in args
			if err := json.Unmarshal(raw, &in); err != nil {
				return "", fmt.Errorf("decode corpus_captures args: %w", err)
			}
			owner, computer, err := corpusScope(ctx, "corpus_captures")
			if err != nil {
				return "", err
			}
			c, err := corpusCaller(deps, "corpus_captures")
			if err != nil {
				return "", err
			}
			out, err := c.CorpusCall(ctx, "page", map[string]any{"owner_id": owner, "computer_id": computer, "since": in.Since, "until": in.Until,
				"source_id": in.SourceID, "kind": in.Kind, "cursor": in.Cursor, "limit": in.Limit})
			return string(out), err
		}}
}

func newCorpusNearTool(deps Dependencies) toolregistry.Tool {
	type args struct {
		Of    string `json:"of"`
		K     int    `json:"k,omitempty"`
		Since string `json:"since,omitempty"`
		Until string `json:"until,omitempty"`
	}
	return toolregistry.Tool{Name: "corpus_near",
		Description: "The nearest captures and published stories to a capture id, a publication id or a text.",
		Parameters: toolregistry.JSONSchemaObject(map[string]any{
			"of": map[string]any{"type": "string"}, "k": map[string]any{"type": "integer"},
			"since": map[string]any{"type": "string"}, "until": map[string]any{"type": "string"},
		}, []string{"of"}, false),
		Func: func(ctx context.Context, raw json.RawMessage) (string, error) {
			var in args
			if err := json.Unmarshal(raw, &in); err != nil {
				return "", fmt.Errorf("decode corpus_near args: %w", err)
			}
			of := strings.TrimSpace(in.Of)
			if of == "" {
				return "", fmt.Errorf("corpus_near: of is required: a capture id, a publication id or a text")
			}
			owner, computer, err := corpusScope(ctx, "corpus_near")
			if err != nil {
				return "", err
			}
			c, err := corpusCaller(deps, "corpus_near")
			if err != nil {
				return "", err
			}
			body := map[string]any{"owner_id": owner, "computer_id": computer, "k": in.K, "since": in.Since, "until": in.Until}
			switch {
			case isCaptureID(of):
				body["capture_id"] = of
			case isPublicationID(of):
				body["story_id"] = of
			default:
				body["text"] = of
			}
			out, err := c.CorpusCall(ctx, "near", body)
			return string(out), err
		}}
}

// isCaptureID: a capture's canonical id, "obj:choir.web_capture:<owner>:<suffix>".
func isCaptureID(s string) bool { return strings.HasPrefix(s, "obj:choir.web_capture:") }

// isPublicationID: a published story's id, "pub-" and 32 hex digits.
func isPublicationID(s string) bool {
	rest, ok := strings.CutPrefix(s, "pub-")
	if !ok || len(rest) != 32 {
		return false
	}
	return strings.Trim(rest, "0123456789abcdef") == ""
}

func newCorpusEdgesTool(deps Dependencies) toolregistry.Tool {
	type args struct {
		Ref string `json:"ref"`
	}
	return toolregistry.Tool{Name: "corpus_edges",
		Description: "The edges around a capture, a published story or one of its versions.",
		Parameters:  toolregistry.JSONSchemaObject(map[string]any{"ref": map[string]any{"type": "string"}}, []string{"ref"}, false),
		Func: func(ctx context.Context, raw json.RawMessage) (string, error) {
			var in args
			if err := json.Unmarshal(raw, &in); err != nil {
				return "", fmt.Errorf("decode corpus_edges args: %w", err)
			}
			owner, computer, err := corpusScope(ctx, "corpus_edges")
			if err != nil {
				return "", err
			}
			c, err := corpusCaller(deps, "corpus_edges")
			if err != nil {
				return "", err
			}
			out, err := c.CorpusCall(ctx, "edges", map[string]any{"owner_id": owner, "computer_id": computer, "ref": strings.TrimSpace(in.Ref)})
			return string(out), err
		}}
}

func newCorpusTextsTool(deps Dependencies) toolregistry.Tool {
	type args struct {
		IDs      []string `json:"ids"`
		MaxChars int      `json:"max_chars,omitempty"`
	}
	return toolregistry.Tool{Name: "corpus_texts",
		Description: "The text of many captures of this computer in one call.",
		Parameters: toolregistry.JSONSchemaObject(map[string]any{
			"ids": map[string]any{"type": "array", "items": map[string]any{"type": "string"}}, "max_chars": map[string]any{"type": "integer"},
		}, []string{"ids"}, false),
		Func: func(ctx context.Context, raw json.RawMessage) (string, error) {
			var in args
			if err := json.Unmarshal(raw, &in); err != nil {
				return "", fmt.Errorf("decode corpus_texts args: %w", err)
			}
			if len(in.IDs) == 0 || len(in.IDs) > corpusTextMaxIDs {
				return "", fmt.Errorf("corpus_texts: name 1 to %d capture ids, got %d", corpusTextMaxIDs, len(in.IDs))
			}
			owner, computer, err := corpusScope(ctx, "corpus_texts")
			if err != nil {
				return "", err
			}
			c, err := corpusCaller(deps, "corpus_texts")
			if err != nil {
				return "", err
			}
			// corpusd reads them under the scope and eligibility of every
			// corpus list (not the older read by id, which serves any capture
			// id to any bound guest) and cuts the answer at its byte bound.
			out, err := c.CorpusCall(ctx, "texts", map[string]any{"owner_id": owner, "computer_id": computer, "ids": in.IDs, "max_chars": in.MaxChars})
			return string(out), err
		}}
}

// CorpusTextLabels reads the provenance labels the captures of a corpus_texts
// answer carry, for the run's labels (agentcore/run_labels.go).
func CorpusTextLabels(result json.RawMessage) []string {
	var a struct {
		Texts []struct {
			Labels []string `json:"labels"`
		} `json:"texts"`
	}
	if json.Unmarshal(result, &a) != nil {
		return nil
	}
	var out []string
	for _, t := range a.Texts {
		out = append(out, t.Labels...)
	}
	return out
}
