package tapestore

import (
	"errors"
	"os"
)

// STUB for the test-first run; replaced by the implementation.
type Item struct {
	NS   NS
	Body []byte
}
type AppenderStats struct {
	Appends, Records, Syncs, SegmentsCreated, IndexesCreated, Seals, TornTails, Errors uint64 `json:"x"`
}
type Appender struct {
	writeAt  func(f *os.File, b []byte, off int64) (int, error)
	syncFile func(*os.File) error
}

func NewAppender(root string) *Appender                   { return &Appender{} }
func newAppender(root string, b int64, r int) *Appender   { return &Appender{} }
func (a *Appender) Append(id string, items ...Item) error { return errors.New("not implemented") }
func (a *Appender) Stats() AppenderStats                  { return AppenderStats{} }
