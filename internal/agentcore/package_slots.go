package agentcore

import (
	"context"
	"encoding/json"
	"sort"
	"strings"
)

// InstalledSlot is one shell slot an installed package provides, as the
// computer's installs declare it (GET /api/plugins/state, field "slots"). The
// shell mounts a surface only from a release whose isolation is "shell" (the
// owner's own track, plan 12.1); that is the provider "active" here, a
// declaration a deployed proof can read without a browser. A surface that fails
// in the shell falls back to the in-tree package in the browser's kernel
// (shell-slots.ts), which the Settings recovery rows show.
type InstalledSlot struct {
	Slot      string `json:"slot"`
	Package   string `json:"package"`
	ReleaseID string `json:"release_id"`
	Isolation string `json:"isolation"`
	Priority  int    `json:"priority"`
	Available bool   `json:"available"`
	// Active is true when the shell realm mounts this provider in place of the
	// base package of the same name.
	Active bool `json:"active"`
}

// InstalledSlots lists the shell slots the owner's installed packages provide.
func (h *APIHandler) InstalledSlots(ctx context.Context, ownerID string) []InstalledSlot {
	out := []InstalledSlot{}
	if h == nil || h.rt == nil || h.rt.store == nil {
		return out
	}
	rt := h.rt
	records, err := rt.packageInstalls(ctx, ownerID, strings.TrimSpace(rt.cfg.ComputerID))
	if err != nil {
		return out
	}
	for name, item := range currentPackageInstalls(records) {
		var m struct {
			Provides []struct {
				Service  string `json:"service"`
				Priority int    `json:"priority"`
			} `json:"provides"`
		}
		if json.Unmarshal(item.Manifest, &m) != nil || len(m.Provides) == 0 {
			continue
		}
		isolation := packageIsolation(item.PublisherTrack, ownerID)
		available := item.ModuleSHA256 != "" && rt.installBytesPresent(*item)
		for _, p := range m.Provides {
			out = append(out, InstalledSlot{
				Slot: p.Service, Package: name, ReleaseID: item.ReleaseID, Isolation: isolation,
				Priority: p.Priority, Available: available, Active: available && isolation == isolationShell,
			})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Slot != out[j].Slot {
			return out[i].Slot < out[j].Slot
		}
		return out[i].Package < out[j].Package
	})
	return out
}
