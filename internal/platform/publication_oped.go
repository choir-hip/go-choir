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
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// feedOverlap re-reads the tail of the last page: object timestamps are whole
// seconds, so an item committed late with an earlier stamp must not be
// skipped. The consumer's deterministic ids make the repeat free.
const feedOverlap = 10 * time.Second

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
}

// OpedFeed is one page of op-eds.
type OpedFeed struct {
	Items      []OpedFeedItem `json:"items"`
	NextCursor string         `json:"next_cursor"`
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

// noticeRows reads the owner's notices created at or after the cursor
// (minus the overlap), oldest first.
func (s *Service) noticeRows(ctx context.Context, ownerID, cursor string, limit int) ([]objectgraph.Object, error) {
	ownerID = strings.TrimSpace(ownerID)
	if ownerID == "" {
		return nil, fmt.Errorf("a notice read needs an owner")
	}
	since := time.Unix(0, 0).UTC()
	if c := strings.TrimSpace(cursor); c != "" {
		t, err := time.Parse(time.RFC3339, c)
		if err != nil {
			return nil, fmt.Errorf("cursor is not a time")
		}
		since = t.Add(-feedOverlap)
	}
	rows, err := s.store.corpus().QueryContext(ctx,
		`SELECT canonical_id, metadata, created_at FROM og_objects
		 WHERE object_kind = 'choir.changelog_notice' AND owner_id = ? AND tombstone = FALSE AND created_at >= ?
		 ORDER BY created_at, canonical_id LIMIT ?`, ownerID, since, normalizeFeedLimit(limit))
	if err != nil {
		return nil, fmt.Errorf("platform notices: %w", err)
	}
	defer rows.Close()
	var out []objectgraph.Object
	for rows.Next() {
		var obj objectgraph.Object
		var metadata string
		if err := rows.Scan(&obj.CanonicalID, &metadata, &obj.CreatedAt); err != nil {
			return nil, fmt.Errorf("platform notices: scan: %w", err)
		}
		obj.OwnerID, obj.Metadata = ownerID, json.RawMessage(metadata)
		out = append(out, obj)
	}
	return out, rows.Err()
}

func decodeNoticeObject(obj objectgraph.Object) (changelogNoticeObject, bool) {
	var n changelogNoticeObject
	if json.Unmarshal(obj.Metadata, &n) != nil || n.Notice.EventID == "" {
		return n, false
	}
	return n, true
}

// ListNoticesForOwner is the recipient's notice feed.
func (s *Service) ListNoticesForOwner(ctx context.Context, ownerID, cursor string, limit int) (NoticeFeed, error) {
	rows, err := s.noticeRows(ctx, ownerID, cursor, limit)
	if err != nil {
		return NoticeFeed{}, err
	}
	feed := NoticeFeed{Items: []NoticeItem{}, NextCursor: strings.TrimSpace(cursor)}
	for _, obj := range rows {
		feed.NextCursor = obj.CreatedAt.UTC().Format(time.RFC3339)
		n, ok := decodeNoticeObject(obj)
		if !ok {
			continue
		}
		feed.Items = append(feed.Items, NoticeItem{ID: obj.CanonicalID, CreatedAt: feed.NextCursor, Notice: n.Notice})
	}
	return feed, nil
}

// ListOpedsForOwner is the op-ed feed of the owner's stories: the oped on
// your story notices, each with the references the paper's computer adopts.
func (s *Service) ListOpedsForOwner(ctx context.Context, ownerID, cursor string, limit int) (OpedFeed, error) {
	rows, err := s.noticeRows(ctx, ownerID, cursor, limit)
	if err != nil {
		return OpedFeed{}, err
	}
	feed := OpedFeed{Items: []OpedFeedItem{}, NextCursor: strings.TrimSpace(cursor)}
	for _, obj := range rows {
		feed.NextCursor = obj.CreatedAt.UTC().Format(time.RFC3339)
		n, ok := decodeNoticeObject(obj)
		if !ok || n.Notice.Relation != types.ChangelogRelationOpedOnYourStory || n.RespondsTo == nil {
			continue
		}
		feed.Items = append(feed.Items, OpedFeedItem{
			VersionID: n.Child.PublicationVersionID, RoutePath: n.Child.RoutePath, AuthorLabel: n.Child.AuthorLabel,
			ManifestHash: n.Child.ManifestHash, PublishedAt: n.Child.PublishedAt, NoticeID: obj.CanonicalID, StoryDocID: n.RespondsTo.SourceDocID,
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
	objects = append(objects, noticeObj)
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
	return &OpedState{VersionID: versionID, Run: best.Run, Verdict: best.Verdict, VerdictHash: best.VerdictHash}, nil
}
