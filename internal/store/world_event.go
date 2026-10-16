package store

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
)

// The event layer's V1 (docs/reports/event-layer-at-ingestion-design-
// 2026-10-16.md). A world event is a real-world occurrence the computer's
// captures report; a membership is Jev's judgment that one capture reports
// one event. Both are immutable objects on the computer, written once: an
// event's identity is its opening capture, a membership's is its capture,
// so a retried resolution writes nothing new. What an event currently
// holds (outlets, members, corroboration) is a projection rebuilt from the
// memberships, never stored as authority. Events are named
// choir.world_event because choir.event is the runtime's event log.
const (
	ogKindWorldEvent      = objectgraph.ObjectKind("choir.world_event")
	ogKindEventMembership = objectgraph.ObjectKind("choir.event_membership")
)

// SettingIntakeEvents turns on the event layer's shadow resolution: new
// captures from source polls are resolved onto events by Jev. Off by
// default; nothing reads the events yet, and turning it off is the
// rollback.
const SettingIntakeEvents = "intake.events"

func init() { KnownComputerSettings[SettingIntakeEvents] = false }

// WorldEvent is one real-world occurrence.
type WorldEvent struct {
	OwnerID      string    `json:"owner_id"`
	ComputerID   string    `json:"computer_id"`
	EventID      string    `json:"event_id"`
	Headline     string    `json:"headline"`
	OpenedBy     string    `json:"opened_by"` // the capture that minted it
	OpenedOutlet string    `json:"opened_outlet,omitempty"`
	FirstSeen    time.Time `json:"first_seen"`
}

// EventMembership is Jev's judgment that a capture reports an event. A
// tentative member matched with less than the join confidence: it is shown
// with the event but never counts toward its outlets.
type EventMembership struct {
	OwnerID    string  `json:"owner_id"`
	ComputerID string  `json:"computer_id"`
	CaptureID  string  `json:"capture_id"`
	EventID    string  `json:"event_id"`
	Opened     bool    `json:"opened,omitempty"` // this capture minted the event
	Tentative  bool    `json:"tentative,omitempty"`
	P          float64 `json:"p"`
	// Outlet is the publisher's key (internal/outlet); Origin, when set,
	// is the wire service the capture carries ("wire:reuters"), which
	// counts once however many outlets carry it.
	Outlet      string    `json:"outlet,omitempty"`
	Origin      string    `json:"origin,omitempty"`
	URL         string    `json:"url,omitempty"`
	Title       string    `json:"title,omitempty"`
	SourceID    string    `json:"source_id,omitempty"`
	CapturedAt  time.Time `json:"captured_at"`
	Model       string    `json:"model,omitempty"`
	InputTokens int       `json:"input_tokens,omitempty"`
	CostUSD     float64   `json:"cost_usd,omitempty"`
	ResolvedAt  time.Time `json:"resolved_at"`
}

func worldEventKey(computerID, eventID string) string {
	return strings.TrimSpace(computerID) + "\x00" + strings.TrimSpace(eventID)
}

// PutWorldEvent writes an event once. created is false when it exists; the
// stored event is returned either way.
func (s *Store) PutWorldEvent(ctx context.Context, ev WorldEvent) (WorldEvent, bool, error) {
	if s.og == nil {
		return WorldEvent{}, false, fmt.Errorf("store: object graph not initialized")
	}
	ev.OwnerID, ev.ComputerID, ev.EventID = strings.TrimSpace(ev.OwnerID), strings.TrimSpace(ev.ComputerID), strings.TrimSpace(ev.EventID)
	if ev.OwnerID == "" || ev.ComputerID == "" || ev.EventID == "" {
		return WorldEvent{}, false, fmt.Errorf("world event: owner_id, computer_id and event_id are required")
	}
	body, err := json.Marshal(ev)
	if err != nil {
		return WorldEvent{}, false, err
	}
	obj, created, err := s.og.CreateObjectIfAbsent(ctx, objectgraph.CreateObjectRequest{
		Kind: ogKindWorldEvent, OwnerID: ev.OwnerID, ComputerID: ev.ComputerID,
		IdentityKey: worldEventKey(ev.ComputerID, ev.EventID), Body: body,
		Metadata: map[string]any{"event_id": ev.EventID, "computer_id": ev.ComputerID},
		Now:      ev.FirstSeen,
	})
	if err != nil {
		return WorldEvent{}, false, fmt.Errorf("world event: %w", err)
	}
	if !created {
		var stored WorldEvent
		if err := ogDecode(obj, &stored); err != nil {
			return WorldEvent{}, false, err
		}
		return stored, false, nil
	}
	return ev, true, nil
}

// PutEventMembership writes a capture's membership once. created is false
// when the capture already has one; the stored membership is returned.
func (s *Store) PutEventMembership(ctx context.Context, m EventMembership) (EventMembership, bool, error) {
	if s.og == nil {
		return EventMembership{}, false, fmt.Errorf("store: object graph not initialized")
	}
	m.OwnerID, m.ComputerID, m.CaptureID, m.EventID = strings.TrimSpace(m.OwnerID), strings.TrimSpace(m.ComputerID), strings.TrimSpace(m.CaptureID), strings.TrimSpace(m.EventID)
	if m.OwnerID == "" || m.ComputerID == "" || m.CaptureID == "" || m.EventID == "" {
		return EventMembership{}, false, fmt.Errorf("event membership: owner_id, computer_id, capture_id and event_id are required")
	}
	if m.ResolvedAt.IsZero() {
		m.ResolvedAt = time.Now().UTC()
	}
	body, err := json.Marshal(m)
	if err != nil {
		return EventMembership{}, false, err
	}
	obj, created, err := s.og.CreateObjectIfAbsent(ctx, objectgraph.CreateObjectRequest{
		Kind: ogKindEventMembership, OwnerID: m.OwnerID, ComputerID: m.ComputerID,
		IdentityKey: worldEventKey(m.ComputerID, m.CaptureID), Body: body,
		Metadata: map[string]any{"capture_id": m.CaptureID, "event_id": m.EventID, "computer_id": m.ComputerID},
		Now:      m.ResolvedAt,
	})
	if err != nil {
		return EventMembership{}, false, fmt.Errorf("event membership: %w", err)
	}
	if !created {
		var stored EventMembership
		if err := ogDecode(obj, &stored); err != nil {
			return EventMembership{}, false, err
		}
		return stored, false, nil
	}
	return m, true, nil
}

// EventMembershipOf returns a capture's membership.
func (s *Store) EventMembershipOf(ctx context.Context, ownerID, computerID, captureID string) (EventMembership, bool, error) {
	objs, err := s.ogListAllByMetadata(ctx, ogKindEventMembership, "capture_id", strings.TrimSpace(captureID))
	if err != nil {
		return EventMembership{}, false, err
	}
	for _, obj := range objs {
		var m EventMembership
		if err := ogDecode(obj, &m); err != nil {
			return EventMembership{}, false, err
		}
		if m.OwnerID == strings.TrimSpace(ownerID) && m.ComputerID == strings.TrimSpace(computerID) {
			return m, true, nil
		}
	}
	return EventMembership{}, false, nil
}

// RecentWorldEvents returns the computer's newest limit events, oldest
// first.
func (s *Store) RecentWorldEvents(ctx context.Context, ownerID, computerID string, limit int) ([]WorldEvent, error) {
	var out []WorldEvent
	err := s.listRecentOwned(ctx, ogKindWorldEvent, ownerID, computerID, limit, func(obj objectgraph.Object) error {
		var ev WorldEvent
		if err := ogDecode(obj, &ev); err != nil {
			return err
		}
		if ev.OwnerID == strings.TrimSpace(ownerID) && ev.ComputerID == strings.TrimSpace(computerID) {
			out = append(out, ev)
		}
		return nil
	})
	sort.SliceStable(out, func(i, j int) bool { return out[i].FirstSeen.Before(out[j].FirstSeen) })
	return out, err
}

// RecentEventMemberships returns the computer's newest limit memberships,
// oldest first.
func (s *Store) RecentEventMemberships(ctx context.Context, ownerID, computerID string, limit int) ([]EventMembership, error) {
	var out []EventMembership
	err := s.listRecentOwned(ctx, ogKindEventMembership, ownerID, computerID, limit, func(obj objectgraph.Object) error {
		var m EventMembership
		if err := ogDecode(obj, &m); err != nil {
			return err
		}
		if m.OwnerID == strings.TrimSpace(ownerID) && m.ComputerID == strings.TrimSpace(computerID) {
			out = append(out, m)
		}
		return nil
	})
	sort.SliceStable(out, func(i, j int) bool { return out[i].ResolvedAt.Before(out[j].ResolvedAt) })
	return out, err
}

func (s *Store) listRecentOwned(ctx context.Context, kind objectgraph.ObjectKind, ownerID, computerID string, limit int, each func(objectgraph.Object) error) error {
	if s.og == nil {
		return fmt.Errorf("store: object graph not initialized")
	}
	if limit <= 0 {
		limit = 500
	}
	objs, err := s.og.ListObjects(ctx, objectgraph.ListFilter{Kind: kind, OwnerID: strings.TrimSpace(ownerID), ComputerID: strings.TrimSpace(computerID), Limit: limit})
	if err != nil {
		return err
	}
	for _, obj := range objs {
		if err := each(obj); err != nil {
			return err
		}
	}
	return nil
}
