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
	restartable := false
	message := "Your computer is starting up."
	detail := "This page will reload by itself."
	if refusal := recoveryRefusalFromError(err); refusal != nil {
		retry = refusal.RetryAfterSeconds
		switch refusal.Kind {
		case "projection_base_missing", "recovery_tail_excess":
			message = "Your computer is being restored."
			detail = "This usually takes about a minute. This page will reload by itself."
		case "guest_reattach_pending":
			// Running but not answering: no automatic path ends it, so the
			// owner gets the restart here
			// (docs/problems/owner-computer-stranded-after-vmctl-restart-2026-10-10.md).
			restartable = true
			message = "Your computer is running but not answering."
			detail = "You can restart it. Restarting ends any work in progress."
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
	if restartable {
		_, _ = fmt.Fprintf(w, computerRestartPage, html.EscapeString(message), html.EscapeString(detail), retry)
		return
	}
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

// computerRestartPage offers the owner's restart. It reloads by a timer the
// click cancels, so a reload never abandons a restart in flight.
const computerRestartPage = `<!doctype html>
<html lang="en"><head><meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>Choir</title>
<style>
:root{--bg:#f7f7f5;--fg:#1d1d1b;--muted:#6b6b66}
@media (prefers-color-scheme: dark){:root{--bg:#141413;--fg:#ececea;--muted:#9a9a94}}
body{margin:0;min-height:100vh;display:grid;place-items:center;background:var(--bg);color:var(--fg);font:16px/1.5 system-ui,-apple-system,sans-serif;padding:16px}
main{max-width:28rem;text-align:center}
h1{font-size:1.25rem;font-weight:600;margin:0 0 .5rem}
p{margin:0 0 1rem;color:var(--muted)}
button{font:inherit;font-weight:600;padding:.5rem 1.25rem;border-radius:999px;border:1px solid currentColor;background:transparent;color:var(--fg);cursor:pointer}
button:disabled{opacity:.6;cursor:progress}
</style></head>
<body><main><h1>%s</h1><p>%s</p>
<button type="button" id="restart" data-computer-restart>Restart computer</button>
<p id="status" role="status" aria-live="polite"></p></main>
<script>
(function(){
  var reload = setTimeout(function(){ location.reload(); }, %d * 1000);
  var button = document.getElementById('restart');
  var status = document.getElementById('status');
  button.addEventListener('click', async function(){
    clearTimeout(reload);
    button.disabled = true;
    status.textContent = 'Restarting. This can take a minute.';
    try {
      var current = await (await fetch('/api/compute/status', {credentials: 'include'})).json();
      var id = current && current.current_computer && current.current_computer.computer_id;
      if (!id) throw new Error('no computer id');
      var res = await fetch('/api/computers/' + encodeURIComponent(id) + '/lifecycle/restart', {
        method: 'POST', credentials: 'include',
        headers: {'Content-Type': 'application/json'},
        body: JSON.stringify({idempotency_key: 'surface-restart-' + id + '-' + Date.now()})
      });
      if (!res.ok) throw new Error('restart returned ' + res.status);
      status.textContent = 'Restarted. Opening your computer.';
      location.reload();
    } catch (err) {
      status.textContent = 'Restart did not finish (' + err.message + '). You can try again.';
      button.disabled = false;
    }
  });
})();
</script></body></html>
`
