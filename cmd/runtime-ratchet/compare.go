package main

import (
	"fmt"
	"os"
	"reflect"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

var itemDispositions = map[string]bool{
	"delete": true, "core": true,
	"api": true, "browser": true, "candidate_package": true, "content": true,
	"desktop": true, "evidence": true, "lifecycle": true, "media": true,
	"model": true, "podcast": true, "promotion": true, "research": true,
	"texture": true, "tools": true, "wire": true,
}

var citerDispositions = map[string]bool{
	"delete": true,
	"redirect_to_successor": true,
	"deletion_target_reference": true,
	"historical_evidence": true,
	"block": true,
}

func readInventory(path string) (Inventory, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Inventory{}, fmt.Errorf("read baseline: %w", err)
	}
	var inv Inventory
	dec := yaml.NewDecoder(strings.NewReader(string(data)))
	dec.KnownFields(true)
	if err := dec.Decode(&inv); err != nil {
		return Inventory{}, fmt.Errorf("decode baseline: %w", err)
	}
	return inv, nil
}

func compareInventory(want, got Inventory) error {
	var problems []string
	if want.Schema != inventorySchema {
		problems = append(problems, fmt.Sprintf("schema = %q, want %q", want.Schema, inventorySchema))
	}
	if want.CanonicalParent != got.CanonicalParent {
		problems = append(problems, fmt.Sprintf("canonical_parent = %q, want %q", want.CanonicalParent, got.CanonicalParent))
	}
	if want.DispatchNonce != got.DispatchNonce {
		problems = append(problems, "dispatch_nonce does not match the canonical dispatch")
	}
	if want.Transition != got.Transition {
		problems = append(problems, "transition does not match the canonical dispatch")
	}

	categories := []struct {
		name  string
		want  []Entry
		got   []Entry
		citer bool
	}{
		{"files", want.Files, got.Files, false},
		{"exports", want.Exports, got.Exports, false},
		{"routes", want.Routes, got.Routes, false},
		{"tools", want.Tools, got.Tools, false},
		{"production_importers", want.ProductionImporters, got.ProductionImporters, false},
		{"wrappers", want.Wrappers, got.Wrappers, false},
		{"compatibility_markers", want.CompatibilityMarkers, got.CompatibilityMarkers, false},
		{"state_writers", want.StateWriters, got.StateWriters, false},
		{"citers", want.Citers, got.Citers, true},
	}
	for _, category := range categories {
		problems = append(problems, validateEntries(category.name, category.want, category.citer)...)
		problems = append(problems, compareEntries(category.name, category.want, category.got)...)
	}
	problems = append(problems, validateEntries("initial_unused_export_debt", want.UnusedExportDebt, false)...)
	problems = append(problems, validateExportCallerContract(want.Exports, want.UnusedExportDebt)...)

	declared := want
	setCounts(&declared)
	if !reflect.DeepEqual(declared.Counts, want.Counts) {
		problems = append(problems, fmt.Sprintf("baseline counts are stale: declared %+v, entries compute %+v", want.Counts, declared.Counts))
	}
	if !reflect.DeepEqual(want.Counts, got.Counts) {
		problems = append(problems, fmt.Sprintf("count drift: baseline %+v, current %+v", want.Counts, got.Counts))
	}
	if len(problems) > 0 {
		sort.Strings(problems)
		return fmt.Errorf("inventory regression:\n  - %s", strings.Join(problems, "\n  - "))
	}
	return nil
}

func validateEntries(category string, entries []Entry, citer bool) []string {
	var problems []string
	seen := map[string]bool{}
	allowed := itemDispositions
	if citer {
		allowed = citerDispositions
	}
	for _, entry := range entries {
		if strings.TrimSpace(entry.ID) == "" {
			problems = append(problems, category+": empty item id")
			continue
		}
		if seen[entry.ID] {
			problems = append(problems, fmt.Sprintf("%s: duplicate item %q", category, entry.ID))
		}
		seen[entry.ID] = true
		if strings.TrimSpace(entry.Disposition) == "" {
			problems = append(problems, fmt.Sprintf("%s: undispositioned item %q", category, entry.ID))
		} else if !allowed[entry.Disposition] {
			problems = append(problems, fmt.Sprintf("%s: invalid disposition %q for %q", category, entry.Disposition, entry.ID))
		}
		if citer && entry.Disposition == "deletion_target_reference" &&
			!strings.Contains(entry.ID, "runtime-package-extinction-target") {
			problems = append(problems, fmt.Sprintf(
				"%s: deletion_target_reference for %q does not name runtime-package-extinction-target",
				category, entry.ID,
			))
		}
	}
	return problems
}

func compareEntries(category string, want, got []Entry) []string {
	wantByID := make(map[string]Entry, len(want))
	gotByID := make(map[string]Entry, len(got))
	for _, entry := range want {
		wantByID[entry.ID] = entry
	}
	for _, entry := range got {
		gotByID[entry.ID] = entry
	}
	var problems []string
	for id, current := range gotByID {
		baseline, ok := wantByID[id]
		if !ok {
			detail := fmt.Sprintf("%s: added item %q is not in the baseline", category, id)
			if category == "exports" && !strings.Contains(id, "_test.go:") {
				detail += "; new production exports also require a production caller"
			}
			problems = append(problems, detail)
			continue
		}
		if category == "files" && baseline.LOC != current.LOC {
			problems = append(problems, fmt.Sprintf("files: LOC drift for %q: baseline %d, current %d", id, baseline.LOC, current.LOC))
		}
		if category == "exports" && !reflect.DeepEqual(baseline.ProductionCallers, current.ProductionCallers) {
			problems = append(problems, fmt.Sprintf(
				"exports: production caller drift for %q: baseline %v, current %v",
				id, baseline.ProductionCallers, current.ProductionCallers,
			))
		}
	}
	for id := range wantByID {
		if _, ok := gotByID[id]; !ok {
			problems = append(problems, fmt.Sprintf("%s: baseline item %q is missing from current inventory; update its explicit disposition/removal atomically", category, id))
		}
	}
	return problems
}
func validateExportCallerContract(exports, debt []Entry) []string {
	exportByID := make(map[string]Entry, len(exports))
	for _, entry := range exports {
		exportByID[entry.ID] = entry
	}
	debtByID := make(map[string]bool, len(debt))
	var problems []string
	for _, entry := range debt {
		if entry.Disposition != "delete" {
			problems = append(problems, fmt.Sprintf(
				"initial_unused_export_debt: %q must use delete disposition, got %q",
				entry.ID, entry.Disposition,
			))
		}
		debtByID[entry.ID] = true
		export, ok := exportByID[entry.ID]
		if !ok {
			problems = append(problems, fmt.Sprintf(
				"initial_unused_export_debt: %q does not name a current export",
				entry.ID,
			))
			continue
		}
		if len(export.ProductionCallers) > 0 {
			problems = append(problems, fmt.Sprintf(
				"initial_unused_export_debt: %q now has production callers and must leave debt",
				entry.ID,
			))
		}
	}
	for _, entry := range exports {
		if strings.Contains(entry.ID, "_test.go:") || len(entry.ProductionCallers) > 0 || debtByID[entry.ID] {
			continue
		}
		problems = append(problems, fmt.Sprintf(
			"exports: %q has no AST-derived non-test production caller and is not canonical initial debt",
			entry.ID,
		))
	}
	return problems
}

