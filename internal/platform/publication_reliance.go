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
	"net/url"
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
	if !ok {
		return ""
	}
	return bare
}

// reliesOnEdges are the publish batch's reliance links: one per distinct
// earlier published publication of the same owner that a cited capture
// names. A capture that cannot be read, names no route, names a route that
// is not an active published publication, or names the publication itself,
// writes nothing; none of them fails the publish.
func (p *PublicationGraphStore) reliesOnEdges(ctx context.Context, params PublishGraphParams, now time.Time) []objectgraph.Edge {
	if p == nil || p.store == nil || params.OwnHost == "" {
		return nil
	}
	var edges []objectgraph.Edge
	seen := map[string]bool{}
	for _, captureID := range citedCaptureIDs(params.Transclusions) {
		capture, err := p.store.GetObject(ctx, captureID)
		if err != nil || capture.Tombstone {
			continue
		}
		meta, err := objectgraph.WebCaptureMetadataFromObject(capture)
		if err != nil {
			continue
		}
		route := ownRoute(meta.URL, params.OwnHost)
		if route == "" {
			route = ownRoute(meta.CanonicalURL, params.OwnHost)
		}
		if route == "" {
			continue
		}
		routeID, err := objectgraph.BuildCanonicalID("choir.public_route", params.OwnerID, objectgraph.StableSuffixFromKey(route))
		if err != nil {
			continue
		}
		routeObj, err := p.store.GetObject(ctx, routeID)
		if err != nil || routeObj.Tombstone {
			continue
		}
		target := routeTarget(routeObj.Metadata)
		if target == "" || target == params.PublicationCanonicalID || seen[target] {
			continue
		}
		pub, err := p.store.GetObject(ctx, target)
		if err != nil || pub.Tombstone || !publicationIsPublished(pub.Metadata) {
			continue
		}
		seen[target] = true
		edges = append(edges, makeEdge(params.PublicationCanonicalID, target, objectgraph.EdgeReliesOn, now, []byte(objectgraph.ReliesOnMetadata)))
	}
	return edges
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
