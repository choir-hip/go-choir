package yaegikernel

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/texturewrite"
)

// Texture reads the captures its frame names
// (problems/texture-opens-stories-blind-to-their-capture-2026-10-16.md;
// autopaper-app-design §4.4 slice C). Ways it can fail, written before the
// code:
//
//   - Texture's surface or handle still lacks ReadCapture, so the cell is
//     told "There is no choir.ReadCapture for this desk";
//   - a frame handle (r1.c1) is passed to the host as if it were a capture
//     id, and the store answers "not a capture id";
//   - a handle resolves to the wrong source, or one the frame does not
//     list resolves to anything at all;
//   - a frame source with only a URL (no capture) reads something else;
//   - Texture gains a world verb with it (Read, SearchStart, PollSource).
func TestTextureReadsItsFrameCapturesByHandle(t *testing.T) {
	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		t.Fatal(err)
	}
	issuer, err := NewHandleIssuer(secret)
	if err != nil {
		t.Fatal(err)
	}
	var asked []ReadCapturePayload
	broker, err := NewBroker(BrokerConfig{ComputerID: "computer-choir", CurrentEpoch: 1, AllowedRoot: t.TempDir(),
		Egress: func(ctx context.Context, action BrokerAction, payload json.RawMessage) (json.RawMessage, error) {
			if action != ActionReadCapture {
				t.Errorf("Texture reached %s", action)
			}
			var in ReadCapturePayload
			if err := json.Unmarshal(payload, &in); err != nil {
				t.Fatal(err)
			}
			asked = append(asked, in)
			return json.RawMessage(`{"capture_id":"` + in.CaptureID + `","blocks":[{"id":"p1","text":"words"}]}`), nil
		}}, issuer)
	if err != nil {
		t.Fatal(err)
	}
	scope, err := NewChoirScope(broker, issuer, "computer-choir", "activation-texture", 1, "texture", "")
	if err != nil {
		t.Fatal(err)
	}
	exports := scope.ChoirExports()["choir/choir"]
	if _, ok := exports["ReadCapture"]; !ok {
		t.Fatal("Texture must export choir.ReadCapture")
	}
	for _, world := range []string{"Read", "SearchStart", "PollSource", "SearchWireCorpus", "Research"} {
		if _, ok := exports[world]; ok {
			t.Errorf("Texture must not export the world verb choir.%s", world)
		}
	}

	const opening = "obj:choir.web_capture:abc:key-1"
	const research = "obj:choir.web_capture:def:key-2"
	hooks := scope.BindCell()
	hooks.Begin(SessionFrame{Doc: &DocSnapshot{DocID: "doc-1", Sources: []texturewrite.Source{
		{Handle: "r1.c1", CaptureID: opening},
		{Handle: "r2.c1", URL: "https://example.com/uncaptured"},
		{Handle: "r2.c2", CaptureID: research, URL: "https://example.com/b"},
	}}})
	defer hooks.End()

	for handle, want := range map[string]string{"r1.c1": opening, " r2.c2 ": research, research: research} {
		asked = nil
		out, err := scope.ReadCapture(handle, 3, 500)
		if err != nil {
			t.Fatalf("ReadCapture(%q): %v", handle, err)
		}
		if len(asked) != 1 || asked[0].CaptureID != want || asked[0].FromBlock != 3 || asked[0].MaxChars != 500 {
			t.Fatalf("ReadCapture(%q) asked the host %+v, want capture %s from block 3, 500 chars", handle, asked, want)
		}
		if !strings.Contains(string(out), want) {
			t.Fatalf("ReadCapture(%q) = %s", handle, out)
		}
	}
	for handle, wantErr := range map[string]string{
		"r9.c1": "r9.c1 is not a source in your frame",
		"r2.c1": "r2.c1 has no capture",
	} {
		asked = nil
		if _, err := scope.ReadCapture(handle, 0, 0); err == nil || !strings.Contains(err.Error(), wantErr) {
			t.Errorf("ReadCapture(%q) error = %v, want %q", handle, err, wantErr)
		}
		if len(asked) != 0 {
			t.Errorf("ReadCapture(%q) reached the host with %+v", handle, asked)
		}
	}
}
