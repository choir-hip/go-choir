package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/tapestore"
)

func TestParseCompsizeTotal(t *testing.T) {
	out := "Processed 830 files, 5148116 regular extents (5148116 refs), 1 inline, 1262245 fragments.\n" +
		"Type       Perc     Disk Usage   Uncompressed Referenced  \n" +
		"TOTAL      100%     50459174260  50459174260  29383652724 \n" +
		"none       100%     50459174260  50459174260  29383652724 \n"
	if n, err := parseCompsizeTotal(out); err != nil || n != 50459174260 {
		t.Fatalf("%d %v", n, err)
	}
	if _, err := parseCompsizeTotal("nothing"); err == nil {
		t.Fatal("parsed garbage")
	}
}

func TestCompactSubcommand(t *testing.T) {
	root := t.TempDir()
	a := tapestore.NewAppender(root)
	for i := 0; i < 3; i++ {
		if err := a.Append("computer-a", tapestore.Item{NS: tapestore.NSEvent, Body: []byte(strings.Repeat("e", 10+i))}); err != nil {
			t.Fatal(err)
		}
		if err := a.Seal("computer-a"); err != nil {
			t.Fatal(err)
		}
	}
	if code, _, _ := tool("compact"); code != 2 {
		t.Fatalf("no --root: %d", code)
	}
	if code, _, e := tool("compact", "--root", root, "--apply"); code != 1 || !strings.Contains(e, "--computer") {
		t.Fatalf("apply unscoped: %d %s", code, e)
	}
	code, out, e := tool("compact", "--root", root, "--min-avg-extent", "-1", "--quiet")
	if code != 0 {
		t.Fatalf("dry run: %d %s", code, e)
	}
	var rep struct {
		Segments     int `json:"sealed_segments"`
		WouldCompact int `json:"would_compact"`
		Compacted    int `json:"compacted"`
	}
	if err := json.Unmarshal([]byte(out), &rep); err != nil || rep.Segments != 3 || rep.WouldCompact != 3 || rep.Compacted != 0 {
		t.Fatalf("dry run report %s %v", out, err)
	}
	code, out, e = tool("compact", "--root", root, "--computer", "computer-a", "--apply", "--min-avg-extent", "-1", "--pause", "1ns", "--quiet")
	if code != 0 {
		t.Fatalf("apply: %d %s", code, e)
	}
	if err := json.Unmarshal([]byte(out), &rep); err != nil || rep.Compacted != 3 {
		t.Fatalf("apply report %s %v", out, err)
	}
}
