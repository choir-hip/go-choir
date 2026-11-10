package proxy

// The links a published revision carries (autopaper contract 2). The proxy
// reads them from the revision's own provenance on the publishing computer,
// never from a request body, and passes them to corpusd, which verifies each
// against the graph before anything is written.

import (
	"encoding/json"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/platform"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// maxPublishLinkWalk bounds the walk up a document's revision chain for the
// link its first revision recorded.
const maxPublishLinkWalk = 64

// maxPublishParents mirrors corpusd's bound.
const maxPublishParents = 8

type publishLinks struct {
	Parents    []platform.PublishParentRef
	RespondsTo *platform.PublishParentRef
}

// derivePublishLinks reads the source links out of one revision's
// provenance. responds_to is not set here: corpusd derives it from the
// verified parents (the first that is an Autopaper story).
func derivePublishLinks(provenance json.RawMessage, _ string) publishLinks {
	var out publishLinks
	if len(provenance) == 0 {
		return out
	}
	var prov types.Provenance
	if json.Unmarshal(provenance, &prov) != nil {
		return out
	}
	seen := map[string]bool{}
	for _, l := range prov.ParentLinks {
		id, pin := strings.TrimSpace(l.PublicationVersionID), strings.TrimSpace(l.ManifestHash)
		if l.Kind != types.ParentLinkSource || id == "" || pin == "" || seen[id] || len(out.Parents) >= maxPublishParents {
			continue
		}
		seen[id] = true
		out.Parents = append(out.Parents, platform.PublishParentRef{PublicationVersionID: id, ManifestHash: pin})
	}
	return out
}

// publishLinksFor is the links of the revision being published. An op-ed is
// a fork the owner may have edited since: an edit records no link of its
// own, so the walk goes up the document's chain to the revision that did
// (the fork's first). Other modes read the revision itself.
func (h *Handler) publishLinksFor(r *http.Request, autoputerURL, userID string, rev autoputerTextureRevision, mode string) publishLinks {
	links := derivePublishLinks(rev.Provenance, mode)
	if len(links.Parents) > 0 || mode != platform.PublishModeOpinion {
		return links
	}
	cursor := rev
	for i := 0; i < maxPublishLinkWalk && strings.TrimSpace(cursor.ParentRevisionID) != ""; i++ {
		var parent autoputerTextureRevision
		if err := h.fetchAutoputerJSON(r, autoputerURL, "/api/texture/revisions/"+url.PathEscape(cursor.ParentRevisionID), userID, &parent); err != nil {
			return publishLinks{}
		}
		if parent.OwnerID != userID || parent.DocID != rev.DocID || parent.RevisionID != cursor.ParentRevisionID {
			return publishLinks{}
		}
		if links = derivePublishLinks(parent.Provenance, mode); len(links.Parents) > 0 {
			return links
		}
		cursor = parent
	}
	return publishLinks{}
}

// tellComputerPublished posts the computer's /published notice for a document
// that just published, so the computer arms its feed polls. Best effort.
func (h *Handler) tellComputerPublished(r *http.Request, autoputerURL, userID, docID string) {
	target, err := joinBasePath(autoputerURL, "/api/texture/documents/"+url.PathEscape(docID)+"/published")
	if err != nil {
		return
	}
	req, err := http.NewRequestWithContext(r.Context(), http.MethodPost, target, nil)
	if err != nil {
		return
	}
	req.Header.Set("X-Authenticated-User", userID)
	resp, err := (&http.Client{Timeout: 10 * time.Second}).Do(req)
	if err != nil {
		log.Printf("proxy: platform publish: tell the computer %s was published: %v", docID, err)
		return
	}
	_ = resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		log.Printf("proxy: platform publish: the computer answered %d to the published notice for %s", resp.StatusCode, docID)
	}
}
