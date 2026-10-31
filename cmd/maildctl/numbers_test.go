package main

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestRunReserveAndBindTestNumber(t *testing.T) {
	dbPath, storageRoot := setupMaildctlStore(t)
	base := []string{"--db", dbPath, "--storage-root", storageRoot}
	var stdout, stderr bytes.Buffer
	if code := run(append([]string{"reserve-test-number"}, base...), &stdout, &stderr); code != 0 {
		t.Fatalf("reserve code=%d stderr=%s", code, stderr.String())
	}
	var res struct {
		Address   string `json:"address"`
		ExpiresAt string `json:"expires_at"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &res); err != nil {
		t.Fatalf("decode reserve: %v", err)
	}
	if res.Address != "10000000@choir.news" || res.ExpiresAt == "" {
		t.Fatalf("reserve = %+v", res)
	}
	bind := func(owner string) (int, string) {
		var o, e bytes.Buffer
		code := run(append([]string{"bind-number", "--address", res.Address, "--owner", owner}, base...), &o, &e)
		return code, e.String()
	}
	if code, msg := bind("acct-a"); code != 0 {
		t.Fatalf("bind code=%d %s", code, msg)
	}
	if code, msg := bind("acct-a"); code != 0 {
		t.Fatalf("double bind code=%d %s", code, msg)
	}
	if code, _ := bind("acct-b"); code == 0 {
		t.Fatal("second account bound the same number")
	}
	stdout.Reset()
	stderr.Reset()
	if code := run(append([]string{"bind-number", "--owner", "x"}, base...), &stdout, &stderr); code != 2 {
		t.Fatalf("missing --address code=%d", code)
	}
}
