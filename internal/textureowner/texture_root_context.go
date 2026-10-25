package textureowner

// The Texture root's shared context (PD3 S2,
// docs/definitions/choir-pd3-texture-root-desk-2026-10-11.md, "Shared
// context (S2)"): at wake, the run of every document other than the master
// carries an excerpt of the master's latest revision in its request turn,
// outside the prompt core, as read-only context: at most 4 KiB, the "Waits
// on the owner" section first, then the other documents' sections newest
// first by their documents' last revision (whatever order the root wrote:
// S2 paired test F4), without the run's own document's section, cut from
// the end. Only with
// texture.root_desk on and a live master the root has written; otherwise
// the run is unchanged.

import (
	"context"
	"log"
	"sort"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/textureprompts"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// textureRootContextMaxBytes bounds the master's excerpt.
const textureRootContextMaxBytes = 4 << 10

const textureRootContextCut = "\n(the rest of the master document is cut here)"

// textureRootSharedContext is the excerpt section for a run on doc, and the
// master revision it shows; empty when the marked master is not live or its
// head is not the root's own writing (its seed, or an owner edit the root
// has not yet rendered).
func (h *Handler) textureRootSharedContext(ctx context.Context, ownerID string, doc types.Document, root store.TextureRoot) (string, string) {
	computerID := strings.TrimSpace(doc.ComputerID)
	if h == nil || h.Store == nil || strings.TrimSpace(root.DocumentID) == "" || root.DocumentID == doc.DocID {
		return "", ""
	}
	exists, live, err := h.textureRootLive(ctx, ownerID, computerID, root)
	if err != nil {
		log.Printf("texture root: shared context on %s: master liveness: %v", computerID, err)
		return "", ""
	}
	if !exists || !live {
		return "", ""
	}
	master, err := h.Store.GetLifecycleDocument(ctx, ownerID, computerID, root.DocumentID)
	if err != nil {
		log.Printf("texture root: shared context on %s: load master: %v", computerID, err)
		return "", ""
	}
	head, err := h.Store.GetLifecycleRevision(ctx, ownerID, computerID, master.CurrentRevisionID)
	if err != nil {
		log.Printf("texture root: shared context on %s: load master head: %v", computerID, err)
		return "", ""
	}
	if head.AuthorKind != types.AuthorAppAgent {
		return "", "" // only the root writes the master as the app agent
	}
	excerpt := textureRootExcerpt(head.Content, doc.DocID, h.textureRootRevisedAt(ctx, ownerID, computerID, root))
	if excerpt == "" {
		return "", ""
	}
	return "\n\n" + textureprompts.RootContextLabel() + "\n\n" + excerpt, head.RevisionID
}

// textureRootRevisedAt is each listed document's last revision time, from
// one read of the document listing the root's ledger reads (most recently
// revised first, the ledger's bound plus the masters). A read error leaves
// every section's order as the root wrote it.
func (h *Handler) textureRootRevisedAt(ctx context.Context, ownerID, computerID string, root store.TextureRoot) map[string]time.Time {
	list := h.textureRootListDocuments
	if list == nil {
		list = h.Store.ListDocumentsByScope
	}
	docs, err := list(ctx, ownerID, computerID, textureRootMaxDocuments+len(textureRootMasterIDs(root)))
	if err != nil {
		log.Printf("texture root: shared context on %s: list documents: %v", computerID, err)
		return nil
	}
	revisedAt := make(map[string]time.Time, len(docs))
	for _, doc := range docs {
		revisedAt[doc.DocID] = doc.UpdatedAt
	}
	return revisedAt
}

// textureRootExcerptSection is one heading's lines in the master, and the
// document its refs line names.
type textureRootExcerptSection struct {
	lines []string
	waits bool
	docID string
}

// textureRootExcerpt orders and bounds the master's text for the run on
// docID: "waits on the owner" sections first; then the sections whose
// document revisedAt knows, newest first; then the rest in their order;
// docID's own section (the one whose refs name it) left out; the whole
// cut from the end at textureRootContextMaxBytes.
func textureRootExcerpt(content, docID string, revisedAt map[string]time.Time) string {
	docID = strings.TrimSpace(docID)
	var sections []*textureRootExcerptSection
	current := &textureRootExcerptSection{}
	sections = append(sections, current)
	for _, line := range strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") {
			heading := strings.ToLower(strings.TrimSpace(strings.TrimLeft(trimmed, "#")))
			current = &textureRootExcerptSection{waits: strings.Contains(heading, "waits on the owner") || strings.Contains(heading, "waiting on the owner")}
			sections = append(sections, current)
		}
		// The root writes refs at the end of a paragraph as often as on a
		// line of their own.
		if at := strings.Index(trimmed, "refs: doc="); current.docID == "" && at >= 0 {
			if fields := strings.Fields(trimmed[at+len("refs: doc="):]); len(fields) > 0 {
				current.docID = fields[0]
			}
		}
		current.lines = append(current.lines, line)
	}
	var waits, known, unknown []*textureRootExcerptSection
	for _, section := range sections {
		switch _, ok := revisedAt[section.docID]; {
		case section.waits:
			waits = append(waits, section)
		case docID != "" && section.docID == docID:
			// the run's own document: the run has the document itself
		case section.docID != "" && ok:
			known = append(known, section)
		default:
			unknown = append(unknown, section)
		}
	}
	sort.SliceStable(known, func(i, j int) bool {
		return revisedAt[known[i].docID].After(revisedAt[known[j].docID])
	})
	var ordered []string
	for _, group := range [][]*textureRootExcerptSection{waits, known, unknown} {
		for _, section := range group {
			if text := strings.TrimSpace(strings.Join(section.lines, "\n")); text != "" {
				ordered = append(ordered, text)
			}
		}
	}
	excerpt := strings.Join(ordered, "\n\n")
	if len(excerpt) <= textureRootContextMaxBytes {
		return excerpt
	}
	limit := textureRootContextMaxBytes - len(textureRootContextCut)
	cut := excerpt[:limit]
	if at := strings.LastIndex(cut, "\n"); at > 0 {
		cut = cut[:at]
	} else {
		cut = cutAtRuneBoundary(excerpt, limit)
	}
	return strings.TrimRight(cut, " \n") + textureRootContextCut
}
