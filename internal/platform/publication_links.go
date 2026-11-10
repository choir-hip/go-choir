package platform

// Published links (docs/design/autopaper-step0-contracts-2026-11-10.md,
// contracts 2 and 3). A published version may name the published versions it
// was made from (parents) and, for an op-ed, the story version it answers
// (responds_to). corpusd verifies every link against the graph before it
// writes anything, and derives one changelog notice for the owner of each
// parent that is not the publisher. Nothing here is model work.

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

var (
	// ErrInvalidParent means a parent or responds_to link names a version
	// that does not exist, is not published, does not match its manifest
	// hash, or is not allowed for this publication.
	ErrInvalidParent = errors.New("publication link is invalid")
	// ErrNotYourStory means a verdict names an op-ed that does not respond to
	// a story of the calling owner.
	ErrNotYourStory = errors.New("the op-ed does not respond to a story of this owner")
)

// maxPublishParents bounds the parents one version names.
const maxPublishParents = 8

// noticesPerPublisherPerDay caps the new notices one publisher causes in 24
// hours (contract 8.1 D11). A notice is one per (child publication, parent
// publication): a later version of the child updates it and does not count.
const noticesPerPublisherPerDay = 20

// PublishParentRef names a published version and pins it by manifest hash.
type PublishParentRef struct {
	PublicationVersionID string `json:"publication_version_id"`
	ManifestHash         string `json:"manifest_hash"`
}

// versionLink is a verified link stored in a version's metadata. Everything
// but the two request fields is read from the graph, never from the request.
type versionLink struct {
	Role                 string `json:"role,omitempty"`
	PublicationVersionID string `json:"publication_version_id"`
	PublicationID        string `json:"publication_id,omitempty"`
	RoutePath            string `json:"route_path,omitempty"`
	ManifestHash         string `json:"manifest_hash"`
	OwnerID              string `json:"owner_id,omitempty"`
	PublishMode          string `json:"publish_mode,omitempty"`
	AuthorLabel          string `json:"author_label,omitempty"`
	// SourceDocID is the document the version was published from. It is read
	// from the graph; the feed shows it only to the owner of that document.
	SourceDocID string `json:"source_doc_id,omitempty"`
}

// resolveVersionLink reads one referenced version and checks it: it exists,
// is not tombstoned, its publication is published, and its manifest hash is
// the reference's.
func (s *Service) resolveVersionLink(ctx context.Context, og *ObjectGraphStore, ref PublishParentRef) (versionLink, error) {
	id, pin := strings.TrimSpace(ref.PublicationVersionID), strings.TrimSpace(ref.ManifestHash)
	if id == "" || pin == "" {
		return versionLink{}, fmt.Errorf("%w: a link needs a publication_version_id and a manifest_hash", ErrInvalidParent)
	}
	found, err := og.ListObjectsByMetadata(ctx, "choir.publication_version", "$.publication_version_id", id, 1)
	if err != nil {
		return versionLink{}, err
	}
	if len(found) == 0 || found[0].Tombstone {
		return versionLink{}, fmt.Errorf("%w: version %s does not exist", ErrInvalidParent, id)
	}
	obj := found[0]
	meta, err := decodeVersionMetadata(obj)
	if err != nil {
		return versionLink{}, err
	}
	if meta.ManifestHash != pin {
		return versionLink{}, fmt.Errorf("%w: version %s does not match its manifest hash", ErrInvalidParent, id)
	}
	identity, err := publicationIdentityFor(obj.OwnerID, meta.SourceDocID)
	if err != nil {
		return versionLink{}, err
	}
	pubObj, err := og.GetObject(ctx, identity.CanonicalID)
	if err != nil {
		if errors.Is(err, objectgraph.ErrNotFound) {
			return versionLink{}, fmt.Errorf("%w: version %s has no publication", ErrInvalidParent, id)
		}
		return versionLink{}, err
	}
	var pub frontPublicationMeta
	if pubObj.Tombstone || json.Unmarshal(pubObj.Metadata, &pub) != nil || pub.State != "published" {
		return versionLink{}, fmt.Errorf("%w: version %s is not published", ErrInvalidParent, id)
	}
	return versionLink{
		Role: types.ParentLinkSource, PublicationVersionID: meta.PublicationVersionID, PublicationID: meta.PublicationID,
		RoutePath: firstNonEmptyString(meta.PermalinkPath, meta.RoutePath), ManifestHash: meta.ManifestHash,
		OwnerID: obj.OwnerID, PublishMode: pub.PublishMode, AuthorLabel: pub.AuthorLabel, SourceDocID: meta.SourceDocID,
	}, nil
}

// verifyPublishLinks verifies the request's links before anything is
// written. previous is the publication's latest version, if any: an op-ed
// answers one story for good, so a later version keeps (or must repeat) it.
func (s *Service) verifyPublishLinks(ctx context.Context, og *ObjectGraphStore, req PublishTextureRequest, previous *versionMetadata) (*versionLink, []versionLink, error) {
	var respondsTo *versionLink
	if req.RespondsTo != nil {
		if req.PublishMode != PublishModeOpinion {
			return nil, nil, fmt.Errorf("%w: only an opinion piece can respond to a story", ErrInvalidParent)
		}
		link, err := s.resolveVersionLink(ctx, og, *req.RespondsTo)
		if err != nil {
			return nil, nil, err
		}
		if link.PublishMode != PublishModeAuto {
			return nil, nil, fmt.Errorf("%w: an op-ed responds to an Autopaper story, and %s is not one", ErrInvalidParent, link.PublicationVersionID)
		}
		respondsTo = &link
	}
	if previous != nil && previous.RespondsTo != nil {
		switch {
		case respondsTo == nil:
			kept := *previous.RespondsTo
			respondsTo = &kept
		case respondsTo.PublicationID != previous.RespondsTo.PublicationID:
			return nil, nil, fmt.Errorf("%w: this op-ed already answers another story", ErrInvalidParent)
		}
	}
	if len(req.Parents) > maxPublishParents {
		return nil, nil, fmt.Errorf("%w: at most %d parents", ErrInvalidParent, maxPublishParents)
	}
	seen := map[string]bool{}
	var parents []versionLink
	for _, ref := range req.Parents {
		if seen[strings.TrimSpace(ref.PublicationVersionID)] {
			continue
		}
		seen[strings.TrimSpace(ref.PublicationVersionID)] = true
		link, err := s.resolveVersionLink(ctx, og, ref)
		if err != nil {
			return nil, nil, err
		}
		// A story's owner never cites an op-ed its own gate rejected.
		if link.PublishMode == PublishModeOpinion {
			if st, serr := s.opedStateLocked(ctx, og, link.OwnerID, link.PublicationVersionID); serr == nil &&
				st.StoryOwnerID == req.OwnerID && strings.HasPrefix(st.Verdict, "rejected_") {
				return nil, nil, fmt.Errorf("%w: the gate rejected op-ed %s, so it cannot be cited as a source", ErrInvalidParent, link.PublicationVersionID)
			}
		}
		parents = append(parents, link)
	}
	// An opinion piece that was forked from a story answers it: with no
	// explicit responds_to, the first verified parent that is an Autopaper
	// story is the one (the proxy never names it; the links come from the
	// revision's own provenance).
	if respondsTo == nil && req.PublishMode == PublishModeOpinion {
		for i := range parents {
			if parents[i].PublishMode == PublishModeAuto {
				link := parents[i]
				respondsTo = &link
				break
			}
		}
	}
	return respondsTo, parents, nil
}

// changelogNoticeObject is the stored form of a notice: the event record the
// recipient's tape will hold, plus the references a feed item needs. It has
// no field for content: a recipient reads a child through its public route.
type changelogNoticeObject struct {
	// PublisherOwnerID is who caused the notice, for the daily cap. Internal;
	// the feeds do not show it.
	PublisherOwnerID string                    `json:"publisher_owner_id,omitempty"`
	Notice           types.CommitmentChangelog `json:"notice"`
	Child            noticeChildRef            `json:"child"`
	RespondsTo       *versionLink              `json:"responds_to,omitempty"`
}

type noticeChildRef struct {
	PublicationVersionID string `json:"publication_version_id"`
	PublicationID        string `json:"publication_id"`
	RoutePath            string `json:"route_path"`
	ManifestHash         string `json:"manifest_hash"`
	AuthorLabel          string `json:"author_label,omitempty"`
	PublishedAt          string `json:"published_at"`
	PublishMode          string `json:"publish_mode"`
}

func noticeRelation(childMode string, respondsTo *versionLink, parent versionLink) string {
	switch childMode {
	case PublishModeAuto:
		return types.ChangelogRelationPaperResponded
	case PublishModeOpinion:
		if parent.PublishMode == PublishModeOpinion {
			return types.ChangelogRelationForkPublished
		}
		if respondsTo != nil && respondsTo.PublicationVersionID == parent.PublicationVersionID {
			return types.ChangelogRelationOpedOnYourStory
		}
	}
	return types.ChangelogRelationTakenInto
}

var noticeSummaries = map[string]string{
	types.ChangelogRelationForkPublished:   "%s published a fork of your op-ed.",
	types.ChangelogRelationPaperResponded:  "%s revised a story using your op-ed.",
	types.ChangelogRelationTakenInto:       "%s took your published version into a document.",
	types.ChangelogRelationOpedOnYourStory: "%s published an op-ed on your story.",
}

func truncateRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n])
}

func noticeObjectFor(recipient string, body types.CommitmentChangelog, child noticeChildRef, respondsTo *versionLink, now time.Time) (objectgraph.Object, error) {
	id, err := objectgraph.BuildCanonicalID("choir.changelog_notice", recipient, objectgraph.StableSuffixFromKey(body.EventID))
	if err != nil {
		return objectgraph.Object{}, err
	}
	meta := mustJSONRaw(changelogNoticeObject{Notice: body, Child: child, RespondsTo: respondsTo})
	return objectgraph.Object{
		CanonicalID: id, ObjectKind: "choir.changelog_notice", OwnerID: recipient,
		ContentHash: objectgraph.ContentHash("choir.changelog_notice", nil, meta),
		Metadata:    meta, CreatedAt: now, UpdatedAt: now,
	}, nil
}

// publishNotices derives the parent_used notices of one publish: one per
// parent whose owner is not the publisher, at most one per (child, parent).
func publishNotices(req PublishTextureRequest, mode, authorLabel, versionID, routePath, manifestHash, publicationID string, respondsTo *versionLink, parents []versionLink, now time.Time) ([]objectgraph.Object, error) {
	actor := firstNonEmptyString(authorLabel, "A reader")
	if mode == PublishModeAuto {
		actor = "The paper"
	}
	child := noticeChildRef{PublicationVersionID: versionID, PublicationID: publicationID, RoutePath: routePath, ManifestHash: manifestHash,
		AuthorLabel: authorLabel, PublishedAt: now.Format(time.RFC3339Nano), PublishMode: mode}
	var out []objectgraph.Object
	for _, parent := range parents {
		if parent.OwnerID == "" || parent.OwnerID == req.OwnerID {
			continue
		}
		relation := noticeRelation(mode, respondsTo, parent)
		body := types.CommitmentChangelog{
			// Keyed on the two publications, not their versions: a republish
			// updates this notice and tells no one again.
			EventID:    types.ChangelogEventID(types.ChangelogParentUsed, parent.OwnerID, publicationID, parent.PublicationID),
			Kind:       types.ChangelogParentUsed,
			Level:      types.ChangelogMajor,
			Relation:   relation,
			OccurredAt: now.Format(time.RFC3339),
			ActorLabel: truncateRunes(authorLabel, types.ChangelogLabelMaxRunes),
			Subject: types.ChangelogSubject{PublicationVersionID: versionID, RoutePath: routePath,
				ParentVersionID: parent.PublicationVersionID, ParentRoutePath: parent.RoutePath},
			Summary: truncateRunes(fmt.Sprintf(noticeSummaries[relation], actor), types.ChangelogSummaryMaxRunes),
			Origin:  types.ChangelogOriginCorpusd,
		}
		obj, err := noticeObjectFor(parent.OwnerID, body, child, respondsTo, now)
		if err != nil {
			return nil, err
		}
		var wrapped changelogNoticeObject
		if json.Unmarshal(obj.Metadata, &wrapped) == nil {
			wrapped.PublisherOwnerID = req.OwnerID
			obj.Metadata = mustJSONRaw(wrapped)
		}
		out = append(out, obj)
	}
	return out, nil
}

// admitNotices applies the publisher's daily cap: an existing notice is
// updated (a republish), a new one is written while the publisher is under
// the cap, and the rest are dropped.
func (s *Service) admitNotices(ctx context.Context, og *ObjectGraphStore, publisher string, objs []objectgraph.Object) []objectgraph.Object {
	if len(objs) == 0 {
		return objs
	}
	used, err := s.countRecentNoticesBy(ctx, publisher, time.Now().UTC().Add(-24*time.Hour))
	if err != nil {
		used = noticesPerPublisherPerDay // fail closed: no new notice without a count
	}
	var out []objectgraph.Object
	for _, o := range objs {
		if _, err := og.GetObject(ctx, o.CanonicalID); err == nil {
			out = append(out, o)
			continue
		}
		if used < noticesPerPublisherPerDay {
			used++
			out = append(out, o)
		}
	}
	return out
}

func (s *Service) countRecentNoticesBy(ctx context.Context, publisher string, since time.Time) (int, error) {
	rows, err := s.store.corpus().QueryContext(ctx,
		`SELECT metadata FROM og_objects WHERE object_kind = 'choir.changelog_notice' AND tombstone = FALSE AND created_at >= ?`, since)
	if err != nil {
		return 0, err
	}
	defer rows.Close()
	n := 0
	for rows.Next() {
		var metadata string
		if err := rows.Scan(&metadata); err != nil {
			return 0, err
		}
		var o changelogNoticeObject
		if json.Unmarshal([]byte(metadata), &o) == nil && o.PublisherOwnerID == publisher {
			n++
		}
	}
	return n, rows.Err()
}
