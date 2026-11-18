package search

import (
	"context"
	"strings"
)

// Attribution says whose search a request is: the run that asked, the desk
// and the conversation. The gateway logs it so a search can be tied to the
// run that spent it (docs/problems/search-volume-outruns-keenable-credits-2026-11-18.md).
// It travels in headers, not in the request body, so an old gateway ignores
// it and an old caller without it still works. Ids and names only.
type Attribution struct {
	RunID        string
	Desk         string
	Conversation string
}

// attributionMax bounds each value.
const attributionMax = 64

type attributionKey struct{}

// WithAttribution returns ctx carrying a, each value cleaned to printable
// characters without spaces and cut to 64.
func WithAttribution(ctx context.Context, a Attribution) context.Context {
	a.RunID, a.Desk, a.Conversation = cleanAttribution(a.RunID), cleanAttribution(a.Desk), cleanAttribution(a.Conversation)
	return context.WithValue(ctx, attributionKey{}, a)
}

// AttributionFrom returns the attribution ctx carries (zero when none).
func AttributionFrom(ctx context.Context) Attribution {
	a, _ := ctx.Value(attributionKey{}).(Attribution)
	return a
}

func cleanAttribution(s string) string {
	s = strings.Map(func(r rune) rune {
		if r <= ' ' || r == 0x7f || r > 0x7e {
			return -1
		}
		return r
	}, s)
	if len(s) > attributionMax {
		s = s[:attributionMax]
	}
	return s
}

// The header names the gateway reads.
const (
	HeaderRun          = "X-Choir-Run"
	HeaderDesk         = "X-Choir-Desk"
	HeaderConversation = "X-Choir-Conversation"
)
