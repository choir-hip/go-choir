package autoputer

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"time"

	"github.com/yusefmosiah/go-choir/internal/modelpolicy"
	"github.com/yusefmosiah/go-choir/internal/ownerfiles"
)

// policyRevisionName is the shape keepPolicyRevision writes.
var policyRevisionName = regexp.MustCompile(`^\d{8}T\d{6}\.\d{6}Z(-\d+)?\.toml$`)

const (
	// PolicyFilePath is the computer's model policy, relative to the files root.
	PolicyFilePath = "System/model-policy.toml"
	// PolicyRevisionsDir holds the files a policy write replaced, named by the
	// UTC time of the replacement, so a rollback is a write of one of them.
	PolicyRevisionsDir = "System/model-policy.revisions"
	// policyRevisionsKept bounds the directory; the oldest go first.
	policyRevisionsKept = 20
)

// isPolicyFile reports whether absPath is the model policy file.
func (fh *FilesHandler) isPolicyFile(absPath string) bool {
	return absPath == filepath.Join(fh.rootDir, filepath.FromSlash(PolicyFilePath))
}

// keepPolicyRevision copies the current policy file into PolicyRevisionsDir
// before a write replaces it, then drops the oldest beyond the bound. It
// returns the revision's name, or "" when there was nothing to keep (no
// current file, or the write changes nothing). The loader falls back
// silently on a bad file and the server kept no earlier one, so this is the
// only way back to a policy that was working.
func (fh *FilesHandler) keepPolicyRevision(absPath string, next []byte) (string, error) {
	current, err := os.ReadFile(absPath)
	if os.IsNotExist(err) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	if string(current) == string(next) {
		return "", nil
	}
	dir := filepath.Join(fh.rootDir, filepath.FromSlash(PolicyRevisionsDir))
	// A symlinked revisions directory would put revisions, and the prune
	// below, outside the files root.
	if info, err := os.Lstat(dir); err == nil && (info.Mode()&os.ModeSymlink != 0 || !info.IsDir()) {
		return "", fmt.Errorf("%s is not a directory", PolicyRevisionsDir)
	}
	if err := ownerfiles.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	stamp := time.Now().UTC().Format("20060102T150405.000000Z")
	name := stamp + ".toml"
	// Two writes in one microsecond must not clobber each other.
	for i := 1; ; i++ {
		_, err := os.Lstat(filepath.Join(dir, name))
		if os.IsNotExist(err) {
			break
		}
		if err != nil || i > 100 {
			return "", fmt.Errorf("no free revision name in %s", PolicyRevisionsDir)
		}
		name = fmt.Sprintf("%s-%d.toml", stamp, i)
	}
	if err := ownerfiles.WriteFile(filepath.Join(dir, name), current, 0o644); err != nil {
		return "", err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return name, nil // the revision is kept; pruning waits for the next write
	}
	var names []string
	for _, e := range entries {
		// Only names this function wrote; an owner's own file here is left alone.
		if !e.IsDir() && policyRevisionName.MatchString(e.Name()) {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	for len(names) > policyRevisionsKept {
		_ = ownerfiles.Remove(filepath.Join(dir, names[0]))
		names = names[1:]
	}
	return name, nil
}

// checkPolicyWrite is the gate on a write to the policy file: the body must
// pass the policy loader's own validation. It returns the message to refuse
// with, or "".
func checkPolicyWrite(body []byte) string {
	if err := modelpolicy.ValidateText(string(body)); err != nil {
		return "model policy refused, not saved: " + err.Error()
	}
	return ""
}
