package vmmanager

import (
	"log"
	"strings"
	"sync"
)

// Release pins (docs/design/ring-ordered-deploys-2026-11-03.md). vmctl's
// ring walk holds every computer outside the open ring, and every computer
// after a rollback, on the last release that finished a walk. The manager asks
// the pinner at every cold boot (bindCurrentRelease) and in every release rule
// (currentReleaseFor), so a pinned computer is not "behind" and its snapshots
// are judged against the release it will launch. A pinned release that does
// not load falls back to the current one: a pin never fails a boot.

// ReleasePinner answers which release pair a computer launches from now; ""
// means the current release. Pinned lists every release a pin can name, so
// retention keeps them. Pin must not call back into the manager.
type ReleasePinner interface {
	Pin(computerID string) (single, layered string)
	Pinned() []string
}

type releasePinnerBox struct{ p ReleasePinner }

// SetReleasePinner installs the pinner; nil removes it. Call it before the
// first boot.
func (m *Manager) SetReleasePinner(p ReleasePinner) {
	if p == nil {
		m.releasePinner.Store(nil)
		return
	}
	m.releasePinner.Store(&releasePinnerBox{p: p})
}

func (m *Manager) pinner() ReleasePinner {
	if b := m.releasePinner.Load(); b != nil {
		return b.p
	}
	return nil
}

var releasePinFallbackLogged sync.Map

// pinnedRelease is the release the pinner holds computerID on, when it loads.
func (m *Manager) pinnedRelease(computerID string) (guestRelease, bool) {
	computerID = strings.TrimSpace(computerID)
	p := m.pinner()
	if p == nil || computerID == "" || !m.releaseRetainOn() {
		return guestRelease{}, false
	}
	single, layered := p.Pin(computerID)
	var tries []string
	if layered != "" && m.layeredSelected(computerID) {
		tries = append(tries, layered)
	}
	if single != "" {
		tries = append(tries, single)
	}
	for _, id := range tries {
		if rel, err := m.releaseByID(id); err == nil {
			return rel, true
		} else if _, seen := releasePinFallbackLogged.LoadOrStore(computerID+"/"+id, true); !seen {
			log.Printf("vmmanager: computer %s is pinned to release %s, which does not load (%v); trying the next choice", computerID, id, err)
		}
	}
	return guestRelease{}, false
}

// pinnedReleaseIDs are the releases pins can name now.
func (m *Manager) pinnedReleaseIDs() []string {
	if p := m.pinner(); p != nil {
		return p.Pinned()
	}
	return nil
}

// LayeredGuestReleaseInfo is the layered link's release and its commit, ""
// when there is none or it does not load.
func (m *Manager) LayeredGuestReleaseInfo() (id, commit string) {
	if rel, ok := m.layeredCurrentRelease(); ok {
		return rel.ID, rel.Policy.BuildCommit
	}
	return "", ""
}

// GuestReleaseInfoFor is the release a cold boot of computerID launches from
// now (pin and layered ring included) and its commit.
func (m *Manager) GuestReleaseInfoFor(computerID string) (id, commit string) {
	if rel, ok := m.currentReleaseFor(computerID); ok {
		return rel.ID, rel.Policy.BuildCommit
	}
	return "", ""
}

// GuestLayeredComputer reports whether computerID is in the layered ring.
func (m *Manager) GuestLayeredComputer(computerID string) bool {
	return m.layeredSelected(computerID)
}
