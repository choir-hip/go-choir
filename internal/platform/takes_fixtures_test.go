package platform

// The phone's fixtures (mobile/fixtures/takes) and what this server writes
// must not drift. Every response fixture is compared with what the server's own
// encoder produces for the same flow: each key the fixture carries must exist
// on the server's answer with the same JSON type (the server may carry more;
// the phone ignores keys it does not read). The only differences allowed are
// named below, and the test fails if one of them stops being a difference.
//
// Known differences (reported to the lead, neither side bent):
//   publish.json carries "publisher_key", which the server never answers
//     (the publisher key is private; the phone does not read it).
//   refused-daily.json says "invalid take: 10 takes in the last 24 hours, the
//     limit is 10"; the server says "daily take limit reached: 10 takes in the
//     last 24 hours, the limit is 10" (status 429 both; the phone shows the
//     words plainly).

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"sort"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/types"
)

func readTakeFixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile("../../mobile/fixtures/takes/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func jsonKind(v any) string {
	switch v.(type) {
	case nil:
		return "null"
	case map[string]any:
		return "object"
	case []any:
		return "array"
	case string:
		return "string"
	case float64:
		return "number"
	case bool:
		return "bool"
	}
	return "?"
}

// fixtureGaps lists every key of fixture that the server's answer lacks or
// types differently. Arrays are compared by their first element.
func fixtureGaps(fixture, server any, path string) []string {
	if jsonKind(fixture) != jsonKind(server) {
		return []string{fmt.Sprintf("%s: fixture %s, server %s", path, jsonKind(fixture), jsonKind(server))}
	}
	var gaps []string
	switch f := fixture.(type) {
	case map[string]any:
		sv := server.(map[string]any)
		keys := make([]string, 0, len(f))
		for k := range f {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			if _, ok := sv[k]; !ok {
				gaps = append(gaps, path+"."+k+": the server does not answer it")
				continue
			}
			gaps = append(gaps, fixtureGaps(f[k], sv[k], path+"."+k)...)
		}
	case []any:
		sv := server.([]any)
		if len(f) > 0 && len(sv) > 0 {
			gaps = append(gaps, fixtureGaps(f[0], sv[0], path+"[0]")...)
		} else if len(f) > 0 {
			gaps = append(gaps, path+": the server answered no element")
		}
	}
	return gaps
}

func decodeAny(t *testing.T, raw []byte) any {
	t.Helper()
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		t.Fatalf("not json: %v: %s", err, raw)
	}
	return v
}

func TestPhoneFixturesAgreeWithWhatTheServerWrites(t *testing.T) {
	svc := newTakesService(t)
	h := NewHandler(svc)
	take := mustPublish(t, svc, "u1", 0)
	for i, l := range []string{"l1", "l2"} {
		verdict := TakeHeard
		if i == 1 {
			verdict = TakeSkipped
		}
		if _, err := outcome(t, svc, take.TakeID, l, "computer-"+strings.Repeat(string(rune('a'+i)), 32), verdict); err != nil {
			t.Fatal(err)
		}
	}
	serve := func(fn func(http.ResponseWriter, *http.Request), path string) []byte {
		w := httptest.NewRecorder()
		fn(w, hostReq(http.MethodGet, path, nil))
		if w.Code != http.StatusOK {
			t.Fatalf("%s = %d %s", path, w.Code, w.Body)
		}
		return w.Body.Bytes()
	}
	published := takesJSON(t, take)
	list := serve(h.HandleTakes, TakesPath)
	mine := serve(h.HandleTakes, TakesPath+"?publisher_key="+types.TakePublisherKey("u1"))
	record := serve(h.HandleTakeRecord, TakesRecordPath+"?publisher="+take.Publisher+"&as="+types.TakePublisherKey("u1"))

	for _, c := range []struct {
		file   string
		server []byte
		known  []string
	}{
		{"list.json", list, nil},
		{"mine.json", mine, nil},
		{"record.json", record, nil},
		{"publish.json", published, []string{".publisher_key: the server does not answer it"}},
	} {
		gaps := fixtureGaps(decodeAny(t, readTakeFixture(t, c.file)), decodeAny(t, c.server), "")
		if strings.Join(gaps, "\n") != strings.Join(c.known, "\n") {
			t.Errorf("%s: differences from the server's own answer:\n  got  %q\n  want %q", c.file, gaps, c.known)
		}
	}

	// The daily refusal: same status, the server's own words.
	var fx struct {
		Error string `json:"error"`
	}
	_ = json.Unmarshal(readTakeFixture(t, "refused-daily.json"), &fx)
	w := httptest.NewRecorder()
	takeFail(w, "publish", fmt.Errorf("%w: %d takes in the last 24 hours, the limit is %d", ErrTakeDailyLimit, 10, TakeDailyLimit))
	var got struct {
		Error string `json:"error"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &got)
	if w.Code != http.StatusTooManyRequests || strings.TrimPrefix(got.Error, "daily take limit reached") != strings.TrimPrefix(fx.Error, "invalid take") {
		t.Errorf("refused-daily: status %d %q, fixture %q", w.Code, got.Error, fx.Error)
	}

	// The boundary list: takes at the header's edges, every one inside the
	// bounds publish and the programme route share.
	var edge TakesResponse
	if err := json.Unmarshal(readTakeFixture(t, "list-boundary.json"), &edge); err != nil || len(edge.Takes) != 3 {
		t.Fatalf("list-boundary.json: %d takes, %v", len(edge.Takes), err)
	}
	for _, tk := range edge.Takes {
		if tk.DurationSeconds < types.TakeHeaderMinSeconds || tk.DurationSeconds > types.TakeHeaderMaxSeconds || !types.TakeIDRe.MatchString(tk.TakeID) {
			t.Errorf("list-boundary.json: %s at %v s is outside the header bounds", tk.TakeID, tk.DurationSeconds)
		}
	}
	if gaps := fixtureGaps(decodeAny(t, readTakeFixture(t, "list-boundary.json")), decodeAny(t, list), ""); len(gaps) != 0 {
		t.Errorf("list-boundary.json differs from the server's list: %q", gaps)
	}

	// The request examples are in routes.json; internal/proxy/takes_test.go
	// sends each one through the registered route.
}
