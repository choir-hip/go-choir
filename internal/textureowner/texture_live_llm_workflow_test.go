//go:build integration || comprehensive

package textureowner_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)


func TestLiveLLMProviderInfo(t *testing.T) {
	authPath := strings.TrimSpace(os.Getenv("CHATGPT_AUTH_PATH"))
	if authPath == "" {
		home, _ := os.UserHomeDir()
		authPath = filepath.Join(home, ".codex", "auth.json")
	}
	if _, err := os.Stat(authPath); err == nil {
		fmt.Fprintf(os.Stderr, "live ChatGPT auth file available at %s; set GO_CHOIR_LIVE_LLM=1 and GO_CHOIR_LIVE_LLM_FAKE_SEARCH=1 to run the live-LLM/fake-search dry-run\n", authPath)
		return
	}
	t.Log("No ChatGPT/Codex auth file found; live workflow test will be skipped unless CHATGPT_AUTH_PATH is configured.")
}
