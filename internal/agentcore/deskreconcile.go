package agentcore

import (
	"context"
	"fmt"
	"log"
	"sort"
	"strings"
	"sync"

	"github.com/yusefmosiah/go-choir/internal/deskmanifest"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// WithDeskReconciler binds the boot-time reconcile of the desk registry to the
// tape (plan MA4.2 and MA4.4): it derives the installed desk set from the
// install events plus the image's required set, registers, updates and retires
// desks to match, and re-seeds a required package a restored head lacks.
func WithDeskReconciler(f func(ctx context.Context) error) RuntimeOption {
	return func(rt *Runtime) { rt.deskReconciler = f }
}

// reconcileDesksAtBoot runs the reconciler once, before any obligation is
// closed or resumed. It is idempotent, so a crash between an install event and
// its reconcile only means the next boot does it.
func (rt *Runtime) reconcileDesksAtBoot(ctx context.Context) {
	if rt.deskReconciler == nil {
		return
	}
	if err := rt.deskReconciler(ctx); err != nil {
		log.Printf("runtime: boot desk reconcile: %v", err)
	}
}

// bootClosureTally counts what the crash boot's one pass closed, by the desk
// each obligation belonged to. A desk the registry does not have (an app desk
// retired, or never installed at this head) is counted under unregistered: its
// work is closed like any other, never left open and never resumed.
type bootClosureTally struct {
	mu           sync.Mutex
	byDesk       map[string]int
	unregistered int
}

func (t *bootClosureTally) add(desk string) {
	if t == nil {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if _, ok := deskmanifest.Lookup(strings.TrimSpace(desk)); !ok {
		t.unregistered++
		return
	}
	if t.byDesk == nil {
		t.byDesk = map[string]int{}
	}
	t.byDesk[strings.TrimSpace(desk)]++
}

func (t *bootClosureTally) String() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	names := make([]string, 0, len(t.byDesk))
	for n := range t.byDesk {
		names = append(names, n)
	}
	sort.Strings(names)
	parts := make([]string, 0, len(names)+1)
	for _, n := range names {
		parts = append(parts, fmt.Sprintf("%s=%d", n, t.byDesk[n]))
	}
	parts = append(parts, fmt.Sprintf("unregistered=%d", t.unregistered))
	return strings.Join(parts, " ")
}

// deskOfInbox is the desk part of an inbox name (<desk>:<instance>).
func deskOfInbox(inbox string) string {
	desk, _, _ := strings.Cut(strings.TrimSpace(inbox), ":")
	// An app desk's inbox is app:<pkg>:<instance>.
	if desk == "app" {
		parts := strings.SplitN(strings.TrimSpace(inbox), ":", 3)
		if len(parts) >= 2 {
			return parts[0] + ":" + parts[1]
		}
	}
	return desk
}

// closeOpenObligationsAtBoot is the crash boot's one pass (CLAUDE.md "Restarts
// End Work"): every open run and every open round gets the fate "interrupted
// by a restart", none is re-dispatched, and the tally names them by the
// registry's desks. It is the only place a markerless boot closes work, so a
// desk added or retired by a package cannot be missed by a closer that
// enumerated the desks it knew.
func (rt *Runtime) closeOpenObligationsAtBoot(ctx context.Context) {
	rt.bootTally = &bootClosureTally{}
	defer func() {
		log.Printf("runtime: crash boot closed open obligations by desk: %s", rt.bootTally)
		rt.bootTally = nil
	}()
	rt.closePreBootRunsAfterCrash(ctx)
	rt.closeOpenRoundsAfterCrash(ctx)
	// The installs waiting on management's install review close with the
	// fate "interrupted by a restart" (install_review_pending.go).
	rt.closeInstallReviewPendingAfterCrash(ctx)
}

// deskOfRun is the desk a run belongs to for the boot tally: its role, or for a
// package desk the app:<pkg> head of its agent id.
func deskOfRun(rec *types.RunRecord) string {
	if rec == nil {
		return ""
	}
	if d := deskOfInbox(rec.AgentID); strings.HasPrefix(d, "app:") {
		return d
	}
	return agentRoleForRun(rec)
}
