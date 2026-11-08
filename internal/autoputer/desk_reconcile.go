package autoputer

import (
	"context"
	"log"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/agentcore"
	"github.com/yusefmosiah/go-choir/internal/appdesk"
	"github.com/yusefmosiah/go-choir/internal/store"
)

// deskLog adapts the store's desk package event log to appdesk.Log.
type deskLog struct {
	st    *store.Store
	owner string
}

func (l deskLog) Events(ctx context.Context) ([]appdesk.Event, error) {
	rows, err := l.st.ListDeskPackageEvents(ctx, l.owner)
	if err != nil {
		return nil, err
	}
	out := make([]appdesk.Event, 0, len(rows))
	for _, r := range rows {
		ev, err := appdesk.DecodeEvent(r.Seq, r.Body)
		if err != nil {
			return nil, err
		}
		out = append(out, ev)
	}
	return out, nil
}

func (l deskLog) Append(ctx context.Context, ev appdesk.Event) (uint64, error) {
	body, err := appdesk.EncodeEvent(ev)
	if err != nil {
		return 0, err
	}
	return l.st.AppendDeskPackageEvent(ctx, l.owner, body)
}

// deskReconcileOption is the boot-time reconcile of the package desk registry
// (plan MA4.2 and MA4.4): the desired state is the fold of the log's events
// union the image's required set; a required package a restored head lacks is
// re-installed by a new event after that head; the registry is made to match.
// It runs in the runtime's boot before any obligation is closed, so the crash
// boot's one pass is keyed on the desks this head has.
func deskReconcileOption(st *store.Store, ownerID string) (agentcore.RuntimeOption, error) {
	if strings.TrimSpace(ownerID) == "" {
		ownerID = "local" // a computer booted without an owner id (tests, bring-up) keeps one log
	}
	required, err := appdesk.ImageRequired()
	if err != nil {
		return nil, err
	}
	return agentcore.WithDeskReconciler(func(ctx context.Context) error {
		set, refusals, err := appdesk.Boot(ctx, deskLog{st: st, owner: ownerID}, required)
		for _, r := range refusals {
			log.Printf("autoputer: desk reconcile: event %d %s: %s: %s", r.Seq, r.Package, r.Code, r.Detail)
		}
		log.Printf("autoputer: desk reconcile: %d package desks, %d required by the image", len(set.Desks), len(required))
		return err
	}), nil
}

// deskLogOption binds the same desk-package log to the runtime, so a package
// install appends the desks it contributes (agentcore desk_install.go) with the
// writer the boot reconcile uses.
func deskLogOption(st *store.Store, ownerID string) agentcore.RuntimeOption {
	if strings.TrimSpace(ownerID) == "" {
		ownerID = "local"
	}
	return agentcore.WithDeskLog(deskLog{st: st, owner: ownerID}, ownerID)
}
