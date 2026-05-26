package searchplane

import (
	"net/url"
	"strings"
)

type providerBatch struct {
	provider string
	results  []Result
}

func appendBatchResult(batches []providerBatch, provider string, result Result) []providerBatch {
	for i := range batches {
		if batches[i].provider == provider {
			batches[i].results = append(batches[i].results, result)
			return batches
		}
	}
	return append(batches, providerBatch{provider: provider, results: []Result{result}})
}

func mergeBatches(batches []providerBatch, maxResults int) []Result {
	if maxResults <= 0 || len(batches) == 0 {
		return nil
	}
	results := make([]Result, 0, maxResults)
	seenURLs := make(map[string]struct{})
	positions := make([]int, len(batches))
	for len(results) < maxResults {
		advanced := false
		for i := range batches {
			for positions[i] < len(batches[i].results) {
				result := batches[i].results[positions[i]]
				positions[i]++
				key := normalizeResultURL(result.URL)
				if key == "" {
					continue
				}
				if _, exists := seenURLs[key]; exists {
					continue
				}
				seenURLs[key] = struct{}{}
				results = append(results, result)
				advanced = true
				break
			}
			if len(results) >= maxResults {
				break
			}
		}
		if !advanced {
			break
		}
	}
	return results
}

func normalizeResultURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	u, err := url.Parse(raw)
	if err != nil {
		return strings.ToLower(raw)
	}
	u.Fragment = ""
	u.RawQuery = ""
	host := strings.ToLower(u.Host)
	path := strings.TrimSuffix(u.EscapedPath(), "/")
	if path == "" {
		path = "/"
	}
	return host + path
}
