package autoputer

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"time"

	"github.com/yusefmosiah/go-choir/internal/modelpolicy"
	"github.com/yusefmosiah/go-choir/internal/ownerfiles"
	"github.com/yusefmosiah/go-choir/internal/server"
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

// isPolicyFile reports whether absPath is the model policy file, or a symlink
// inside the root that resolves to it (a write through the link is a write of
// the policy and takes the same validated path).
func (fh *FilesHandler) isPolicyFile(absPath string) bool {
	policy := filepath.Join(fh.rootDir, filepath.FromSlash(PolicyFilePath))
	if absPath == policy {
		return true
	}
	info, err := os.Lstat(absPath)
	if err != nil || info.Mode()&os.ModeSymlink == 0 {
		return false
	}
	resolved, err := filepath.EvalSymlinks(absPath)
	if err != nil {
		return false
	}
	root, err := filepath.EvalSymlinks(fh.rootDir)
	if err != nil {
		return false
	}
	return resolved == filepath.Join(root, filepath.FromSlash(PolicyFilePath))
}

// PolicyAct is the operator receipt for one write of the model policy: who
// (an account id, never an email), what, which revision kept, and digests of
// the file before and after. Digests are SHA-256 hex of the file bytes; the
// old digest is empty when there was no file.
type PolicyAct struct {
	OperatorAccountID string
	Act               string
	Path              string
	Revision          string
	OldSHA256         string
	NewSHA256         string
	Bytes             int
	// ReceiptID is set by the recorder: the id of the tape event, which the
	// write's answer returns so the operator can find the receipt.
	ReceiptID string
}

// PolicyActRecorder writes a PolicyAct to the computer's tape. It returns an
// error when the record is not durable; the write it precedes is then refused.
type PolicyActRecorder func(r *http.Request, act *PolicyAct) error

// SetPolicyActRecorder sets the recorder for platform operator acts. With
// none set, an operator write is refused: an operator act with no receipt
// must not exist.
func (fh *FilesHandler) SetPolicyActRecorder(rec PolicyActRecorder) { fh.policyRecorder = rec }

// operatorFilesGuard admits a request that carries the platform operator
// marker only for the policy surface: reads of the policy file and its kept
// revisions, and the policy file's validated write. Anything else with the
// marker is refused (the proxy admits no other path, so this is the guest's
// own check). It returns false after writing the refusal.
func (fh *FilesHandler) operatorFilesGuard(w http.ResponseWriter, r *http.Request, absPath string) (operator string, ok bool) {
	id, present, err := server.PlatformOperator(r)
	if !present {
		return "", true
	}
	if err != nil {
		writeFileError(w, http.StatusForbidden, "access denied")
		return "", false
	}
	// Tell the proxy this build records operator acts. A guest from before
	// this change ignores the marker and says nothing, and the proxy then
	// writes nothing to it.
	if fh.policyRecorder != nil {
		w.Header().Set("X-Choir-Operator-Receipts", "1")
	}
	revisions := filepath.Join(fh.rootDir, filepath.FromSlash(PolicyRevisionsDir))
	switch r.Method {
	case http.MethodGet:
		if fh.isPolicyFile(absPath) || absPath == revisions || filepath.Dir(absPath) == revisions {
			return id, true
		}
	case http.MethodPut:
		if absPath == filepath.Join(fh.rootDir, filepath.FromSlash(PolicyFilePath)) {
			return id, true
		}
	}
	writeFileError(w, http.StatusForbidden, "access denied")
	return "", false
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

func sha256Hex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
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
