package maild

import (
	"regexp"
	"strings"
	"time"
)

// The digest's story selection (design section 4.3): a pure function over the
// front rows, a clock and the versions already sent. No model, no I/O.
const (
	digestMaxStories      = 8
	digestMaxPerSection   = 3
	digestMinStories      = 3
	digestCandidateWindow = 24 * time.Hour
	digestPubRoutePrefix  = "/pub/texture/"
)

// digestSlugRe is the shape of a slug a link may be built from. Anything else
// is dropped rather than percent-encoded: a link is built from a slug, never
// from model text.
var digestSlugRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._~-]{0,199}$`)

// digestStory is one selected story.
type digestStory struct {
	PublicationID string
	Title         string
	Dek           string
	Section       string
	Slug          string
	SourceCount   *int
	UpdatedAt     time.Time
	Revision      int
	// Updated marks a story sent before that has a newer version now.
	Updated bool
}

type digestSelection struct {
	Stories []digestStory
	// RankedBy is "importance" or "recency": which of the page's two ranks
	// ordered the list (recency when too few rows carry a score).
	RankedBy string
	// Skipped is "too_few" when fewer than digestMinStories qualify.
	Skipped string
}

func digestSlug(bare string) string {
	bare = strings.TrimSpace(bare)
	if !strings.HasPrefix(bare, digestPubRoutePrefix) {
		return ""
	}
	slug := strings.TrimPrefix(bare, digestPubRoutePrefix)
	if !digestSlugRe.MatchString(slug) {
		return ""
	}
	return slug
}

// selectDigest picks the day's stories. sent maps a publication id to the
// latest revision already sent to this recipient; nil disables the no-repeat
// rule (preview and send-now are not the scheduled series).
//
//  1. Rank the whole front list as the page does, then walk it in that order.
//  2. A candidate was updated within the last 24 hours of now, may lead (not
//     promotional, not a leaked instruction, at least two sources or depth
//     unknown) and has a safe slug.
//  3. A story already sent is skipped unless it has a newer version; that
//     one is marked Updated.
//  4. Take at most 8, at most 3 from one section (stories with no section
//     are not capped as a group).
//  5. Fewer than 3 selected: nothing is sent.
func selectDigest(rows []FrontRow, now time.Time, sent map[string]int) digestSelection {
	sel := digestSelection{RankedBy: "recency"}
	if frontRowsUseImportance(rows) {
		sel.RankedBy = "importance"
	}
	cutoff := now.Add(-digestCandidateWindow)
	perSection := map[string]int{}
	for _, row := range rankFrontRows(rows) {
		if len(sel.Stories) >= digestMaxStories {
			break
		}
		updatedMs, ok := row.updatedMs()
		if !ok {
			continue
		}
		updated := time.UnixMilli(updatedMs).UTC()
		if updated.Before(cutoff) {
			continue
		}
		if !row.canLead() {
			continue
		}
		slug := digestSlug(row.Route.Bare)
		if slug == "" || strings.TrimSpace(row.PublicationID) == "" {
			continue
		}
		revised := false
		if sent != nil {
			if last, was := sent[row.PublicationID]; was {
				if row.LatestRevision <= last {
					continue
				}
				revised = true
			}
		}
		section := strings.TrimSpace(row.Section)
		if section != "" && perSection[section] >= digestMaxPerSection {
			continue
		}
		perSection[section]++
		var sources *int
		if n, ok := finiteCount(row.SourceCount); ok {
			v := int(n)
			sources = &v
		}
		sel.Stories = append(sel.Stories, digestStory{
			PublicationID: row.PublicationID,
			Title:         row.Title,
			Dek:           row.Dek,
			Section:       section,
			Slug:          slug,
			SourceCount:   sources,
			UpdatedAt:     updated,
			Revision:      row.LatestRevision,
			Updated:       revised,
		})
	}
	if len(sel.Stories) < digestMinStories {
		sel.Stories = nil
		sel.Skipped = "too_few"
	}
	return sel
}
