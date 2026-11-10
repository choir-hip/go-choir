package textureowner

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/agentcore"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// The merge path: the request revision carries the links; the desk's revision
// inherits them; the desk is shown each source as data.

// revisionSourceLinks are the source links an owner revision's provenance
// carries; a revision with none (nearly all) returns nil.
func revisionSourceLinks(rev types.Revision) []types.ParentLink {
	if len(rev.Provenance) == 0 {
		return nil
	}
	var prov types.Provenance
	if json.Unmarshal(rev.Provenance, &prov) != nil {
		return nil
	}
	var out []types.ParentLink
	for _, l := range prov.ParentLinks {
		if l.Kind == types.ParentLinkSource {
			out = append(out, l)
		}
	}
	return out
}

// inheritParentLinks stamps the links of the revision the desk answers onto
// the desk's own provenance. The desk cannot set them: only an owner-authored
// base carries links to inherit, and the desk's turn never reads or writes
// them.
func inheritParentLinks(provenance json.RawMessage, base types.Revision) json.RawMessage {
	if base.AuthorKind != types.AuthorUser || len(base.Provenance) == 0 {
		return provenance
	}
	var from types.Provenance
	if json.Unmarshal(base.Provenance, &from) != nil || len(from.ParentLinks) == 0 {
		return provenance
	}
	var prov types.Provenance
	if json.Unmarshal(provenance, &prov) != nil {
		return provenance
	}
	prov.ParentLinks = from.ParentLinks
	raw, err := prov.CanonicalJSON()
	if err != nil {
		return provenance
	}
	return raw
}

// reviseSourceMaxChars caps one source's text in the desk's request.
const reviseSourceMaxChars = 24000

// reviseSourcesSection shows the desk each source of the owner revision it
// answers as a world_untrusted data block, fetched now and pinned by the
// link's manifest hash. A source that cannot be read or does not match its
// pin is named as unavailable, never shown. Empty when the revision has none.
func (rt *Handler) reviseSourcesSection(ctx context.Context, current types.Revision) string {
	links := revisionSourceLinks(current)
	if len(links) == 0 {
		return ""
	}
	reader := rt.versionReader()
	var b strings.Builder
	b.WriteString("\n\nPublished versions the owner brought in (pinned by manifest hash):")
	for i, l := range links {
		label := fmt.Sprintf("\n\nSource %d: %s", i+1, firstNonEmpty(l.RoutePath, l.PublicationVersionID))
		if l.AuthorLabel != "" {
			label += " by " + l.AuthorLabel
		}
		if reader == nil {
			b.WriteString(label + " (unavailable: the platform is not reachable)")
			continue
		}
		v, err := reader.ReadPublicationVersion(ctx, l.PublicationVersionID)
		switch {
		case err != nil:
			b.WriteString(label + " (unavailable: it could not be read)")
		case v.ManifestHash != l.ManifestHash || v.PublicationVersionID != l.PublicationVersionID:
			b.WriteString(label + " (unavailable: it does not match its pinned manifest hash)")
		default:
			b.WriteString(label + ":\n" + agentcore.FrameWorldUntrusted(truncatePromptSnippet(v.Content, reviseSourceMaxChars)))
		}
	}
	return b.String()
}
