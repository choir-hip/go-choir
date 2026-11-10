package platform

// The op-ed feed, the notice feed and the courier copy of the paper's gate
// verdict (contract 2 sections 3.3 and 3.4, contract 3 section 5.2). Both
// feeds are reads of the recipient's own changelog notices: the owner is the
// bound owner (the route takes no owner parameter), and a page replays the
// same items, so the consumer deduplicates by a deterministic id.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/types"
)

const (
	feedDefaultLimit = 100
	feedMaxLimit     = 200
)

// NoticeItem is one notice as the recipient's poll sees it.
type NoticeItem struct {
	ID        string                    `json:"id"`
	CreatedAt string                    `json:"created_at"`
	Notice    types.CommitmentChangelog `json:"notice"`
}

// NoticeFeed is one page of a recipient's notices.
type NoticeFeed struct {
	Items      []NoticeItem `json:"items"`
	NextCursor string       `json:"next_cursor"`
	// CursorReset says the cursor sent was not one of ours, so the page is
	// from the start of the feed.
	CursorReset bool `json:"cursor_reset,omitempty"`
	// Skipped names the rows that could not be read, by id and why.
	Skipped []FeedSkipped `json:"skipped,omitempty"`
}

// FeedSkipped is a feed row passed over: named, never silently dropped.
type FeedSkipped struct {
	ID     string `json:"id"`
	Reason string `json:"reason"`
}

// OpedFeedItem is one op-ed on a story of the bound owner.
type OpedFeedItem struct {
	VersionID    string               `json:"version_id"`
	RoutePath    string               `json:"route_path"`
	AuthorLabel  string               `json:"author_label"`
	ManifestHash string               `json:"manifest_hash"`
	PublishedAt  string               `json:"published_at"`
	RespondsTo   types.OpedRespondsTo `json:"responds_to"`
	NoticeID     string               `json:"notice_id"`
	// StoryDocID is the paper's own document the op-ed answers.
	StoryDocID string `json:"story_doc_id"`
	// PublisherID is the publishing owner, from the graph: the key of the
	// paper's per-publisher bounds (a byline is free to pick).
	PublisherID string `json:"publisher_id"`
}

// OpedFeed is one page of op-eds.
type OpedFeed struct {
	Items       []OpedFeedItem `json:"items"`
	NextCursor  string         `json:"next_cursor"`
	CursorReset bool           `json:"cursor_reset,omitempty"`
	Skipped     []FeedSkipped  `json:"skipped,omitempty"`
}

func normalizeFeedLimit(n int) int {
	if n <= 0 {
		return feedDefaultLimit
	}
	if n > feedMaxLimit {
		return feedMaxLimit
	}
	return n
}

// seqNotice is a notice row with its sequence.
type seqNotice struct {
	obj objectgraph.Object
	seq int64
}

// noticeRows reads the owner's notices after the cursor, in write order. The
// cursor is the opaque sequence of the last row a page returned: it moves on
// every page however many rows share a second, and a row written later is
// always after it.
func (s *Service) noticeRows(ctx context.Context, ownerID, cursor string, limit int) (rows []seqNotice, reset bool, err error) {
	ownerID = strings.TrimSpace(ownerID)
	if ownerID == "" {
		return nil, false, fmt.Errorf("a notice read needs an owner")
	}
	if err := s.ensureNoticeSeq(ctx); err != nil {
		return nil, false, err
	}
	after := int64(0)
	if c := strings.TrimSpace(cursor); c != "" {
		n, perr := strconv.ParseInt(c, 10, 64)
		if perr != nil || n < 0 {
			// Not one of ours (an older cursor format): start over, and say so.
			reset = true
		} else {
			after = n
		}
	}
	const seqExpr = `COALESCE(((metadata::jsonb) ->> 'seq')::bigint, 0)`
	dbRows, err := s.store.corpus().QueryContext(ctx,
		`SELECT canonical_id, metadata, created_at, `+seqExpr+` FROM og_objects
		 WHERE object_kind = 'choir.changelog_notice' AND owner_id = ? AND tombstone = FALSE AND `+seqExpr+` > ?
		 ORDER BY `+seqExpr+`, canonical_id LIMIT ?`, ownerID, after, normalizeFeedLimit(limit))
	if err != nil {
		return nil, false, fmt.Errorf("platform notices: %w", err)
	}
	defer dbRows.Close()
	var out []seqNotice
	for dbRows.Next() {
		var obj objectgraph.Object
		var metadata string
		var seq int64
		if err := dbRows.Scan(&obj.CanonicalID, &metadata, &obj.CreatedAt, &seq); err != nil {
			return nil, false, fmt.Errorf("platform notices: scan: %w", err)
		}
		obj.OwnerID, obj.Metadata = ownerID, json.RawMessage(metadata)
		out = append(out, seqNotice{obj: obj, seq: seq})
	}
	return out, reset, dbRows.Err()
}

func decodeNoticeObject(obj objectgraph.Object) (changelogNoticeObject, bool) {
	var n changelogNoticeObject
	if json.Unmarshal(obj.Metadata, &n) != nil || n.Notice.EventID == "" {
		return n, false
	}
	return n, true
}

func undecodable(obj objectgraph.Object) FeedSkipped {
	return FeedSkipped{ID: obj.CanonicalID, Reason: "the notice row could not be decoded"}
}

// ListNoticesForOwner is the recipient's notice feed.
func (s *Service) ListNoticesForOwner(ctx context.Context, ownerID, cursor string, limit int) (NoticeFeed, error) {
	rows, reset, err := s.noticeRows(ctx, ownerID, cursor, limit)
	if err != nil {
		return NoticeFeed{}, err
	}
	feed := NoticeFeed{Items: []NoticeItem{}, NextCursor: strings.TrimSpace(cursor), CursorReset: reset}
	if reset {
		feed.NextCursor = ""
	}
	for _, row := range rows {
		feed.NextCursor = strconv.FormatInt(row.seq, 10)
		n, ok := decodeNoticeObject(row.obj)
		if !ok {
			feed.Skipped = append(feed.Skipped, undecodable(row.obj))
			continue
		}
		feed.Items = append(feed.Items, NoticeItem{ID: row.obj.CanonicalID, CreatedAt: row.obj.CreatedAt.UTC().Format(time.RFC3339), Notice: n.Notice})
	}
	return feed, nil
}

// ListOpedsForOwner is the op-ed feed of the owner's stories: the oped on
// your story notices, each with the references the paper's computer adopts.
func (s *Service) ListOpedsForOwner(ctx context.Context, ownerID, cursor string, limit int) (OpedFeed, error) {
	rows, reset, err := s.noticeRows(ctx, ownerID, cursor, limit)
	if err != nil {
		return OpedFeed{}, err
	}
	feed := OpedFeed{Items: []OpedFeedItem{}, NextCursor: strings.TrimSpace(cursor), CursorReset: reset}
	if reset {
		feed.NextCursor = ""
	}
	for _, row := range rows {
		obj := row.obj
		feed.NextCursor = strconv.FormatInt(row.seq, 10)
		n, ok := decodeNoticeObject(obj)
		if !ok {
			feed.Skipped = append(feed.Skipped, undecodable(obj))
			continue
		}
		if n.Notice.Relation != types.ChangelogRelationOpedOnYourStory || n.RespondsTo == nil {
			continue
		}
		feed.Items = append(feed.Items, OpedFeedItem{
			VersionID: n.Child.PublicationVersionID, RoutePath: n.Child.RoutePath, AuthorLabel: n.Child.AuthorLabel,
			ManifestHash: n.Child.ManifestHash, PublishedAt: n.Child.PublishedAt, NoticeID: obj.CanonicalID, StoryDocID: n.RespondsTo.SourceDocID, PublisherID: n.PublisherOwnerID,
			RespondsTo: types.OpedRespondsTo{PublicationVersionID: n.RespondsTo.PublicationVersionID,
				RoutePath: n.RespondsTo.RoutePath, PublicationID: n.RespondsTo.PublicationID},
		})
	}
	return feed, nil
}

// OpedVerdictPost is the paper's computer posting a gate verdict.
type OpedVerdictPost struct {
	// OwnerID is the caller's bound owner (the paper's owner). The route sets
	// it from the binding, never from the body.
	OwnerID      string `json:"owner_id"`
	VersionID    string `json:"version_id"`
	Run          int    `json:"run"`
	Verdict      string `json:"verdict"`
	Reason       string `json:"reason,omitempty"`
	VerdictHash  string `json:"verdict_hash"`
	TapeRecordID string `json:"tape_record_id"`
}

// opedVerdictObject is the stored courier copy.
type opedVerdictObject struct {
	VersionID       string `json:"version_id"`
	Run             int    `json:"run"`
	Verdict         string `json:"verdict"`
	Reason          string `json:"reason,omitempty"`
	ReasonUntrusted bool   `json:"reason_untrusted,omitempty"`
	VerdictHash     string `json:"verdict_hash"`
	TapeRecordID    string `json:"tape_record_id"`
	StoryOwnerID    string `json:"story_owner_id"`
	AuthorOwnerID   string `json:"author_owner_id"`
	RespondsToID    string `json:"responds_to_version_id"`
	PostedAt        string `json:"posted_at"`
}

// OpedState is the latest verdict on an op-ed.
type OpedState struct {
	VersionID   string `json:"version_id"`
	Run         int    `json:"run"`
	Verdict     string `json:"verdict"`
	VerdictHash string `json:"verdict_hash"`
	// StoryOwnerID is the owner who judged it.
	StoryOwnerID string `json:"-"`
}

func opedVerdictCanonicalID(authorOwner, versionID string, run int) (string, error) {
	return objectgraph.BuildCanonicalID("choir.oped_verdict", authorOwner,
		objectgraph.StableSuffixFromKey(fmt.Sprintf("%s#%d", versionID, run)))
}

// PostOpedVerdict writes the courier copy of a gate verdict and the author's
// notice. The op-ed's responds_to story must belong to the posting owner.
// One verdict per (op-ed version, run): a repeat changes nothing.
func (s *Service) PostOpedVerdict(ctx context.Context, post OpedVerdictPost) (*OpedState, error) {
	if s == nil || s.store == nil {
		return nil, fmt.Errorf("platform service unavailable")
	}
	post.OwnerID, post.VersionID = strings.TrimSpace(post.OwnerID), strings.TrimSpace(post.VersionID)
	if post.OwnerID == "" || post.VersionID == "" || post.Run < 1 || strings.TrimSpace(post.VerdictHash) == "" {
		return nil, fmt.Errorf("a verdict needs an owner, a version id, a run of 1 or more and a verdict hash")
	}
	if !types.OpedVerdictValue(post.Verdict).IsValid() {
		return nil, fmt.Errorf("%q is not a known verdict", post.Verdict)
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	og := s.ogStore()
	if og == nil {
		return nil, fmt.Errorf("platform service: object graph store unavailable")
	}
	found, err := og.ListObjectsByMetadata(ctx, "choir.publication_version", "$.publication_version_id", post.VersionID, 1)
	if err != nil {
		return nil, err
	}
	if len(found) == 0 || found[0].Tombstone {
		return nil, ErrNotYourStory
	}
	obj := found[0]
	meta, err := decodeVersionMetadata(obj)
	if err != nil {
		return nil, err
	}
	if meta.RespondsTo == nil || meta.RespondsTo.OwnerID != post.OwnerID {
		return nil, ErrNotYourStory
	}
	id, err := opedVerdictCanonicalID(obj.OwnerID, post.VersionID, post.Run)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	if _, err := og.GetObject(ctx, id); err == nil {
		return s.opedStateLocked(ctx, og, obj.OwnerID, post.VersionID)
	} else if !errors.Is(err, objectgraph.ErrNotFound) {
		return nil, err
	}
	body := opedVerdictObject{VersionID: post.VersionID, Run: post.Run, Verdict: post.Verdict,
		Reason: truncateRunes(post.Reason, types.OpedReasonMaxRunes), ReasonUntrusted: post.Reason != "",
		VerdictHash: post.VerdictHash, TapeRecordID: truncateRunes(post.TapeRecordID, types.ChangelogRefMaxRunes),
		StoryOwnerID: post.OwnerID, AuthorOwnerID: obj.OwnerID, RespondsToID: meta.RespondsTo.PublicationVersionID,
		PostedAt: now.Format(time.RFC3339Nano)}
	verdictMeta := mustJSONRaw(body)
	objects := []objectgraph.Object{{
		CanonicalID: id, ObjectKind: "choir.oped_verdict", OwnerID: obj.OwnerID,
		ContentHash: objectgraph.ContentHash("choir.oped_verdict", nil, verdictMeta),
		Metadata:    verdictMeta, CreatedAt: now, UpdatedAt: now,
	}}
	// The author's notice: told once per (op-ed, run).
	childLabel := ""
	if pubLabel, err := s.opedAuthorLabel(ctx, og, obj, meta); err == nil {
		childLabel = pubLabel
	}
	route := firstNonEmptyString(meta.PermalinkPath, meta.RoutePath)
	notice := types.CommitmentChangelog{
		EventID: types.ChangelogEventID(types.ChangelogOpedVerdict, obj.OwnerID, post.VersionID, fmt.Sprint(post.Run)),
		Kind:    types.ChangelogOpedVerdict, Level: types.ChangelogMajor,
		OccurredAt: now.Format(time.RFC3339), Subject: types.ChangelogSubject{PublicationVersionID: post.VersionID, RoutePath: route,
			ParentVersionID: meta.RespondsTo.PublicationVersionID, ParentRoutePath: meta.RespondsTo.RoutePath},
		Summary: truncateRunes("The paper's verdict on your op-ed: "+post.Verdict+".", types.ChangelogSummaryMaxRunes),
		Reason:  truncateRunes(post.Reason, types.ChangelogReasonMaxRunes), ReasonUntrusted: post.Reason != "",
		VerdictHash: post.VerdictHash, Origin: types.ChangelogOriginCorpusd,
	}
	noticeObj, err := noticeObjectFor(obj.OwnerID, notice, noticeChildRef{PublicationVersionID: post.VersionID,
		PublicationID: meta.PublicationID, RoutePath: route, ManifestHash: meta.ManifestHash, AuthorLabel: childLabel,
		PublishedAt: meta.PublishedAt, PublishMode: PublishModeOpinion}, meta.RespondsTo, now)
	if err != nil {
		return nil, err
	}
	objects = append(objects, withNoticeSeq(noticeObj, s.nextNoticeSeq()))
	if err := og.PutBatch(ctx, objectgraph.Batch{Objects: objects}); err != nil {
		return nil, fmt.Errorf("platform oped verdict: %w", err)
	}
	return s.opedStateLocked(ctx, og, obj.OwnerID, post.VersionID)
}

func (s *Service) opedAuthorLabel(ctx context.Context, og *ObjectGraphStore, obj objectgraph.Object, meta versionMetadata) (string, error) {
	identity, err := publicationIdentityFor(obj.OwnerID, meta.SourceDocID)
	if err != nil {
		return "", err
	}
	pubObj, err := og.GetObject(ctx, identity.CanonicalID)
	if err != nil {
		return "", err
	}
	var pub frontPublicationMeta
	if err := json.Unmarshal(pubObj.Metadata, &pub); err != nil {
		return "", err
	}
	return pub.AuthorLabel, nil
}

// OpedState returns the latest gate verdict on an op-ed version, or
// ErrOpedUnjudged when none has been posted.
func (s *Service) OpedState(ctx context.Context, versionID string) (*OpedState, error) {
	og := s.ogStore()
	if og == nil {
		return nil, fmt.Errorf("platform service: object graph store unavailable")
	}
	found, err := og.ListObjectsByMetadata(ctx, "choir.publication_version", "$.publication_version_id", strings.TrimSpace(versionID), 1)
	if err != nil {
		return nil, err
	}
	if len(found) == 0 {
		return nil, ErrOpedUnjudged
	}
	return s.opedStateLocked(ctx, og, found[0].OwnerID, strings.TrimSpace(versionID))
}

// ErrOpedUnjudged means no verdict has been posted for the op-ed.
var ErrOpedUnjudged = errors.New("the op-ed has no verdict yet")

func (s *Service) opedStateLocked(ctx context.Context, og *ObjectGraphStore, authorOwner, versionID string) (*OpedState, error) {
	found, err := og.ListObjectsByMetadataForOwner(ctx, "choir.oped_verdict", "$.version_id", versionID, authorOwner, 100)
	if err != nil {
		return nil, err
	}
	var best *opedVerdictObject
	for _, o := range found {
		var v opedVerdictObject
		if json.Unmarshal(o.Metadata, &v) != nil {
			continue
		}
		if best == nil || v.Run > best.Run {
			best = &v
		}
	}
	if best == nil {
		return nil, ErrOpedUnjudged
	}
	return &OpedState{VersionID: versionID, Run: best.Run, Verdict: best.Verdict, VerdictHash: best.VerdictHash, StoryOwnerID: best.StoryOwnerID}, nil
}
