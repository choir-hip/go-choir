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

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
)

const publicationExportPath = "/api/platform/publications/export"

// ownRoute is the bare publication route a captured URL names on host, or
// "" when the URL is not a page of the paper's own stories.
func ownRoute(rawURL, host string) string {
	u, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || host == "" || asciiLowerHost(u.Host) != asciiLowerHost(host) {
		return ""
	}
	route := u.Path
	if u.Path == publicationExportPath {
		route = u.Query().Get("route")
	}
	bare, _, ok := parsePublicationRoute(route)
	if !ok || !strings.HasPrefix(bare, publicTexturePrefix) {
		return ""
	}
	return bare
}

// relianceLogMax bounds the skipped candidates one log line names.
const relianceLogMax = 5

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
	var skipped []string
	captures, ownHost := 0, 0
	skip := func(code, route string) {
		if route != "" {
			code += ":" + route
		}
		skipped = append(skipped, code)
	}
	for _, captureID := range citedCaptureIDs(params.Transclusions) {
		captures++
		capture, err := p.store.GetObject(ctx, captureID)
		if err != nil || capture.Tombstone {
			skip("capture_unreadable", "")
			continue
		}
		meta, err := objectgraph.WebCaptureMetadataFromObject(capture)
		if err != nil {
			skip("capture_unreadable", "")
			continue
		}
		route := ownRoute(meta.URL, params.OwnHost)
		if route == "" {
			route = ownRoute(meta.CanonicalURL, params.OwnHost)
		}
		if route == "" {
			skip("not_own_host", "")
			continue
		}
		ownHost++
		routeID, err := objectgraph.BuildCanonicalID("choir.public_route", params.OwnerID, objectgraph.StableSuffixFromKey(route))
		if err != nil {
			skip("route_not_found", route)
			continue
		}
		routeObj, err := p.store.GetObject(ctx, routeID)
		if err != nil || routeObj.Tombstone {
			if other, oerr := p.store.GetObjectByMetadata(ctx, "choir.public_route", "$.route_path", route); oerr == nil && !other.Tombstone && other.OwnerID != params.OwnerID {
				skip("different_owner", route)
			} else {
				skip("route_not_found", route)
			}
			continue
		}
		target := routeTarget(routeObj.Metadata)
		if target == "" {
			skip("route_not_found", route)
			continue
		}
		if target == params.PublicationCanonicalID {
			skip("self", route)
			continue
		}
		if seen[target] {
			skip("duplicate", route)
			continue
		}
		pub, err := p.store.GetObject(ctx, target)
		if err != nil || pub.Tombstone || !publicationIsPublished(pub.Metadata) {
			skip("not_published", route)
			continue
		}
		seen[target] = true
		edges = append(edges, makeEdge(params.PublicationCanonicalID, target, objectgraph.EdgeReliesOn, now, []byte(objectgraph.ReliesOnMetadata)))
	}
	if ownHost > 0 {
		relianceLog(params, captures, ownHost, len(edges), skipped)
	}
	return edges
}

// relianceLog is the step's one log line: the citing publication's id, counts
// and the first few reason codes with route paths. It carries no story text and
// no URL. The link is data only, so staging cannot show it by any read; this
// line is how a deploy proof sees that the step ran. It is not a reader of the
// link: nothing ranks, lists or prompts by it.
func relianceLog(params PublishGraphParams, captures, ownHost, edges int, skipped []string) {
	shown := skipped
	more := ""
	if len(shown) > relianceLogMax {
		more = " more=" + strconv.Itoa(len(shown)-relianceLogMax)
		shown = shown[:relianceLogMax]
	}
	log.Printf("corpusd: reliance: publication=%s captures=%d own_host=%d edges=%d skipped=[%s]%s",
		params.PublicationID, captures, ownHost, edges, strings.Join(shown, " "), more)
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
