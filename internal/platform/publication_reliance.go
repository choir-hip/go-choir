package platform

// The reliance link (docs/problems/published-search-recall-2026-11-10.md part 2).
// When a published story cites a capture of an earlier published story of the
// paper, the publish batch also carries one directed relies_on edge from the
// citing publication to the cited one. It is written where the cited captures
// are already visited (beside citesCaptureEdges) and is data only: nothing
// ranks by it and no desk prompt shows it.
//
// A capture names a story by the URL it was fetched from: the plain-text
// export research reads (publicationExportReadURL) or the public page
// (/pub/texture/<slug>[/v<n>]). Only a URL on the paper's own host counts, so
// another site's page with a path that looks like a route is not a story.

import (
	"context"
	"encoding/json"
	"log"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
)

const publicationExportPath = "/api/platform/publications/export"

// ownRoute is the bare publication route a captured URL names on host, or
// "" when the URL is not a page of the paper's own stories.
func ownRoute(rawURL, host string) string {
	route, _ := classifyOwnURL(rawURL, host)
	return route
}

// classifyOwnURL is ownRoute and whether the URL is on host at all, so a log
// can tell another site's page from an own-host URL that names no story.
func classifyOwnURL(rawURL, host string) (route string, onHost bool) {
	u, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || host == "" || asciiLowerHost(u.Host) != asciiLowerHost(host) {
		return "", false
	}
	route = u.Path
	if u.Path == publicationExportPath {
		route = u.Query().Get("route")
	}
	bare, _, ok := parsePublicationRoute(route)
	if !ok || !strings.HasPrefix(bare, publicTexturePrefix) {
		return "", true
	}
	return bare, true
}

const (
	// relianceLogMax bounds the skipped candidates one log line names.
	relianceLogMax = 5
	// relianceRouteMax bounds the bytes of a route in a log entry.
	relianceRouteMax = 160
)

// relianceSkips tallies the candidates that wrote no edge: a total, and the
// first few entries, those that carry a route kept ahead of those that do not.
type relianceSkips struct {
	total     int
	withRoute []string
	bare      []string
}

func (k *relianceSkips) add(code, route string) {
	k.total++
	if route == "" {
		if len(k.bare) < relianceLogMax {
			k.bare = append(k.bare, code)
		}
		return
	}
	if len(k.withRoute) < relianceLogMax {
		k.withRoute = append(k.withRoute, code+":"+relianceRoute(route))
	}
}

// shown is the entries the line names, routes first, at most relianceLogMax.
func (k *relianceSkips) shown() []string {
	out := append(append([]string{}, k.withRoute...), k.bare...)
	if len(out) > relianceLogMax {
		out = out[:relianceLogMax]
	}
	return out
}

// relianceRoute is a route made safe for a log line. The route comes from a
// cited capture's URL, which a page controls (percent-encoded newlines and
// escape bytes decode into it, and a slug is any length). It is cut to
// relianceRouteMax bytes at a rune boundary and written with strconv.Quote:
// that escapes every control character and invalid byte, so the entry cannot
// end the line or forge another, and it stays readable and unambiguous
// (stripping would silently change the route and could be spoofed).
func relianceRoute(route string) string {
	if len(route) > relianceRouteMax {
		cut := relianceRouteMax
		for cut > 0 && !utf8.RuneStart(route[cut]) {
			cut--
		}
		route = route[:cut] + "..."
	}
	return strconv.Quote(route)
}

// reliesOnEdges are the publish batch's reliance links: one per distinct
// earlier published publication of the same owner that a cited capture
// names. A capture that cannot be read, names no route, names a route that
// is not an active published publication, or names the publication itself,
// writes nothing; none of them fails the publish. When a cited capture is on
// the paper's own host the step logs one line (relianceLog).
func (p *PublicationGraphStore) reliesOnEdges(ctx context.Context, params PublishGraphParams, now time.Time) []objectgraph.Edge {
	if p == nil || p.store == nil || params.OwnHost == "" {
		return nil
	}
	var edges []objectgraph.Edge
	seen := map[string]bool{}
	var skips relianceSkips
	captures, ownHost := 0, 0
	for _, captureID := range citedCaptureIDs(params.Transclusions) {
		captures++
		capture, err := p.store.GetObject(ctx, captureID)
		if err != nil || capture.Tombstone {
			skips.add("capture_unreadable", "")
			continue
		}
		meta, err := objectgraph.WebCaptureMetadataFromObject(capture)
		if err != nil {
			skips.add("capture_unreadable", "")
			continue
		}
		route, onHost := classifyOwnURL(meta.URL, params.OwnHost)
		if route == "" {
			var canonicalOnHost bool
			route, canonicalOnHost = classifyOwnURL(meta.CanonicalURL, params.OwnHost)
			onHost = onHost || canonicalOnHost
		}
		if onHost {
			ownHost++
		}
		if route == "" {
			if onHost {
				skips.add("not_story_route", "")
			} else {
				skips.add("not_own_host", "")
			}
			continue
		}
		routeID, err := objectgraph.BuildCanonicalID("choir.public_route", params.OwnerID, objectgraph.StableSuffixFromKey(route))
		if err != nil {
			skips.add("route_not_found", route)
			continue
		}
		routeObj, err := p.store.GetObject(ctx, routeID)
		if err != nil || routeObj.Tombstone {
			if other, oerr := p.store.GetObjectByMetadata(ctx, "choir.public_route", "$.route_path", route); oerr == nil && !other.Tombstone && other.OwnerID != params.OwnerID {
				skips.add("different_owner", route)
			} else {
				skips.add("route_not_found", route)
			}
			continue
		}
		target := routeTarget(routeObj.Metadata)
		if target == "" {
			skips.add("route_not_found", route)
			continue
		}
		if target == params.PublicationCanonicalID {
			skips.add("self", route)
			continue
		}
		if seen[target] {
			skips.add("duplicate", route)
			continue
		}
		pub, err := p.store.GetObject(ctx, target)
		if err != nil || pub.Tombstone || !publicationIsPublished(pub.Metadata) {
			skips.add("not_published", route)
			continue
		}
		seen[target] = true
		edges = append(edges, makeEdge(params.PublicationCanonicalID, target, objectgraph.EdgeReliesOn, now, []byte(objectgraph.ReliesOnMetadata)))
	}
	if ownHost > 0 {
		relianceLog(params, captures, ownHost, len(edges), &skips)
	}
	return edges
}

// relianceLog is the step's one log line: the citing publication's id, counts
// and the first few reason codes with quoted route paths. It carries no story
// text and no URL, and its length is bounded. The link is data only, so
// staging cannot show it by any read; this line is how a deploy proof sees
// that the step ran. It is not a reader of the link: nothing ranks, lists or
// prompts by it.
func relianceLog(params PublishGraphParams, captures, ownHost, edges int, skips *relianceSkips) {
	shown := skips.shown()
	more := ""
	if n := skips.total - len(shown); n > 0 {
		more = " more=" + strconv.Itoa(n)
	}
	log.Printf("corpusd: reliance: publication=%s captures=%d own_host=%d edges=%d skipped_total=%d skipped=[%s]%s",
		params.PublicationID, captures, ownHost, edges, skips.total, strings.Join(shown, " "), more)
}

func routeTarget(metadata []byte) string {
	var m struct {
		TargetID string `json:"target_id"`
		State    string `json:"state"`
	}
	if json.Unmarshal(metadata, &m) != nil || m.State != "active" {
		return ""
	}
	return m.TargetID
}

func publicationIsPublished(metadata []byte) bool {
	var m struct {
		State string `json:"state"`
	}
	return json.Unmarshal(metadata, &m) == nil && m.State == "published"
}

// publicBaseHost is the host part of the public base URL.
func publicBaseHost(base string) string {
	u, err := url.Parse(base)
	if err != nil {
		return ""
	}
	return u.Host
}

// asciiLowerHost lowercases A-Z only: Unicode folding would match a host that
// is not the paper's.
func asciiLowerHost(s string) string {
	b := []byte(s)
	for i, c := range b {
		if c >= 'A' && c <= 'Z' {
			b[i] = c + 'a' - 'A'
		}
	}
	return string(b)
}
