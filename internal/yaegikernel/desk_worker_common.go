package yaegikernel

import (
	"strings"
)

// joinCSV/splitCSV render the package allowlist as a worker env value.
func joinCSV(values []string) string { return strings.Join(values, ",") }
func splitCSV(value string) []string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}
