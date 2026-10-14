package llmcost

// Cost is an estimated USD cost for a single LLM call or an aggregation of
// calls. All amounts are approximate and derived from the hardcoded pricing
// table, not from provider invoices.
type Cost struct {
	// USD is the estimated dollar amount.
	USD float64

	// InputTokens is the total input (prompt) token count contributing to
	// this cost.
	InputTokens int

	// OutputTokens is the total output (completion) token count
	// contributing to this cost.
	OutputTokens int

	// CachedInputTokens is the part of InputTokens served from the
	// provider's prompt cache, priced at the cached-input rate.
	CachedInputTokens int

	// CacheSavingsUSD is what the cached tokens would have cost more at the
	// uncached input rate.
	CacheSavingsUSD float64

	// Model is the model identifier used for the estimate. For aggregations
	// that span multiple models this is empty.
	Model string

	// Provider is the provider name for the estimate. For aggregations that
	// span multiple providers this is empty.
	Provider string

	// Found is false when the model had no pricing entry and the estimate is
	// zero purely because pricing is unknown (not because the call was free).
	Found bool
}

// EstimateCall returns the approximate USD cost for a single LLM call given
// the model identifier and input/output token counts. When the model is not
// in the pricing table, the returned Cost has Found=false and USD=0 so callers
// can distinguish unpriced models from genuinely free calls.
func EstimateCall(model string, inputTokens, outputTokens int) Cost {
	return EstimateCallWithCache(model, inputTokens, 0, outputTokens)
}

// EstimateCallWithCache is EstimateCall for a call whose prompt was partly
// served from the provider's prompt cache. inputTokens is the whole prompt;
// cachedInputTokens (clamped to inputTokens) is priced at the model's cached
// rate and the rest at the input rate.
func EstimateCallWithCache(model string, inputTokens, cachedInputTokens, outputTokens int) Cost {
	if cachedInputTokens < 0 {
		cachedInputTokens = 0
	}
	if cachedInputTokens > inputTokens {
		cachedInputTokens = inputTokens
	}
	entry, ok := LookupPricing(model)
	if !ok {
		return Cost{
			InputTokens:       inputTokens,
			OutputTokens:      outputTokens,
			CachedInputTokens: cachedInputTokens,
			Model:             model,
			Found:             false,
		}
	}
	uncached := inputTokens - cachedInputTokens
	usd := float64(uncached)*entry.InputPerMillion/1_000_000 +
		float64(cachedInputTokens)*entry.cachedRate()/1_000_000 +
		float64(outputTokens)*entry.OutputPerMillion/1_000_000
	return Cost{
		USD:               usd,
		InputTokens:       inputTokens,
		OutputTokens:      outputTokens,
		CachedInputTokens: cachedInputTokens,
		CacheSavingsUSD:   float64(cachedInputTokens) * (entry.InputPerMillion - entry.cachedRate()) / 1_000_000,
		Model:             model,
		Provider:          entry.Provider,
		Found:             true,
	}
}

// Add accumulates another Cost into this one. Token counts always add; the USD
// amount adds only when both costs are found (priced). If either side is
// unfound, the result is unfound so unpriced calls are never silently counted
// as zero-cost.
func (c *Cost) Add(other Cost) {
	c.USD += other.USD
	c.InputTokens += other.InputTokens
	c.OutputTokens += other.OutputTokens
	c.CachedInputTokens += other.CachedInputTokens
	c.CacheSavingsUSD += other.CacheSavingsUSD
	if c.Model == "" {
		c.Model = other.Model
	} else if other.Model != "" && c.Model != other.Model {
		c.Model = "" // mixed models
	}
	if c.Provider == "" {
		c.Provider = other.Provider
	} else if other.Provider != "" && c.Provider != other.Provider {
		c.Provider = "" // mixed providers
	}
	c.Found = c.Found && other.Found
}
