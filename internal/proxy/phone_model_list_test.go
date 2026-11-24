package proxy

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
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
// That the served list equals the list this build of the app carries is the
// phone side's test (ModelListTests.testServedListIsTheBuiltInList).
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
		if strings.Contains(rec.Body.String(), `"models"`) {
			t.Fatalf("%s served the model list (status %d)", path, rec.Code)
		}
	}
}

var (
	hex40   = regexp.MustCompile(`^[0-9a-f]{40}$`)
	hex64   = regexp.MustCompile(`^[0-9a-f]{64}$`)
	hfRepo  = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*/[A-Za-z0-9][A-Za-z0-9._-]*$`)
	pathSeg = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)
)

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
		if !hfRepo.MatchString(*m.Repository) {
			bad("repository is not owner/name")
		}
		paths := map[string]bool{}
		var total int64
		for _, f := range m.Files {
			if paths[f.Path] {
				bad("path listed twice: " + f.Path)
			}
			paths[f.Path] = true
			for _, seg := range strings.Split(f.Path, "/") {
				if !pathSeg.MatchString(seg) || seg == "." || seg == ".." {
					bad("unsafe path " + f.Path)
				}
			}
			if f.Bytes <= 0 || !hex64.MatchString(f.SHA256) {
				bad("file " + f.Path + " has no size or a bad digest")
			}
			total += f.Bytes
		}
		if total > 8<<30 {
			bad("more than 8 GiB of files")
		}
	}
}
