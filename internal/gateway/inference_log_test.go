package gateway

import (
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/provider"
)

func TestInferenceSuccessLineKeepsPrefixAndAddsCacheWriteAndModel(t *testing.T) {
	t.Parallel()
	resp := &provider.LLMResponse{
		ProviderName: "bedrock", Model: "us.anthropic.claude-haiku-5-5", Text: "ok",
		Usage: provider.Usage{InputTokens: 910, OutputTokens: 9, CachedInputTokens: 600, CacheWrite5mTokens: 100, CacheWrite1hTokens: 200},
	}
	got := inferenceSuccessLine("streaming inference succeeded", "vm-1", resp)
	want := "streaming inference succeeded for autoputer vm-1 (provider=bedrock tokens=910+9 cached=600 text_len=2 cache_write=300 cache_write_1h=200 model=us.anthropic.claude-haiku-5-5)"
	if got != want {
		t.Fatalf("line =\n%s\nwant\n%s", got, want)
	}
	if !strings.Contains(got, "tokens=910+9 cached=600 text_len=2") {
		t.Fatal("existing prefix changed")
	}
}

func TestInferenceSuccessLineZeroWritesStillCarriesFields(t *testing.T) {
	t.Parallel()
	resp := &provider.LLMResponse{ProviderName: "chatgpt", Model: "m", Usage: provider.Usage{InputTokens: 5, OutputTokens: 1}}
	got := inferenceSuccessLine("inference succeeded", "vm-2", resp)
	want := "inference succeeded for autoputer vm-2 (provider=chatgpt tokens=5+1 cached=0 text_len=0 cache_write=0 cache_write_1h=0 model=m)"
	if got != want {
		t.Fatalf("line = %s", got)
	}
}
