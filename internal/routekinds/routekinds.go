// Package routekinds is the route layer of custody (modular architecture plan
// MA3.10, section 7.3 layer 4): the table that says which app owns each HTTP
// route that serves an app's kinds, and the identity check on it.
//
// An app UI names itself on every call with the X-Choir-App header (an
// EventSource, which cannot set headers, uses the choir_app query parameter).
// The check follows the object-graph gates: a route serving built-in kinds
// counts a call from another app, or from none, and logs the first of each,
// but lets it through; a route serving package kinds refuses it. Until the
// Phase 6 iframe sandbox the identity is the app's own claim and same-origin
// code can lie about it, so the count is evidence and the refusal protects
// packages from each other's honest mistakes; Phase 6 makes it unforgeable.
//
// A request that already declares a desk reader (a ProductAPI call a desk
// makes in process) is not an app UI and passes unchecked: the object-graph
// reader gate decides it.
package routekinds

import (
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"

	"github.com/yusefmosiah/go-choir/internal/deskmanifest"
	"github.com/yusefmosiah/go-choir/internal/objectgraph"
)

const (
	// AppHeader carries the calling app's identity.
	AppHeader = "X-Choir-App"
	// AppQuery carries it where a header cannot be set (EventSource).
	AppQuery = "choir_app"

	// PackageRoutePrefix is the owner's route to package kinds; the segment
	// after it is the owning package.
	PackageRoutePrefix = "/api/app-packages/"

	packageKindNamespace = "choir.app."
)

// Route is one row of the table: the patterns a route registers under, the
// app that owns them, and the kinds they serve.
type Route struct {
	// Patterns are the mux patterns, as registered.
	Patterns []string
	// Owner is the app that owns the route. Empty with PackageSegment: the
	// package named by the path segment after the pattern.
	Owner string
	// Admit are other first-party surfaces that may call it.
	Admit []string
	// Kinds are the kinds the route serves, for the custody check below.
	Kinds []objectgraph.ObjectKind
	// Custodians are the writers of those kinds; a kind whose custodian is not
	// listed is a kind the route should not serve.
	Custodians []objectgraph.Writer
	// PackageSegment marks a route whose owner is the package in its path.
	PackageSegment bool
}

// Table is every app-facing route that serves kinds.
var Table = []Route{
	{
		Patterns: []string{"/api/texture/", "/api/texture/documents"},
		Owner:    deskmanifest.Texture,
		Kinds: []objectgraph.ObjectKind{
			"choir.texture_document", "choir.texture_revision", "choir.story_index", "choir.support_verdict",
			// The lifecycle and engineering-evidence projections (MA3.10).
			"choir.trajectory", "choir.lifecycle_event", "choir.lifecycle_command", "choir.lifecycle_cancel_intent", "choir.engineering_lineage",
		},
		Custodians: []objectgraph.Writer{objectgraph.WriterTexture, objectgraph.WriterKernel, objectgraph.WriterEngineering},
	},
	{
		Patterns:   []string{"/api/podcast/subscriptions", "/api/podcast/subscriptions/refresh", "/api/podcast/search"},
		Owner:      "podcast",
		Kinds:      []objectgraph.ObjectKind{"choir.podcast_subscription"},
		Custodians: []objectgraph.Writer{objectgraph.WriterShell},
	},
	{
		Patterns:   []string{"/api/autopaper/sources/schedules", "/api/autopaper/events"},
		Owner:      "autopaper",
		Kinds:      []objectgraph.ObjectKind{"choir.source_schedule", "choir.world_event"},
		Custodians: []objectgraph.Writer{objectgraph.WriterManagement},
	},
	{
		// The owner root (modular plan 8.4, row 6.2-9). The Texture UI reads
		// lifecycle through /api/texture/lifecycle/ and is not admitted.
		Patterns:   []string{"/api/trajectories", "/api/trajectories/"},
		Owner:      "shell",
		Admit:      []string{"compute-monitor", "pulse"},
		Kinds:      []objectgraph.ObjectKind{"choir.trajectory", "choir.lifecycle_event", "choir.lifecycle_command", "choir.lifecycle_cancel_intent"},
		Custodians: []objectgraph.Writer{objectgraph.WriterKernel},
	},
	{
		Patterns:       []string{PackageRoutePrefix},
		PackageSegment: true,
	},
}

var (
	mu     sync.Mutex
	counts = map[string]uint64{}
)

// Counts returns the calls seen from an app the route does not admit, keyed
// "pattern|calling app (or undeclared)|outcome", where the outcome is
// wrong_app or undeclared_app.
func Counts() map[string]uint64 {
	mu.Lock()
	defer mu.Unlock()
	out := make(map[string]uint64, len(counts))
	for k, v := range counts {
		out[k] = v
	}
	return out
}

// ResetCounts clears the counts (tests).
func ResetCounts() {
	mu.Lock()
	defer mu.Unlock()
	counts = map[string]uint64{}
}

// RowFor returns the row a pattern registers under.
func RowFor(pattern string) (Route, bool) {
	for _, row := range Table {
		for _, p := range row.Patterns {
			if p == pattern {
				return row, true
			}
		}
	}
	return Route{}, false
}

// Enforced reports whether a row refuses a call from the wrong app: it serves
// package kinds, as the object-graph gates enforce for package kinds only.
func (r Route) Enforced() bool {
	if r.PackageSegment {
		return true
	}
	for _, k := range r.Kinds {
		if strings.HasPrefix(string(k), packageKindNamespace) {
			return true
		}
	}
	return false
}

// Identity is the app a request names itself as, or "".
func Identity(r *http.Request) string {
	if v := strings.TrimSpace(r.Header.Get(AppHeader)); v != "" {
		return v
	}
	return strings.TrimSpace(r.URL.Query().Get(AppQuery))
}

// owner returns the app that owns the row for this request.
func (r Route) owner(pattern string, req *http.Request) string {
	if !r.PackageSegment {
		return r.Owner
	}
	rest := strings.TrimPrefix(req.URL.Path, pattern)
	if i := strings.IndexByte(rest, '/'); i >= 0 {
		rest = rest[:i]
	}
	return rest
}

func (r Route) admits(owner, who string) bool {
	if who == owner {
		return true
	}
	for _, a := range r.Admit {
		if a == who {
			return true
		}
	}
	return false
}

// Gate wraps the handler registered under pattern with its row's identity
// check. A pattern with no row is a programming error and panics at
// registration, so the table cannot drift from the routes.
func Gate(pattern string, next http.HandlerFunc) http.HandlerFunc {
	row, ok := RowFor(pattern)
	if !ok {
		panic(fmt.Sprintf("routekinds: no table row for route %q", pattern))
	}
	return func(w http.ResponseWriter, req *http.Request) {
		if _, desk := objectgraph.ReaderFrom(req.Context()); desk {
			next(w, req)
			return
		}
		owner := row.owner(pattern, req)
		who := Identity(req)
		if row.admits(owner, who) && who != "" {
			next(w, req)
			return
		}
		outcome := "wrong_app"
		if who == "" {
			outcome, who = "undeclared_app", "undeclared"
		}
		note(pattern, who, outcome, row.Enforced())
		if row.Enforced() {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte(`{"error":"route belongs to another app"}` + "\n"))
			return
		}
		next(w, req)
	}
}

func note(pattern, who, outcome string, enforced bool) {
	key := pattern + "|" + who + "|" + outcome
	mu.Lock()
	counts[key]++
	first := counts[key] == 1
	mu.Unlock()
	if first {
		mode := "observe"
		if enforced {
			mode = "enforce"
		}
		log.Printf("routekinds: app gate (%s): %s", mode, key)
	}
}
