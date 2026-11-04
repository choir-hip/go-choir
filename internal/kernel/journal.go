package kernel

import (
	"sync"
	"time"
)

// Record kinds. A unit is declared once, started per occurrence, and ends
// stopped (clean exit or shutdown) or closed (with a cause).
const (
	RecordDeclared = "declared"
	RecordStarted  = "started"
	RecordStopped  = "stopped"
	RecordClosed   = "closed"
)

// CauseInterrupted is the cause recorded for an occurrence cut off by a restart.
const CauseInterrupted = "interrupted by a restart"

// Record is one durable fact about a unit.
type Record struct {
	Unit  string    `json:"unit"`
	Kind  string    `json:"kind"`
	Cause string    `json:"cause,omitempty"`
	At    time.Time `json:"at"`
}

// Journal is the durable fact log the boot fold reads. Implementations must
// return records in append order.
type Journal interface {
	Append(Record) error
	Load() ([]Record, error)
}

// MemJournal is an in-memory Journal.
type MemJournal struct {
	mu   sync.Mutex
	recs []Record
}

// Append adds a record.
func (j *MemJournal) Append(r Record) error {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.recs = append(j.recs, r)
	return nil
}

// Load returns a copy of the records.
func (j *MemJournal) Load() ([]Record, error) {
	j.mu.Lock()
	defer j.mu.Unlock()
	return append([]Record(nil), j.recs...), nil
}
