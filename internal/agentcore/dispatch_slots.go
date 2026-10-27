package agentcore

import (
	"context"
	"log"
	"os"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/store"
)

// dispatchSlotsWide is the shared dispatcher cap with dispatch.wide_slots
// on. Off asks the dispatcher for its own default (8).
const dispatchSlotsWide = 12

// SetSharedSlotsActor binds the dispatcher's live cap and applies the
// computer's dispatch.wide_slots setting to it at once, so a boot starts at
// the owner's cap. The adapter calls it after the dispatcher exists.
func (rt *Runtime) SetSharedSlotsActor(fn func(int)) {
	if rt == nil {
		return
	}
	rt.deliveryHooksMu.Lock()
	rt.setSharedSlots = fn
	rt.deliveryHooksMu.Unlock()
	ownerID := strings.TrimSpace(os.Getenv("CHOIR_OWNER_ID"))
	if rt.computerSettingOn(context.Background(), ownerID, rt.cfg.ComputerID, store.SettingDispatchWideSlots) {
		rt.applyDispatchSlotsSetting(true)
	}
}

// applyDispatchSlotsSetting is the owner's setting change at runtime: on
// raises the shared slots to 12, off lowers them to 8. Running activations
// are never interrupted.
func (rt *Runtime) applyDispatchSlotsSetting(value bool) {
	if rt == nil {
		return
	}
	rt.deliveryHooksMu.Lock()
	fn := rt.setSharedSlots
	rt.deliveryHooksMu.Unlock()
	if fn == nil {
		return
	}
	n := 0 // the dispatcher's default
	if value {
		n = dispatchSlotsWide
	}
	fn(n)
	log.Printf("dispatcher: shared slots set to %d, 0 is the default (%s)", n, store.SettingDispatchWideSlots)
}
