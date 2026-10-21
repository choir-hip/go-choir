package pkgmirror

import (
	"bufio"
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"regexp"
	"strings"
)

// crates: cargo's sparse registry protocol. The mirror writes its own
// config.json (downloads come back to it), passes index files through, and
// caches a .crate once it matches the index entry's cksum.
//
//	/crates/index/config.json                   the registry config
//	/crates/index/<1|2|3/c|ab/cd>/<name>         index file
//	/crates/dl/<name>/<version>/download         .crate
//
// A capsule's cargo uses it as a source replacement for crates-io:
// registry = "sparse+http://mirror.choir.internal/crates/index/".

var (
	crateIndexName = regexp.MustCompile(`^[a-z][a-z0-9_-]{0,63}$`)
	crateName      = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_-]{0,63}$`)
	crateVersion   = regexp.MustCompile(`^[0-9][0-9A-Za-z.+-]{0,127}$`)
)

// crateIndexPath is the sparse index path of a lowercase crate name.
func crateIndexPath(name string) string {
	switch len(name) {
	case 1:
		return "1/" + name
	case 2:
		return "2/" + name
	case 3:
		return "3/" + name[:1] + "/" + name
	}
	return name[:2] + "/" + name[2:4] + "/" + name
}

func (s *Server) parseCrates(rest string) (request, error) {
	if strings.Contains(rest, "%") {
		return request{}, errRefused
	}
	if p, ok := strings.CutPrefix(rest, "index/"); ok {
		if p == "config.json" {
			cfg, _ := json.Marshal(map[string]any{"dl": s.publicBase + "/crates/dl"})
			return request{eco: EcosystemCrates, kind: kindLocal, ctype: "application/json", local: cfg}, nil
		}
		name := p[strings.LastIndexByte(p, '/')+1:]
		if !crateIndexName.MatchString(name) || crateIndexPath(name) != p {
			return request{}, errRefused
		}
		return request{eco: EcosystemCrates, kind: kindPassthrough, up: s.cratesIndex, upPath: "/" + p, ctype: "text/plain; charset=utf-8"}, nil
	}
	if p, ok := strings.CutPrefix(rest, "dl/"); ok {
		segs := strings.Split(p, "/")
		if len(segs) != 3 || segs[2] != "download" || !crateName.MatchString(segs[0]) || !crateVersion.MatchString(segs[1]) {
			return request{}, errRefused
		}
		name, ver := segs[0], segs[1]
		return request{
			eco: EcosystemCrates, kind: kindArtifact, up: s.cratesDL,
			upPath: "/crates/" + name + "/" + name + "-" + ver + ".crate",
			ctype:  "application/gzip", cacheRel: "crates/" + name + "/" + ver + ".crate",
			expect: func(ctx context.Context) ([]expectedHash, error) { return s.crateChecksum(ctx, name, ver) },
		}, nil
	}
	return request{}, errRefused
}

// crateChecksum reads the cksum the index publishes for name@ver.
func (s *Server) crateChecksum(ctx context.Context, name, ver string) ([]expectedHash, error) {
	what := name + "@" + ver
	body, _, err := s.getMeta(ctx, s.cratesIndex, "/"+crateIndexPath(strings.ToLower(name)), "")
	if err != nil {
		return nil, err
	}
	sc := bufio.NewScanner(bytes.NewReader(body))
	sc.Buffer(make([]byte, 0, 64<<10), int(s.metaCap))
	for sc.Scan() {
		var entry struct {
			Name  string `json:"name"`
			Vers  string `json:"vers"`
			Cksum string `json:"cksum"`
		}
		if json.Unmarshal(sc.Bytes(), &entry) != nil || entry.Vers != ver || !strings.EqualFold(entry.Name, name) {
			continue
		}
		sum, err := hex.DecodeString(entry.Cksum)
		if err != nil || len(sum) != 32 {
			return nil, &unverifiableError{what: what, why: "the index entry's cksum is not a sha256"}
		}
		return []expectedHash{{algo: "sha256", sum: sum}}, nil
	}
	return nil, &unverifiableError{what: what, why: "the index does not list this version"}
}
