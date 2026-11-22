package gatewayruntime

import (
	"context"
	"net/http"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/provideriface"
	"github.com/yusefmosiah/go-choir/internal/toolregistry"
)

// setCallerHeaders names the desk and the run of a gateway call, so the
// gateway's log line for it can be counted by desk (first design, F0). An
// explicit provideriface.Caller wins field by field; otherwise the run's
// execution context gives the run id and its role (or profile) as the desk.
// A field with no value sends no header; the gateway logs desk=unknown.
func setCallerHeaders(ctx context.Context, h http.Header) {
	c := provideriface.CallerFrom(ctx)
	exec := toolregistry.ExecutionContextFrom(ctx)
	run := firstNonEmpty(c.RunID, exec.RunID)
	desk := firstNonEmpty(c.Desk, exec.Role, exec.Profile)
	if run = strings.TrimSpace(run); run != "" {
		h.Set(provideriface.CallerHeaderRun, run)
	}
	if desk = strings.TrimSpace(desk); desk != "" {
		h.Set(provideriface.CallerHeaderDesk, desk)
	}
}
