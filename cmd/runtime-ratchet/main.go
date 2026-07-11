package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
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
		if readErr == nil && previous.Schema == inventorySchema {
			inventory.UnusedExportDebt = previous.UnusedExportDebt
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
	if err := enforceDebtAuthority(repo, baselinePath, want, false, *bootstrapDebt); err != nil {
		fatal(err)
	}
	if err := compareInventory(want, inventory); err != nil {
		fatal(err)
	}
	fmt.Println("runtime dissolution inventory: PASS")
	printCounts(inventory.Counts)
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
	fmt.Printf("counts: go_files=%d production_files=%d test_files=%d production_loc=%d test_loc=%d exports=%d export_caller_edges=%d initial_unused_export_debt=%d routes=%d tools=%d production_importers=%d wrappers=%d compatibility_markers=%d state_writers=%d declared_store_reads=%d citers=%d\n",
		c.GoFiles, c.ProductionFiles, c.TestFiles, c.ProductionLOC, c.TestLOC, c.Exports, c.ExportCallerEdges, c.InitialUnusedExportDebt, c.Routes, c.Tools, c.ProductionImporters, c.Wrappers, c.CompatibilityMarkers, c.StateWriters, c.DeclaredStoreReads, c.Citers)
}
