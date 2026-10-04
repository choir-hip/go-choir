package updater

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// nix-store --export ("narchive") reader + guest-side materializer.
//
// The host builder produces a delta closure as a narchive stream. The guest
// has no nix binary (deliberately — a small base is the point of layering),
// so the updater replays the stream itself into a per-computer GC-rooted
// private store on the data disk.
//
// The NAR payload is itself serialized: "nix-archive-1" + a typed node tree
// (directory/regular/symlink). This reader decodes both layers and writes the
// referenced file tree under a caller-chosen store root.
//
// Wire format (empirically pinned against nix-store 2.34 --export). The
// stream is a sequence of objects with no leading count, followed by one
// trailing u64 signature-presence flag (0 for unsigned). Each object is:
//   u64  object marker (1)
//   nar  NAR payload, self-delimiting (see decodeNarNode; ends at the close
//        of the root node's balanced tuple tree)
//   u64  trailer magic "NIXE" (little-endian 0x000000004558494e)
//   str  storePath        (/nix/store/<hash>-<name>)
//   u64  nrefs, then nrefs str FULL store-path references
//   str  deriver          (full .drv path; "" when absent)
//   u64  signatureCount;  then that many str signatures (0 for unsigned)
//
// The NAR node grammar uses "(", ")", and keyword strings as length-1..N
// padded tokens (not u64 markers):
//   node      := "(" "type" <type> fields... ")"
//   regular   := ["executable" ""] "contents" <str bytes>
//   directory := ("entry" "(" "name" <str> "node" <node> ")")*
//   symlink   := "target" <str>
// Integrity is not carried as an in-stream narSize/narHash (this format has
// none): authenticity comes from the signature (absent here) or, for the
// content-addressed paths the builder emits, the store path's own hash. The
// caller binds the closure to the release manifest's declared path set.

const (
	exportTrailerMagic = 0x000000004558494e // "NIXE" little-endian u64
	objectMarker       = 1
	narMagic           = "nix-archive-1"
)

// ClosureObject is one exported store path and its metadata.
type ClosureObject struct {
	// StorePath is the canonical /nix/store/<hash>-<name> the payload binds.
	StorePath string
	// References are the full store paths this object depends on.
	References []string
	// Deriver is the producing .drv path ("" when absent).
	Deriver string
}

// narReader decodes the nix serialization wire format (little-endian int64
// lengths; strings/blob are length-prefixed and zero-padded to a multiple
// of 8).
type narReader struct {
	r   *bytes.Reader
	err error
}

func (n *narReader) u64() uint64 {
	if n.err != nil {
		return 0
	}
	var b [8]byte
	if _, err := io.ReadFull(n.r, b[:]); err != nil {
		n.err = fmt.Errorf("nar: read u64: %w", err)
		return 0
	}
	return binary.LittleEndian.Uint64(b[:])
}

func (n *narReader) str(max uint64) string {
	if n.err != nil {
		return ""
	}
	b := n.blob(max)
	if n.err != nil {
		return ""
	}
	return string(b)
}

// blob reads a length-prefixed, zero-padded-to-8 byte field — the framing
// used for "contents" and opaque values (same as str, returned as bytes).
func (n *narReader) blob(max uint64) []byte {
	if n.err != nil {
		return nil
	}
	length := n.u64()
	if n.err != nil {
		return nil
	}
	if length > max {
		n.err = fmt.Errorf("nar: field length %d exceeds bound %d", length, max)
		return nil
	}
	buf := make([]byte, length)
	if _, err := io.ReadFull(n.r, buf); err != nil {
		n.err = fmt.Errorf("nar: read field: %w", err)
		return nil
	}
	if pad := (8 - (length % 8)) % 8; pad > 0 {
		padBuf := make([]byte, pad)
		if _, err := io.ReadFull(n.r, padBuf); err != nil {
			n.err = fmt.Errorf("nar: read field padding: %w", err)
			return nil
		}
	}
	return buf
}

// ParseClosure reads a `nix-store --export` stream into its objects and
// decoded NAR node roots. The stream is self-delimiting: objects are consumed
// until the 8-byte trailing signature flag, which must be 0 (unsigned).
func ParseClosure(blob []byte) ([]ClosureObject, map[string]*narNode, error) {
	if len(blob) == 0 {
		return nil, nil, fmt.Errorf("nar: empty export")
	}
	objects := []ClosureObject{}
	narRoots := map[string]*narNode{}
	r := bytes.NewReader(blob)
	for r.Len() > 8 {
		n := &narReader{r: r}
		if marker := n.u64(); marker != objectMarker {
			return nil, nil, fmt.Errorf("nar: object %d missing marker (got %#x)", len(objects), marker)
		}
		if magic := n.str(16); magic != narMagic {
			return nil, nil, fmt.Errorf("nar: object %d bad nar magic %q", len(objects), magic)
		}
		root, err := decodeNarNode(n, "")
		if err != nil {
			return nil, nil, fmt.Errorf("nar: object %d decode: %w", len(objects), err)
		}
		if n.err != nil {
			return nil, nil, n.err
		}
		if magic := n.u64(); magic != exportTrailerMagic {
			return nil, nil, fmt.Errorf("nar: object %d missing NIXE trailer (got %#x)", len(objects), magic)
		}
		obj := ClosureObject{}
		obj.StorePath = n.str(4096)
		nrefs := n.u64()
		if nrefs > 1<<16 {
			return nil, nil, fmt.Errorf("nar: implausible reference count %d", nrefs)
		}
		for j := uint64(0); j < nrefs; j++ {
			obj.References = append(obj.References, n.str(4096))
		}
		obj.Deriver = n.str(4096)
		sigCount := n.u64()
		for j := uint64(0); j < sigCount && j < 64; j++ {
			_ = n.str(4096)
		}
		if n.err != nil {
			return nil, nil, n.err
		}
		if !strings.HasPrefix(obj.StorePath, "/nix/store/") {
			return nil, nil, fmt.Errorf("nar: object %d has non-store path %q", len(objects), obj.StorePath)
		}
		objects = append(objects, obj)
		narRoots[obj.StorePath] = root
	}
	// Trailing u64 signature-presence flag must be 0 (unsigned) and exhaust
	// the stream exactly.
	n := &narReader{r: r}
	if flag := n.u64(); flag != 0 {
		return nil, nil, fmt.Errorf("nar: signed export unsupported (flag=%d)", flag)
	}
	if n.err != nil {
		return nil, nil, n.err
	}
	if r.Len() != 0 {
		return nil, nil, fmt.Errorf("nar: %d trailing bytes after export", r.Len())
	}
	return objects, narRoots, nil
}

// narNode is one decoded NAR tree entry.
type narNode struct {
	name       string
	kind       string // "regular" | "directory" | "symlink"
	executable bool
	contents   []byte
	linkTarget string
	children   []*narNode
}

// decodeNarNode consumes one "(" type <t> ... ")" tuple into a node. The
// open/close and all keywords are length-1..N padded string tokens.
func decodeNarNode(n *narReader, name string) (*narNode, error) {
	if open := n.str(8); open != "(" {
		return nil, fmt.Errorf("nar: node %q missing '(' open (got %q)", name, open)
	}
	node := &narNode{name: name}
	if k := n.str(64); k != "type" {
		return nil, fmt.Errorf("nar: node %q missing type field", name)
	}
	typ := n.str(64)
	node.kind = typ
	switch typ {
	case "regular":
		// optional "executable" "" then "contents" <data> then ")"
		for {
			tok := n.str(64)
			if n.err != nil {
				return nil, n.err
			}
			switch tok {
			case "executable":
				_ = n.str(8) // empty marker value
				node.executable = true
			case "contents":
				node.contents = n.blob(1 << 31)
			case ")":
				return node, nil
			default:
				return nil, fmt.Errorf("nar: regular node %q unexpected token %q", name, tok)
			}
		}
	case "directory":
		// "entry" ( "name" <str> "node" <node> ) ... until ")"
		for {
			tok := n.str(64)
			if n.err != nil {
				return nil, n.err
			}
			if tok == ")" {
				return node, nil
			}
			if tok != "entry" {
				return nil, fmt.Errorf("nar: directory %q unexpected token %q", name, tok)
			}
			if open := n.str(8); open != "(" {
				return nil, fmt.Errorf("nar: dir %q entry missing '('", name)
			}
			if k := n.str(64); k != "name" {
				return nil, fmt.Errorf("nar: dir %q entry missing name", name)
			}
			childName := n.str(4096)
			if k := n.str(64); k != "node" {
				return nil, fmt.Errorf("nar: dir %q entry missing node", name)
			}
			child, err := decodeNarNode(n, childName)
			if err != nil {
				return nil, err
			}
			node.children = append(node.children, child)
			if closeTok := n.str(8); closeTok != ")" {
				return nil, fmt.Errorf("nar: dir %q entry missing ')'", name)
			}
		}
	case "symlink":
		if k := n.str(64); k != "target" {
			return nil, fmt.Errorf("nar: symlink %q missing target", name)
		}
		node.linkTarget = n.str(4096)
		if closeTok := n.str(8); closeTok != ")" {
			return nil, fmt.Errorf("nar: symlink %q missing ')' close", name)
		}
		return node, nil
	default:
		return nil, fmt.Errorf("nar: node %q unknown type %q", name, typ)
	}
}

// MaterializeClosure replays a narchive blob into storeRoot so each object's
// canonical /nix/store/<hash>-<name> lands at storeRoot/<hash>-<name>. Files
// are written read-only; each store path is staged then renamed atomically.
// The GC root is the caller's symlink under storeRoot/../gc-roots.
// Returns the sorted list of materialized store basenames.
func MaterializeClosure(blob []byte, storeRoot string) ([]string, error) {
	objects, roots, err := ParseClosure(blob)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(storeRoot, 0o755); err != nil {
		return nil, fmt.Errorf("nar: create store root: %w", err)
	}
	var materialized []string
	for _, obj := range objects {
		base := filepath.Base(obj.StorePath)
		target := filepath.Join(storeRoot, base)
		// Content-addressed + immutable: a present target means this object is
		// already fully materialized (atomic rename guarantees no partial
		// paths). Skip so replays/resumed applies are idempotent.
		if _, statErr := os.Lstat(target); statErr == nil {
			materialized = append(materialized, base)
			continue
		}
		tmp, err := os.MkdirTemp(storeRoot, ".staging-")
		if err != nil {
			return nil, fmt.Errorf("nar: stage %s: %w", base, err)
		}
		tmpTarget := filepath.Join(tmp, base)
		if err := writeNarNode(roots[obj.StorePath], tmpTarget); err != nil {
			os.RemoveAll(tmp)
			return nil, fmt.Errorf("nar: write %s: %w", base, err)
		}
		if err := os.Rename(tmpTarget, target); err != nil {
			os.RemoveAll(tmp)
			return nil, fmt.Errorf("nar: place %s: %w", base, err)
		}
		os.RemoveAll(tmp)
		if err := makeTreeReadOnly(target); err != nil {
			return nil, fmt.Errorf("nar: mark %s read-only: %w", base, err)
		}
		materialized = append(materialized, base)
	}
	sort.Strings(materialized)
	return materialized, nil
}

func writeNarNode(node *narNode, target string) error {
	switch node.kind {
	case "regular":
		mode := os.FileMode(0o444)
		if node.executable {
			mode = 0o555
		}
		return writeNarFile(target, node.contents, mode)
	case "symlink":
		if _, err := os.Lstat(target); err == nil {
			return nil // already materialized
		}
		return os.Symlink(node.linkTarget, target)
	case "directory":
		// Build writable (0o755) first — a read-only dir can't receive
		// children on every fs, and the final path is only made read-only
		// after the atomic rename by materializeFinalizeReadOnly.
		if err := os.MkdirAll(target, 0o755); err != nil {
			return err
		}
		for _, child := range node.children {
			childPath := filepath.Join(target, filepath.FromSlash(child.name))
			if err := writeNarNode(child, childPath); err != nil {
				return err
			}
		}
		return nil
	default:
		return fmt.Errorf("nar: unknown node kind %q at %q", node.kind, target)
	}
}

// makeTreeReadOnly walks a fully materialized store path and marks every
// entry read-only, matching nix store immutability.
func makeTreeReadOnly(root string) error {
	return filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return nil // symlinks carry no mode
		}
		if entry.IsDir() {
			return os.Chmod(path, 0o555)
		}
		// preserve executable bits; everything else read-only
		info, infoErr := entry.Info()
		if infoErr != nil {
			return infoErr
		}
		mode := os.FileMode(0o444)
		if info.Mode().Perm()&0o111 != 0 {
			mode = 0o555
		}
		return os.Chmod(path, mode)
	})
}

// writeNarFile writes contents to target read-only with create-exclusive
// semantics. A store path is content-addressed and immutable, so an existing
// file at target means the path is already materialized — treat as written.
func writeNarFile(target string, contents []byte, mode os.FileMode) error {
	output, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, mode)
	if err != nil {
		if os.IsExist(err) {
			return nil
		}
		return err
	}
	if _, err := output.Write(contents); err != nil {
		output.Close()
		return err
	}
	if err := output.Sync(); err != nil {
		output.Close()
		return err
	}
	if err := output.Close(); err != nil {
		return err
	}
	return os.Chmod(target, mode)
}

// materializeReleaseClosure replays a layered release's `closure.nar` into
// the updater's private store (u.root/store) and GC-roots it under
// u.root/gc-roots/<releaseDigest>. No-op for plain file releases (empty
// ClosureDigest). Runs inside Apply between stageRelease and the pointer
// swap, so a replay failure leaves the running release untouched.
func (u *Updater) materializeReleaseClosure(releaseDir string, manifest ReleaseManifest) error {
	if manifest.ClosureDigest == "" {
		return nil
	}
	narPath := filepath.Join(releaseDir, "closure.nar")
	blob, err := os.ReadFile(narPath)
	if err != nil {
		return fmt.Errorf("updater: layered release missing closure.nar: %w", err)
	}
	sum := fmt.Sprintf("%x", sha256Hex(blob))
	if sum != manifest.ClosureDigest {
		return fmt.Errorf("updater: closure.nar digest %s != manifest closure_digest %s", sum, manifest.ClosureDigest)
	}
	storeRoot := filepath.Join(u.root, "store")
	paths, err := MaterializeClosure(blob, storeRoot)
	if err != nil {
		return fmt.Errorf("updater: replay app-layer closure: %w", err)
	}
	gcDir := filepath.Join(u.root, "gc-roots", filepath.Base(releaseDir))
	if err := os.MkdirAll(gcDir, 0o700); err != nil {
		return fmt.Errorf("updater: create gc-root dir: %w", err)
	}
	for _, base := range paths {
		link := filepath.Join(gcDir, base)
		target := filepath.Join(storeRoot, base)
		if _, err := os.Lstat(link); err == nil {
			continue
		}
		if err := os.Symlink(target, link); err != nil {
			return fmt.Errorf("updater: gc-root %s: %w", base, err)
		}
	}
	return nil
}

func sha256Hex(b []byte) [32]byte { return sha256.Sum256(b) }
