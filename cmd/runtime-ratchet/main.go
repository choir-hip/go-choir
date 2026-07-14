package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func main() {
	root := flag.String("root", "", "repository root (defaults to the directory containing go.mod)")
	baseline := flag.String("baseline", "docs/runtime-dissolution-inventory.yaml", "inventory baseline, relative to root")
	writeBaseline := flag.Bool("write-baseline", false, "write a new baseline with explicit conservative dispositions")
	bootstrapDebt := flag.Bool("bootstrap-initial-debt", false, "allow initial debt only when no baseline exists in canonical Git history")
	flag.Parse()

	repo, err := repositoryRoot(*root)
	if err != nil {
		fatal(err)
	}
	inventory, err := scanRepository(repo)
	if err != nil {
		fatal(err)
	}
	baselinePath := *baseline
	if !filepath.IsAbs(baselinePath) {
		baselinePath = filepath.Join(repo, baselinePath)
	}
	if *writeBaseline {
		previous, readErr := readInventory(baselinePath)
		if readErr == nil {
			inventory.UnusedExportDebt = rebasePackageCutoverDebt(inventory.Exports, previous.UnusedExportDebt)
		}
		applyAuthoritativeInterfaceCandidateDispositions(&inventory)
		applyInterfaceCandidateAuthority(&inventory, inventory)
		applyAuthoritativeStoreDispositions(&inventory)
		if problems := validateInterfaceCandidates(inventory.InterfaceCandidates); len(problems) > 0 {
			fatal(fmt.Errorf("cannot write baseline:\n  - %s", strings.Join(problems, "\n  - ")))
		}
		if problems := validateStoreCalls(inventory.StoreCalls); len(problems) > 0 {
			fatal(fmt.Errorf("cannot write baseline:\n  - %s", strings.Join(problems, "\n  - ")))
		}
		setCounts(&inventory)
		if err := enforceDebtAuthority(repo, baselinePath, inventory, true, *bootstrapDebt); err != nil {
			fatal(err)
		}
		if err := writeInventory(baselinePath, inventory); err != nil {
			fatal(err)
		}
		fmt.Printf("wrote %s\n", filepath.ToSlash(*baseline))
		printCounts(inventory.Counts)
		return
	}
	want, err := readInventory(baselinePath)
	if err != nil {
		fatal(err)
	}
	applyInterfaceCandidateAuthority(&inventory, want)
	setCounts(&inventory)
	if err := enforceDebtAuthority(repo, baselinePath, want, false, *bootstrapDebt); err != nil {
		fatal(err)
	}
	if err := compareInventory(want, inventory); err != nil {
		fatal(err)
	}
	fmt.Println("runtime dissolution inventory: PASS")
	printCounts(inventory.Counts)
}
func rebasePackageCutoverDebt(exports, previous []Entry) []Entry {
	current := make(map[string]Entry, len(exports))
	for _, entry := range exports {
		current[entry.ID] = entry
	}
	rebased := make([]Entry, 0, len(previous))
	for _, entry := range previous {
		candidates := []string{entry.ID}
		if strings.HasPrefix(entry.ID, "internal/runtime/") {
			suffix := strings.TrimPrefix(entry.ID, "internal/runtime/")
			candidates = append(candidates,
				"internal/agentcore/"+suffix,
				strings.Replace("internal/textureowner/"+suffix, "method(*Runtime):", "method(*Handler):", 1),
			)
		}
		for _, candidate := range candidates {
			export, exists := current[candidate]
			if exists && len(export.ProductionCallers) == 0 {
				entry.ID = candidate
				rebased = append(rebased, entry)
				break
			}
		}
	}
	sort.Slice(rebased, func(i, j int) bool { return rebased[i].ID < rebased[j].ID })
	return rebased
}

func applyAuthoritativeInterfaceCandidateDispositions(current *Inventory) {
	for index := range current.InterfaceCandidates {
		entry := &current.InterfaceCandidates[index]
		entry.Disposition = storeMethodSemantics[storeCallMethod(entry.ID)]
	}
}

func applyInterfaceCandidateAuthority(current *Inventory, authority Inventory) {
	dispositions := map[string]string{}
	for _, entry := range authority.InterfaceCandidates {
		dispositions[entry.ID] = entry.Disposition
	}
	existing := map[string]bool{}
	for _, entry := range current.StoreCalls {
		existing[entry.ID] = true
	}
	for _, candidate := range current.InterfaceCandidates {
		if !existing[candidate.ID] {
			current.StoreCalls = append(current.StoreCalls, Entry{
				ID: candidate.ID, Disposition: dispositions[candidate.ID],
			})
			existing[candidate.ID] = true
		}
	}
	sort.Slice(current.StoreCalls, func(i, j int) bool {
		return current.StoreCalls[i].ID < current.StoreCalls[j].ID
	})
}

func applyAuthoritativeStoreDispositions(current *Inventory) {
	for index := range current.StoreCalls {
		entry := &current.StoreCalls[index]
		entry.Disposition = storeMethodSemantics[storeCallMethod(entry.ID)]
	}
}

func storeCallMethod(id string) string {
	method := id[strings.LastIndex(id, ".")+1:]
	if ordinal := strings.Index(method, "#"); ordinal >= 0 {
		method = method[:ordinal]
	}
	return method
}

func enforceDebtAuthority(root, baselinePath string, current Inventory, writing, bootstrap bool) error {
	prior, exists, err := priorCanonicalInventory(root, baselinePath, writing)
	if err != nil {
		return err
	}
	if !exists {
		if bootstrap {
			return nil
		}
		return fmt.Errorf(
			"no prior tracked runtime inventory baseline; initial debt requires -bootstrap-initial-debt",
		)
	}
	if bootstrap {
		return errors.New("-bootstrap-initial-debt is forbidden when prior canonical Git authority exists")
	}
	prior.UnusedExportDebt = rebasePackageCutoverDebt(current.Exports, prior.UnusedExportDebt)
	setCounts(&prior)
	return validateDebtNoGrowth(prior, current)
}

func repositoryRoot(explicit string) (string, error) {
	if explicit != "" {
		root, err := filepath.Abs(explicit)
		if err != nil {
			return "", err
		}
		if _, err := os.Stat(filepath.Join(root, "go.mod")); err != nil {
			return "", fmt.Errorf("repository root %s: %w", root, err)
		}
		return root, nil
	}
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", errors.New("could not find repository root containing go.mod")
		}
		dir = parent
	}
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "runtime-ratchet:", err)
	os.Exit(1)
}

func printCounts(c Counts) {
	fmt.Printf("counts: go_files=%d production_files=%d test_files=%d production_loc=%d test_loc=%d exports=%d export_caller_edges=%d initial_unused_export_debt=%d routes=%d tools=%d production_importers=%d wrappers=%d compatibility_markers=%d store_calls=%d interface_candidates=%d citers=%d\n",
		c.GoFiles, c.ProductionFiles, c.TestFiles, c.ProductionLOC, c.TestLOC, c.Exports, c.ExportCallerEdges, c.InitialUnusedExportDebt, c.Routes, c.Tools, c.ProductionImporters, c.Wrappers, c.CompatibilityMarkers, c.StoreCalls, c.InterfaceCandidates, c.Citers)
}
