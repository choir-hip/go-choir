package proxy

import (
	"fmt"
	"html"
	"net/http"
	"strconv"
	"strings"
)

// writeComputerSurfaceResolveError answers a desktop page load whose
// computer is starting, recovering or briefly blocked. A browser gets a
// small page in plain words that retries itself; API callers keep the
// structured JSON (docs/problems/recovering-computer-shows-raw-json-2026-10-09.md).
func writeComputerSurfaceResolveError(w http.ResponseWriter, r *http.Request, err error) {
	if !wantsHTMLPage(r) {
		writeResolveError(w, err)
		return
	}
	retry := 5
	message := "Your computer is starting up."
	detail := "This page will reload by itself."
	if refusal := recoveryRefusalFromError(err); refusal != nil {
		retry = refusal.RetryAfterSeconds
		switch refusal.Kind {
		case "projection_base_missing", "recovery_tail_excess":
			message = "Your computer is being restored."
			detail = "This usually takes about a minute. This page will reload by itself."
		case "guest_reattach_pending":
			// Running but not answering. vmctl restarts a guest that stays
			// unreachable by itself (wedge.go); the owner never has to
			// (docs/problems/owner-computer-stranded-after-vmctl-restart-2026-10-10.md).
			message = "Your computer is not answering."
			detail = "If it stays this way it restarts by itself within a few minutes. This page will keep checking."
		case "privacy_key_unavailable":
			message = "Your computer is waiting for its key."
			detail = "It cannot start until its key is available. This page will keep checking."
		default:
			message = "Your computer cannot start right now."
			detail = "This page will keep checking."
		}
	}
	if retry <= 0 {
		retry = 5
	}
	if retry > 30 {
		retry = 30
	}
	w.Header().Set("Retry-After", strconv.Itoa(retry))
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusServiceUnavailable)
	_, _ = fmt.Fprintf(w, computerStartingPage, retry, html.EscapeString(message), html.EscapeString(detail))
}

func wantsHTMLPage(r *http.Request) bool {
	if r == nil || (r.Method != http.MethodGet && r.Method != http.MethodHead) {
		return false
	}
	return strings.Contains(r.Header.Get("Accept"), "text/html")
}

const computerStartingPage = `<!doctype html>
<html lang="en"><head><meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<meta http-equiv="refresh" content="%d">
<title>Choir</title>
<style>
:root{--bg:#f7f7f5;--fg:#1d1d1b;--muted:#6b6b66}
@media (prefers-color-scheme: dark){:root{--bg:#141413;--fg:#ececea;--muted:#9a9a94}}
body{margin:0;min-height:100vh;display:grid;place-items:center;background:var(--bg);color:var(--fg);font:16px/1.5 system-ui,-apple-system,sans-serif;padding:16px}
main{max-width:28rem;text-align:center}
h1{font-size:1.25rem;font-weight:600;margin:0 0 .5rem}
p{margin:0;color:var(--muted)}
</style></head>
<body><main><h1>%s</h1><p>%s</p></main></body></html>
`
