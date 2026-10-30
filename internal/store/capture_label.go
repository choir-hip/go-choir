package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
)

// Intake labels (L2a A1 / R2-3,
// docs/archive/reports/l2a-synthesis-intake-design-2026-10-14.md). Jev labels each
// new capture against the computer's story shortlist. A label is its own
// object, keyed by (capture, version), and never part of the arrival
// record: the arrival's command digest covers the whole record, so a
// relabel on retry would refuse its replay. Versions are immutable: a
// relabel writes the next version, and management's corrections are linked
// override objects naming the version they correct (A3 / R2-5).
const (
	ogKindCaptureLabel         = objectgraph.ObjectKind("choir.capture_label")
	ogKindCaptureLabelOverride = objectgraph.ObjectKind("choir.capture_label_override")
)

// What a label's event answer names.
const (
	CaptureLabelEventStory   = "story"   // the same event as a candidate story
	CaptureLabelEventSibling = "sibling" // the same event as another new capture of the poll
	CaptureLabelEventNew     = "new"     // no candidate reports this event
)

// CaptureLabel is one version of Jev's label for one capture.
type CaptureLabel struct {
	OwnerID    string `json:"owner_id"`
	ComputerID string `json:"computer_id"`
	CaptureID  string `json:"capture_id"`
	Version    int    `json:"version"`
	SourceID   string `json:"source_id,omitempty"`
	// The capture as Jev read it, kept so a correction can be shown as a
	// worked example without reading Store B.
	Title    string `json:"title,omitempty"`
	Outlet   string `json:"outlet,omitempty"`
	Language string `json:"language,omitempty"`
	Script   string `json:"script,omitempty"`
	// The event answer: Event is story, sibling or new; EventDocID and
	// EventHeadline name the story, EventCaptureID the sibling capture.
	Event          string  `json:"event"`
	EventDocID     string  `json:"event_doc_id,omitempty"`
	EventHeadline  string  `json:"event_headline,omitempty"`
	EventCaptureID string  `json:"event_capture_id,omitempty"`
	EventP         float64 `json:"event_p"`
	// The topic answer: a candidate story that shares the topic, or none.
	TopicDocID    string  `json:"topic_doc_id,omitempty"`
	TopicHeadline string  `json:"topic_headline,omitempty"`
	TopicP        float64 `json:"topic_p,omitempty"`
	// What Jev was shown: the candidate stories and sibling captures.
	CandidateDocIDs   []string  `json:"candidate_doc_ids,omitempty"`
	SiblingCaptureIDs []string  `json:"sibling_capture_ids,omitempty"`
	Model             string    `json:"model"`
	InputTokens       int       `json:"input_tokens,omitempty"`
	OutputTokens      int       `json:"output_tokens,omitempty"`
	CostUSD           float64   `json:"cost_usd,omitempty"`
	LabelledAt        time.Time `json:"labelled_at"`
}

// CaptureLabelOverride records management disposing of a capture otherwise
// than its label said: fed into another story, opened as a new story, or
// declined. It names the label version it corrects and never edits it.
type CaptureLabelOverride struct {
	OwnerID      string    `json:"owner_id"`
	ComputerID   string    `json:"computer_id"`
	CaptureID    string    `json:"capture_id"`
	LabelVersion int       `json:"label_version"`
	CaptureTitle string    `json:"capture_title,omitempty"`
	JevSaid      string    `json:"jev_said"`
	Disposition  string    `json:"disposition"`
	DocID        string    `json:"doc_id,omitempty"`
	Headline     string    `json:"headline,omitempty"`
	RecordID     string    `json:"record_id"`
	Reason       string    `json:"reason,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
}

func captureLabelKey(computerID, captureID string, version int) string {
	return strings.Join([]string{strings.TrimSpace(computerID), strings.TrimSpace(captureID), fmt.Sprint(version)}, "\x00")
}

// AppendCaptureLabel writes label as the capture's next version and returns
// it with its version set. An existing version is never overwritten.
func (s *Store) AppendCaptureLabel(ctx context.Context, label CaptureLabel) (CaptureLabel, error) {
	if s.og == nil {
		return CaptureLabel{}, fmt.Errorf("store: object graph not initialized")
	}
	label.OwnerID, label.ComputerID, label.CaptureID = strings.TrimSpace(label.OwnerID), strings.TrimSpace(label.ComputerID), strings.TrimSpace(label.CaptureID)
	if label.OwnerID == "" || label.ComputerID == "" || label.CaptureID == "" {
		return CaptureLabel{}, fmt.Errorf("capture label: owner_id, computer_id and capture_id are required")
	}
	if label.LabelledAt.IsZero() {
		label.LabelledAt = time.Now().UTC()
	}
	latest, found, err := s.LatestCaptureLabel(ctx, label.OwnerID, label.ComputerID, label.CaptureID)
	if err != nil {
		return CaptureLabel{}, err
	}
	label.Version = 1
	if found {
		label.Version = latest.Version + 1
	}
	for attempt := 0; attempt < 4; attempt++ {
		body, err := json.Marshal(label)
		if err != nil {
			return CaptureLabel{}, err
		}
		_, created, err := s.og.CreateObjectIfAbsent(ctx, objectgraph.CreateObjectRequest{
			Kind: ogKindCaptureLabel, OwnerID: label.OwnerID, ComputerID: label.ComputerID,
			IdentityKey: captureLabelKey(label.ComputerID, label.CaptureID, label.Version), Body: body,
			Metadata: map[string]any{"capture_id": label.CaptureID, "computer_id": label.ComputerID, "version": label.Version},
			Now:      label.LabelledAt,
		})
		if err != nil {
			return CaptureLabel{}, fmt.Errorf("capture label: %w", err)
		}
		if created {
			return label, nil
		}
		label.Version++
	}
	return CaptureLabel{}, fmt.Errorf("capture label: %s: versions kept changing under the write", label.CaptureID)
}

// LatestCaptureLabel returns the capture's newest label version.
func (s *Store) LatestCaptureLabel(ctx context.Context, ownerID, computerID, captureID string) (CaptureLabel, bool, error) {
	labels, err := s.CaptureLabels(ctx, ownerID, computerID, captureID)
	if err != nil || len(labels) == 0 {
		return CaptureLabel{}, false, err
	}
	return labels[len(labels)-1], true, nil
}

// CaptureLabels returns every label version of a capture, oldest first.
// AppendCaptureLabel keys each version captureLabelKey(computer, capture,
// version) and numbers them 1..N with no gaps, so the read gets versions by
// primary key until the first missing one. It used to scan every capture
// label's $.capture_id, which held the paper's single engine connection for
// seconds (docs/problems/research-more-slow-and-hidden-2026-10-21.md, E6).
func (s *Store) CaptureLabels(ctx context.Context, ownerID, computerID, captureID string) ([]CaptureLabel, error) {
	ownerID, computerID, captureID = strings.TrimSpace(ownerID), strings.TrimSpace(computerID), strings.TrimSpace(captureID)
	if ownerID == "" || computerID == "" || captureID == "" {
		return nil, nil
	}
	var out []CaptureLabel
	for version := 1; ; version++ {
		// The key already names the computer, and the label was created
		// with that key unscoped, so the get passes no lifecycle scope.
		obj, err := s.ogGetByIdentityKey(ctx, ogKindCaptureLabel, ownerID, "", captureLabelKey(computerID, captureID, version))
		if errors.Is(err, objectgraph.ErrNotFound) {
			return out, nil
		}
		if err != nil {
			return nil, err
		}
		var label CaptureLabel
		if err := ogDecode(obj, &label); err != nil {
			return nil, err
		}
		out = append(out, label)
	}
}

// RecentCaptureLabels returns the newest label version of each capture
// among the computer's newest limit label objects, keyed by capture id.
// Corpus search reads them as topic and outlet hints for captures.
func (s *Store) RecentCaptureLabels(ctx context.Context, ownerID, computerID string, limit int) (map[string]CaptureLabel, error) {
	if s.og == nil {
		return nil, fmt.Errorf("store: object graph not initialized")
	}
	if limit <= 0 {
		limit = 500
	}
	ownerID, computerID = strings.TrimSpace(ownerID), strings.TrimSpace(computerID)
	objs, err := s.og.ListObjects(ctx, objectgraph.ListFilter{Kind: ogKindCaptureLabel, OwnerID: ownerID, ComputerID: computerID, Limit: limit})
	if err != nil {
		return nil, err
	}
	out := map[string]CaptureLabel{}
	for _, obj := range objs {
		var label CaptureLabel
		if err := ogDecode(obj, &label); err != nil {
			return nil, err
		}
		if label.OwnerID != ownerID || label.ComputerID != computerID {
			continue
		}
		if have, ok := out[label.CaptureID]; !ok || label.Version > have.Version {
			out[label.CaptureID] = label
		}
	}
	return out, nil
}

// PutCaptureLabelOverride records one correction. The same disposition
// record correcting the same label version is one override.
func (s *Store) PutCaptureLabelOverride(ctx context.Context, o CaptureLabelOverride) error {
	if s.og == nil {
		return fmt.Errorf("store: object graph not initialized")
	}
	o.OwnerID, o.ComputerID, o.CaptureID, o.RecordID = strings.TrimSpace(o.OwnerID), strings.TrimSpace(o.ComputerID), strings.TrimSpace(o.CaptureID), strings.TrimSpace(o.RecordID)
	if o.OwnerID == "" || o.ComputerID == "" || o.CaptureID == "" || o.RecordID == "" || o.LabelVersion <= 0 {
		return fmt.Errorf("capture label override: owner, computer, capture, label version and record are required")
	}
	if o.CreatedAt.IsZero() {
		o.CreatedAt = time.Now().UTC()
	}
	body, err := json.Marshal(o)
	if err != nil {
		return err
	}
	_, _, err = s.og.CreateObjectIfAbsent(ctx, objectgraph.CreateObjectRequest{
		Kind: ogKindCaptureLabelOverride, OwnerID: o.OwnerID, ComputerID: o.ComputerID,
		IdentityKey: captureLabelKey(o.ComputerID, o.CaptureID, o.LabelVersion) + "\x00" + o.RecordID, Body: body,
		Metadata: map[string]any{"capture_id": o.CaptureID, "computer_id": o.ComputerID, "label_version": o.LabelVersion, "record_id": o.RecordID},
		Now:      o.CreatedAt,
	})
	return err
}

// CaptureLabelOverrides returns a capture's corrections, oldest first.
func (s *Store) CaptureLabelOverrides(ctx context.Context, ownerID, computerID, captureID string) ([]CaptureLabelOverride, error) {
	objs, err := s.ogListAllByMetadata(ctx, ogKindCaptureLabelOverride, "capture_id", strings.TrimSpace(captureID))
	if err != nil {
		return nil, err
	}
	var out []CaptureLabelOverride
	for _, obj := range objs {
		var o CaptureLabelOverride
		if err := ogDecode(obj, &o); err != nil {
			return nil, err
		}
		if o.OwnerID == strings.TrimSpace(ownerID) && o.ComputerID == strings.TrimSpace(computerID) {
			out = append(out, o)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.Before(out[j].CreatedAt) })
	return out, nil
}

// RecentCaptureLabelOverrides returns the computer's newest corrections,
// newest first, at most limit.
func (s *Store) RecentCaptureLabelOverrides(ctx context.Context, ownerID, computerID string, limit int) ([]CaptureLabelOverride, error) {
	if s.og == nil {
		return nil, fmt.Errorf("store: object graph not initialized")
	}
	if limit <= 0 {
		limit = 12
	}
	objs, err := s.og.ListObjects(ctx, objectgraph.ListFilter{Kind: ogKindCaptureLabelOverride, OwnerID: strings.TrimSpace(ownerID), ComputerID: strings.TrimSpace(computerID), Limit: limit})
	if err != nil {
		return nil, err
	}
	out := make([]CaptureLabelOverride, 0, len(objs))
	for _, obj := range objs {
		var o CaptureLabelOverride
		if err := ogDecode(obj, &o); err != nil {
			return nil, err
		}
		out = append(out, o)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	return out, nil
}
