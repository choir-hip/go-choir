package apppackage

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"time"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
)

// Selection records (slice L2; docs/design/modular-architecture-2026-11-04.md
// section 2.4, step 2 of the resolution order): which package the owner chose
// to provide each single-slot shell service. A selection is a fact on the
// owner's tape, kept as the user preference SelectionKey, so the working store
// folds it back at boot like every preference; the shell's kernel reads it at
// sign-in and ranks the chosen provider first. Choosing the slot's default
// package ("Use default") is a selection like any other: nothing is
// uninstalled. A selection naming a package that is not installed, or not
// healthy, is passed over by the ranking; it is never an error.

// SelectionKey is the preference the selections live under.
const SelectionKey = "packages.selection"

// Selection is one slot's record: the package chosen, the release in force
// when it was chosen (empty for a base-image package) and when.
type Selection struct {
	Package string `json:"package"`
	Release string `json:"release,omitempty"`
	At      string `json:"at"`
}

// Selections are the owner's records by slot.
type Selections map[string]Selection

var sha256Hex = regexp.MustCompile(`^[0-9a-f]{64}$`)

// maxSelections bounds a write: one record per shell service.
const maxSelections = 16

// ParseSelections reads a selections value strictly: every slot is a shell
// service, every package a package name, a release a sha256 when present and
// the time RFC 3339. An empty object clears every selection.
func ParseSelections(raw []byte) (Selections, error) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	var s Selections
	if err := dec.Decode(&s); err != nil || dec.More() {
		return nil, fmt.Errorf("selections are {\"<slot>\": {\"package\": ..., \"release\": ..., \"at\": ...}}")
	}
	if s == nil {
		return nil, fmt.Errorf("selections are an object")
	}
	if len(s) > maxSelections {
		return nil, fmt.Errorf("at most %d selections", maxSelections)
	}
	for slot, sel := range s {
		if !isShellService(slot) {
			return nil, fmt.Errorf("%q is not a shell service", slot)
		}
		if !objectgraph.ValidPackageName(sel.Package) {
			return nil, fmt.Errorf("%s: %q is not a package name", slot, sel.Package)
		}
		if sel.Release != "" && !sha256Hex.MatchString(sel.Release) {
			return nil, fmt.Errorf("%s: release must be a sha256", slot)
		}
		if _, err := time.Parse(time.RFC3339Nano, sel.At); err != nil {
			return nil, fmt.Errorf("%s: at must be an RFC 3339 time", slot)
		}
	}
	return s, nil
}
