package vmmanager

// Guest tool layers (docs/design/guest-tool-layers-2026-10-31.md). A layered
// release whose layers.json is schema 2 names tool slots: per name, a tool
// layer (the tool's store paths) and an empty placeholder with the same erofs
// label. Every slot is attached, after the base, in the release's order; the
// computer's profile picks, per slot, the layer or the placeholder. The guest
// mounts each slot by label, so its mount set never depends on the profile,
// and a tool whose layer is not attached is simply absent from its store.
//
//	VM_GUEST_TOOLS_DIR     where tool layers live (<id>/tool.erofs); default
//	                       guest-tools beside the releases directory
//	VM_GUEST_TOOL_PROFILE  layer=selector;layer=selector;...
//
// A selector is the standard computer selector (computer_select.go: ids,
// class:<warmness class>, "*"), plus "none". Every layer is attached for
// every computer (nix/node-b.nix guestToolProfile, owner 2026-11-08): an
// unread layer costs no page cache, and the Claude delegate's authority is the
// gateway relay's grant list, not the binary. The browser is not a grant: every computer
// gets it (docs/problems/selfdev-screens-are-not-files-2026-11-03.md). A layer the profile does not name is attached
// for every computer, so an unconfigured or new layer never takes a tool away.
// The choice is made at a cold boot and recorded in the launch config; a wake
// rebuilds it, so a changed profile moves the launch hash and the wake refuses.

import (
	"fmt"
	"log"
	"path/filepath"
	"regexp"
	"strings"
)

const (
	toolLayerFile    = "tool.erofs"
	toolSelectorNone = "none"
	maxToolLayers    = 8
)

var toolLayerNamePattern = regexp.MustCompile(`^[a-z][a-z0-9]{0,15}$`)

// releaseLayerTool is one tool slot in a release's layers.json.
type releaseLayerTool struct {
	Name    string `json:"name"`
	ID      string `json:"id"`
	EmptyID string `json:"empty_id"`
}

// releaseTool is a loaded tool slot: the layer and its placeholder files.
type releaseTool struct {
	Name      string
	ID        string
	EmptyID   string
	Disk      string
	EmptyDisk string
}

// ToolDisk is one attached tool slot of a launch: the slot name and the file
// behind it (the tool layer, or the slot's empty placeholder).
type ToolDisk struct {
	Name     string `json:"name"`
	Path     string `json:"path"`
	Attached bool   `json:"attached"`
}

// toolsDir is where tool layers live, one directory per layer id, written
// once and never rewritten.
func (m *Manager) toolsDir() string {
	if d := strings.TrimSpace(m.cfg.GuestToolsDir); d != "" {
		return d
	}
	return filepath.Join(filepath.Dir(m.releasesDir()), "guest-tools")
}

// loadReleaseTools checks a schema 2 layers.json's tool slots and resolves
// their files under the tools directory.
func (m *Manager) loadReleaseTools(relID string, tools []releaseLayerTool) ([]releaseTool, error) {
	if len(tools) == 0 || len(tools) > maxToolLayers {
		return nil, fmt.Errorf("release %s: layers.json names %d tool layers (want 1 to %d)", relID, len(tools), maxToolLayers)
	}
	seen := map[string]bool{}
	out := make([]releaseTool, 0, len(tools))
	for _, t := range tools {
		if !toolLayerNamePattern.MatchString(t.Name) || seen[t.Name] {
			return nil, fmt.Errorf("release %s: tool layer name %q is malformed or repeated", relID, t.Name)
		}
		if !releaseIDPattern.MatchString(t.ID) || !releaseIDPattern.MatchString(t.EmptyID) || t.ID == t.EmptyID {
			return nil, fmt.Errorf("release %s: tool layer %s has malformed ids", relID, t.Name)
		}
		seen[t.Name] = true
		out = append(out, releaseTool{
			Name:      t.Name,
			ID:        t.ID,
			EmptyID:   t.EmptyID,
			Disk:      filepath.Join(m.toolsDir(), t.ID, toolLayerFile),
			EmptyDisk: filepath.Join(m.toolsDir(), t.EmptyID, toolLayerFile),
		})
	}
	return out, nil
}

// parseToolProfile reads VM_GUEST_TOOL_PROFILE: layer=selector rules,
// separated by ';'. Malformed rules are dropped with a log line (the layer
// then falls back to attached, which never takes a tool away).
func parseToolProfile(raw string) map[string][]string {
	out := map[string][]string{}
	for _, rule := range strings.Split(raw, ";") {
		rule = strings.TrimSpace(rule)
		if rule == "" {
			continue
		}
		name, sel, ok := strings.Cut(rule, "=")
		name = strings.TrimSpace(name)
		if !ok || !toolLayerNamePattern.MatchString(name) {
			log.Printf("vmmanager: VM_GUEST_TOOL_PROFILE rule %q is malformed; ignored", rule)
			continue
		}
		toks := parseComputerSelector(sel)
		if toks == nil {
			toks = []string{}
		}
		out[name] = append(out[name], toks...)
	}
	return out
}

// toolAttached reports whether the profile gives cfg's computer the layer.
func (m *Manager) toolAttached(name string, cfg VMConfig) bool {
	rule, ok := m.cfg.GuestToolProfile[name]
	if !ok {
		return true
	}
	var plain []string
	for _, tok := range rule {
		switch tok {
		case toolSelectorNone:
		default:
			if !strings.HasPrefix(tok, "grant:") {
				plain = append(plain, tok)
			}
		}
	}
	return selectsComputer(plain, cfg)
}

// toolDisksFor is the tool slots a launch of cfg from rel attaches, in the
// release's order. nil for a release without tool slots.
func (m *Manager) toolDisksFor(cfg VMConfig, rel guestRelease) []ToolDisk {
	if len(rel.Tools) == 0 {
		return nil
	}
	out := make([]ToolDisk, 0, len(rel.Tools))
	for _, t := range rel.Tools {
		if m.toolAttached(t.Name, cfg) {
			out = append(out, ToolDisk{Name: t.Name, Path: t.Disk, Attached: true})
		} else {
			out = append(out, ToolDisk{Name: t.Name, Path: t.EmptyDisk})
		}
	}
	return out
}

// toolDisksMatchRelease reports whether disks are a valid choice for rel:
// one per slot, in order, each the slot's layer or its placeholder. A launch
// record that fails this is not trusted (reattach recomputes instead).
func toolDisksMatchRelease(disks []ToolDisk, rel guestRelease) bool {
	if len(disks) != len(rel.Tools) {
		return false
	}
	for i, t := range rel.Tools {
		d := disks[i]
		if d.Name != t.Name || (d.Attached && d.Path != t.Disk) || (!d.Attached && d.Path != t.EmptyDisk) {
			return false
		}
	}
	return true
}

// toolProfileSummary is the launch log's line: attached and empty slots.
func toolProfileSummary(disks []ToolDisk) string {
	var on, off []string
	for _, d := range disks {
		if d.Attached {
			on = append(on, d.Name)
		} else {
			off = append(off, d.Name)
		}
	}
	join := func(s []string) string {
		if len(s) == 0 {
			return "none"
		}
		return strings.Join(s, ",")
	}
	return join(on) + " attached; " + join(off) + " empty"
}

// logToolProfile records a layered launch's tool choice.
func logToolProfile(cfg VMConfig) {
	if len(cfg.ToolDisks) == 0 {
		return
	}
	log.Printf("vmmanager: computer %s (VM %s) tool layers: %s", cfg.ComputerID, cfg.VMID, toolProfileSummary(cfg.ToolDisks))
}

// toolDiskIDs lists the tool layer ids (real and empty) rel names, for
// retention.
func toolDiskIDs(rel guestRelease) []string {
	var ids []string
	for _, t := range rel.Tools {
		ids = append(ids, t.ID, t.EmptyID)
	}
	return ids
}
