package visualcapture

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
)

// The launch grant (docs/design/visual-evidence-slice-2026-10-15.md §1):
// vmctl writes it onto a granted computer's root-only credential disk at
// cold boot (internal/vmmanager/visual_capture.go), and the guest's capsule
// executor reads it before giving a capsule broker the Chromium path.
const (
	// GrantFile is the grant's name on the credential disk.
	GrantFile = "visual-capture.json"
	// GrantGranted is the only status that admits capture.
	GrantGranted = "granted"
	// ClassTestToken in the host allowlist grants every test computer.
	ClassTestToken = "class:test"
	// BasisComputer and BasisClassTest say why a computer was granted.
	BasisComputer  = "computer"
	BasisClassTest = ClassTestToken
)

// Grant is the grant record.
type Grant struct {
	ComputerID string `json:"computer_id"`
	Status     string `json:"status"`
	Basis      string `json:"basis"`
	GrantedAt  string `json:"granted_at"`
}

// Status is visual capture's availability on this computer, for the
// executor, the prompts and /api/runtime/observability.
type Status struct {
	Granted    bool   `json:"granted"`
	Reason     string `json:"reason"`
	ComputerID string `json:"computer_id,omitempty"`
	Basis      string `json:"basis,omitempty"`
	Browser    string `json:"browser,omitempty"`
}

// ReadGrant reads the grant from the credential directory and checks the
// image names a Chromium binary. Reasons never quote file contents.
func ReadGrant(dir, browser string) Status {
	if strings.TrimSpace(dir) == "" {
		return Status{Reason: "no credential directory is configured"}
	}
	raw, err := os.ReadFile(filepath.Join(dir, GrantFile))
	if errors.Is(err, os.ErrNotExist) {
		return Status{Reason: "this computer is not granted visual capture by the host (no grant on the credential disk)"}
	}
	if err != nil {
		return Status{Reason: "the visual capture grant is unreadable"}
	}
	var g Grant
	if err := json.Unmarshal(raw, &g); err != nil {
		return Status{Reason: "the visual capture grant is malformed"}
	}
	st := Status{ComputerID: g.ComputerID, Basis: g.Basis}
	if g.Status != GrantGranted {
		st.Reason = "the host withheld visual capture"
		return st
	}
	browser = strings.TrimSpace(browser)
	if browser == "" {
		st.Reason = "the guest image names no Chromium binary"
		return st
	}
	if info, err := os.Stat(browser); err != nil || info.IsDir() || info.Mode()&0o111 == 0 {
		st.Reason = "the Chromium binary is missing from the guest image"
		return st
	}
	st.Granted, st.Browser = true, browser
	st.Reason = "granted (" + g.Basis + ")"
	return st
}
