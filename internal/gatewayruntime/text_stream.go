package gatewayruntime

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/provideriface"
)

// StreamText implements provideriface.TextStreamProvider: one streamed call
// through the gateway (handleStreamingInference), no tools, text deltas only.
//
// It does not retry and does not wait out a rate limit: the caller is a spoken
// answer, which walks to its next pool member instead. A refusal the gateway
// writes before it streams (the typed provider quota, any non-200) comes back
// as an error before any delta.
//
// The stream must end with the gateway's [DONE] marker. The gateway reports an
// upstream failure after it has begun to stream as a data line {"error":...}
// with no type, and a dropped connection ends the body with no marker; both
// are errors here, because a spoken answer cut short must not be taken for a
// finished one. (parseGatewaySSE, which the desk loop's Execute uses, skips
// the untyped error line.)
func (p *Provider) StreamText(ctx context.Context, req provideriface.TextStreamRequest, onDelta func(string)) (*provideriface.TextStreamResponse, error) {
	content := []block{{Type: "text", Text: req.User}}
	if req.StableUser != "" {
		// The stable prefix is its own block, marked as the end of the part a
		// provider may cache; the volatile rest follows it.
		content = []block{{Type: "text", Text: req.StableUser, CacheBreak: true}, {Type: "text", Text: req.User}}
	}
	llmReq := llmRequest{
		Provider:        firstNonEmpty(req.Provider, p.llmProvider),
		Model:           firstNonEmpty(req.Model, p.llmModel),
		System:          req.System,
		ReasoningOff:    req.ReasoningOff,
		Messages:        []message{{Role: "user", Content: content}},
		MaxTokens:       req.MaxTokens,
		Stream:          true,
		ReasoningEffort: firstNonEmpty(req.ReasoningEffort, p.reasoningEffort),
		ConversationID:  req.ConversationID,
		CacheKey:        req.CacheKey,
	}
	httpResp, err := p.do(ctx, llmReq, "text/event-stream")
	if err != nil {
		return nil, err
	}
	defer func() { _ = httpResp.Body.Close() }()
	if httpResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(httpResp.Body, 1<<16))
		return nil, gatewayStatusError(httpResp.Status, body)
	}
	return readTextStream(ctx, httpResp.Body, onDelta)
}

// readTextStream reads the gateway's SSE body (see StreamText).
func readTextStream(ctx context.Context, body io.Reader, onDelta func(string)) (*provideriface.TextStreamResponse, error) {
	out := &provideriface.TextStreamResponse{}
	var text strings.Builder
	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	done := false
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data: "))
		if data == "" {
			continue
		}
		if data == "[DONE]" {
			done = true
			break
		}
		var chunk struct {
			streamChunk
			Error string `json:"error"`
		}
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
		}
		if chunk.Error != "" {
			return nil, fmt.Errorf("gateway client: stream error: %s", chunk.Error)
		}
		if chunk.Type == "error" {
			return nil, fmt.Errorf("gateway client: stream error: %s", chunk.Delta)
		}
		if chunk.Model != "" {
			out.Model = chunk.Model
		}
		if chunk.StopReason != "" {
			out.StopReason = chunk.StopReason
		}
		if chunk.Usage != nil {
			// An Anthropic-style stream reports input tokens in one chunk
			// (message_start) and output tokens in a later one
			// (message_delta, input 0); a non-zero count is never replaced by
			// zero. An OpenAI-style stream sends one merged chunk.
			if chunk.Usage.InputTokens > 0 {
				out.Usage.InputTokens = chunk.Usage.InputTokens
			}
			if chunk.Usage.OutputTokens > 0 {
				out.Usage.OutputTokens = chunk.Usage.OutputTokens
			}
			if chunk.Usage.CacheReported {
				if chunk.Usage.CachedInputTokens > 0 || !out.Usage.CacheReported {
					out.Usage.CachedInputTokens = chunk.Usage.CachedInputTokens
				}
				out.Usage.CacheReported = true
			}
		}
		if chunk.Delta != "" {
			text.WriteString(chunk.Delta)
			onDelta(chunk.Delta)
		}
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("gateway client: read stream: %w", err)
	}
	if !done {
		return nil, errors.New("gateway client: stream ended before its completion marker")
	}
	out.Text = text.String()
	return out, nil
}
