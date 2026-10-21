package platform

// Opinion pieces (docs/problems/oped-not-connected-2026-10-20.md): a signed-in
// user's own Texture, published into a labelled Opinion section of the paper.
// An opinion piece is not an Autopaper story: it never joins the news front,
// the related-stories walk or the corroboration inputs, and it carries a
// human author label instead of the AI disclosure.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
)

// FrontSectionOpinion is the front list's section filter for opinion pieces.
// The news front (no section, or a scored news section) never contains them.
const FrontSectionOpinion = "opinion"

// DefaultOpinionDailyLimit is how many new opinion pieces one user may open
// in 24 hours when CORPUSD_OPINION_DAILY_LIMIT is unset.
const DefaultOpinionDailyLimit = 5

var (
	// ErrOpinionListingOff means the kill switch (CORPUSD_OPINION_LISTING) is
	// off: no opinion piece is published, listed or searched.
	ErrOpinionListingOff = errors.New("opinion publishing is switched off")
	// ErrOpinionDailyLimit means the user already opened the day's limit of
	// new opinion pieces.
	ErrOpinionDailyLimit = errors.New("daily opinion limit reached")
	// ErrInvalidAuthorLabel means the byline is empty, too long, or not a
	// short handle (an email address is never one).
	ErrInvalidAuthorLabel = errors.New("author label must be a short handle of letters, digits, spaces, dots, dashes or underscores")
	// ErrOpinionModeConflict means the Texture was already published under
	// another mode, which a publication keeps for good.
	ErrOpinionModeConflict = errors.New("this Texture was already published as a plain link; publish a copy as opinion")
)

var authorLabelPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9 ._-]{0,39}$`)

// ValidAuthorLabel reports whether label is a byline an opinion piece may
// carry.
func ValidAuthorLabel(label string) bool {
	return authorLabelPattern.MatchString(strings.TrimSpace(label))
}

// SetOpinionListing is the kill switch (CORPUSD_OPINION_LISTING=on). Off, the
// default, no opinion piece is published, listed or found by research.
func (s *Service) SetOpinionListing(on bool) {
	s.frontMu.Lock()
	defer s.frontMu.Unlock()
	s.opinionListing = on
}

// SetOpinionDailyLimit sets how many new opinion pieces one user opens per 24
// hours; zero or less restores DefaultOpinionDailyLimit.
func (s *Service) SetOpinionDailyLimit(n int) {
	s.frontMu.Lock()
	defer s.frontMu.Unlock()
	s.opinionDailyLimit = n
}

// ParseOpinionListing reads CORPUSD_OPINION_LISTING: "on" (or true/1) turns
// opinion publishing and listing on; anything else, unset included, is off.
func ParseOpinionListing(raw string) bool {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "on", "true", "1", "yes":
		return true
	}
	return false
}

func (s *Service) opinionEnabled() bool {
	s.frontMu.RLock()
	defer s.frontMu.RUnlock()
	return s.opinionListing
}

func (s *Service) opinionLimit() int {
	s.frontMu.RLock()
	defer s.frontMu.RUnlock()
	if s.opinionDailyLimit <= 0 {
		return DefaultOpinionDailyLimit
	}
	return s.opinionDailyLimit
}

// isOpinionPiece decides whether a publication is a listed opinion piece:
// opinion mode, and the kill switch on. It does not depend on the Autopaper
// owner allowlist.
func (s *Service) isOpinionPiece(meta frontPublicationMeta) bool {
	return meta.PublishMode == PublishModeOpinion && s.opinionEnabled()
}

// checkOpinionPublish enforces the opinion rules before a version is written:
// the switch is on, the byline is a handle, the Texture was not published
// under another mode, and a new piece stays within the user's daily limit.
// existingMode is the stored mode of an existing publication ("" for a new
// one).
func (s *Service) checkOpinionPublish(ctx context.Context, req PublishTextureRequest, newPublication bool, existingMode string) error {
	if !s.opinionEnabled() {
		return ErrOpinionListingOff
	}
	if !ValidAuthorLabel(req.AuthorLabel) {
		return ErrInvalidAuthorLabel
	}
	if !newPublication {
		if existingMode != "" && existingMode != PublishModeOpinion {
			return ErrOpinionModeConflict
		}
		return nil
	}
	opened, err := s.countRecentOpinionPublications(ctx, req.OwnerID, time.Now().UTC().Add(-24*time.Hour))
	if err != nil {
		return err
	}
	if limit := s.opinionLimit(); opened >= limit {
		return fmt.Errorf("%w: %d new opinion pieces in the last 24 hours, the limit is %d", ErrOpinionDailyLimit, opened, limit)
	}
	return nil
}

// countRecentOpinionPublications counts the owner's opinion publications
// opened since the cutoff. The count is derived from the publications'
// created_at, so it survives a restart.
func (s *Service) countRecentOpinionPublications(ctx context.Context, ownerID string, since time.Time) (int, error) {
	rows, err := s.store.corpus().QueryContext(ctx,
		`SELECT metadata FROM og_objects
		 WHERE object_kind = 'choir.publication' AND owner_id = ? AND tombstone = FALSE AND created_at >= ?`,
		ownerID, since)
	if err != nil {
		return 0, fmt.Errorf("platform opinion: count recent pieces: %w", err)
	}
	defer rows.Close()
	n := 0
	for rows.Next() {
		var metadata string
		if err := rows.Scan(&metadata); err != nil {
			return 0, fmt.Errorf("platform opinion: scan: %w", err)
		}
		var meta frontPublicationMeta
		if json.Unmarshal([]byte(metadata), &meta) == nil && meta.PublishMode == PublishModeOpinion {
			n++
		}
	}
	return n, rows.Err()
}
