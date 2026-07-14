package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
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
	"delete":                    true,
	"redirect_to_successor":     true,
	"deletion_target_reference": true,
	"historical_evidence":       true,
	"block":                     true,
}

var storeMethodSemantics = map[string]string{
	"ActiveCoSuperSlotRun":                   "read",
	"AppendChannelMessage":                   "lifecycle",
	"AppendEvent":                            "lifecycle",
	"AppendRunMemoryEntry":                   "lifecycle",
	"CancelAgentMutation":                    "lifecycle",
	"CancelTrajectoryAuthority":              "lifecycle",
	"ClaimCoSuperSlot":                       "lifecycle",
	"CoSuperSlotByAgent":                     "read",
	"CoSuperSlotByAgentAndTrajectory":        "read",
	"CoSuperSlotRun":                         "read",
	"CompleteAgentMutation":                  "lifecycle",
	"CountActiveCoSuperSlots":                "read",
	"CountPendingWorkerUpdatesByTrajectory":  "read",
	"CountRevisionsByDoc":                    "read",
	"CreateAgentMutation":                    "lifecycle",
	"CreateBrowserSession":                   "lifecycle",
	"CreateContentItem":                      "wire",
	"CreateDocument":                         "wire",
	"CreateEvidence":                         "lifecycle",
	"CreateRevision":                         "wire",
	"CreateRevisionWithSourceGraph":          "wire",
	"CreateRun":                              "lifecycle",
	"CreateTextureDecision":                  "wire",
	"CreateTrajectoryIfAbsent":               "wire",
	"CreateWorkItem":                         "wire",
	"CurrentVersionNumberByDoc":              "read",
	"DeferAgentMutation":                     "lifecycle",
	"DeleteDocument":                         "wire",
	"DispatchWorkerUpdate":                   "lifecycle",
	"FailAgentMutation":                      "lifecycle",
	"FindWorkItemByFingerprint":              "read",
	"GetAgent":                               "read",
	"GetAgentMutationByRun":                  "read",
	"GetAppAdoption":                         "read",
	"GetAppChangePackage":                    "read",
	"GetAppChangePackageForViewer":           "read",
	"GetBlame":                               "read",
	"GetBrowserSession":                      "read",
	"GetCandidatePackageIntake":              "read",
	"GetComputerSourceLineage":               "read",
	"GetContentItem":                         "read",
	"GetDesktopStateForDesktop":              "read",
	"GetDesktopStateForSession":              "read",
	"GetDiff":                                "read",
	"GetDocument":                            "read",
	"GetDocumentAlias":                       "read",
	"GetDocumentAliasSourcePath":             "read",
	"GetEvidence":                            "read",
	"GetHistory":                             "read",
	"GetLatestActiveRunByAgent":              "read",
	"GetLatestPassivatedRunByAgent":          "read",
	"GetMediaProgress":                       "read",
	"GetPendingAgentMutationByDoc":           "read",
	"GetRevision":                            "read",
	"GetRevisionUnscoped":                    "read",
	"GetRun":                                 "read",
	"GetRunAcceptance":                       "read",
	"GetRunAcceptanceByID":                   "read",
	"GetRunMemoryEntry":                      "read",
	"GetTextureControllerCheckpoint":         "read",
	"GetTrajectory":                          "read",
	"GetUserPreference":                      "read",
	"GetWorkerUpdate":                        "read",
	"LatestActorRunMemoryEntries":            "read",
	"ListActiveRunsByTrajectory":             "read",
	"ListAllDocuments":                       "read",
	"ListAppAdoptions":                       "read",
	"ListAppChangePackages":                  "read",
	"ListBrowserSessions":                    "read",
	"ListCandidatePackageIntakes":            "read",
	"ListChannelMessages":                    "read",
	"ListCoagentMailboxBacklog":              "read",
	"ListCoagentMailboxBacklogAll":           "read",
	"ListContentItems":                       "read",
	"ListDocumentsByOwner":                   "read",
	"ListEvents":                             "read",
	"ListEventsByOwner":                      "read",
	"ListEventsByOwnerAfter":                 "read",
	"ListEventsByTrajectory":                 "read",
	"ListEvidenceByAgent":                    "read",
	"ListMediaRecents":                       "read",
	"ListOpenAssignedWorkItems":              "read",
	"ListOpenWorkItemsByKind":                "read",
	"ListPodcastSubscriptions":               "read",
	"ListRevisionsByDoc":                     "read",
	"ListRunAcceptances":                     "read",
	"ListRunAcceptancesByTrajectory":         "read",
	"ListRunMemoryEntries":                   "read",
	"ListRunsByChannel":                      "read",
	"ListRunsByIngestionHandoff":             "read",
	"ListRunsByOwner":                        "read",
	"ListRunsByState":                        "read",
	"ListTextureDecisionsByDocument":         "read",
	"ListTextureSourceEntitiesForRevision":   "read",
	"ListTextureSourceGraphForRevisions":     "read",
	"ListTextureSourceRefsForRevision":       "read",
	"ListTrajectoriesByOwner":                "read",
	"ListWorkItemsByTrajectory":              "read",
	"ListWorkerUpdatesByTrajectory":          "read",
	"MarkAgentMutationStale":                 "lifecycle",
	"MarkWorkerUpdatesDelivered":             "lifecycle",
	"PatchRevisionMetadata":                  "wire",
	"Path":                                   "read",
	"ReactivateAgentMutation":                "lifecycle",
	"RecordAgentMutationRevision":            "wire",
	"ReleaseCoSuperSlotClaim":                "lifecycle",
	"SaveDesktopStateForDesktop":             "lifecycle",
	"SaveDesktopStateForSession":             "lifecycle",
	"SaveUserPreference":                     "lifecycle",
	"SearchPublishedDocuments":               "read",
	"SleepAgentMutation":                     "lifecycle",
	"TexturePath":                            "read",
	"UpdateAppAdoptionIfCurrent":             "promotion",
	"UpdateBrowserSession":                   "lifecycle",
	"UpdateCandidatePackageIntakeIfCurrent":  "promotion",
	"UpdateDocument":                         "wire",
	"UpdateRun":                              "lifecycle",
	"UpdateRunAndMarkWorkerUpdatesDelivered": "lifecycle",
	"UpdateTrajectoryStatus":                 "wire",
	"UpdateTrajectorySubjectRefs":            "wire",
	"UpdateWorkItemDetails":                  "wire",
	"UpdateWorkItemStatus":                   "wire",
	"UpsertAgent":                            "lifecycle",
	"UpsertAppAdoption":                      "promotion",
	"UpsertAppChangePackage":                 "promotion",
	"UpsertCandidatePackageIntake":           "promotion",
	"UpsertComputerSourceLineage":            "promotion",
	"UpsertDocumentAlias":                    "wire",
	"UpsertMediaProgress":                    "lifecycle",
	"UpsertMediaRecent":                      "lifecycle",
	"UpsertPodcastSubscription":              "lifecycle",
	"UpsertRunAcceptance":                    "lifecycle",
	"UpsertTextureControllerCheckpoint":      "wire",
}

func readInventory(path string) (Inventory, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Inventory{}, fmt.Errorf("read baseline: %w", err)
	}
	return decodeInventory(data)
}

func decodeInventory(data []byte) (Inventory, error) {
	var inv Inventory
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(&inv); err != nil {
		return Inventory{}, fmt.Errorf("decode baseline: %w", err)
	}
	return inv, nil
}

func priorCanonicalInventory(root, baselinePath string, writing bool) (Inventory, bool, error) {
	relative, err := filepath.Rel(root, baselinePath)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return Inventory{}, false, fmt.Errorf("baseline %s is outside repository root", baselinePath)
	}
	relative = filepath.ToSlash(relative)
	show := func(ref string) ([]byte, bool) {
		command := exec.Command("git", "-C", root, "show", ref+":"+relative)
		output, showErr := command.Output()
		return output, showErr == nil
	}
	head, headExists := show("HEAD")
	if !headExists {
		return Inventory{}, false, nil
	}
	priorData := head
	if !writing {
		current, readErr := os.ReadFile(baselinePath)
		if readErr != nil {
			return Inventory{}, false, fmt.Errorf("read current baseline: %w", readErr)
		}
		if bytes.Equal(current, head) {
			parent, parentExists := show("HEAD^")
			if !parentExists {
				return Inventory{}, false, nil
			}
			priorData = parent
		}
	}
	prior, decodeErr := decodeInventory(priorData)
	if decodeErr != nil {
		return Inventory{}, false, fmt.Errorf("decode prior canonical baseline: %w", decodeErr)
	}
	return prior, true, nil
}

func validateDebtNoGrowth(prior, current Inventory) error {
	allowed := make(map[string]bool, len(prior.UnusedExportDebt))
	for _, entry := range prior.UnusedExportDebt {
		allowed[entry.ID] = true
	}
	var additions []string
	for _, entry := range current.UnusedExportDebt {
		if !allowed[entry.ID] {
			additions = append(additions, entry.ID)
		}
	}
	if len(additions) == 0 {
		return nil
	}
	sort.Strings(additions)
	return fmt.Errorf(
		"initial unused export debt grew beyond prior canonical Git authority: %s",
		strings.Join(additions, ", "),
	)
}

func compareInventory(want, got Inventory) error {
	applyInterfaceCandidateAuthority(&got, want)
	setCounts(&got)
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
		{"citers", want.Citers, got.Citers, true},
	}
	for _, category := range categories {
		problems = append(problems, validateEntries(category.name, category.want, category.citer)...)
		problems = append(problems, compareEntries(category.name, category.want, category.got)...)
	}
	problems = append(problems, validateInterfaceCandidates(want.InterfaceCandidates)...)
	problems = append(problems, validateInterfaceCandidateAuthority(want)...)
	problems = append(problems, compareEntries("interface_candidates", want.InterfaceCandidates, got.InterfaceCandidates)...)
	problems = append(problems, validateStoreCalls(want.StoreCalls)...)
	problems = append(problems, compareEntries("store_calls", want.StoreCalls, got.StoreCalls)...)
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

func validateInterfaceCandidates(entries []Entry) []string {
	seen := map[string]bool{}
	var problems []string
	for _, entry := range entries {
		if strings.TrimSpace(entry.ID) == "" {
			problems = append(problems, "interface_candidates: empty item id")
			continue
		}
		if seen[entry.ID] {
			problems = append(problems, fmt.Sprintf("interface_candidates: duplicate item %q", entry.ID))
		}
		seen[entry.ID] = true
		method := storeCallMethod(entry.ID)
		expected := storeMethodSemantics[method]
		if expected == "" || entry.Disposition != expected {
			problems = append(problems, fmt.Sprintf(
				"interface_candidates: %s must use authoritative %q disposition, got %q for %q",
				method, expected, entry.Disposition, entry.ID,
			))
		}
	}
	return problems
}

func validateInterfaceCandidateAuthority(inventory Inventory) []string {
	storeCalls := map[string]Entry{}
	for _, entry := range inventory.StoreCalls {
		storeCalls[entry.ID] = entry
	}
	var problems []string
	for _, candidate := range inventory.InterfaceCandidates {
		storeCall, exists := storeCalls[candidate.ID]
		if !exists {
			problems = append(problems, fmt.Sprintf(
				"interface_candidates: candidate %q is missing from potential store_calls authority",
				candidate.ID,
			))
		} else if storeCall.Disposition != candidate.Disposition {
			problems = append(problems, fmt.Sprintf(
				"interface_candidates: candidate %q disposition %q does not match store_calls disposition %q",
				candidate.ID, candidate.Disposition, storeCall.Disposition,
			))
		}
	}
	return problems
}

func validateStoreCalls(entries []Entry) []string {
	allowed := map[string]bool{
		"read": true, "lifecycle": true, "wire": true, "promotion": true,
	}
	knownWriters := storeMethodSemantics
	seen := map[string]bool{}
	var problems []string
	for _, entry := range entries {
		if strings.TrimSpace(entry.ID) == "" {
			problems = append(problems, "store_calls: empty item id")
			continue
		}
		if seen[entry.ID] {
			problems = append(problems, fmt.Sprintf("store_calls: duplicate item %q", entry.ID))
		}
		seen[entry.ID] = true
		if !allowed[entry.Disposition] {
			problems = append(problems, fmt.Sprintf(
				"store_calls: missing or invalid disposition %q for %q; want read, lifecycle, wire, or promotion",
				entry.Disposition, entry.ID,
			))
		}
		method := entry.ID[strings.LastIndex(entry.ID, ".")+1:]
		if ordinal := strings.Index(method, "#"); ordinal >= 0 {
			method = method[:ordinal]
		}
		expected := knownWriters[method]
		if expected == "" {
			problems = append(problems, fmt.Sprintf(
				"store_calls: %s has no authoritative semantic disposition", method,
			))
		} else if entry.Disposition != expected {
			problems = append(problems, fmt.Sprintf(
				"store_calls: %s must use %s disposition, got %q",
				method, expected, entry.Disposition,
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
