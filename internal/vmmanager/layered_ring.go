package vmmanager

import (
	"os"
	"path/filepath"
	"strings"
)

// The layered guest image ring (docs/design/layered-guest-image-2026-10-31.md
// S2; docs/design/progressive-delivery-2026-10-26.md rings). The fleet
// launches the single image the guest link names. A computer named here
// launches the layered image the second link names:
//
//	VM_GUEST_LAYERED_COMPUTERS     ComputerIDs, comma or space separated
//	VM_GUEST_LAYERED_RELEASE_LINK  symlink to the layered image's store path
//
// ComputerIDs only: no class: token and no "*", so a computer is in the ring
// by name and nothing widens by accident. The selection is read at a cold
// boot. A snapshot keeps its own release (releaseByID), so a hibernated
// computer wakes onto the layout it hibernated on; a layout change reaches it
// at its next cold boot, because the floor digests of the two layouts differ
// and the wake refuses (release_floor_moved).
//
// A ring computer whose layered release or base cannot be loaded launches the
// single image (bindCurrentRelease logs it): the layered image is an
// optimization, never a reason a boot fails. A computer outside the ring never
// resolves the layered link.

// parseLayeredComputers keeps the plain ComputerID tokens of the selector.
func parseLayeredComputers(raw string) []string {
	var out []string
	for _, tok := range parseComputerSelector(raw) {
		if strings.Contains(tok, ":") || strings.Contains(tok, "*") {
			continue
		}
		out = append(out, tok)
	}
	return out
}

// layeredSelected reports whether computerID is in the layered ring.
func (m *Manager) layeredSelected(computerID string) bool {
	computerID = strings.TrimSpace(computerID)
	if computerID == "" || strings.TrimSpace(m.cfg.GuestLayeredLink) == "" {
		return false
	}
	for _, c := range m.cfg.GuestLayeredComputers {
		if c == computerID {
			return true
		}
	}
	return false
}

// layeredImageTarget is the target of the layered link now.
func (m *Manager) layeredImageTarget() (string, bool) {
	link := strings.TrimSpace(m.cfg.GuestLayeredLink)
	if link == "" {
		return "", false
	}
	target, err := os.Readlink(link)
	if err != nil || target == "" {
		return "", false
	}
	return target, true
}

// layeredCurrentRelease resolves the layered link to a loadable layered
// release (its files and its base present, installed.json naming the target).
func (m *Manager) layeredCurrentRelease() (guestRelease, bool) {
	if !m.releaseRetainOn() {
		return guestRelease{}, false
	}
	target, ok := m.layeredImageTarget()
	if !ok {
		return guestRelease{}, false
	}
	base := filepath.Base(target)
	if len(base) < 32 {
		return guestRelease{}, false
	}
	rel, err := m.releaseByID(base[:32])
	if err != nil || !rel.Layered || rel.StorePath != target {
		return guestRelease{}, false
	}
	return rel, true
}

// currentReleaseFor is the release a cold boot of the computer launches from
// now: the layered current for a ring computer whose layered release loads,
// otherwise the single current.
func (m *Manager) currentReleaseFor(computerID string) (guestRelease, bool) {
	if m.layeredSelected(computerID) {
		if rel, ok := m.layeredCurrentRelease(); ok {
			return rel, true
		}
	}
	return m.currentRelease()
}
