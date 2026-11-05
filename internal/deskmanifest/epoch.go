package deskmanifest

import (
	"fmt"
	"sort"
)

// Desk epochs (modular architecture plan MA4.2, section 6 B2).
//
// A package desk has an epoch: the sequence of the install or update event
// that produced its current row. A built-in desk, and an app desk registered
// without an event, is at epoch 0. An update that narrows a verb set must not
// cut a run already in flight, so the daemon's frame check resolves the row
// retained for the run's own epoch (FrameAllowedAt), and a row is retained
// until the last activation holding it ends (RetainEpoch's release).
//
// DeskEpoch is not the broker epoch. The broker epoch (BrokerRequest.Epoch,
// the worker's session epoch) is the handle issuer's constant and counts
// worker generations; the desk epoch is the install sequence.

type heldRow struct {
	desk Desk
	refs int
}

var (
	// current is the epoch of each registered app desk's live row; absent is 0.
	current = map[string]uint64{}
	// held are the rows an activation still uses, by desk and epoch. A row
	// appears when something retains it and leaves when its last holder
	// releases and it is no longer the live row.
	held = map[string]map[uint64]*heldRow{}
)

// CurrentEpoch is the epoch of the desk's live row, 0 for a built-in desk or
// a name that is not registered.
func CurrentEpoch(name string) uint64 {
	mu.RLock()
	defer mu.RUnlock()
	return current[name]
}

// RegisterAppAt registers an app desk at the epoch of the event that
// installed it (>0). It applies the whole app discipline (RegisterApp).
func RegisterAppAt(d Desk, epoch uint64) error {
	if epoch == 0 {
		return fmt.Errorf("app desk %s: an install epoch is the event sequence, not 0", d.Name)
	}
	if err := RegisterApp(d); err != nil {
		return err
	}
	mu.Lock()
	current[d.Name] = epoch
	mu.Unlock()
	return nil
}

// UpdateApp replaces a live app desk's row with the row of a later event. The
// old row is retained for the activations that already hold it and dropped
// when the last one ends. A row that is not newer, or that does not pass the
// app discipline, is refused and nothing changes.
func UpdateApp(d Desk, epoch uint64) error {
	canonical, err := NewAppDesk(d.AppSpec())
	if err != nil {
		return err
	}
	if !d.Equal(canonical) {
		return fmt.Errorf("app desk %s: the row carries authority an app desk does not have", d.Name)
	}
	mu.Lock()
	defer mu.Unlock()
	old, ok := registry[d.Name]
	if !ok || old.Kind != KindApp {
		return fmt.Errorf("app desk %s is not registered", d.Name)
	}
	if epoch <= current[d.Name] {
		return fmt.Errorf("app desk %s: update epoch %d is not after the live epoch %d", d.Name, epoch, current[d.Name])
	}
	prev := current[d.Name]
	if h := held[d.Name][prev]; h != nil {
		h.desk = old.clone() // the row the holders resolve against
	}
	registry[d.Name] = canonical.clone()
	current[d.Name] = epoch
	dropIfUnheld(d.Name, prev)
	return nil
}

// dropIfUnheld removes a retained row nobody holds that is not the live one.
// The caller holds mu.
func dropIfUnheld(name string, epoch uint64) {
	h := held[name][epoch]
	if h == nil || h.refs > 0 || current[name] == epoch {
		return
	}
	delete(held[name], epoch)
	if len(held[name]) == 0 {
		delete(held, name)
	}
}

// RetainEpoch holds the row of the desk at epoch for an activation and
// returns the function that releases the hold. It fails when that row is
// neither the live row nor still held: an activation cannot adopt an epoch
// that was already dropped. A built-in desk (epoch 0) is always the live row.
func RetainEpoch(name string, epoch uint64) (release func(), ok bool) {
	mu.Lock()
	defer mu.Unlock()
	live, registered := registry[name]
	h := held[name][epoch]
	if h == nil {
		if !registered || current[name] != epoch {
			return nil, false
		}
		if held[name] == nil {
			held[name] = map[uint64]*heldRow{}
		}
		h = &heldRow{desk: live.clone()}
		held[name][epoch] = h
	}
	h.refs++
	once := false
	return func() {
		mu.Lock()
		defer mu.Unlock()
		if once {
			return
		}
		once = true
		h.refs--
		if h.refs <= 0 && held[name][epoch] == h {
			delete(held[name], epoch)
			if len(held[name]) == 0 {
				delete(held, name)
			}
		}
	}, true
}

// LookupAt returns the row of the desk at epoch: the live row when epoch is
// current, otherwise the retained row. Unknown is deny.
func LookupAt(name string, epoch uint64) (Desk, bool) {
	mu.RLock()
	defer mu.RUnlock()
	if _, ok := registry[name]; ok && current[name] == epoch {
		return registry[name].clone(), true
	}
	if h := held[name][epoch]; h != nil {
		return h.desk.clone(), true
	}
	return Desk{}, false
}

// FrameAllowedAt is the daemon's question for a worker started at a desk
// epoch: may it be served action under the row of that epoch? A row the
// update narrowed still serves the activations that started under it; a
// retired or unknown row serves nothing.
func FrameAllowedAt(name string, epoch uint64, action string) bool {
	d, ok := LookupAt(name, epoch)
	return ok && d.AllowsFrame(action)
}

// EpochState is the registry's retention state, for the leak differential
// and the health listing.
type EpochState struct {
	Registered int
	Retained   int
	Holders    int
}

// EpochStateNow counts the registry's rows, the rows held beside the live
// ones and the holds on them.
func EpochStateNow() EpochState {
	mu.RLock()
	defer mu.RUnlock()
	s := EpochState{Registered: len(registry)}
	for name, byEpoch := range held {
		for epoch, h := range byEpoch {
			s.Holders += h.refs
			if current[name] != epoch || registry[name].Name == "" {
				s.Retained++
			}
		}
	}
	return s
}

// AppEpochs lists the live app desks with their epochs, sorted by name.
func AppEpochs() []string {
	mu.RLock()
	defer mu.RUnlock()
	var out []string
	for n, d := range registry {
		if d.Kind == KindApp {
			out = append(out, fmt.Sprintf("%s@%d", n, current[n]))
		}
	}
	sort.Strings(out)
	return out
}
