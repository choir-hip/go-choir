package vmmanager

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/visualcapture"
)

// Visual capture, host side (docs/design/visual-evidence-slice-2026-10-15.md).
// Chromium is in every guest's store but on no PATH; the right to launch it
// in a capsule is a grant vmctl writes onto the computer's root-only
// credential disk at cold boot. Off while the list is empty (nix/node-b.nix,
// go-choir-vmctl Environment):
//
//	CHOIR_VISUAL_CAPTURE_COMPUTERS  ComputerIDs and/or class:test, comma or
//	                                space separated
//
// class:test grants every computer vmctl boots with warmness class test (a
// test account's computer). A snapshot resume keeps the old credential disk,
// so a changed list reaches a computer at its next cold boot.
const visualCaptureComputersEnv = "CHOIR_VISUAL_CAPTURE_COMPUTERS"

// VisualCaptureConfig is the host switch.
type VisualCaptureConfig struct {
	Computers map[string]bool
	ClassTest bool
}

func loadVisualCaptureConfig(getenv func(string) string) VisualCaptureConfig {
	cfg := VisualCaptureConfig{Computers: map[string]bool{}}
	for _, entry := range strings.FieldsFunc(getenv(visualCaptureComputersEnv), func(r rune) bool { return r == ',' || r == ' ' || r == '\n' || r == '\t' }) {
		if entry == visualcapture.ClassTestToken {
			cfg.ClassTest = true
			continue
		}
		cfg.Computers[entry] = true
	}
	return cfg
}

// visualCaptureMaterial is the grant a computer's credential disk carries,
// or nothing for a computer the switch does not name. warmnessClass is the
// class vmctl booted the computer under ("test" for a test account's).
func (c VisualCaptureConfig) visualCaptureMaterial(computerID, warmnessClass string, now time.Time) (map[string][]byte, string) {
	computerID = strings.TrimSpace(computerID)
	if computerID == "" {
		return nil, ""
	}
	basis := ""
	switch {
	case c.Computers[computerID]:
		basis = visualcapture.BasisComputer
	case c.ClassTest && strings.TrimSpace(warmnessClass) == "test":
		basis = visualcapture.BasisClassTest
	default:
		return nil, ""
	}
	raw, _ := json.Marshal(visualcapture.Grant{ComputerID: computerID, Status: visualcapture.GrantGranted, Basis: basis, GrantedAt: now.UTC().Format(time.RFC3339)})
	return map[string][]byte{visualcapture.GrantFile: append(raw, '\n')}, "granted (" + basis + ")"
}
