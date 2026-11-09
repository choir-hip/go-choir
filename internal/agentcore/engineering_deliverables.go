package agentcore

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/capsule"
	"github.com/yusefmosiah/go-choir/internal/ownerfiles"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// Assignment deliverables (docs/design/capsule-overlay-computer-state-2026-11-03.md:
// the capsule sees Files read-only, so a PDF or chart it makes would die with
// it). The assignment writes them to capsule.DeliverablesDir; when the
// assignment's capsule is frozen for its terminal report, the host stages the
// regular files there (bounded, type-sniffed, symlinks never followed) and
// files them in the owner's Files at Reports/<short assignment id>/<name>
// through ownerfiles, the one writer. The report's summary lists the Files
// paths and every refusal, so the caller (Texture or management) can embed
// them. The summary is derivation, never identity input, so the report's
// proposition digest is unchanged. A retry (crash mid-copy, report re-drive)
// replaces the same names by rename, so it is idempotent.

// deliverablesReportsDir is the Files directory all assignment deliverables
// file under.
const deliverablesReportsDir = "Reports"

var shortAssignmentIDPattern = regexp.MustCompile(`[^A-Za-z0-9]`)

// shortAssignmentID is the assignment's short directory name: the first eight
// characters of its digest ("assignment-<hex>").
func shortAssignmentID(assignmentID string) string {
	id := strings.TrimPrefix(assignmentID, "assignment-")
	id = strings.TrimPrefix(id, "sha256:")
	id = shortAssignmentIDPattern.ReplaceAllString(id, "")
	if id == "" {
		return "unknown"
	}
	return id[:min(len(id), 8)]
}

// landedDeliverable is one file filed in Files.
type landedDeliverable struct {
	capsule.DeliverableFile
	Path string // Files path, slash separated, relative to the files root
}

// landDeliverables copies the staged deliverables from stageDir into the
// owner's Files under Reports/<short id>/. A file that fails to copy is
// returned as a refusal with the error. It never fails the caller: the report
// states what landed and what did not.
func landDeliverables(filesRoot, assignmentID, stageDir string, accepted []capsule.DeliverableFile) ([]landedDeliverable, []capsule.DeliverableRefusal) {
	if len(accepted) == 0 {
		return nil, nil
	}
	var landed []landedDeliverable
	var refused []capsule.DeliverableRefusal
	if strings.TrimSpace(filesRoot) == "" {
		for _, f := range accepted {
			refused = append(refused, capsule.DeliverableRefusal{Name: f.Name, Reason: "no file tree on this computer"})
		}
		return nil, refused
	}
	rel := deliverablesReportsDir + "/" + shortAssignmentID(assignmentID)
	dir, err := mkdirBeneath(filesRoot, rel)
	if err != nil {
		for _, f := range accepted {
			refused = append(refused, capsule.DeliverableRefusal{Name: f.Name, Reason: "Files directory unavailable: " + err.Error()})
		}
		return nil, refused
	}
	for _, f := range accepted {
		if _, err := capsule.SanitizeDeliverableName(f.Name); err != nil || f.Name != filepath.Base(f.Name) {
			refused = append(refused, capsule.DeliverableRefusal{Name: f.Name, Reason: "not a plain file name"})
			continue
		}
		src := filepath.Join(stageDir, f.Name)
		if err := copyDeliverable(src, filepath.Join(dir, f.Name)); err != nil {
			refused = append(refused, capsule.DeliverableRefusal{Name: f.Name, Reason: "copy into Files failed: " + err.Error()})
			continue
		}
		landed = append(landed, landedDeliverable{DeliverableFile: f, Path: rel + "/" + f.Name})
	}
	return landed, refused
}

// copyDeliverable streams src into dst through ownerfiles (temporary file,
// fsync, rename), replacing any earlier copy.
func copyDeliverable(src, dst string) error {
	info, err := os.Lstat(src)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("staged file is not a regular file")
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	return ownerfiles.WriteFrom(dst, 0o644, func(w io.Writer) error {
		_, err := io.Copy(w, in)
		return err
	})
}

// deliverablesSummary is the text appended to the report's summary: the Files
// paths that landed and each refusal with its reason. Empty when there was
// nothing to say.
func deliverablesSummary(landed []landedDeliverable, refused []capsule.DeliverableRefusal) string {
	if len(landed) == 0 && len(refused) == 0 {
		return ""
	}
	var b strings.Builder
	if len(landed) > 0 {
		b.WriteString("Deliverables in the owner's Files (embed or link these paths):\n")
		for _, f := range landed {
			fmt.Fprintf(&b, "- %s (%s, %d bytes)\n", f.Path, f.MediaType, f.Bytes)
		}
	}
	if len(refused) > 0 {
		b.WriteString("Deliverables not delivered:\n")
		for _, r := range refused {
			fmt.Fprintf(&b, "- %s: %s\n", r.Name, r.Reason)
		}
	}
	return strings.TrimRight(b.String(), "\n")
}

// withDeliverablesSummary returns summary with the deliverables text after it.
func withDeliverablesSummary(summary, deliverables string) string {
	if deliverables == "" {
		return summary
	}
	if strings.Contains(summary, deliverables) {
		return summary // a re-drive of the same report
	}
	return strings.TrimSpace(summary) + "\n\n" + deliverables
}

// landAssignmentDeliverables stages and files the frozen capsule's
// deliverables and returns report with their paths and refusals in its
// summary. A staging failure is reported in the summary, never fails the
// report: the assignment's work is already done and the report is the record.
func (rt *Runtime) landAssignmentDeliverables(ctx context.Context, assignment types.EngineeringAssignment, runID, handle string, report types.EngineeringAssignmentReport) types.EngineeringAssignmentReport {
	if rt == nil || rt.capsuleExecutor == nil || strings.TrimSpace(handle) == "" {
		return report
	}
	stageRoot := rt.selfdevUpdaterRoot
	if strings.TrimSpace(stageRoot) == "" {
		stageRoot = filepath.Join(filepath.Dir(rt.filesRoot()), ".stage")
	}
	staged, err := rt.capsuleExecutor.StageGrantedDeliverables(ctx, runID, handle, stageRoot)
	if err != nil {
		log.Printf("agentcore: assignment %s deliverables staging failed: %v", assignment.AssignmentID, err)
		report.Summary = withDeliverablesSummary(report.Summary, "Deliverables not delivered:\n- staging the output directory failed: "+err.Error())
		return report
	}
	defer staged.Remove()
	accepted, refused, stageDir := staged.Files, staged.Refused, staged.Dir
	landed, failed := landDeliverables(rt.filesRoot(), assignment.AssignmentID, stageDir, accepted)
	refused = append(refused, failed...)
	report.Summary = withDeliverablesSummary(report.Summary, deliverablesSummary(landed, refused))
	return report
}
