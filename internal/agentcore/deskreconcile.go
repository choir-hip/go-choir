package agentcore

import (
	"context"
	"fmt"
	"log"
	"sort"
	"strings"
	"sync"
	"time"

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
	// The installs' desks join the log first (desk_install.go), so a computer
	// that installed a desk package registers it again after a restart.
	if rt.deskLog != nil {
		if err := rt.syncInstalledDesks(ctx, rt.deskLogOwner, rt.cfg.ComputerID); err == nil {
			return
		} else {
			log.Printf("runtime: boot desk sync from installs: %v", err)
		}
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
	// Last, so the runs and rounds above are already closed: an open Texture
	// work item with no run left is served by nothing.
	rt.closeUnservedTextureWorkAfterCrash(ctx)
}

// Bounds of the crash boot's Texture work closure: a computer with thousands
// of open items closes the oldest bootUnservedTextureCloseCap within
// bootUnservedTextureBudget and logs the rest, which the next crash boot (or
// the item's own next Wait) closes.
var (
	bootUnservedTextureCloseCap = 200
	bootUnservedTextureBudget   = 60 * time.Second
)

// SetUnservedTextureCloser binds the Texture owner's close of one open work
// item nothing serves (textureowner closeUnservedAtBoot).
func (rt *Runtime) SetUnservedTextureCloser(fn func(context.Context, types.WorkItemRecord) error) {
	rt.deliveryHooksMu.Lock()
	defer rt.deliveryHooksMu.Unlock()
	rt.unservedTextureCloser = fn
}

// closeUnservedTextureWorkAfterCrash is the crash boot's last leg: the boot
// cancelled every run, so an open Texture work item with no live run and no
// round, assignment or operation serving it will never be served. Each is
// refused with the fate "interrupted by a restart" and management is told.
// Nothing is resumed, re-dispatched or armed, and only the markerless boot
// reaches here (Start). A planned restart keeps its runs for the one resume.
func (rt *Runtime) closeUnservedTextureWorkAfterCrash(ctx context.Context) {
	rt.deliveryHooksMu.RLock()
	closer := rt.unservedTextureCloser
	rt.deliveryHooksMu.RUnlock()
	computerID := strings.TrimSpace(rt.TextureComputerID())
	if closer == nil || rt.store == nil || computerID == "" {
		return
	}
	started := time.Now()
	ctx, cancel := context.WithTimeout(ctx, bootUnservedTextureBudget)
	defer cancel()
	items, err := rt.store.ListOpenAssignedLifecycleWorkItems(ctx, computerID, 0)
	if err != nil {
		log.Printf("runtime: crash boot Texture work closure: %v", err)
		return
	}
	prefix := deskmanifest.DocumentCustodianName() + ":"
	closed, skipped, remaining := 0, 0, 0
	for _, item := range items {
		if strings.TrimSpace(item.AuthorityProfile) != deskmanifest.DocumentCustodianName() || !strings.HasPrefix(strings.TrimSpace(item.AssignedAgentID), prefix) {
			continue
		}
		if closed >= bootUnservedTextureCloseCap || ctx.Err() != nil {
			remaining++
			continue
		}
		// A run started since the boot closed the old ones serves its work.
		if _, live, err := rt.activeRunByAgent(ctx, item.OwnerID, item.AssignedAgentID); err != nil || live {
			skipped++
			continue
		}
		if err := closer(ctx, item); err != nil {
			log.Printf("runtime: crash boot Texture work closure %s: %v", item.WorkItemID, err)
			continue
		}
		closed++
	}
	log.Printf("runtime: crash boot Texture work closure: examined %d open items, asked to close %d, skipped %d with a live run, %d beyond the bound left for the next boot, dur=%s", len(items), closed, skipped, remaining, time.Since(started).Round(time.Millisecond))
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
