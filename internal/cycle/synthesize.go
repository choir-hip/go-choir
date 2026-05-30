package cycle

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/provider"
	"github.com/yusefmosiah/go-choir/internal/sources"
)

type Synthesizer struct {
	Provider provider.Provider
	Model    string
}

func NewSynthesizer() (*Synthesizer, error) {
	// DeepSeek v4-flash is hosted on Fireworks AI in the go-choir environment
	model := "accounts/fireworks/models/deepseek-v4-flash"
	p, err := provider.NewFireworksProviderFromEnv(model)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize fireworks provider: %w", err)
	}

	return &Synthesizer{
		Provider: p,
		Model:    model,
	}, nil
}

func (s *Synthesizer) Synthesize(ctx context.Context, clusters [][]sources.Item) (string, error) {
	if len(clusters) == 0 {
		return "", fmt.Errorf("no clusters provided for synthesis")
	}

	// Prepare the prompt
	var sb strings.Builder
	sb.WriteString("You are the lead editor for the Choir Global Wire, the Automatic Newspaper of Record for Planet Earth.\n")
	sb.WriteString("Your mission is to synthesize high-signal global information into a deeply contextualized, multilingual news issue.\n\n")
	sb.WriteString("CORE INSTRUCTIONS:\n")
	sb.WriteString("1. VOLUME: Produce approximately 4,000 words across 5-10 distinct, deeply researched stories.\n")
	sb.WriteString("2. MULTILINGUAL SIGNAL: Prioritize information from non-English sources. Synthesize their content into English but explicitly cite the original source and language.\n")
	sb.WriteString("3. STRUCTURE: For each story, provide: Headline, Summary, What Changed, Why It Matters, Who Disagrees/What is Contested, What Story is Being Pushed, Confidence/Gaps, and What to Watch Next.\n")
	sb.WriteString("4. CITATION: Cite every significant claim inline using source IDs (e.g., [S1], [S2]). Provide a 'Source Notes' section at the end of each story.\n\n")
	sb.WriteString("INPUT DATA (CLUSTERS OF SOURCES):\n")

	for i, cluster := range clusters {
		sb.WriteString(fmt.Sprintf("--- CLUSTER %d ---\n", i+1))
		for j, item := range cluster {
			sourceID := fmt.Sprintf("S%d-%d", i+1, j+1)
			sb.WriteString(fmt.Sprintf("[%s] Title: %s\nURL: %s\nSource: %s\nLanguage: %s\nBody: %s\n\n", 
				sourceID, item.Title, item.URL, item.SourceID, item.Language, item.Body))
		}
	}

	req := provider.LLMRequest{
		Model: s.Model,
		System: "You are the Choir Global Wire synthesis engine. Output high-fidelity, source-backed global news.",
		Messages: []provider.Message{
			{
				Role: "user",
				Content: []provider.Block{
					{
						Type: "text",
						Text: sb.String(),
					},
				},
			},
		},
		MaxTokens: 8192, // High limit for 4,000-word output
	}

	resp, err := s.Provider.Call(ctx, req)
	if err != nil {
		return "", fmt.Errorf("LLM call failed: %w", err)
	}

	return resp.Text, nil
}
