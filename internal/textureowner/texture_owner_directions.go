package textureowner

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// The owner's later directions (docs/design/texture-spec-follows-owner-2026-10-25.md).
// An owner /revise stores its direction as owner_prompt on that one user
// revision; seed_prompt, the original request, is carried on every revision.
// Reading the head alone loses the direction after one Texture revision and
// leaves the seed standing as the owner's whole request (run 230c5a4e
// re-cast "delete PdfApp and EpubApp" over the owner's "keep them"). This
// projection walks the parent chain for the earlier directions instead.
const (
	// textureOwnerDirectionWalkLimit bounds the revisions read per Texture
	// run start (point reads by id, the parent already loaded is reused).
	textureOwnerDirectionWalkLimit = 64
	// textureOwnerDirectionKeep is how many of the newest directions print.
	textureOwnerDirectionKeep = 8
	// textureOwnerDirectionMaxChars caps one direction's text.
	textureOwnerDirectionMaxChars = 4000
)

// textureOwnerDirection is one owner revise on the document: the revision
// that carries it, when, and what the owner said.
type textureOwnerDirection struct {
	RevisionID string
	At         time.Time
	Text       string
}

// collectTextureOwnerDirections returns the owner directions on head's
// ancestors (head's own direction excluded: the turn prints it as "The
// owner's direction for this revision"), oldest first, the newest
// textureOwnerDirectionKeep kept. It reads at most
// textureOwnerDirectionWalkLimit revisions, stops at a revision with no
// parent, a revision it cannot load, or one it has already seen.
func collectTextureOwnerDirections(head types.Revision, previous *types.Revision, load func(string) (types.Revision, error)) []textureOwnerDirection {
	seen := map[string]bool{strings.TrimSpace(head.RevisionID): true}
	var newestFirst []textureOwnerDirection
	parentID := strings.TrimSpace(head.ParentRevisionID)
	for reads := 0; parentID != "" && !seen[parentID] && len(newestFirst) < textureOwnerDirectionKeep; {
		var rev types.Revision
		if previous != nil && strings.TrimSpace(previous.RevisionID) == parentID {
			rev, previous = *previous, nil
		} else {
			if reads >= textureOwnerDirectionWalkLimit || load == nil {
				break
			}
			reads++
			loaded, err := load(parentID)
			if err != nil {
				break
			}
			rev = loaded
		}
		seen[parentID] = true
		if strings.TrimSpace(rev.DocID) != strings.TrimSpace(head.DocID) {
			// A parent from another document is never this document's
			// owner direction (review bunch 232 finding 1).
			break
		}
		if rev.AuthorKind == types.AuthorUser {
			if text := strings.TrimSpace(metadataString(decodeRevisionMetadata(rev.Metadata), "owner_prompt")); text != "" {
				newestFirst = append(newestFirst, textureOwnerDirection{
					RevisionID: strings.TrimSpace(rev.RevisionID),
					At:         rev.CreatedAt.UTC(),
					Text:       truncatePromptSnippet(text, textureOwnerDirectionMaxChars),
				})
			}
		}
		parentID = strings.TrimSpace(rev.ParentRevisionID)
	}
	out := make([]textureOwnerDirection, 0, len(newestFirst))
	for i := len(newestFirst) - 1; i >= 0; i-- {
		out = append(out, newestFirst[i])
	}
	return out
}

// textureOwnerDirectionsOn reads the computer setting; any read failure is off.
func (rt *Handler) textureOwnerDirectionsOn(ctx context.Context, ownerID, computerID string) bool {
	if rt == nil || rt.Store == nil || strings.TrimSpace(ownerID) == "" || strings.TrimSpace(computerID) == "" {
		return false
	}
	setting, err := rt.Store.GetComputerSetting(ctx, ownerID, computerID, store.SettingTextureOwnerDirections)
	if err != nil {
		if !errors.Is(err, store.ErrNotFound) {
			log.Printf("texture api: read %s on %s: %v; treating it as off", store.SettingTextureOwnerDirections, computerID, err)
		}
		return false
	}
	return setting.Value
}

// writeTextureOwnerDirections prints the earlier directions under the rule
// that the newest governs.
func writeTextureOwnerDirections(b *strings.Builder, directions []textureOwnerDirection) {
	b.WriteString("\n\nThe owner's later directions, oldest first. Each overrides the original request and any earlier direction wherever they differ; the newest governs. Keep the document and every cast you send consistent with it, and never undo an owner direction unless a later owner direction says so:")
	for i, direction := range directions {
		b.WriteString(fmt.Sprintf("\n\n%d.", i+1))
		if !direction.At.IsZero() {
			b.WriteString(" " + direction.At.UTC().Format(time.RFC3339))
		}
		if direction.RevisionID != "" {
			b.WriteString(" (revision " + direction.RevisionID + ")")
		}
		b.WriteString(":\n" + direction.Text)
	}
}
