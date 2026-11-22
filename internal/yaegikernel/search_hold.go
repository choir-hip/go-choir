package yaegikernel

import (
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/searchquery"
)

// nearOf is a query's near key and its terms at now; both empty when nothing is
// left. The same-terms answer (reuseSearch) is the one hold a run still meets: it
// is correctness, not a need rule.
func nearOf(query string, now time.Time) (string, []string) {
	k := searchquery.NearKeyAt(query, now)
	if k == "" {
		return "", nil
	}
	return k, strings.Fields(k)
}
