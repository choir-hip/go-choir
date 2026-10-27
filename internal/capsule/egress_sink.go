package capsule

import (
	"encoding/json"
	"log"
	"os"
	"sync"
	"time"
)

// egressSink receives every EgressRecord of one capsule.
type egressSink struct {
	mu   sync.Mutex
	path string
	logf func(format string, args ...any)
}

func newEgressSink(path string, logf func(format string, args ...any), now func() time.Time, interval time.Duration) *egressSink {
	if logf == nil {
		logf = log.Printf
	}
	return &egressSink{path: path, logf: logf}
}

func (s *egressSink) Record(record EgressRecord) {
	line, _ := json.Marshal(record)
	s.logf("capsule-egress: %s", line)
	s.mu.Lock()
	defer s.mu.Unlock()
	if f, err := os.OpenFile(s.path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600); err == nil {
		_, _ = f.Write(append(line, '\n'))
		_ = f.Close()
	}
}

func (s *egressSink) Close() {}
