package agentcore

import (
	"encoding/json"
	"log"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/jevbudget"
)

// jevSplit is the plan of one job's Jev calls over n items, in order: the
// half-open item ranges, one range a call, and the limit that forced each
// split after the first call.
type jevSplit struct {
	Ranges [][2]int
	Limits []string
}

// planJevCalls puts as many of the n items as the vendor's limits (jevbudget)
// allow into each call: the fewest calls for items taken in order. It builds
// the request for a range with build and takes the longest range that fits;
// one call when everything fits. A lone item over a limit stands alone (the
// vendor will refuse it; nothing smaller can be asked). Every split names the
// limit that forced it and is logged with its site, so call volume is seen.
// Concurrency and queuing are not remedies: when a split remains, shrink the
// context.
func planJevCalls(site string, n int, build func(lo, hi int) (json.RawMessage, map[string]json.RawMessage, error)) (jevSplit, error) {
	var plan jevSplit
	limitOf := func(lo, hi int) (string, error) {
		state, questions, err := build(lo, hi)
		if err != nil {
			return "", err
		}
		return jevbudget.Limit(state, questions), nil
	}
	for lo := 0; lo < n; {
		hi := n
		limit, err := limitOf(lo, hi)
		if err != nil {
			return plan, err
		}
		if limit != "" {
			// The longest prefix that fits, by bisection (a longer range is
			// never smaller).
			a, b := lo+1, n // a is the answer when nothing longer fits
			for a < b {
				mid := (a + b + 1) / 2
				l, err := limitOf(lo, mid)
				if err != nil {
					return plan, err
				}
				if l == "" {
					a = mid
				} else {
					b = mid - 1
				}
			}
			hi = a
			if limit, err = limitOf(lo, hi+1); err != nil {
				return plan, err
			}
			plan.Limits = append(plan.Limits, limit)
		}
		plan.Ranges = append(plan.Ranges, [2]int{lo, hi})
		lo = hi
	}
	if len(plan.Ranges) > 1 {
		log.Printf("jev split: site=%s items=%d calls=%d limits=%s", site, n, len(plan.Ranges), strings.Join(plan.Limits, ","))
	}
	return plan, nil
}
