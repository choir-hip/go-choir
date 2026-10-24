package main

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"
	"unicode/utf8"
)

// selfDevPatchInlineMax caps a patch returned inline (MCP, or the CLI
// without --out) so a large change does not flood a model's context; --out
// takes any size the route serves.
const selfDevPatchInlineMax = 256 << 10

type selfDevSourcePatch struct {
	OperationID           string `json:"operation_id"`
	ComputerID            string `json:"computer_id"`
	State                 string `json:"state"`
	BundleDigest          string `json:"bundle_digest"`
	SourcePatchSHA256     string `json:"source_patch_sha256"`
	SourcePatchBaseCommit string `json:"source_patch_base_commit"`
	PatchBytes            int    `json:"patch_bytes"`
	PatchBase64           string `json:"patch_base64"`
}

// selfDevPatch reads a frozen candidate's source patch, verifies its sha256
// client-side, and either writes it to --out (CLI) or returns it inline.
// Nothing is written unless the bytes verify.
func selfDevPatch(x *call) (any, error) {
	operationID := x.trim("operation")
	var resp selfDevSourcePatch
	if err := x.c.do(http.MethodGet, selfDevPath(x.str("computer"), "operations/"+url.PathEscape(operationID)+"/source-patch"), nil, &resp); err != nil {
		return nil, err
	}
	if resp.OperationID != operationID {
		return nil, fmt.Errorf("route answered operation %q, not %q", resp.OperationID, operationID)
	}
	patch, err := base64.StdEncoding.DecodeString(resp.PatchBase64)
	if err != nil {
		return nil, fmt.Errorf("decode source patch: %w", err)
	}
	sum := sha256.Sum256(patch)
	if got := hex.EncodeToString(sum[:]); got != strings.ToLower(resp.SourcePatchSHA256) || len(patch) != resp.PatchBytes {
		return nil, fmt.Errorf("source patch sha256 mismatch: route recorded %s (%d bytes), received bytes hash to %s (%d bytes)", resp.SourcePatchSHA256, resp.PatchBytes, got, len(patch))
	}
	out := map[string]any{
		"operation_id": resp.OperationID, "computer_id": resp.ComputerID, "state": resp.State,
		"bundle_digest": resp.BundleDigest, "source_patch_sha256": resp.SourcePatchSHA256,
		"source_patch_base_commit": resp.SourcePatchBaseCommit, "patch_bytes": len(patch), "sha256_verified": true,
	}
	if path := x.trim("out"); x.surface == "cli" && path != "" {
		if err := os.WriteFile(path, patch, 0o600); err != nil {
			return nil, err
		}
		out["out"] = path
		return out, nil
	}
	if len(patch) > selfDevPatchInlineMax {
		return nil, fmt.Errorf("source patch is %d bytes, over the inline cap of %d; use the CLI: choir self-dev patch --out <file>", len(patch), selfDevPatchInlineMax)
	}
	if !utf8.Valid(patch) {
		return nil, errors.New("source patch is not UTF-8 text and cannot be returned inline without changing its bytes; use the CLI: choir self-dev patch --out <file>")
	}
	out["patch"] = string(patch)
	return out, nil
}

// Operation states that never move again; wait stops at them.
var selfDevSettledStates = []string{"applied", "rejected", "rolled_back", "failed"}

type selfDevOperation struct {
	OperationID  string   `json:"operation_id"`
	State        string   `json:"state"`
	BundleDigest string   `json:"bundle_digest"`
	VerifierRefs []string `json:"verifier_refs"`
}

type selfDevHead struct {
	CanonicalEventHead       string `json:"canonical_event_head"`
	Sequence                 uint64 `json:"sequence"`
	DesiredEventHead         string `json:"desired_event_head"`
	EffectiveEventHead       string `json:"effective_event_head"`
	PendingTransitionRef     string `json:"pending_transition_ref"`
	DesiredStateCommitment   string `json:"desired_state_commitment"`
	EffectiveStateCommitment string `json:"effective_state_commitment"`
}

func selfDevPath(computerID, rest string) string {
	return "/api/computers/" + url.PathEscape(strings.TrimSpace(computerID)) + "/self-development/" + rest
}

// stringList is a repeatable flag that also splits commas.
type stringList []string

func (l *stringList) String() string { return strings.Join(*l, ",") }
func (l *stringList) Set(value string) error {
	for _, part := range strings.Split(value, ",") {
		if part = strings.TrimSpace(part); part != "" {
			*l = append(*l, part)
		}
	}
	return nil
}

// selfDevFlags parses a self-dev subcommand that targets one computer and
// refuses positional arguments.
func selfDevFlags(name string, args []string, stdout, stderr io.Writer, define func(fs *flag.FlagSet)) (*client, string, bool) {
	fs := flag.NewFlagSet("choir self-dev "+name, flag.ContinueOnError)
	fs.SetOutput(stderr)
	computerID := fs.String("computer", "", "Stable ComputerID")
	if define != nil {
		define(fs)
	}
	c, err := newClient(fs, args, stdout, stderr)
	if err != nil {
		fmt.Fprintf(stderr, "choir self-dev %s: %v\n", name, err)
		return nil, "", false
	}
	if strings.TrimSpace(*computerID) == "" || len(fs.Args()) != 0 {
		fmt.Fprintf(stderr, "choir self-dev %s: --computer is required and positional arguments are forbidden\n", name)
		return nil, "", false
	}
	return c, strings.TrimSpace(*computerID), true
}

func runSelfDevWait(args []string, stdout, stderr io.Writer) int {
	var operationID *string
	var states stringList
	var interval, deadline *time.Duration
	c, computerID, ok := selfDevFlags("wait", args, stdout, stderr, func(fs *flag.FlagSet) {
		operationID = fs.String("operation", "", "Operation ID")
		fs.Var(&states, "state", "Stop when the operation reaches this state (repeatable or comma-separated)")
		interval = fs.Duration("interval", 10*time.Second, "Poll interval")
		deadline = fs.Duration("deadline", 2*time.Hour, "Give up after this long")
	})
	if !ok || strings.TrimSpace(*operationID) == "" || len(states) == 0 {
		if ok {
			fmt.Fprintln(stderr, "choir self-dev wait: --operation and --state are required")
		}
		return 2
	}
	path := selfDevPath(computerID, "operations/"+url.PathEscape(strings.TrimSpace(*operationID)))
	giveUp := time.Now().Add(*deadline)
	last := ""
	for {
		var raw json.RawMessage
		var operation selfDevOperation
		err := c.do(http.MethodGet, path, nil, &raw)
		if err == nil {
			err = json.Unmarshal(raw, &operation)
		}
		if err != nil {
			fmt.Fprintf(stderr, "choir self-dev wait: %v\n", err)
		} else {
			if operation.State != last {
				fmt.Fprintf(stderr, "%s %s\n", time.Now().UTC().Format(time.RFC3339), operation.State)
				last = operation.State
			}
			if slices.Contains(states, operation.State) {
				return writeJSON(stdout, raw)
			}
			if slices.Contains(selfDevSettledStates, operation.State) {
				fmt.Fprintf(stderr, "choir self-dev wait: operation settled %s, not %s\n", operation.State, strings.Join(states, "|"))
				writeJSON(stdout, raw)
				return 1
			}
		}
		if time.Now().Add(*interval).After(giveUp) {
			fmt.Fprintf(stderr, "choir self-dev wait: deadline passed in state %q\n", last)
			return 1
		}
		time.Sleep(*interval)
	}
}

// selfDevDecidedStates are the states a landed decision leaves an operation
// in, by decision, and selfDevDecisionEventKinds the tape event it landed as.
// An operation in one of them may be replayed with the caller's same key: the
// guest answers 200 only for the exact decision that landed, so the caller
// learns its first call succeeded
// (docs/problems/selfdev-reject-lands-but-answers-409-2026-10-22.md).
var (
	selfDevDecidedStates = map[string][]string{
		"reject":  {"rejected"},
		"approve": {"accepted", "materializing", "applied"},
	}
	selfDevDecisionEventKinds = map[string]string{"reject": "effect_rejected", "approve": "effect_accepted"}
)

// selfDevDecisionBinding reads the frozen candidate and returns the binding
// a decision on exactly that candidate carries. Awaiting approval, that is
// the computer's current event head. Already in the state this decision
// leaves it in, it is the landed decision's own decision-time binding, read
// from the release witness, and replay is true: the decision moved the
// current head, and the guest matches a replay exactly.
func selfDevDecisionBinding(c *client, computerID, operationID, decision string) (selfDevOperation, map[string]any, bool, error) {
	var operation selfDevOperation
	if err := c.do(http.MethodGet, selfDevPath(computerID, "operations/"+url.PathEscape(operationID)), nil, &operation); err != nil {
		return operation, nil, false, err
	}
	replay := slices.Contains(selfDevDecidedStates[decision], operation.State)
	if operation.State != "awaiting_approval" && !replay {
		return operation, nil, false, fmt.Errorf("operation %s is %s, not awaiting_approval", operationID, operation.State)
	}
	if operation.BundleDigest == "" || len(operation.VerifierRefs) == 0 {
		return operation, nil, false, errors.New("operation has no frozen bundle or verifier")
	}
	if replay {
		var witness struct {
			Decision struct {
				Present                          bool   `json:"present"`
				EventKind                        string `json:"event_kind"`
				ExpectedDesiredEventHead         string `json:"expected_desired_event_head"`
				ExpectedEffectiveEventHead       string `json:"expected_effective_event_head"`
				ExpectedPendingTransitionRef     string `json:"expected_pending_transition_ref"`
				ExpectedDesiredStateCommitment   string `json:"expected_desired_state_commitment"`
				ExpectedEffectiveStateCommitment string `json:"expected_effective_state_commitment"`
			} `json:"decision"`
		}
		if err := c.do(http.MethodGet, selfDevPath(computerID, "operations/"+url.PathEscape(operationID)+"/release-witness"), nil, &witness); err != nil {
			return operation, nil, false, fmt.Errorf("read the landed decision: %w", err)
		}
		landed := witness.Decision
		if !landed.Present || landed.EventKind != selfDevDecisionEventKinds[decision] || landed.ExpectedDesiredEventHead == "" ||
			landed.ExpectedEffectiveEventHead == "" || landed.ExpectedDesiredStateCommitment == "" || landed.ExpectedEffectiveStateCommitment == "" {
			return operation, nil, false, fmt.Errorf("operation %s is %s, and the tape holds no %s decision for it to replay", operationID, operation.State, decision)
		}
		return operation, map[string]any{
			"bundle_digest":                       operation.BundleDigest,
			"expected_desired_event_head":         landed.ExpectedDesiredEventHead,
			"expected_effective_event_head":       landed.ExpectedEffectiveEventHead,
			"expected_pending_transition_ref":     landed.ExpectedPendingTransitionRef,
			"expected_desired_state_commitment":   landed.ExpectedDesiredStateCommitment,
			"expected_effective_state_commitment": landed.ExpectedEffectiveStateCommitment,
		}, true, nil
	}
	var head selfDevHead
	if err := c.do(http.MethodGet, selfDevPath(computerID, "head"), nil, &head); err != nil {
		return operation, nil, false, err
	}
	binding := map[string]any{
		"bundle_digest":                       operation.BundleDigest,
		"expected_desired_event_head":         head.DesiredEventHead,
		"expected_effective_event_head":       head.EffectiveEventHead,
		"expected_pending_transition_ref":     head.PendingTransitionRef,
		"expected_desired_state_commitment":   head.DesiredStateCommitment,
		"expected_effective_state_commitment": head.EffectiveStateCommitment,
	}
	return operation, binding, false, nil
}

// selfDevVisual lists a frozen candidate's visual-evidence captures. With
// --out (CLI only) it also fetches each resolvable capture's PNG from the
// computer's asset route, re-hashes the bytes against the row's digest, and
// writes <route-slug>-<digest12>.png into the directory. A capture that fails
// (unresolvable, hostile row, fetch error, digest mismatch) is named in the
// manifest and fails the command after the others are written; a refused
// body never reaches a file.
func selfDevVisual(x *call) (any, error) {
	operationID := x.trim("operation")
	listingPath := selfDevPath(x.str("computer"), "operations/"+url.PathEscape(operationID)+"/visual-evidence")
	dir := x.trim("out")
	if x.surface != "cli" || dir == "" {
		return getRaw(x, listingPath)
	}
	var listing selfDevVisualListing
	if err := x.c.do(http.MethodGet, listingPath, nil, &listing); err != nil {
		return nil, err
	}
	if listing.OperationID != operationID {
		return nil, fmt.Errorf("route answered operation %q, not %q", listing.OperationID, operationID)
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	manifest := &selfDevVisualManifest{selfDevVisualListing: listing, Out: dir}
	for i := range manifest.Captures {
		row := &manifest.Captures[i]
		row.File, row.Error = "", ""
		if err := selfDevVisualFetch(x.c, dir, row); err != nil {
			row.File, row.Error = "", err.Error()
		}
	}
	return manifest, nil
}

type selfDevVisualRow struct {
	Artifact   string `json:"artifact"`
	Digest     string `json:"digest"`
	Route      string `json:"route"`
	Width      int    `json:"width"`
	Height     int    `json:"height"`
	Bytes      int64  `json:"bytes"`
	Blank      bool   `json:"blank"`
	Resolvable bool   `json:"resolvable"`
	AssetURL   string `json:"asset_url"`
	File       string `json:"file,omitempty"`
	Error      string `json:"error,omitempty"`
}

type selfDevVisualListing struct {
	OperationID    string             `json:"operation_id"`
	ComputerID     string             `json:"computer_id"`
	State          string             `json:"state"`
	BundleDigest   string             `json:"bundle_digest"`
	ManifestSHA256 string             `json:"manifest_sha256"`
	FrontendSHA256 string             `json:"frontend_sha256"`
	CaptureReceipt string             `json:"capture_receipt,omitempty"`
	Captures       []selfDevVisualRow `json:"captures"`
}

type selfDevVisualManifest struct {
	selfDevVisualListing
	Out string `json:"out"`
}

const selfDevAssetPrefix = "/api/assets/sha256/"

var (
	selfDevDigestPattern = regexp.MustCompile(`^[0-9a-f]{64}$`)
	selfDevSlugRuns      = regexp.MustCompile(`[^a-z0-9]+`)
)

// selfDevRouteSlug turns a captured route into a file-name stem: lowercase
// alphanumerics joined by single dashes, never a path separator or dot.
func selfDevRouteSlug(route string) string {
	slug := strings.Trim(selfDevSlugRuns.ReplaceAllString(strings.ToLower(route), "-"), "-")
	if len(slug) > 60 {
		slug = strings.Trim(slug[:60], "-")
	}
	if slug == "" {
		return "root"
	}
	return slug
}

func selfDevVisualFetch(c *client, dir string, row *selfDevVisualRow) error {
	if !selfDevDigestPattern.MatchString(row.Digest) {
		return fmt.Errorf("row digest %q is not a lowercase sha256", row.Digest)
	}
	if row.Artifact != "artifact:sha256:"+row.Digest {
		return fmt.Errorf("row artifact %q does not name digest %s", row.Artifact, row.Digest)
	}
	if !row.Resolvable {
		return errors.New("not resolvable: the computer's asset store does not hold the bytes")
	}
	if row.AssetURL != selfDevAssetPrefix+row.Digest {
		return fmt.Errorf("asset_url %q is not %s<digest>", row.AssetURL, selfDevAssetPrefix)
	}
	png, err := c.doRawBytes(http.MethodGet, row.AssetURL, nil)
	if err != nil {
		return fmt.Errorf("fetch asset: %w", err)
	}
	sum := sha256.Sum256(png)
	if got := hex.EncodeToString(sum[:]); got != row.Digest {
		return fmt.Errorf("asset sha256 mismatch: listing recorded %s, received bytes hash to %s (%d bytes); not written", row.Digest, got, len(png))
	}
	// Written under a temporary name and renamed, so a failed write never
	// leaves a truncated file under the verified name.
	tmp, err := os.CreateTemp(dir, ".visual-*.tmp")
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(tmp.Name()) }()
	if _, err := tmp.Write(png); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	file := filepath.Join(dir, selfDevRouteSlug(row.Route)+"-"+row.Digest[:12]+".png")
	if err := os.Rename(tmp.Name(), file); err != nil {
		return err
	}
	row.File, row.Bytes = file, int64(len(png))
	return nil
}

// renderSelfDevVisual prints the manifest and fails the command when any
// capture did not verify and land.
func renderSelfDevVisual(w io.Writer, result any, x *call) int {
	code := writeJSON(w, result)
	if manifest, ok := result.(*selfDevVisualManifest); ok {
		for _, row := range manifest.Captures {
			if row.Error != "" {
				fmt.Fprintf(x.c.stderr, "choir self-dev visual: %s: %s\n", row.Route, row.Error)
				code = 1
			}
		}
	}
	return code
}
