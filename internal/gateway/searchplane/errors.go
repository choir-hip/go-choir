package searchplane

import "fmt"

// OutageError is returned when no provider can return merged results.
type OutageError struct {
	Query    string
	Health   map[string]ProviderHealth
	Attempts []Attempt
}

func (e *OutageError) Error() string {
	return fmt.Sprintf("search_outage: no results for query %q", e.Query)
}

// Code returns the stable API error code.
func (e *OutageError) Code() string { return "search_outage" }
