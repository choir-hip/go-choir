package agentcore

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/appdesk"
	"github.com/yusefmosiah/go-choir/internal/deskmanifest"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// A package desk wakes like any records desk (modular architecture, doctrine
// I26): a record addressed to app:<package>:<owner> mints the inbox wake row
// every desk gets (store/inbox.go), the actor handler calls WakeRecordsInbox,
// and that starts a run under the desk's own profile. Its verbs, grants, cell
// tool and import set come from its registered row, as for every desk.

// appInboxDesk is the registered, runnable package desk an inbox belongs to:
// an inbox of this owner, of a first-party desk (R24: another publisher's desk
// does not run while the sandbox is absent), with a prompt the image ships.
func (rt *Runtime) appInboxDesk(ownerID, inbox string) (deskmanifest.Desk, bool) {
	d, ok := deskmanifest.AppInboxDesk(inbox)
	if !ok || !d.FirstParty {
		return deskmanifest.Desk{}, false
	}
	if inbox != d.Name+":"+strings.TrimSpace(ownerID) {
		return deskmanifest.Desk{}, false
	}
	if _, ok := appDeskPrompt(d); !ok {
		return deskmanifest.Desk{}, false
	}
	return d, true
}

// appDeskPrompt is the first user turn of a package desk's run: the prompt the
// image ships for its first-party desk, never text a release supplies.
func appDeskPrompt(d deskmanifest.Desk) (string, bool) {
	return appdesk.FirstPartyPrompt(strings.TrimPrefix(d.Name, "app:"))
}

// ensureAppDeskAgent loads or creates the persistent agent of a package desk's
// owner instance (app:<package>:<owner>).
func (rt *Runtime) ensureAppDeskAgent(ctx context.Context, ownerID, computerID, agentID string, d deskmanifest.Desk) (types.AgentRecord, error) {
	agent, err := rt.store.GetAgentByScope(ctx, ownerID, computerID, agentID)
	if err == nil {
		return agent, nil
	}
	if !errors.Is(err, store.ErrNotFound) {
		return types.AgentRecord{}, err
	}
	now := time.Now().UTC()
	rec := types.AgentRecord{
		AgentID: agentID, OwnerID: ownerID, ComputerID: computerID,
		Profile: d.Name, Role: d.Name, ChannelID: agentID, CreatedAt: now, UpdatedAt: now,
	}
	if err := rt.store.UpsertAgent(ctx, rec); err != nil {
		return types.AgentRecord{}, fmt.Errorf("persist package desk agent %s: %w", agentID, err)
	}
	return rec, nil
}
