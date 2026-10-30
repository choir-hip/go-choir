package store

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
)

// The guest keeps no event bodies (data system build sheet decision 8). The
// event index holds, per event, the sequence, digest, kind, idempotency key
// and receipt digest; a prepared row also holds the event body until it
// finalizes. A lookup that needs a finalized event's body (EventByDigest,
// EventByIdempotency, the receipt lookups) is served from a small recent-event
// cache, then from the tape: the canonical event and its receipt are one
// record of the host's replay page at sequence-1.

// ErrEventBodyUnavailable is returned when a finalized event's body is not in
// the recent-event cache and no tape source is configured.
var ErrEventBodyUnavailable = errors.New("computer event projection: event body is not held by the guest and no tape source is configured")

// eventBodyCacheLimit bounds the recent-event cache (entries).
const eventBodyCacheLimit = 1024

type eventBodyEntry struct {
	event      computerevent.Event
	hasEvent   bool
	receipt    computerevent.Receipt
	hasReceipt bool
}

// eventBodyCache is a bounded FIFO of recent event bodies, keyed by computer
// and event digest. Events are immutable, so it is never invalidated.
type eventBodyCache struct {
	mu      sync.Mutex
	entries map[string]*eventBodyEntry
	order   []string
}

func eventBodyKey(computerID, digest string) string { return computerID + "\x00" + digest }

func (c *eventBodyCache) entryLocked(key string) *eventBodyEntry {
	if c.entries == nil {
		c.entries = make(map[string]*eventBodyEntry)
	}
	entry, ok := c.entries[key]
	if !ok {
		entry = &eventBodyEntry{}
		c.entries[key] = entry
		c.order = append(c.order, key)
		for len(c.order) > eventBodyCacheLimit {
			delete(c.entries, c.order[0])
			c.order = c.order[1:]
		}
	}
	return entry
}

func (c *eventBodyCache) putEvent(computerID, digest string, event computerevent.Event) {
	c.mu.Lock()
	defer c.mu.Unlock()
	entry := c.entryLocked(eventBodyKey(computerID, digest))
	entry.event, entry.hasEvent = event, true
}

func (c *eventBodyCache) putReceipt(computerID, digest string, receipt computerevent.Receipt) {
	c.mu.Lock()
	defer c.mu.Unlock()
	entry := c.entryLocked(eventBodyKey(computerID, digest))
	entry.receipt, entry.hasReceipt = receipt, true
}

func (c *eventBodyCache) get(computerID, digest string) (eventBodyEntry, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	entry, ok := c.entries[eventBodyKey(computerID, digest)]
	if !ok || !entry.hasEvent || !entry.hasReceipt {
		return eventBodyEntry{}, false
	}
	return *entry, true
}

// eventSourceHolder lets the tape source be swapped atomically.
type eventSourceHolder struct {
	source computerevent.PagedEventSource
}

// SetEventSource wires the tape source finalized-event lookups fall back to
// (the guest's computerevent.HTTPClient). Call it once at boot, before the
// first lookup that can miss the recent-event cache.
func (s *Store) SetEventSource(source computerevent.PagedEventSource) {
	if s == nil {
		return
	}
	if source == nil {
		s.eventSource.Store(nil)
		return
	}
	s.eventSource.Store(&eventSourceHolder{source: source})
}

// finalizedEventBody returns a finalized event and its receipt.
func (s *Store) finalizedEventBody(ctx context.Context, computerID, digest string, sequence uint64) (computerevent.DurableEvent, error) {
	if entry, ok := s.eventBodies.get(computerID, digest); ok {
		return computerevent.DurableEvent{Request: computerevent.CASRequest{Event: entry.event, EventDigest: digest}, Receipt: entry.receipt}, nil
	}
	holder := s.eventSource.Load()
	if holder == nil || holder.source == nil {
		return computerevent.DurableEvent{}, ErrEventBodyUnavailable
	}
	if sequence == 0 {
		return computerevent.DurableEvent{}, fmt.Errorf("computer event projection: event %s has no sequence", digest)
	}
	page, err := holder.source.EventsPage(ctx, computerID, sequence-1, 1)
	if err != nil {
		return computerevent.DurableEvent{}, fmt.Errorf("computer event projection: read event from tape: %w", err)
	}
	if len(page) != 1 || page[0].Request.EventDigest != digest || page[0].Request.Event.Sequence != sequence {
		return computerevent.DurableEvent{}, fmt.Errorf("computer event projection: tape does not hold event %s at sequence %d", digest, sequence)
	}
	record := page[0]
	s.eventBodies.putEvent(computerID, digest, record.Request.Event)
	s.eventBodies.putReceipt(computerID, digest, record.Receipt)
	return record, nil
}
