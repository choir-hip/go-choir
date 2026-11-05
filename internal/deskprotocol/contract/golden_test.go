package contract

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/deskprotocol"
	"github.com/yusefmosiah/go-choir/internal/release"
	"github.com/yusefmosiah/go-choir/internal/types"
	"github.com/yusefmosiah/go-choir/internal/yaegikernel"
)

type fixture struct {
	Type string          `json:"type"`
	Case string          `json:"case"`
	Body json.RawMessage `json:"body"`
}

var decoders = map[string]func() any{
	"yaegikernel.StagedIntent":   func() any { return &yaegikernel.StagedIntent{} },
	"types.CommitmentRecord":     func() any { return &types.CommitmentRecord{} },
	"types.WorkItemRecord":       func() any { return &types.WorkItemRecord{} },
	"types.TrajectoryRecord":     func() any { return &types.TrajectoryRecord{} },
	"release.Precommitment":      func() any { return &release.Precommitment{} },
	"yaegikernel.BrokerRequest":  func() any { return &yaegikernel.BrokerRequest{} },
	"yaegikernel.BrokerResponse": func() any { return &yaegikernel.BrokerResponse{} },
	"yaegikernel.InboxRecord":    func() any { return &yaegikernel.InboxRecord{} },
	"yaegikernel.PendingUpdate":  func() any { return &yaegikernel.PendingUpdate{} },
}

func fixtures(t *testing.T) []fixture {
	t.Helper()
	dir := filepath.Join(repoRoot(t), "internal", "deskprotocol", "testdata", "v1.0.0")
	files, err := filepath.Glob(filepath.Join(dir, "*.json"))
	if err != nil || len(files) < 20 {
		t.Fatalf("golden corpus has %d files in %s (%v)", len(files), dir, err)
	}
	var out []fixture
	for _, f := range files {
		raw, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		var fx fixture
		if err := json.Unmarshal(raw, &fx); err != nil {
			t.Fatalf("%s: %v", f, err)
		}
		if want := fx.Type + "." + fx.Case + ".json"; filepath.Base(f) != want {
			t.Fatalf("%s should be named %s", f, want)
		}
		out = append(out, fx)
	}
	return out
}

func normalize(t *testing.T, raw []byte) any {
	t.Helper()
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		t.Fatal(err)
	}
	return v
}

// Every build must decode every frozen body of a released version: this is
// the deploy-skew test, new code reading old tape. Decoding is strict (an
// unknown field fails) and the decoded value re-encodes to the same JSON, so
// a removed or renamed field is seen, not absorbed.
func TestGoldenCorpusDecodesStrictlyAndRoundTrips(t *testing.T) {
	for _, fx := range fixtures(t) {
		t.Run(fx.Type+"."+fx.Case, func(t *testing.T) {
			mk, ok := decoders[fx.Type]
			if !ok {
				t.Fatalf("no decoder for %s", fx.Type)
			}
			v := mk()
			dec := json.NewDecoder(bytes.NewReader(fx.Body))
			dec.DisallowUnknownFields()
			if err := dec.Decode(v); err != nil {
				t.Fatalf("current build cannot decode a v%s body: %v", deskprotocol.Version, err)
			}
			again, err := json.Marshal(v)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(normalize(t, fx.Body), normalize(t, again)) {
				t.Fatalf("round trip changed the body:\n was %s\n now %s", fx.Body, again)
			}
		})
	}
}

// The decoder of the locked version (the lock's field set) accepts everything
// the current build emits, and the current build's types accept everything the
// locked version emitted: skew within a major in both directions.
func TestSkewWithinAMajor(t *testing.T) {
	lk := lock(t)
	for _, fx := range fixtures(t) {
		if probs := CheckShape(lk, fx.Type, fx.Body); len(probs) > 0 {
			t.Errorf("%s.%s outside the lock: %s", fx.Type, fx.Case, strings.Join(probs, "; "))
		}
		// Re-emit through the current type; the old decoder (the lock) must
		// accept what the new code writes.
		v := decoders[fx.Type]()
		if err := json.Unmarshal(fx.Body, v); err != nil {
			t.Fatal(err)
		}
		out, _ := json.Marshal(v)
		if probs := CheckShape(lk, fx.Type, out); len(probs) > 0 {
			t.Errorf("%s.%s re-emitted by this build is outside the lock: %s", fx.Type, fx.Case, strings.Join(probs, "; "))
		}
	}
	// An older reader ignores an optional field it does not know: a v1.0.x
	// record with a later optional field still decodes under the current
	// struct.
	var rec types.CommitmentRecord
	if err := json.Unmarshal([]byte(`{"schema_id":"commitment_record.v1","record_id":"r","a_later_optional_field":1}`), &rec); err != nil {
		t.Fatalf("a record with an unknown optional field must decode: %v", err)
	}
}

// Fixtures are built from constructors in tests. Nothing in them looks like
// an email address, a credential or a production identifier.
func TestFixturesCarryNoProductionData(t *testing.T) {
	bad := []*regexp.Regexp{
		regexp.MustCompile(`[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[a-z]{2,}`),
		regexp.MustCompile(`(?i)bearer\s+[a-z0-9._-]{16,}`),
		regexp.MustCompile(`sk-[A-Za-z0-9]{16,}`),
		regexp.MustCompile(`[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`),
	}
	for _, fx := range fixtures(t) {
		for _, re := range bad {
			if m := re.Find(fx.Body); m != nil {
				t.Errorf("%s.%s contains %q", fx.Type, fx.Case, m)
			}
		}
	}
}

// The corpus covers the PSA records: every record kind and directive subtype
// the lock lists as a closed enum is represented in a fixture, so a decoder
// change cannot pass without reading one of each.
func TestCorpusCoversEveryCommitmentKind(t *testing.T) {
	lk := lock(t)
	seen := map[string]bool{}
	for _, fx := range fixtures(t) {
		if fx.Type != "types.CommitmentRecord" {
			continue
		}
		var rec types.CommitmentRecord
		_ = json.Unmarshal(fx.Body, &rec)
		seen[string(rec.Kind)] = true
		if rec.Directive != nil {
			seen["directive:"+string(rec.Directive.Subtype)] = true
		}
	}
	for _, lit := range lk.Names["types.CommitmentRecordKind"].Values {
		if k := strings.Trim(lit, `"`); !seen[k] {
			t.Errorf("no fixture of record kind %q", k)
		}
	}
	for _, lit := range lk.Names["types.CommitmentDirectiveSubtype"].Values {
		if k := strings.Trim(lit, `"`); !seen["directive:"+k] {
			t.Errorf("no fixture of directive subtype %q", k)
		}
	}
}
