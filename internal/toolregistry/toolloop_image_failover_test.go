package toolregistry

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/provideriface"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// Failure modes of image-aware failover (docs/design/desk-shots-and-review-
// 2026-11-07.md section 3.4): an image turn calls a text-only member and the
// gateway refuses it; failover after an error lands on a text-only member;
// the pre-call move happens without the trace saying why; a run with no
// image-capable member fails with the gateway's refusal instead of a named
// error; a pinned or NoFailover model (no fallbacks) gets rerouted or
// reaches the gateway; a text-only turn changes behaviour.

var (
	textOnlyA = provideriface.LLMSelection{Provider: "opencode-go", Model: "beam-free"}
	textOnlyB = provideriface.LLMSelection{Provider: "opencode-go", Model: "iceman-contributor-free"}
	visionOK  = provideriface.LLMSelection{Provider: "chatgpt", Model: "gpt-6-luna"}
)

var imageMessages = []json.RawMessage{
	json.RawMessage(`{"role":"user","content":[{"type":"text","text":"look"},{"type":"image","source":{"kind":"artifact_ref"}}]}`),
}
var textMessages = []json.RawMessage{json.RawMessage(`{"role":"user","content":"hello"}`)}

func runImageLoop(provider *modelFailingProvider, messages []json.RawMessage, cur provideriface.LLMSelection, fallbacks ...provideriface.LLMSelection) (string, error, []map[string]any) {
	var events []map[string]any
	emit := func(kind types.EventKind, name string, payload json.RawMessage) {
		if name == "model_failover" {
			m := map[string]any{}
			_ = json.Unmarshal(payload, &m)
			events = append(events, m)
		}
	}
	text, _, err := RunToolLoop(context.Background(), provider, nil, messages, "system", 0, emit, nil,
		WithToolLoopLLMConfig(cur),
		WithToolLoopFallbackLLMConfigs(fallbacks),
		WithToolLoopConversationID("run-1"))
	return text, err, events
}

func TestImageTurnOnTextOnlyMemberMovesToFirstCapableBeforeCalling(t *testing.T) {
	p := &modelFailingProvider{}
	text, err, events := runImageLoop(p, imageMessages, textOnlyA, textOnlyB, visionOK)
	if err != nil || text != "done by gpt-6-luna" {
		t.Fatalf("text %q err %v", text, err)
	}
	if len(p.requests) != 1 || p.requests[0].Model != "gpt-6-luna" {
		t.Fatalf("requests = %d, want one call to gpt-6-luna and no doomed call", len(p.requests))
	}
	if len(events) != 1 || events[0]["from_model"] != "beam-free" || events[0]["to_model"] != "gpt-6-luna" {
		t.Fatalf("failover events = %v", events)
	}
	if r, _ := events[0]["provider_error"].(string); !strings.Contains(r, "image") {
		t.Fatalf("event does not say why: %v", events[0])
	}
}

func TestImageTurnFailoverSkipsTextOnlyMembers(t *testing.T) {
	p := &modelFailingProvider{fail: map[string]error{"stealth/crouton-alpha": fmt.Errorf("status 503")}}
	crouton := provideriface.LLMSelection{Provider: "openrouter", Model: "stealth/crouton-alpha"}
	text, err, events := runImageLoop(p, imageMessages, crouton, textOnlyA, textOnlyB, visionOK)
	if err != nil || text != "done by gpt-6-luna" {
		t.Fatalf("text %q err %v", text, err)
	}
	for _, r := range p.requests {
		if r.Model == "beam-free" || r.Model == "iceman-contributor-free" {
			t.Fatalf("image sent to text-only %s", r.Model)
		}
	}
	if len(events) != 1 || events[0]["to_model"] != "gpt-6-luna" {
		t.Fatalf("events = %v", events)
	}
}

func TestImageTurnWithNoCapableMemberFailsNamed(t *testing.T) {
	p := &modelFailingProvider{}
	_, err, _ := runImageLoop(p, imageMessages, textOnlyA, textOnlyB)
	if err == nil || !strings.Contains(err.Error(), "no model in this run's order can take image input: tried") ||
		!strings.Contains(err.Error(), "beam-free") || !strings.Contains(err.Error(), "iceman-contributor-free") {
		t.Fatalf("err = %v", err)
	}
	if len(p.requests) != 0 {
		t.Fatalf("made %d doomed calls", len(p.requests))
	}
}

func TestImageTurnFailoverWithOnlyTextOnlyLeftFailsNamed(t *testing.T) {
	p := &modelFailingProvider{fail: map[string]error{"gpt-6-luna": fmt.Errorf("status 503")}}
	_, err, _ := runImageLoop(p, imageMessages, visionOK, textOnlyA)
	if err == nil || !strings.Contains(err.Error(), "no model in this run's order can take image input") {
		t.Fatalf("err = %v", err)
	}
	for _, r := range p.requests {
		if r.Model == "beam-free" {
			t.Fatal("image sent to text-only member")
		}
	}
}

// A pin and a NoFailover pool both arrive as a run with no fallbacks.
func TestImageTurnOnPinnedTextOnlyModelFailsWithoutReroute(t *testing.T) {
	p := &modelFailingProvider{}
	_, err, events := runImageLoop(p, imageMessages, textOnlyA)
	if err == nil || !strings.Contains(err.Error(), "no model in this run's order can take image input") {
		t.Fatalf("err = %v", err)
	}
	if len(p.requests) != 0 || len(events) != 0 {
		t.Fatalf("requests %d events %v", len(p.requests), events)
	}
}

func TestTextOnlyTurnIsUnchanged(t *testing.T) {
	p := &modelFailingProvider{}
	text, err, events := runImageLoop(p, textMessages, textOnlyA, visionOK)
	if err != nil || text != "done by beam-free" || len(events) != 0 || len(p.requests) != 1 {
		t.Fatalf("text %q err %v events %v requests %d", text, err, events, len(p.requests))
	}
	p = &modelFailingProvider{fail: map[string]error{"beam-free": fmt.Errorf("status 503")}}
	text, err, events = runImageLoop(p, textMessages, textOnlyA, textOnlyB)
	if err != nil || text != "done by iceman-contributor-free" || len(events) != 1 {
		t.Fatalf("text failover: %q %v %v", text, err, events)
	}
}
