// Package gomodule is the desk Go import gate (plan MA6.5, rulings R13, R26,
// R42; docs/design/ma65-yaegi-go-modules-2026-11-06.md): the one list of
// standard-library packages a desk's yaegi cells and the Go modules they load
// may import, the curated list of third-party modules with their Go checksum
// database hashes, the mechanical import-closure check, and the verified
// bundle a desk worker loads into its interpreter.
//
// It is a leaf: the publish path (internal/apppackage) and the desk worker
// (internal/yaegikernel) both import it, so the two cannot disagree.
package gomodule

import (
	"sort"
	"strings"
)

// stdlib is the desk standard-library set. A package is here only when yaegi
// ships its symbols, no function in it reaches the OS (files, processes,
// sockets, environment, exit) or the interpreter's internals, and it adds
// expressivity (design section 2). The list is closed: a package not named is
// refused whatever the banned list says.
var stdlib = []string{
	// The 16 of the set before MA6.5.
	"bytes", "encoding/base64", "encoding/hex", "encoding/json", "errors", "fmt",
	"math", "math/big", "math/rand/v2", "regexp", "sort", "strconv", "strings", "time",
	"unicode", "unicode/utf8",
	// Code mode (R26): join and bound fanned-out verb calls.
	"context", "sync", "sync/atomic",
	// Generic helpers and containers.
	"cmp", "maps", "slices", "container/heap", "container/list", "container/ring",
	// Numeric completeness.
	"math/bits", "math/cmplx", "math/rand",
	// Text over values the desk already holds.
	"bufio", "html", "io", "path", "text/scanner", "text/tabwriter", "unicode/utf16",
	// Formats a desk reads through its verbs.
	"encoding", "encoding/base32", "encoding/binary", "encoding/csv", "encoding/pem", "encoding/xml",
	// Digests.
	"crypto/hmac", "crypto/md5", "crypto/sha1", "crypto/sha256", "crypto/sha512",
	"hash", "hash/adler32", "hash/crc32", "hash/fnv",
	// URL parsing; it opens no socket.
	"net/url",
	// Unpacking bytes a verb returned.
	"compress/flate", "compress/gzip", "compress/zlib",
}

var stdlibSet = func() map[string]bool {
	m := make(map[string]bool, len(stdlib))
	for _, p := range stdlib {
		m[p] = true
	}
	return m
}()

// Stdlib returns the desk standard-library set, sorted (a copy).
func Stdlib() []string {
	out := append([]string{}, stdlib...)
	sort.Strings(out)
	return out
}

// InStdlib reports whether p is in the desk standard-library set.
func InStdlib(p string) bool { return stdlibSet[p] }

// banned names the packages that are never imported by desk code, with the
// reason a refusal gives. The set is closed without it; it exists so a
// refusal says why.
var banned = map[string]string{
	"unsafe":    "unsafe memory access is forbidden",
	"reflect":   "runtime reflection is forbidden in the sandbox",
	"runtime":   "runtime introspection and manipulation is forbidden",
	"plugin":    "dynamic plugin loading is forbidden",
	"syscall":   "raw syscalls are forbidden",
	"os":        "direct OS access must route through the desk's verbs",
	"os/exec":   "process execution must route through the desk's verbs",
	"os/signal": "signal handling is forbidden",
	"net":       "raw network access is forbidden",
	"net/http":  "raw HTTP network access is forbidden",
	"net/rpc":   "raw network access is forbidden",
	"io/ioutil": "raw filesystem I/O is forbidden",
	"embed":     "embedding files is forbidden",
	"C":         "cgo is forbidden",
}

// bannedPrefixes ban whole trees.
var bannedPrefixes = map[string]string{
	"runtime/":  "runtime introspection and manipulation is forbidden",
	"os/":       "direct OS access must route through the desk's verbs",
	"syscall/":  "raw syscalls are forbidden",
	"debug/":    "binary and runtime debugging packages are forbidden",
	"net/http/": "raw HTTP network access is forbidden",
	"internal/": "the standard library's internal packages are forbidden",
	"vendor/":   "the standard library's vendored packages are forbidden",
}

// BannedReason reports why p is banned for desk code, if it is.
func BannedReason(p string) (string, bool) {
	if r, ok := banned[p]; ok {
		return r, true
	}
	for pre, r := range bannedPrefixes {
		if strings.HasPrefix(p, pre) {
			return r, true
		}
	}
	return "", false
}

// IsStdlibPath reports whether an import path names the standard library (or
// a path shaped like it): its first element has no dot. Third-party module
// paths always have one.
func IsStdlibPath(p string) bool {
	first := p
	if i := strings.IndexByte(p, '/'); i >= 0 {
		first = p[:i]
	}
	return !strings.Contains(first, ".")
}
