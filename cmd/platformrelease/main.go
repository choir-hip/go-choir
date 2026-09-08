package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/platformrelease"
)

func resolveCommit(flagCommit string) (string, error) {
	if strings.TrimSpace(flagCommit) != "" {
		return strings.TrimSpace(flagCommit), nil
	}
	if envCommit := strings.TrimSpace(os.Getenv("DEPLOY_COMMIT")); envCommit != "" {
		return envCommit, nil
	}
	cmd := exec.Command("git", "rev-parse", "HEAD")
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("resolve commit from git: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

func main() {
	commitFlag := flag.String("commit", "", "commit SHA for platform release")
	outputFlag := flag.String("output", "", "output file path for platform-release.json")
	printFlag := flag.Bool("print", false, "print generated platform release JSON to stdout")
	flag.Parse()

	commit, err := resolveCommit(*commitFlag)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "platformrelease: %v\n", err)
		os.Exit(1)
	}

	pr := platformrelease.GenerateBaselineRelease(commit, time.Now().UTC())

	if *outputFlag != "" {
		if err := platformrelease.Save(pr, *outputFlag); err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "platformrelease: save to %s: %v\n", *outputFlag, err)
			os.Exit(1)
		}
		fmt.Printf("Wrote platform release %s to %s (digest: %s)\n", pr.ReleaseID, *outputFlag, pr.ContentDigest)
	}

	if *printFlag || *outputFlag == "" {
		fmt.Printf("Release ID:       %s\n", pr.ReleaseID)
		fmt.Printf("Platform Base Ref:%s\n", pr.PlatformBaseRef)
		fmt.Printf("Content Digest:   %s\n", pr.ContentDigest)
		fmt.Printf("Components:       %d\n", len(pr.Components))
	}
}
