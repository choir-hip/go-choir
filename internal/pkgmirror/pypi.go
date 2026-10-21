package pkgmirror

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html"
	"net/http"
	"regexp"
	"strings"
)

// PyPI: the simple repository API (PEP 503/691) and the package files it
// links. The index is fetched as PEP 691 JSON and rewritten so every link
// names this mirror; a file that lives anywhere but files.pythonhosted.org's
// content-addressed tree, or has no sha256, is dropped. Files are cached
// once their bytes match the BLAKE2b-256 their path is made of.
//
//	/pypi/simple/<project>/                                   index
//	/pypi/packages/<b2[0:2]>/<b2[2:4]>/<b2[4:]>/<filename>    file
//	/pypi/packages/.../<filename>.metadata                    PEP 658 metadata (passed through)

// pypiFilesCanonical is where pypi.org's index links files.
const pypiFilesCanonical = "https://files.pythonhosted.org/packages/"

const (
	pypiJSON    = "application/vnd.pypi.simple.v1+json"
	pypiHTML    = "text/html; charset=utf-8"
	pypiVersion = "1.1"
)

var (
	pypiProject  = regexp.MustCompile(`^[A-Za-z0-9]([A-Za-z0-9._-]{0,198}[A-Za-z0-9])?$`)
	pypiSep      = regexp.MustCompile(`[-_.]+`)
	pypiFilename = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._+!~-]{0,254}$`)
	hex2         = regexp.MustCompile(`^[0-9a-f]{2}$`)
	hex60        = regexp.MustCompile(`^[0-9a-f]{60}$`)
	hex64        = regexp.MustCompile(`^[0-9a-f]{64}$`)
)

func (s *Server) parsePyPI(rest string) (request, error) {
	if strings.Contains(rest, "%") {
		return request{}, errRefused
	}
	if name, ok := strings.CutPrefix(rest, "simple/"); ok {
		name = strings.TrimSuffix(name, "/")
		if !pypiProject.MatchString(name) {
			return request{}, errRefused
		}
		norm := pypiSep.ReplaceAllString(strings.ToLower(name), "-")
		return request{eco: EcosystemPyPI, kind: kindPyPIIndex, up: s.pypiIndex, upPath: "/simple/" + norm + "/", accept: pypiJSON}, nil
	}
	if p, ok := strings.CutPrefix(rest, "packages/"); ok {
		digest, filename, ok := parsePyPIFilePath(p)
		if !ok {
			return request{}, errRefused
		}
		upPath := "/packages/" + p
		if strings.HasSuffix(filename, ".metadata") {
			// Its hash is in the index, not the path; the client checks it.
			return request{eco: EcosystemPyPI, kind: kindPassthrough, up: s.pypiFiles, upPath: upPath, ctype: "text/plain; charset=utf-8"}, nil
		}
		return request{
			eco: EcosystemPyPI, kind: kindArtifact, up: s.pypiFiles, upPath: upPath,
			ctype: "application/octet-stream", cacheRel: "pypi/packages/" + p,
			expect: func(context.Context) ([]expectedHash, error) {
				return []expectedHash{{algo: "blake2b-256", sum: digest}}, nil
			},
		}, nil
	}
	return request{}, errRefused
}

// parsePyPIFilePath checks aa/bb/<60 hex>/<filename> and returns the
// BLAKE2b-256 digest the three directories spell.
func parsePyPIFilePath(p string) ([]byte, string, bool) {
	segs := strings.Split(p, "/")
	if len(segs) != 4 || !hex2.MatchString(segs[0]) || !hex2.MatchString(segs[1]) || !hex60.MatchString(segs[2]) || !pypiFilename.MatchString(segs[3]) {
		return nil, "", false
	}
	digest, err := hex.DecodeString(segs[0] + segs[1] + segs[2])
	if err != nil {
		return nil, "", false
	}
	return digest, segs[3], true
}

// pypiFile is the part of a PEP 691 file entry the mirror reads; the rest of
// the entry is kept as it came.
type pypiFile struct {
	Filename       string            `json:"filename"`
	URL            string            `json:"url"`
	Hashes         map[string]string `json:"hashes"`
	RequiresPython string            `json:"requires-python"`
	Yanked         any               `json:"yanked"`
	CoreMetadata   any               `json:"core-metadata"`
	DistInfoMeta   any               `json:"dist-info-metadata"`
}

func (s *Server) servePyPIIndex(w http.ResponseWriter, r *http.Request, req request, rec *Record, finish finishFunc) {
	release, err := s.acquire(r.Context(), rec.Source)
	if err != nil {
		http.Error(w, "request cancelled", http.StatusServiceUnavailable)
		finish(OutcomeUpstreamError, http.StatusServiceUnavailable, 0, err.Error())
		return
	}
	body, hdr, err := s.getMeta(r.Context(), req.up, req.upPath, req.accept)
	release()
	if err == nil && !strings.HasPrefix(hdr.Get("Content-Type"), pypiJSON) {
		err = fmt.Errorf("upstream index is %q, not PEP 691 JSON", hdr.Get("Content-Type"))
	}
	var out []byte
	var ctype string
	if err == nil {
		out, ctype, err = s.rewritePyPIIndex(body, strings.Contains(r.Header.Get("Accept"), pypiJSON))
	}
	if err != nil {
		status, outcome, msg := failure(err)
		http.Error(w, msg, status)
		finish(outcome, status, 0, err.Error())
		return
	}
	w.Header().Set("Content-Type", ctype)
	w.Header().Set("Content-Length", fmt.Sprint(len(out)))
	w.WriteHeader(http.StatusOK)
	if r.Method == http.MethodGet {
		_, _ = w.Write(out)
	}
	finish(OutcomePassthrough, http.StatusOK, int64(len(out)), "")
}

// rewritePyPIIndex keeps the files that live in files.pythonhosted.org's
// content-addressed tree under their own name and carry a sha256, points
// each at this mirror, and renders JSON or PEP 503 HTML.
func (s *Server) rewritePyPIIndex(body []byte, asJSON bool) ([]byte, string, error) {
	var doc map[string]json.RawMessage
	if err := json.Unmarshal(body, &doc); err != nil {
		return nil, "", fmt.Errorf("upstream index does not decode: %w", err)
	}
	var raws []map[string]json.RawMessage
	if err := json.Unmarshal(doc["files"], &raws); err != nil {
		return nil, "", fmt.Errorf("upstream index files do not decode: %w", err)
	}
	var name string
	_ = json.Unmarshal(doc["name"], &name)
	keptRaw := []map[string]json.RawMessage{}
	var kept []pypiFile
	for _, raw := range raws {
		b, _ := json.Marshal(raw)
		var f pypiFile
		if json.Unmarshal(b, &f) != nil {
			continue
		}
		p, ok := strings.CutPrefix(f.URL, pypiFilesCanonical)
		if !ok {
			continue
		}
		if _, filename, ok := parsePyPIFilePath(p); !ok || filename != f.Filename || strings.HasSuffix(filename, ".metadata") {
			continue
		}
		sha := f.Hashes["sha256"]
		if !hex64.MatchString(sha) {
			continue
		}
		f.URL = s.publicBase + "/pypi/packages/" + p
		f.Hashes = map[string]string{"sha256": sha}
		raw["url"], _ = json.Marshal(f.URL)
		raw["hashes"], _ = json.Marshal(f.Hashes)
		keptRaw = append(keptRaw, raw)
		kept = append(kept, f)
	}
	if asJSON {
		doc["files"], _ = json.Marshal(keptRaw)
		out, err := json.Marshal(doc)
		return out, pypiJSON, err
	}
	var b strings.Builder
	title := html.EscapeString(name)
	fmt.Fprintf(&b, "<!DOCTYPE html>\n<html><head><meta name=\"pypi:repository-version\" content=\"%s\"><title>Links for %s</title></head><body><h1>Links for %s</h1>\n", pypiVersion, title, title)
	for _, f := range kept {
		fmt.Fprintf(&b, `<a href="%s#sha256=%s"`, html.EscapeString(f.URL), f.Hashes["sha256"])
		if f.RequiresPython != "" {
			fmt.Fprintf(&b, ` data-requires-python="%s"`, html.EscapeString(f.RequiresPython))
		}
		switch y := f.Yanked.(type) {
		case bool:
			if y {
				b.WriteString(` data-yanked=""`)
			}
		case string:
			fmt.Fprintf(&b, ` data-yanked="%s"`, html.EscapeString(y))
		}
		meta := f.CoreMetadata
		if meta == nil {
			meta = f.DistInfoMeta
		}
		switch m := meta.(type) {
		case bool:
			if m {
				b.WriteString(` data-core-metadata="true" data-dist-info-metadata="true"`)
			}
		case map[string]any:
			if sha, _ := m["sha256"].(string); hex64.MatchString(sha) {
				fmt.Fprintf(&b, ` data-core-metadata="sha256=%s" data-dist-info-metadata="sha256=%s"`, sha, sha)
			}
		}
		fmt.Fprintf(&b, ">%s</a><br>\n", html.EscapeString(f.Filename))
	}
	b.WriteString("</body></html>\n")
	return []byte(b.String()), pypiHTML, nil
}
