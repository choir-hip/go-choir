package pkgmirror

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/url"
	"path"
	"regexp"
	"strings"
)

// npm: the registry protocol's two read paths. Packuments pass through (npm
// 9 and later fetch a tarball named on registry.npmjs.org from the
// configured registry, replace-registry-host's default, so no rewrite is
// needed); tarballs are cached once they match the packument's integrity.
//
//	/npm/<name>                      packument
//	/npm/@<scope>/<name>             packument (npm sends the slash as %2F)
//	/npm/<name>/-/<name>-<ver>.tgz   tarball
//	/npm/@<scope>/<name>/-/<name>-<ver>.tgz

var (
	// npmComponent is a name or scope as the registry stores them (legacy
	// names keep their capitals); never "." or "_" first, never empty.
	npmComponent = regexp.MustCompile(`^[A-Za-z0-9~-][A-Za-z0-9._~-]{0,213}$`)
	// npmVersion is a semantic version as a tarball name carries it.
	npmVersion = regexp.MustCompile(`^[0-9][0-9A-Za-z.+-]{0,127}$`)
)

// npmAbbreviated is the install metadata format npm itself asks for.
const npmAbbreviated = "application/vnd.npm.install-v1+json; q=1.0, application/json; q=0.8"

func (s *Server) parseNPM(rest string) (request, error) {
	if strings.HasPrefix(rest, "@") {
		// The scope's slash may arrive escaped; only that one.
		if i := strings.IndexByte(rest, '%'); i >= 0 && (strings.HasPrefix(rest[i:], "%2F") || strings.HasPrefix(rest[i:], "%2f")) && !strings.Contains(rest[:i], "/") {
			rest = rest[:i] + "/" + rest[i+3:]
		}
	}
	if strings.Contains(rest, "%") {
		return request{}, errRefused
	}
	segs := strings.Split(rest, "/")
	var name, base string
	switch {
	case len(segs) >= 2 && strings.HasPrefix(segs[0], "@"):
		if !npmComponent.MatchString(segs[0][1:]) || !npmComponent.MatchString(segs[1]) {
			return request{}, errRefused
		}
		name, base, segs = segs[0]+"/"+segs[1], segs[1], segs[2:]
	case len(segs) >= 1 && npmComponent.MatchString(segs[0]) && segs[0] != "-":
		name, base, segs = segs[0], segs[0], segs[1:]
	default:
		return request{}, errRefused
	}
	switch len(segs) {
	case 0:
		return request{eco: EcosystemNPM, kind: kindPassthrough, up: s.npm, upPath: npmDocPath(name), accept: npmAbbreviated, ctype: "application/json"}, nil
	case 2:
		file := segs[1]
		if segs[0] != "-" {
			return request{}, errRefused
		}
		ver, ok := strings.CutPrefix(file, base+"-")
		if ok {
			ver, ok = strings.CutSuffix(ver, ".tgz")
		}
		if !ok || !npmVersion.MatchString(ver) {
			return request{}, errRefused
		}
		rel := "npm/" + name + "/-/" + file
		return request{
			eco: EcosystemNPM, kind: kindArtifact, up: s.npm, upPath: "/" + name + "/-/" + file,
			ctype: "application/octet-stream", cacheRel: rel,
			expect: func(ctx context.Context) ([]expectedHash, error) { return s.npmIntegrity(ctx, name, ver, file) },
		}, nil
	}
	return request{}, errRefused
}

// npmDocPath is a packument's upstream path: a scoped name's slash escaped.
func npmDocPath(name string) string {
	if scope, pkg, ok := strings.Cut(name, "/"); ok {
		return "/" + scope + "%2F" + pkg
	}
	return "/" + name
}

// npmIntegrity reads the published integrity of name@ver from the
// packument, requiring that the version's tarball is this file. It prefers
// sha512; a version old enough to have only a sha1 shasum is checked
// against that, as npm itself does.
func (s *Server) npmIntegrity(ctx context.Context, name, ver, file string) ([]expectedHash, error) {
	what := name + "@" + ver
	ctx, cancel := context.WithTimeout(ctx, metaTimeout)
	defer cancel()
	resp, err := s.get(ctx, s.npm, npmDocPath(name), npmAbbreviated)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.ContentLength > s.metaCap {
		return nil, errTooLarge
	}
	var doc struct {
		Versions map[string]struct {
			Dist struct {
				Tarball   string `json:"tarball"`
				Integrity string `json:"integrity"`
				Shasum    string `json:"shasum"`
			} `json:"dist"`
		} `json:"versions"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, s.metaCap)).Decode(&doc); err != nil {
		return nil, &unverifiableError{what: what, why: "the packument does not decode"}
	}
	v, ok := doc.Versions[ver]
	if !ok {
		return nil, &unverifiableError{what: what, why: "the packument does not list this version"}
	}
	if u, err := url.Parse(v.Dist.Tarball); err != nil || path.Base(u.Path) != file {
		return nil, &unverifiableError{what: what, why: "the packument names another tarball"}
	}
	for _, field := range strings.Fields(v.Dist.Integrity) {
		if b64, ok := strings.CutPrefix(field, "sha512-"); ok {
			sum, err := base64.StdEncoding.DecodeString(b64)
			if err == nil && len(sum) == 64 {
				return []expectedHash{{algo: "sha512", sum: sum}}, nil
			}
		}
	}
	if sum, err := hex.DecodeString(v.Dist.Shasum); err == nil && len(sum) == 20 {
		return []expectedHash{{algo: "sha1", sum: sum}}, nil
	}
	return nil, &unverifiableError{what: what, why: "the packument publishes no sha512 integrity or sha1 shasum"}
}
