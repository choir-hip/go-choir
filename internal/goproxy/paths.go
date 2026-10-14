// Package goproxy is the host-side Go module proxy that engineering capsules
// reach through their L2 egress route (docs/reports/capsule-go-module-proxy-
// design-2026-10-14.md). It serves only the GOPROXY protocol and the
// checksum-database proxy paths, from an append-only disk cache that it
// fills from fixed upstreams on a miss.
package goproxy

import (
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
)

// SumDBName is the only checksum database this proxy serves.
const SumDBName = "sum.golang.org"

type requestKind int

const (
	kindInfo requestKind = iota
	kindMod
	kindZip
	kindList
	kindLatest
	kindSumDBSupported
	kindSumDBLatest
	kindSumDBLookup
	kindSumDBTile
)

// request is a checked proxy request. upstreamPath is built only from
// checked elements and never carries a host, a scheme or a query.
type request struct {
	kind         requestKind
	module       string // escaped module path (module kinds and lookups)
	version      string // escaped version (info/mod/zip/lookup)
	upstreamPath string // relative to the upstream base, no leading slash
	cacheRel     string // relative cache path; empty when not cacheable
}

func (r request) sumdb() bool { return r.kind >= kindSumDBSupported }

func (r request) cacheable() bool { return r.cacheRel != "" }

// errRefused is a path this proxy does not serve.
var errRefused = errors.New("not a Go module proxy path")

const (
	maxModulePathLen = 512
	maxVersionLen    = 256
)

var (
	// canonicalVersion is a canonical semantic version as the go command
	// writes it (pseudo-versions included). Only these answers are immutable.
	canonicalVersion = regexp.MustCompile(`^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(-[0-9A-Za-z-]+(\.[0-9A-Za-z-]+)*)?(\+incompatible)?$`)
	// tilePath is a checksum-database tile: tile/H/L/[xNNN/]*NNN[.p/W],
	// with L a level or "data".
	tilePath = regexp.MustCompile(`^tile/[0-9]{1,2}/([0-9]{1,2}|data)/(x[0-9]{3}/)*[0-9]{3}(\.p/[0-9]{1,3})?$`)
)

// parseRequest checks a request path against the GOPROXY and sumdb-proxy
// grammar. Anything else is errRefused.
func parseRequest(urlPath string) (request, error) {
	if !strings.HasPrefix(urlPath, "/") || strings.ContainsAny(urlPath, "\\\x00?#%") {
		return request{}, errRefused
	}
	p := urlPath[1:]
	if rest, ok := strings.CutPrefix(p, "sumdb/"); ok {
		return parseSumDB(rest)
	}
	if mod, ok := strings.CutSuffix(p, "/@latest"); ok {
		if err := checkEscapedModulePath(mod); err != nil {
			return request{}, err
		}
		return request{kind: kindLatest, module: mod, upstreamPath: p}, nil
	}
	mod, file, ok := strings.Cut(p, "/@v/")
	if !ok {
		return request{}, errRefused
	}
	if err := checkEscapedModulePath(mod); err != nil {
		return request{}, err
	}
	if file == "list" {
		return request{kind: kindList, module: mod, upstreamPath: p}, nil
	}
	var kind requestKind
	var version string
	switch {
	case strings.HasSuffix(file, ".info"):
		kind, version = kindInfo, strings.TrimSuffix(file, ".info")
	case strings.HasSuffix(file, ".mod"):
		kind, version = kindMod, strings.TrimSuffix(file, ".mod")
	case strings.HasSuffix(file, ".zip"):
		kind, version = kindZip, strings.TrimSuffix(file, ".zip")
	default:
		return request{}, errRefused
	}
	if err := checkEscapedVersion(version); err != nil {
		return request{}, err
	}
	r := request{kind: kind, module: mod, version: version, upstreamPath: p}
	if isCanonicalEscapedVersion(version) {
		r.cacheRel = "cache/" + p
	} else if kind != kindInfo {
		// .mod and .zip are defined only for canonical versions; a query
		// is answered through .info alone.
		return request{}, errRefused
	}
	return r, nil
}

func parseSumDB(rest string) (request, error) {
	name, sub, ok := strings.Cut(rest, "/")
	if !ok || name != SumDBName {
		return request{}, errRefused
	}
	switch {
	case sub == "supported":
		return request{kind: kindSumDBSupported}, nil
	case sub == "latest":
		return request{kind: kindSumDBLatest, upstreamPath: sub}, nil
	case strings.HasPrefix(sub, "lookup/"):
		mod, version, ok := strings.Cut(strings.TrimPrefix(sub, "lookup/"), "@")
		if !ok {
			return request{}, errRefused
		}
		if err := checkEscapedModulePath(mod); err != nil {
			return request{}, err
		}
		if err := checkEscapedVersion(version); err != nil {
			return request{}, err
		}
		return request{kind: kindSumDBLookup, module: mod, version: version, upstreamPath: sub}, nil
	case tilePath.MatchString(sub):
		r := request{kind: kindSumDBTile, upstreamPath: sub}
		if !strings.Contains(sub, ".p/") {
			// Full tiles never change; partial tiles grow.
			r.cacheRel = "sumdb/" + SumDBName + "/" + sub
		}
		return r, nil
	}
	return request{}, errRefused
}

// checkEscapedModulePath accepts a module path as the go command escapes it
// in proxy URLs: lowercase letters, digits, "-._~", and "!"+lowercase for an
// uppercase letter; no empty, ".", ".." or dot-edged element; a first
// element with a dot that does not start with "-".
func checkEscapedModulePath(p string) error {
	if p == "" || len(p) > maxModulePathLen {
		return errRefused
	}
	elems := strings.Split(p, "/")
	for i, e := range elems {
		if e == "" || e[0] == '.' || e[len(e)-1] == '.' {
			return errRefused
		}
		if err := checkEscapedChars(e, false); err != nil {
			return err
		}
		if i == 0 && (!strings.Contains(e, ".") || e[0] == '-') {
			return errRefused
		}
	}
	return nil
}

// checkEscapedVersion accepts an escaped version or version query.
func checkEscapedVersion(v string) error {
	if v == "" || len(v) > maxVersionLen || v[0] == '.' || v == "." || v == ".." {
		return errRefused
	}
	return checkEscapedChars(v, true)
}

func checkEscapedChars(s string, version bool) error {
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c >= 'a' && c <= 'z', c >= '0' && c <= '9', c == '-', c == '.', c == '_', c == '~':
		case c == '+' && version:
		case c == '!':
			if i+1 >= len(s) || s[i+1] < 'a' || s[i+1] > 'z' {
				return errRefused
			}
			i++
		default:
			return errRefused
		}
	}
	return nil
}

func isCanonicalEscapedVersion(v string) bool {
	u, err := unescape(v)
	return err == nil && canonicalVersion.MatchString(u)
}

// escape is the go command's case-encoding for proxy paths: an uppercase
// letter becomes "!" plus its lowercase form.
func escape(s string) (string, error) {
	var b strings.Builder
	for _, r := range s {
		switch {
		case r == '!' || r >= 0x80:
			return "", fmt.Errorf("goproxy: cannot escape %q", s)
		case r >= 'A' && r <= 'Z':
			b.WriteByte('!')
			b.WriteRune(r + ('a' - 'A'))
		default:
			b.WriteRune(r)
		}
	}
	return b.String(), nil
}

func unescape(s string) (string, error) {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c >= 'A' && c <= 'Z':
			return "", fmt.Errorf("goproxy: %q is not escaped", s)
		case c == '!':
			if i+1 >= len(s) || s[i+1] < 'a' || s[i+1] > 'z' {
				return "", fmt.Errorf("goproxy: bad escape in %q", s)
			}
			b.WriteByte(s[i+1] - ('a' - 'A'))
			i++
		default:
			b.WriteByte(c)
		}
	}
	return b.String(), nil
}

// cacheFile joins a checked relative cache path under root.
func cacheFile(root, rel string) string {
	return filepath.Join(root, filepath.FromSlash(rel))
}

// sumKey is the go.sum key for a module request: "mod ver" for a zip,
// "mod ver/go.mod" for a go.mod.
func sumKey(r request) (string, bool) {
	if r.kind != kindMod && r.kind != kindZip {
		return "", false
	}
	mod, err := unescape(r.module)
	if err != nil {
		return "", false
	}
	ver, err := unescape(r.version)
	if err != nil {
		return "", false
	}
	if r.kind == kindMod {
		return mod + " " + ver + "/go.mod", true
	}
	return mod + " " + ver, true
}
