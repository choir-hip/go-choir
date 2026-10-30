package provider

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
)

// cacheLogMeta is what the per-call usage line says about the request, so the
// journal can separate "our prompt bytes drifted between calls" from "the
// backend routed the call to a cold cache"
// (docs/archive/reports/chatgpt-cache-ratio-2026-10-15.md, lever 1). Hashes and
// counts only: no prompt text, tool output or argument value is kept.
type cacheLogMeta struct {
	key        string
	label      string
	messages   int
	prefixHash string
	prevHash   string
}

// newCacheLogMeta summarises a chatgpt Responses request.
//
// prefix_hash is the short SHA-256 of the full input, hashed in wire order:
// instructions, the tool definitions, then each input item as JSON. It is the
// prefix the next call of the thread must extend.
//
// prefix_hash_prev is the same hash with the last appended turn removed. The
// last appended turn is the trailing run of non-model items (tool outputs,
// user messages) plus the block of model-output items (function calls,
// assistant messages) just before it; that is what the previous call's
// response and its follow-up added. For consecutive calls of one thread,
// call N's prefix_hash_prev equals call N-1's prefix_hash when our bytes are
// stable. It is "-" when the input holds no model output (the first call).
//
// One extra marshal and one SHA-256 pass per call.
func newCacheLogMeta(body openAIRequest) *cacheLogMeta {
	meta := &cacheLogMeta{messages: len(body.Input), prevHash: "-"}
	meta.key, meta.label = loggableCacheKey(body.PromptCacheKey)

	cut := -1
	last := len(body.Input) - 1
	for last >= 0 && !isModelOutputItem(body.Input[last]) {
		last--
	}
	if last >= 0 {
		cut = last
		for cut > 0 && isModelOutputItem(body.Input[cut-1]) {
			cut--
		}
	}

	h := sha256.New()
	h.Write([]byte(body.Instructions))
	h.Write([]byte{0})
	toolsJSON, _ := json.Marshal(body.Tools)
	h.Write(toolsJSON)
	for i, item := range body.Input {
		if i == cut {
			meta.prevHash = shortHashSum(h.Sum(nil))
		}
		h.Write([]byte{0})
		itemJSON, _ := json.Marshal(item)
		h.Write(itemJSON)
	}
	meta.prefixHash = shortHashSum(h.Sum(nil))
	return meta
}

func isModelOutputItem(item openAIItem) bool {
	return item.Type == "function_call" || (item.Type == "" && item.Role == "assistant")
}

func shortHashSum(sum []byte) string {
	return hex.EncodeToString(sum)[:16]
}

// loggableCacheKey returns the key to log and its label. A key built by
// toolregistry.PromptCacheKey ("choir-<label>-<scope hash>-<prompt hash>")
// carries only a sanitised label and hashes, so it is logged as sent. Any
// other key (the fallback is the run's conversation id) is logged as a short
// hash so an id never reaches the journal.
func loggableCacheKey(key string) (string, string) {
	key = strings.TrimSpace(key)
	if key == "" {
		return "-", "-"
	}
	if rest, ok := strings.CutPrefix(key, "choir-"); ok {
		parts := strings.Split(rest, "-")
		// A model failover appends "-m<6 hex>", the tag of the serving
		// provider/model (toolregistry failoverCacheKey); older builds
		// appended "-failover<N>". The label carries the suffix so failover
		// calls are counted apart from keyless ones.
		if n := len(parts); n == 4 && parts[0] != "" && (strings.HasPrefix(parts[3], "failover") || isModelTag(parts[3])) {
			return key, parts[0] + "+" + parts[3]
		}
		if len(parts) == 3 && parts[0] != "" {
			return key, parts[0]
		}
	}
	sum := sha256.Sum256([]byte(key))
	return "h:" + shortHashSum(sum[:]), "-"
}

// isModelTag reports whether s is a failover model tag: "m" and six hex digits.
func isModelTag(s string) bool {
	if len(s) != 7 || s[0] != 'm' {
		return false
	}
	for _, c := range s[1:] {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return false
		}
	}
	return true
}

// logFields renders the extra fields for the usage line; empty for a nil meta.
func (m *cacheLogMeta) logFields() string {
	if m == nil {
		return ""
	}
	return fmt.Sprintf(" cache_key=%s cache_label=%s input_messages=%d prefix_hash=%s prefix_hash_prev=%s",
		m.key, m.label, m.messages, m.prefixHash, m.prevHash)
}
