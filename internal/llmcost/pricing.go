// Package llmcost provides approximate LLM API cost estimation and aggregation
// derived from trace events. It implements the M14 conjecture: per-cycle,
// per-article LLM API cost can be tracked through trace events and aggregated
// without adding a separate billing system.
//
// All cost estimates are approximate. The pricing table is hardcoded from
// publicly known provider pricing and changes rarely; this is v0 and does not
// fetch live pricing. Costs are expressed in USD. Unknown models report a zero
// estimate with a Found=false flag so callers can distinguish "free" from
// "unpriced".
package llmcost

import "strings"

// ModelPricing describes the per-1M-token price for a single model. Prices are
// in USD. InputTokens and OutputTokens are the USD cost per one million tokens
// of the respective kind.
type ModelPricing struct {
	// Provider is the canonical provider name ("openai", "anthropic").
	Provider string

	// Model is the canonical model identifier matched against trace event
	// model fields. Matching is case-insensitive and suffix-tolerant so that
	// dated snapshots (e.g. "gpt-4-0613") resolve to the base pricing entry.
	Model string

	// InputPerMillion is the USD price per 1M input (prompt) tokens.
	InputPerMillion float64

	// OutputPerMillion is the USD price per 1M output (completion) tokens.
	OutputPerMillion float64
}

// pricingTable is the hardcoded v0 pricing table. Prices are USD per 1M tokens
// and reflect publicly known OpenAI and Anthropic pricing as of mid-2024.
// This table changes rarely; update it when providers publish new rates.
var pricingTable = []ModelPricing{
	// --- OpenAI ---
	{Provider: "openai", Model: "gpt-4", InputPerMillion: 30.0, OutputPerMillion: 60.0},
	{Provider: "openai", Model: "gpt-4-turbo", InputPerMillion: 10.0, OutputPerMillion: 30.0},
	{Provider: "openai", Model: "gpt-4o", InputPerMillion: 5.0, OutputPerMillion: 15.0},
	{Provider: "openai", Model: "gpt-4o-mini", InputPerMillion: 0.15, OutputPerMillion: 0.60},
	{Provider: "openai", Model: "gpt-3.5-turbo", InputPerMillion: 0.50, OutputPerMillion: 1.50},

	// --- Anthropic ---
	{Provider: "anthropic", Model: "claude-3-opus", InputPerMillion: 15.0, OutputPerMillion: 75.0},
	{Provider: "anthropic", Model: "claude-3-sonnet", InputPerMillion: 3.0, OutputPerMillion: 15.0},
	{Provider: "anthropic", Model: "claude-3.5-sonnet", InputPerMillion: 3.0, OutputPerMillion: 15.0},
	{Provider: "anthropic", Model: "claude-3-haiku", InputPerMillion: 0.25, OutputPerMillion: 1.25},
}

// LookupPricing returns the pricing entry for the given model identifier. The
// match is case-insensitive and prefix-based so dated model snapshots
// (e.g. "gpt-4o-2024-08-06", "claude-3-5-sonnet-20240620") resolve to their
// base pricing entry. Returns the entry and true when a match is found.
func LookupPricing(model string) (ModelPricing, bool) {
	model = strings.ToLower(strings.TrimSpace(model))
	if model == "" {
		return ModelPricing{}, false
	}
	// First try an exact match.
	for _, entry := range pricingTable {
		if strings.ToLower(entry.Model) == model {
			return entry, true
		}
	}
	// Then try prefix matching so dated variants resolve to the base entry.
	// Iterate longest-first so "claude-3.5-sonnet" wins over "claude-3".
	sorted := make([]ModelPricing, len(pricingTable))
	copy(sorted, pricingTable)
	// Simple insertion sort by descending model length.
	for i := 1; i < len(sorted); i++ {
		for j := i; j > 0 && len(sorted[j].Model) > len(sorted[j-1].Model); j-- {
			sorted[j], sorted[j-1] = sorted[j-1], sorted[j]
		}
	}
	for _, entry := range sorted {
		key := strings.ToLower(entry.Model)
		if strings.HasPrefix(model, key) {
			return entry, true
		}
	}
	// Handle the anthropic dated-snapshot naming convention where dots are
	// replaced with hyphens (e.g. "claude-3-5-sonnet-20240620" should match
	// "claude-3.5-sonnet"). Normalize both sides by stripping hyphens and
	// dots before comparing prefixes.
	normalizedModel := stripSeparators(model)
	for _, entry := range sorted {
		key := stripSeparators(strings.ToLower(entry.Model))
		if key != "" && strings.HasPrefix(normalizedModel, key) {
			return entry, true
		}
	}
	return ModelPricing{}, false
}

func stripSeparators(s string) string {
	s = strings.ReplaceAll(s, "-", "")
	s = strings.ReplaceAll(s, ".", "")
	s = strings.ReplaceAll(s, "_", "")
	return s
}

// KnownModels returns the list of models with pricing entries. This is useful
// for API responses that enumerate which models have cost estimates.
func KnownModels() []ModelPricing {
	out := make([]ModelPricing, len(pricingTable))
	copy(out, pricingTable)
	return out
}
