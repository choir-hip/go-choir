package main

// Failure modes of the harness command (MA7.9), written before the code:
//  C1 it reads a computer other than the one named (an owner-wide key
//     answering for the wrong computer, such as the paper);
//  C2 the key reaches an artifact, stdout or an error;
//  C3 the stored inputs do not reproduce the stored report byte for byte,
//     or an edited report passes the check;
//  C4 a field the computer's build did not serve is stored as zero.

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const testKey = "choir_sk_testsecret"

func fakeComputer(t *testing.T, answerAs string, serveUnits bool) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer "+testKey || r.Header.Get("X-Choir-Computer") == "" {
			http.Error(w, `{"error":"no"}`, http.StatusUnauthorized)
			return
		}
		var body any
		switch r.URL.Path {
		case "/api/runtime/observability":
			body = map[string]any{"computer_id": answerAs, "build": map[string]any{"deployed_commit": "abc"}, "act_gate": map[string]any{"counts": map[string]int{}}}
		case "/api/trust":
			b := map[string]any{"computer_id": answerAs, "view": map[string]any{"rows": nil},
				"replay":      map[string]any{"pairs": 0, "insufficient": true, "candidates": []any{}, "grid": []any{}, "chosen": map[string]any{"mode": "capped"}},
				"gate_replay": map[string]any{"acts": 0, "gated": 0, "breaker": 0, "days": 1, "taps_per_day": 0}}
			if serveUnits {
				b["prereg_units"] = map[string]any{"window": "336h0m0s", "items": 0, "material_items": 0, "cutoffs": []any{}, "dropped_no_act": 0, "dropped_open": 0, "units": []any{}}
			}
			body = b
		case "/api/subcalls":
			body = map[string]any{"computer_id": answerAs, "summary": map[string]any{}, "inert": map[string]any{"rates": []any{}, "judge_agreement": map[string]any{}}}
		case "/api/leases":
			body = map[string]any{"leases": []any{}}
		default:
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(body)
	}))
}

func setup(t *testing.T) (dir, keyFile, ledger string) {
	dir = t.TempDir()
	keyFile = filepath.Join(dir, "key")
	ledger = filepath.Join(dir, "ledger.md")
	if err := os.WriteFile(keyFile, []byte(testKey+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(ledger, []byte("**R0-01.** x\n- p = 0.5.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return
}

func TestRefusesAnotherComputer(t *testing.T) {
	srv := fakeComputer(t, "computer-paper", true)
	defer srv.Close()
	dir, key, ledger := setup(t)
	var out bytes.Buffer
	err := run([]string{"-base", srv.URL, "-key-file", key, "-computer", "computer-dande000", "-ledger", ledger, "-transcription", "", "-out", filepath.Join(dir, "o")}, &out)
	if err == nil || !strings.Contains(err.Error(), "not the named") {
		t.Fatalf("C1: %v", err)
	}
	if strings.Contains(err.Error(), testKey) {
		t.Fatal("C2: the key reached an error")
	}
	if _, statErr := os.Stat(filepath.Join(dir, "o", "inputs.json")); statErr == nil {
		t.Fatal("C1: inputs were written for another computer")
	}
	if err := run([]string{"-base", srv.URL, "-key-file", key, "-ledger", ledger}, &out); err == nil {
		t.Fatal("C1: ran with no computer named")
	}
}

func TestFetchStoresAndReproduces(t *testing.T) {
	for _, serve := range []bool{true, false} {
		srv := fakeComputer(t, "computer-dande000", serve)
		dir, key, ledger := setup(t)
		o := filepath.Join(dir, "o")
		var out bytes.Buffer
		if err := run([]string{"-base", srv.URL, "-key-file", key, "-computer", "computer-dande000", "-ledger", ledger, "-transcription", "", "-out", o}, &out); err != nil {
			t.Fatal(err)
		}
		srv.Close()
		for _, name := range []string{"inputs.json", "report.json", "report.md"} {
			b, err := os.ReadFile(filepath.Join(o, name))
			if err != nil {
				t.Fatal(err)
			}
			if bytes.Contains(b, []byte(testKey)) {
				t.Fatalf("C2: the key reached %s", name)
			}
		}
		if strings.Contains(out.String(), testKey) {
			t.Fatal("C2: the key reached stdout")
		}
		rep, _ := os.ReadFile(filepath.Join(o, "report.json"))
		if !serve && !bytes.Contains(rep, []byte("served no prereg_units")) {
			t.Fatal("C4: an unserved field was not reported as unserved")
		}
		if err := run([]string{"-from", o, "-check"}, &out); err != nil {
			t.Fatalf("C3: %v", err)
		}
		if err := os.WriteFile(filepath.Join(o, "report.md"), append(mustRead(t, filepath.Join(o, "report.md")), 'x'), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := run([]string{"-from", o, "-check"}, &out); err == nil {
			t.Fatal("C3: an edited report passed the check")
		}
	}
}

func mustRead(t *testing.T, p string) []byte {
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
