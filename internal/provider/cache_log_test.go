package provider

import (
	"bytes"
	"encoding/json"
	"log"
	"regexp"
	"strings"
	"testing"
)

// The chatgpt usage line must carry the cache key, its label, the input item
// count and the two prefix hashes, so analysis can tell prompt drift from
// backend routing (docs/archive/reports/chatgpt-cache-ratio-2026-10-15.md, lever 1).
// Failure modes pinned: a field is missing; call N's prefix_hash_prev differs
// from call N-1's prefix_hash although the bytes only grew; prompt text, tool
// output or a non-hash conversation id reaches the log.
func TestChatGPTUsageLineCarriesCacheKeyAndPrefixHashes(t *testing.T) {
	const (
		secretSystem = "SECRET-SYSTEM-PROMPT-TEXT"
		secretUser   = "SECRET-USER-QUESTION"
		secretOutput = "SECRET-TOOL-OUTPUT"
		secretArgs   = "SECRET-TOOL-ARGUMENT"
		runID        = "run-owner-1234-computer-5678"
		key          = "choir-texture-0123456789ab-ba9876543210"
	)
	p := &ChatGPTProvider{}
	call1 := LLMRequest{
		System:   secretSystem,
		CacheKey: key,
		Messages: []Message{
			{Role: "user", Content: []Block{{Type: "text", Text: secretUser}}},
		},
	}
	call2 := call1
	call2.Messages = append(append([]Message{}, call1.Messages...),
		Message{Role: "assistant", Content: []Block{{Type: "tool_use", ID: "call_A", Name: "go_eval", Input: json.RawMessage(`{"source":"` + secretArgs + `"}`)}}},
		Message{Role: "user", Content: []Block{{Type: "tool_result", ToolUseID: "call_A", Text: secretOutput}}},
	)
	call3 := call2
	call3.Messages = append(append([]Message{}, call2.Messages...),
		Message{Role: "assistant", Content: []Block{{Type: "text", Text: "SECRET-ASSISTANT-TEXT"}, {Type: "tool_use", ID: "call_B", Name: "go_eval", Input: json.RawMessage(`{}`)}}},
		Message{Role: "user", Content: []Block{{Type: "tool_result", ToolUseID: "call_B", Text: secretOutput}}},
	)
	noKey := call1
	noKey.CacheKey = ""
	noKey.ConversationID = runID

	stream := "data: " + `{"type":"response.completed","response":{"id":"r","model":"gpt-6-luna","usage":{"input_tokens":100,"output_tokens":5,"input_tokens_details":{"cached_tokens":64}}}}` + "\n\n"
	prev := log.Writer()
	defer log.SetOutput(prev)
	run := func(req LLMRequest) string {
		var buf bytes.Buffer
		log.SetOutput(&buf)
		defer log.SetOutput(prev)
		body := p.buildRequestBody(req, "gpt-6-luna")
		if _, err := parseOpenAIStreamLogged(strings.NewReader(stream), "gpt-6-luna", "chatgpt", func(StreamChunk) {}, newCacheLogMeta(body)); err != nil {
			t.Fatal(err)
		}
		return buf.String()
	}
	fieldRe := func(name string) *regexp.Regexp { return regexp.MustCompile(name + `=(\S+)`) }
	field := func(line, name string) string {
		m := fieldRe(name).FindStringSubmatch(line)
		if m == nil {
			t.Fatalf("field %s missing from %q", name, line)
		}
		return m[1]
	}

	l1, l2, l3, lNoKey := run(call1), run(call2), run(call3), run(noKey)

	for _, line := range []string{l1, l2, l3, lNoKey} {
		for _, secret := range []string{secretSystem, secretUser, secretOutput, secretArgs, "SECRET-ASSISTANT-TEXT", runID} {
			if strings.Contains(line, secret) {
				t.Fatalf("usage line leaks %q: %s", secret, line)
			}
		}
		if !strings.Contains(line, "cached_in=64") {
			t.Fatalf("existing usage fields lost: %s", line)
		}
	}

	if got := field(l1, "cache_key"); got != key {
		t.Fatalf("cache_key = %q, want %q", got, key)
	}
	if got := field(l1, "cache_label"); got != "texture" {
		t.Fatalf("cache_label = %q, want texture", got)
	}
	for line, want := range map[string]string{l1: "1", l2: "3", l3: "6"} {
		if got := field(line, "input_messages"); got != want {
			t.Fatalf("input_messages = %s, want %s: %s", got, want, line)
		}
	}
	if got := field(l1, "prefix_hash_prev"); got != "-" {
		t.Fatalf("first call prefix_hash_prev = %q, want -", got)
	}
	// Prefix chain: call N's prefix_hash_prev is call N-1's prefix_hash.
	if field(l2, "prefix_hash_prev") != field(l1, "prefix_hash") {
		t.Fatalf("call 2 prev != call 1 hash:\n%s\n%s", l1, l2)
	}
	if field(l3, "prefix_hash_prev") != field(l2, "prefix_hash") {
		t.Fatalf("call 3 prev != call 2 hash:\n%s\n%s", l2, l3)
	}
	if field(l1, "prefix_hash") == field(l2, "prefix_hash") || field(l2, "prefix_hash") == field(l3, "prefix_hash") {
		t.Fatal("prefix_hash did not change as input grew")
	}
	if !regexp.MustCompile(`^[0-9a-f]{16}$`).MatchString(field(l3, "prefix_hash")) {
		t.Fatalf("prefix_hash is not a short hex hash: %s", l3)
	}

	// No cache key: the conversation id fallback is logged as a hash only.
	if got := field(lNoKey, "cache_key"); !strings.HasPrefix(got, "h:") || strings.Contains(got, "owner") {
		t.Fatalf("fallback cache_key = %q, want h:<hash>", got)
	}
	if got := field(lNoKey, "cache_label"); got != "-" {
		t.Fatalf("fallback cache_label = %q, want -", got)
	}

	// Drift is visible: changing the system prompt changes the chain.
	drift := call2
	drift.System = secretSystem + "x"
	if field(run(drift), "prefix_hash_prev") == field(l1, "prefix_hash") {
		t.Fatal("system prompt drift not reflected in the prefix hash")
	}
}

func TestLoggableCacheKeyLabelsFailoverKeys(t *testing.T) {
	cases := []struct{ key, label string }{
		{"choir-management-abc123-def456", "management"},
		{"choir-management-abc123-def456-failover", "management+failover"},
		{"choir-research_child-abc123-def456-failover2", "research_child+failover2"},
		{"choir-management-abc123-def456-m1a2b3c", "management+m1a2b3c"},
		{"choir-texture-abc123-def456-m1a2b3g", "-"},
		{"conversation-1234", "-"},
	}
	for _, c := range cases {
		got, label := loggableCacheKey(c.key)
		if label != c.label {
			t.Errorf("loggableCacheKey(%q) label = %q, want %q", c.key, label, c.label)
		}
		if c.label == "-" && !strings.HasPrefix(got, "h:") {
			t.Errorf("loggableCacheKey(%q) = %q, want an h: hash", c.key, got)
		}
	}
}
