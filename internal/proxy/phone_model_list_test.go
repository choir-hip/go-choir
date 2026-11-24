package proxy

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/server"
)

// The phone fetches its model list here (owner, 2026-11-24: "we should be
// downloading the model list from the server"). Failure list:
//
//	M1 the route needs a sign-in, redirects or is wrapped in the HTML shell: a
//	   phone that is signed out (voice setup runs before sign-in) gets no list.
//	M2 the body is not the shape the phone decodes (version, models), or is empty.
//	M3 an entry the phone would drop as invalid is served: the phone skips it
//	   silently, so a typo here removes a model from every phone. Every served
//	   entry must pass the phone's own rules (Voice/ModelList.swift isValid).
//	M4 a method other than GET or HEAD changes or reads something.
//	M5 a neighbouring path serves the list.
//
//	M6 the app's model tables change and the served list is not regenerated:
//	   the equality test is Swift and runs only on a Mac, so CI would stay
//	   green. TestPhoneModelListSourcesUnchanged closes that here.
//	M7 a hand-edit serves an entry over the phone's size bounds (4 GiB a
//	   file, 8 GiB a model, 256 files): the phone drops it silently.
func TestPhoneModelListThroughRealChain(t *testing.T) {
	handler, _, _ := testProxyEnv(t)
	srv := server.NewServer("proxy-test", "0")
	RegisterRoutes(srv, handler)

	for _, method := range []string{http.MethodGet, http.MethodHead} {
		req := httptest.NewRequest(method, "https://choir.news/api/phone/model-list", nil)
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK { // M1
			t.Fatalf("%s status = %d, want 200 (location %q)", method, rec.Code, rec.Header().Get("Location"))
		}
		if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
			t.Fatalf("%s content type = %q", method, ct)
		}
		if cl := rec.Header().Get("Content-Length"); cl != strconv.Itoa(len(phoneModelListBody)) {
			t.Fatalf("%s content length = %q, want %d", method, cl, len(phoneModelListBody))
		}
		if cc := rec.Header().Get("Cache-Control"); cc != "public, max-age=300" {
			t.Fatalf("%s cache control = %q", method, cc)
		}
		if method == http.MethodHead {
			if rec.Body.Len() != 0 {
				t.Fatalf("HEAD wrote %d bytes", rec.Body.Len())
			}
			continue
		}
		checkPhoneModelList(t, rec.Body.Bytes())
	}

	for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodDelete} { // M4
		req := httptest.NewRequest(method, "/api/phone/model-list", strings.NewReader("{}"))
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)
		if rec.Code != http.StatusMethodNotAllowed || rec.Header().Get("Allow") != "GET, HEAD" {
			t.Fatalf("%s status = %d Allow = %q, want 405 and GET, HEAD", method, rec.Code, rec.Header().Get("Allow"))
		}
	}
	for _, path := range []string{"/api/phone/model-list/", "/api/phone/model-list/x", "/api/phone", "/api/phone/"} { // M5
		req := httptest.NewRequest(http.MethodGet, "https://choir.news"+path, nil)
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)
		if rec.Code == http.StatusOK || strings.Contains(rec.Body.String(), `"models"`) {
			t.Fatalf("%s answered %d or served the model list; only the exact path is public", path, rec.Code)
		}
	}
}

// The phone's rules for a served entry, rule for rule
// (mobile/AutoradioKit/Sources/AutoradioKit/Voice/ModelList.swift isValid and
// ModelStore.swift: isValidRepository, isSafeRelativePath, maxFileBytes,
// maxModelBytes, maxFiles). Nothing is omitted; change both together.
var (
	hex40      = regexp.MustCompile(`^[0-9a-f]{40}$`)
	hex64      = regexp.MustCompile(`^[0-9a-f]{64}$`)
	hfRepoPart = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)
)

const (
	phoneMaxFileBytes  = int64(4) << 30
	phoneMaxModelBytes = int64(8) << 30
	phoneMaxFiles      = 256
)

func phoneValidRepository(r string) bool {
	parts := strings.Split(r, "/")
	if len(parts) != 2 {
		return false
	}
	for _, p := range parts {
		if p == "." || p == ".." || !hfRepoPart.MatchString(p) {
			return false
		}
	}
	return true
}

func phoneSafeRelativePath(p string) bool {
	if p == "" || strings.HasPrefix(p, "/") || strings.ContainsAny(p, "\\\x00") {
		return false
	}
	for _, seg := range strings.Split(p, "/") {
		if seg == "" || seg == "." || seg == ".." {
			return false
		}
	}
	return true
}

// checkPhoneModelList applies the phone's rules for a served entry (M2, M3).
func checkPhoneModelList(t *testing.T, body []byte) {
	t.Helper()
	var list struct {
		Version *int `json:"version"`
		Models  []struct {
			ID         string  `json:"id"`
			Job        string  `json:"job"`
			Label      *string `json:"label"`
			Repository *string `json:"repository"`
			Revision   *string `json:"revision"`
			Files      []struct {
				Path   string `json:"path"`
				Bytes  int64  `json:"bytes"`
				SHA256 string `json:"sha256"`
			} `json:"files"`
			License        *string `json:"license"`
			RecommendedFor *[]struct {
				MachinePrefixes *[]string `json:"machinePrefixes"`
				MinMemoryGB     *float64  `json:"minMemoryGB"`
			} `json:"recommendedFor"`
			Rank *int `json:"rank"`
		} `json:"models"`
	}
	if err := json.Unmarshal(body, &list); err != nil {
		t.Fatalf("body is not the model list: %v", err)
	}
	if list.Version == nil || *list.Version < 1 || len(list.Models) == 0 {
		t.Fatalf("no version or no models")
	}
	jobs := map[string]bool{"vad": true, "transcriber": true, "voice": true, "decision": true, "responder": true}
	seen := map[string]bool{}
	for _, m := range list.Models {
		bad := func(why string) { t.Errorf("model %q: %s (the phone would drop it)", m.ID, why) }
		// One model may serve two jobs (the same id under each), never the same job twice.
		if m.ID == "" || seen[m.Job+"/"+m.ID] {
			bad("empty id, or repeated within its job")
		}
		seen[m.Job+"/"+m.ID] = true
		if !jobs[m.Job] {
			bad("unknown job " + m.Job)
		}
		// Fields the phone's decoder requires: a missing one fails the entry.
		if m.Label == nil || m.Repository == nil || m.License == nil || m.RecommendedFor == nil || m.Rank == nil {
			bad("a required field is missing")
			continue
		}
		for _, r := range *m.RecommendedFor {
			if r.MachinePrefixes == nil || r.MinMemoryGB == nil {
				bad("a device rule lacks machinePrefixes or minMemoryGB")
			}
		}
		if m.Revision != nil && !hex40.MatchString(*m.Revision) {
			bad("revision is not 40 lower-case hex digits")
		}
		if len(m.Files) == 0 {
			continue
		}
		if m.Revision == nil {
			bad("files without a revision")
		}
		if !phoneValidRepository(*m.Repository) {
			bad("repository is not owner/name")
		}
		if len(m.Files) > phoneMaxFiles {
			bad("more than 256 files")
		}
		paths := map[string]bool{}
		var total int64
		for _, f := range m.Files {
			if paths[f.Path] {
				bad("path listed twice: " + f.Path)
			}
			paths[f.Path] = true
			if !phoneSafeRelativePath(f.Path) {
				bad("unsafe path " + f.Path)
			}
			if f.Bytes <= 0 || f.Bytes > phoneMaxFileBytes || !hex64.MatchString(f.SHA256) {
				bad("file " + f.Path + " has no size, is over 4 GiB or has a bad digest")
			}
			total += f.Bytes
		}
		if total > phoneMaxModelBytes {
			bad("more than 8 GiB of files")
		}
	}
}

// M6. The served list is the app's built-in list written out by the Swift
// test (ModelListTests.testServedListIsTheBuiltInList with
// AR_WRITE_MODEL_LIST=1), which also records the SHA-256 of every Swift file
// the list is built from. That test runs only on a Mac; this one runs in CI
// and fails when any of those files has changed since the list was written,
// so a change to the app's model tables cannot land without regenerating.
// A comment-only edit to one of them also asks for a regeneration: that is
// the price of a check that needs no Swift.
func TestPhoneModelListSourcesUnchanged(t *testing.T) {
	raw, err := os.ReadFile("phone_model_list.sources.json")
	if err != nil {
		t.Fatalf("no record of the sources the served list was built from: %v", err)
	}
	var rec struct {
		ListSHA256 string            `json:"list_sha256"`
		Sources    map[string]string `json:"sources"`
	}
	if err := json.Unmarshal(raw, &rec); err != nil {
		t.Fatal(err)
	}
	sum := func(b []byte) string { h := sha256.Sum256(b); return hex.EncodeToString(h[:]) }
	if got := sum(phoneModelListBody); got != rec.ListSHA256 {
		t.Errorf("phone_model_list.json was edited after it was generated (sha256 %s, recorded %s): regenerate it from the app's tables", got, rec.ListSHA256)
	}
	for _, want := range []string{"KnownModels.swift", "ModelCandidates.swift", "ModelStore.swift", "Voice/ModelList.swift"} {
		if _, ok := rec.Sources[want]; !ok {
			t.Errorf("the record does not cover %s", want)
		}
	}
	for name, want := range rec.Sources {
		if !phoneSafeRelativePath(name) {
			t.Fatalf("unsafe source name %q", name)
		}
		b, err := os.ReadFile("../../mobile/AutoradioKit/Sources/AutoradioKit/" + name)
		if err != nil {
			t.Errorf("%s: %v", name, err)
			continue
		}
		if got := sum(b); got != want {
			t.Errorf("%s changed since the served model list was written: rerun ModelListTests with AR_WRITE_MODEL_LIST=1 (through the Swift slot) and commit phone_model_list.json and phone_model_list.sources.json", name)
		}
	}
}
