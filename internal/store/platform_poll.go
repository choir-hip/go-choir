package store

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
)

// ogKindPlatformPoll is one computer's poll of a corpusd feed (autopaper
// contracts 2 and 3, amendment A3): the paper's oped_intake and a reader's
// notice_intake. A poll makes no model call. It is a standing intent: a
// restart leaves an active poll active, a crash closes only the poll in
// flight, and a poll that was in flight at two crash boots in a row is closed
// with that cause (the same rule as a source schedule).
const ogKindPlatformPoll = objectgraph.ObjectKind("choir.platform_poll")

// Platform poll names.
const (
	PlatformPollOpedIntake   = "oped_intake"
	PlatformPollNoticeIntake = "notice_intake"
)

// Platform poll states.
const (
	PlatformPollActive = "active"
	PlatformPollClosed = "closed"
)

// PlatformPollCrashLoopLimit is how many crash boots in a row may find a poll
// in flight before the poll is closed.
const PlatformPollCrashLoopLimit = 2

// PlatformPoll is the persisted state of one poll.
type PlatformPoll struct {
	OwnerID         string `json:"owner_id"`
	ComputerID      string `json:"computer_id"`
	Name            string `json:"name"`
	Cursor          string `json:"cursor,omitempty"`
	IntervalSeconds int64  `json:"interval_seconds"`
	State           string `json:"state"`
	StateReason     string `json:"state_reason,omitempty"`
	// LastPollAt is when the last poll finished, LastOKAt when one last
	// succeeded; LastError is the last failure (bounded).
	LastPollAt          time.Time `json:"last_poll_at,omitempty"`
	LastOKAt            time.Time `json:"last_ok_at,omitempty"`
	LastError           string    `json:"last_error,omitempty"`
	ConsecutiveFailures int       `json:"consecutive_failures"`
	// PollInFlightSince is set durably when a poll starts and cleared when it
	// records its result; a crash boot that finds it set closes that poll and
	// counts the crash.
	PollInFlightSince       time.Time `json:"poll_in_flight_since,omitempty"`
	CrashesWithPollInFlight int       `json:"crashes_with_poll_in_flight,omitempty"`
	UpdatedAt               time.Time `json:"updated_at"`
}

const platformPollErrorLimit = 512

func platformPollKey(name string) string { return strings.TrimSpace(name) }

// PutPlatformPoll writes the poll, replacing its previous state.
func (s *Store) PutPlatformPoll(ctx context.Context, poll PlatformPoll) error {
	poll.OwnerID, poll.Name = strings.TrimSpace(poll.OwnerID), strings.TrimSpace(poll.Name)
	if poll.OwnerID == "" || poll.Name == "" {
		return fmt.Errorf("platform poll: owner_id and name are required")
	}
	if len(poll.LastError) > platformPollErrorLimit {
		poll.LastError = strings.ToValidUTF8(poll.LastError[:platformPollErrorLimit], "") + "…"
	}
	if poll.UpdatedAt.IsZero() {
		poll.UpdatedAt = time.Now().UTC()
	}
	metadata := map[string]any{
		"name": poll.Name, "state": poll.State, "updated_at": poll.UpdatedAt.UTC().Format(time.RFC3339Nano),
	}
	_, err := s.ogPut(ctx, ogKindPlatformPoll, poll.OwnerID, platformPollKey(poll.Name), poll, metadata, poll.UpdatedAt)
	return err
}

// GetPlatformPoll returns the owner's poll by name, or ErrNotFound.
func (s *Store) GetPlatformPoll(ctx context.Context, ownerID, name string) (PlatformPoll, error) {
	if s.og == nil {
		return PlatformPoll{}, fmt.Errorf("store: object graph not initialized")
	}
	id, err := objectgraph.BuildCanonicalID(ogKindPlatformPoll, strings.TrimSpace(ownerID), objectgraph.StableSuffixFromKey(platformPollKey(name)))
	if err != nil {
		return PlatformPoll{}, err
	}
	obj, err := s.og.GetObject(ctx, id)
	if errors.Is(err, objectgraph.ErrNotFound) {
		return PlatformPoll{}, ErrNotFound
	}
	if err != nil {
		return PlatformPoll{}, err
	}
	var poll PlatformPoll
	if err := ogDecode(obj, &poll); err != nil {
		return PlatformPoll{}, err
	}
	return poll, nil
}

// ListPlatformPolls lists the polls of one owner, or all (ownerID "").
func (s *Store) ListPlatformPolls(ctx context.Context, ownerID string) ([]PlatformPoll, error) {
	if s.og == nil {
		return nil, fmt.Errorf("store: object graph not initialized")
	}
	objs, err := s.og.ListObjects(ctx, objectgraph.ListFilter{Kind: ogKindPlatformPoll, OwnerID: strings.TrimSpace(ownerID), Limit: 100})
	if err != nil {
		return nil, err
	}
	out := make([]PlatformPoll, 0, len(objs))
	for _, obj := range objs {
		var poll PlatformPoll
		if err := ogDecode(obj, &poll); err != nil {
			return nil, err
		}
		out = append(out, poll)
	}
	return out, nil
}
