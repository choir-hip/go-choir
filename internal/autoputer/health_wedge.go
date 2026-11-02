package autoputer

import (
	"bytes"
	"encoding/json"
	"net/http"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
)

// A persistent appender wedge (the event appender keeps refusing appends)
// degrades /health. Semantics, chosen to be conservative:
//
//   - The HTTP status stays 200. Every consumer that gates on readiness
//     (vmmanager.probeGuestHealthDetailed, vmctl routing, the probe scripts)
//     keys on status code 200, so a degraded body cannot mark the guest
//     unhealthy, re-route it or trigger a vmctl recovery restart. A restart
//     closes open work ("a crash restart never resumes work"); making health
//     fail would turn a visible wedge into an automatic restart loop.
//   - The body status becomes "degraded" and gains projection_repair_required,
//     so a human or a body-reading probe sees the wedge, its first cause and
//     the heads. Transient refusals do not degrade (see the appender's
//     threshold: 3 consecutive refusals and 2 minutes).
//
// Whether a persistent wedge should also fail readiness is a separate,
// deliberate decision for the owner (restart policy), not made here.
func (g *replayHealthGate) wedgeStatus(app *computerevent.ComputerEventAppender) computerevent.WedgeStatus {
	if g.wedge != nil {
		return g.wedge()
	}
	return app.WedgeStatus()
}

// serveDegradedHealth runs the base handler into a buffer and, when it
// answered a JSON object with 200, rewrites its status and adds the wedge.
// Any other base answer passes through untouched.
func serveDegradedHealth(w http.ResponseWriter, r *http.Request, base http.HandlerFunc, wedge computerevent.WedgeStatus) {
	buf := &bufferedResponse{header: http.Header{}}
	base(buf, r)
	var body map[string]any
	if buf.status() != http.StatusOK || json.Unmarshal(buf.body.Bytes(), &body) != nil || body == nil {
		for k, v := range buf.header {
			w.Header()[k] = v
		}
		w.WriteHeader(buf.status())
		_, _ = w.Write(buf.body.Bytes())
		return
	}
	body["status"] = "degraded"
	body["projection_repair_required"] = map[string]any{
		"since":        wedge.Since.UTC().Format(time.RFC3339),
		"consecutive":  wedge.Consecutive,
		"local_seq":    wedge.LocalSeq,
		"platform_seq": wedge.PlatformSeq,
		"first_error":  wedge.FirstError,
		"last_error":   wedge.LastError,
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(body)
}

type bufferedResponse struct {
	header http.Header
	code   int
	body   bytes.Buffer
}

func (b *bufferedResponse) Header() http.Header { return b.header }
func (b *bufferedResponse) WriteHeader(code int) {
	if b.code == 0 {
		b.code = code
	}
}
func (b *bufferedResponse) Write(p []byte) (int, error) {
	if b.code == 0 {
		b.code = http.StatusOK
	}
	return b.body.Write(p)
}
func (b *bufferedResponse) status() int {
	if b.code == 0 {
		return http.StatusOK
	}
	return b.code
}
