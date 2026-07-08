// Package transaction provides the capsule transaction classifier, builder,
// and tape. It runs on the host (trusted zone) alongside the capsule executor.
//
// The classifier groups file changes by ledger kind. The builder converts
// classified diffs into structured transaction records. The tape is a
// tamper-evident append-only log of transaction records.
//
// v7 decision: Unknown paths are REJECTED at commit time. Silently
// classifying as LedgerVM creates a trust-bearing catch-all.
package transaction

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"path"
	"sort"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/capsule"
)

// LedgerKind classifies which ledger a file change belongs to.
// This determines how the MutationTransaction records the change.
type LedgerKind int

const (
	LedgerVM LedgerKind = iota       // V: /boot, /lib/modules, /etc/systemd
	LedgerDolt LedgerKind = iota     // D: /var/lib/dolt
	LedgerSource LedgerKind = iota   // S: /home/user/src, /workspace
	LedgerBlob LedgerKind = iota     // B: /var/lib/blob
	LedgerArtifact LedgerKind = iota // A: /var/lib/artifact
	LedgerRoute LedgerKind = iota    // R: /etc/choir/route
	LedgerUnknown LedgerKind = iota  // rejected at commit time (v7 decision)
)

func (k LedgerKind) String() string {
	switch k {
	case LedgerVM:
		return "VM"
	case LedgerDolt:
		return "Dolt"
	case LedgerSource:
		return "Source"
	case LedgerBlob:
		return "Blob"
	case LedgerArtifact:
		return "Artifact"
	case LedgerRoute:
		return "Route"
	case LedgerUnknown:
		return "Unknown"
	default:
		return fmt.Sprintf("LedgerKind(%d)", int(k))
	}
}

// MarshalJSON encodes LedgerKind as its string name.
func (k LedgerKind) MarshalJSON() ([]byte, error) {
	return json.Marshal(k.String())
}

// PathPattern matches file paths against a prefix or glob pattern.
type PathPattern struct {
	Prefix string // e.g. "/var/lib/dolt" — matches if path starts with this
	Glob   string // e.g. "*.go" — matches if path matches this glob (optional)
}

// Match checks if a path matches this pattern.
func (p PathPattern) Match(filePath string) bool {
	if p.Prefix != "" {
		if !strings.HasPrefix(filePath, p.Prefix) {
			return false
		}
	}
	if p.Glob != "" {
		matched, err := path.Match(p.Glob, path.Base(filePath))
		if err != nil || !matched {
			return false
		}
	}
	return true
}

// Classifier groups file changes by ledger kind. It runs on the host
// (trusted zone) and determines how each change is recorded in the
// MutationTransaction.
type Classifier struct {
	Version string                       `json:"version"` // "v1"
	Rules   map[LedgerKind][]PathPattern `json:"rules"`
	Ignore  []PathPattern                `json:"ignore"` // ephemeral paths
}

// NewClassifier creates the default v1 classifier with standard path rules.
func NewClassifier() *Classifier {
	return &Classifier{
		Version: "v1",
		Rules: map[LedgerKind][]PathPattern{
			LedgerVM: {
				{Prefix: "/boot"},
				{Prefix: "/lib/modules"},
				{Prefix: "/etc/systemd"},
				{Prefix: "/usr/lib"},
				{Prefix: "/usr/share"},
			},
			LedgerDolt: {
				{Prefix: "/var/lib/dolt"},
			},
			LedgerSource: {
				{Prefix: "/home/user/src"},
				{Prefix: "/workspace"},
				{Prefix: "/root/src"},
			},
			LedgerBlob: {
				{Prefix: "/var/lib/blob"},
			},
			LedgerArtifact: {
				{Prefix: "/var/lib/artifact"},
			},
			LedgerRoute: {
				{Prefix: "/etc/choir/route"},
			},
		},
		Ignore: []PathPattern{
			{Prefix: "/tmp/"},
			{Prefix: "/run"},
			{Prefix: "/var/log"},
			{Prefix: "/var/cache"},
			{Prefix: "/dev"},
			{Prefix: "/proc"},
			{Prefix: "/sys"},
			{Glob: "*.cache"},
			{Glob: "*.tmp"},
			{Glob: "*.log"},
		},
	}
}

// ClassifyResult is the output of classification.
type ClassifyResult struct {
	Version string                          `json:"version"`
	Groups  map[LedgerKind][]capsule.FileChange `json:"groups"`
	Ignored []capsule.FileChange            `json:"ignored"`
	Unknown []capsule.FileChange            `json:"unknown"`
	Digest  string                          `json:"digest"` // SHA-256 of the classification
}

// Classify groups file changes by ledger kind. Ephemeral paths are ignored.
// Unknown paths are returned separately for commit-time rejection.
func (c *Classifier) Classify(changes []capsule.FileChange) *ClassifyResult {
	result := &ClassifyResult{
		Version: c.Version,
		Groups:  make(map[LedgerKind][]capsule.FileChange),
	}

	for _, change := range changes {
		// Check ignore patterns first.
		if c.isIgnored(change.Path) {
			result.Ignored = append(result.Ignored, change)
			continue
		}

		// Find matching ledger kind.
		kind := c.classifyPath(change.Path)
		if kind == LedgerUnknown {
			result.Unknown = append(result.Unknown, change)
		} else {
			result.Groups[kind] = append(result.Groups[kind], change)
		}
	}

	// Sort each group by path for deterministic ordering.
	for _, group := range result.Groups {
		sort.Slice(group, func(i, j int) bool {
			return group[i].Path < group[j].Path
		})
	}
	sort.Slice(result.Ignored, func(i, j int) bool {
		return result.Ignored[i].Path < result.Ignored[j].Path
	})
	sort.Slice(result.Unknown, func(i, j int) bool {
		return result.Unknown[i].Path < result.Unknown[j].Path
	})

	// Compute digest of the classification.
	result.Digest = c.computeDigest(result)

	return result
}

// isIgnored checks if a path matches any ignore pattern.
func (c *Classifier) isIgnored(filePath string) bool {
	for _, pattern := range c.Ignore {
		if pattern.Match(filePath) {
			return true
		}
	}
	return false
}

// classifyPath determines the LedgerKind for a given path.
// Iterates kinds in a deterministic order (not map iteration order)
// to ensure consistent classification when prefixes overlap.
func (c *Classifier) classifyPath(filePath string) LedgerKind {
	// Check rules in deterministic order by iterating sorted kinds.
	// The order matters: /var/lib/dolt must be checked before /var/lib.
	for _, kind := range sortedLedgerKinds {
		patterns, ok := c.Rules[kind]
		if !ok {
			continue
		}
		for _, pattern := range patterns {
			if pattern.Match(filePath) {
				return kind
			}
		}
	}
	return LedgerUnknown
}

// sortedLedgerKinds is the deterministic iteration order for classifyPath.
var sortedLedgerKinds = []LedgerKind{
	LedgerDolt,     // /var/lib/dolt (most specific)
	LedgerBlob,     // /var/lib/blob
	LedgerArtifact, // /var/lib/artifact
	LedgerRoute,    // /etc/choir/route
	LedgerSource,   // /home/user/src, /workspace
	LedgerVM,       // /boot, /lib/modules, /etc/systemd, /usr/lib
}

// computeDigest computes a SHA-256 digest of the classification result.
// This is used for versioning and audit trail.
func (c *Classifier) computeDigest(result *ClassifyResult) string {
	h := sha256.New()

	// Write version.
	h.Write([]byte(result.Version))

	// Write each group in a deterministic order.
	kinds := []LedgerKind{LedgerVM, LedgerDolt, LedgerSource, LedgerBlob, LedgerArtifact, LedgerRoute}
	for _, kind := range kinds {
		changes, ok := result.Groups[kind]
		if !ok {
			continue
		}
		h.Write([]byte(kind.String()))
		for _, change := range changes {
			fmt.Fprintf(h, "%s:%s:%s", change.Path, change.Kind, change.Mode)
		}
	}

	return hex.EncodeToString(h.Sum(nil))
}

// RulesDigest computes a SHA-256 digest of the classifier ruleset.
// This is used to verify that all hosts in a fleet use the same rules.
func (c *Classifier) RulesDigest() string {
	data, err := json.Marshal(c)
	if err != nil {
		return ""
	}
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

// HasUnknown returns true if the classification result contains any
// unknown paths that would be rejected at commit time.
func (r *ClassifyResult) HasUnknown() bool {
	return len(r.Unknown) > 0
}

// Summary returns a human-readable summary of the classification.
func (r *ClassifyResult) Summary() string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Classifier %s (digest: %s...)\n", r.Version, r.Digest[:16]))
	for _, kind := range []LedgerKind{LedgerVM, LedgerDolt, LedgerSource, LedgerBlob, LedgerArtifact, LedgerRoute} {
		if changes, ok := r.Groups[kind]; ok {
			sb.WriteString(fmt.Sprintf("  %s: %d changes\n", kind, len(changes)))
		}
	}
	if len(r.Ignored) > 0 {
		sb.WriteString(fmt.Sprintf("  Ignored: %d changes\n", len(r.Ignored)))
	}
	if len(r.Unknown) > 0 {
		sb.WriteString(fmt.Sprintf("  Unknown (REJECTED): %d changes\n", len(r.Unknown)))
	}
	return sb.String()
}
