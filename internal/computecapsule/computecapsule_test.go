package computecapsule

import (
	"errors"
	"strings"
	"testing"
)

// The failure modes are design §6 C1-C12
// (docs/design/app-desk-compute-capsule-2026-10-21.md), written before the code.

const hexA = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
const hexB = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"

func pdfRecipe() Recipe {
	return Recipe{
		Package:          "slides",
		Name:             "pdf_export",
		Tools:            []string{"/nix/store/abc-chromium/bin/chromium"},
		Argv:             []string{"/nix/store/abc-chromium/bin/chromium", "--headless", "--print-to-pdf-dir", "{out}", "{in:deck}"},
		Inputs:           []string{"deck"},
		OutputMediaTypes: []string{"application/pdf"},
		MaxOutputs:       4,
		MaxOutputBytes:   64 << 20,
		MemoryMiB:        768,
		WallClockMS:      120_000,
	}
}

func pdfRequest() Request {
	return Request{
		ID:      "req-1",
		Desk:    "app:slides",
		Package: "slides",
		Recipe:  "pdf_export",
		Inputs:  []Input{{Slot: "deck", Ref: "sha256:" + hexA, Bytes: 4096}},
	}
}

func codeOf(t *testing.T, err error) Code {
	t.Helper()
	if err == nil {
		t.Fatalf("expected a refusal, got nil")
	}
	var r *Refusal
	if !errors.As(err, &r) {
		t.Fatalf("error %v is not a *Refusal", err)
	}
	return r.Code
}

func TestValidateAcceptsAPackageRecipe(t *testing.T) {
	plan, err := Validate(pdfRequest(), pdfRecipe())
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	want := []string{"/nix/store/abc-chromium/bin/chromium", "--headless", "--print-to-pdf-dir", "/out", "/in/deck"}
	if strings.Join(plan.Argv, "\x00") != strings.Join(want, "\x00") {
		t.Fatalf("argv = %q, want %q", plan.Argv, want)
	}
	if plan.MemoryMiB != 768 || plan.WallClockMS != 120_000 {
		t.Fatalf("resources = %d MiB %d ms, want the recipe's", plan.MemoryMiB, plan.WallClockMS)
	}
	if !strings.HasPrefix(plan.CapsuleID, "compute-") {
		t.Fatalf("capsule id %q lacks the compute- prefix", plan.CapsuleID)
	}
	again, _ := Validate(pdfRequest(), pdfRecipe())
	if again.CapsuleID != plan.CapsuleID {
		t.Fatalf("capsule id is not derived from the request id")
	}
	other := pdfRequest()
	other.ID = "req-2"
	if p2, _ := Validate(other, pdfRecipe()); p2.CapsuleID == plan.CapsuleID {
		t.Fatalf("two requests share a capsule id")
	}
}

func TestValidateRefusals(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*Request, *Recipe)
		want   Code
	}{
		{"C0 bad request id", func(q *Request, _ *Recipe) { q.ID = "a b" }, CodeRequestInvalid},
		// C1
		{"C1 empty desk", func(q *Request, _ *Recipe) { q.Desk = "" }, CodeDeskNotApp},
		{"C1 management", func(q *Request, _ *Recipe) { q.Desk = "management" }, CodeDeskNotApp},
		{"C1 engineering", func(q *Request, _ *Recipe) { q.Desk = "engineering" }, CodeDeskNotApp},
		{"C1 research", func(q *Request, _ *Recipe) { q.Desk = "Research " }, CodeDeskNotApp},
		// C2
		{"C2 other package", func(q *Request, _ *Recipe) { q.Package = "mail" }, CodeRecipeNotOwned},
		{"C2 other recipe name", func(q *Request, _ *Recipe) { q.Recipe = "video" }, CodeRecipeNotOwned},
		// C3
		{"C3 relative argv0", func(_ *Request, r *Recipe) { r.Argv[0] = "chromium"; r.Tools = []string{"chromium"} }, CodeRecipeInvalid},
		{"C3 undeclared tool", func(_ *Request, r *Recipe) { r.Argv[0] = "/usr/bin/chromium" }, CodeRecipeInvalid},
		{"C3 shell", func(_ *Request, r *Recipe) { r.Tools = []string{"/bin/sh"}; r.Argv[0] = "/bin/sh" }, CodeRecipeInvalid},
		{"C3 env", func(_ *Request, r *Recipe) { r.Tools = []string{"/usr/bin/env"}; r.Argv[0] = "/usr/bin/env" }, CodeRecipeInvalid},
		{"C3 unclean tool", func(_ *Request, r *Recipe) {
			r.Tools = []string{"/nix/store/../bin/x"}
			r.Argv[0] = "/nix/store/../bin/x"
		}, CodeRecipeInvalid},
		{"C3 placeholder argv0", func(_ *Request, r *Recipe) { r.Argv[0] = "{out}" }, CodeRecipeInvalid},
		// C4
		{"C4 unknown placeholder", func(_ *Request, r *Recipe) { r.Argv[2] = "{env:HOME}" }, CodeRecipeInvalid},
		{"C4 undeclared slot", func(_ *Request, r *Recipe) { r.Argv[4] = "{in:other}" }, CodeRecipeInvalid},
		{"C4 spliced placeholder", func(_ *Request, r *Recipe) { r.Argv[3] = "--out={out}" }, CodeRecipeInvalid},
		{"C4 stray brace", func(_ *Request, r *Recipe) { r.Argv[2] = "x}" }, CodeRecipeInvalid},
		{"C4 no output bound", func(_ *Request, r *Recipe) { r.MaxOutputs = 0 }, CodeRecipeInvalid},
		{"C4 no media type", func(_ *Request, r *Recipe) { r.OutputMediaTypes = nil }, CodeRecipeInvalid},
		{"C4 malformed media type", func(_ *Request, r *Recipe) { r.OutputMediaTypes = []string{"pdf"} }, CodeRecipeInvalid},
		{"C4 duplicate slot", func(_ *Request, r *Recipe) { r.Inputs = []string{"deck", "deck"} }, CodeRecipeInvalid},
		// C5
		{"C5 bad ref", func(q *Request, _ *Recipe) { q.Inputs[0].Ref = "sha256:XYZ" }, CodeInputInvalid},
		{"C5 uppercase ref", func(q *Request, _ *Recipe) { q.Inputs[0].Ref = "sha256:" + strings.ToUpper(hexA) }, CodeInputInvalid},
		{"C5 path ref", func(q *Request, _ *Recipe) { q.Inputs[0].Ref = "/etc/passwd" }, CodeInputInvalid},
		{"C5 unbound slot", func(q *Request, _ *Recipe) { q.Inputs = nil }, CodeInputInvalid},
		{"C5 unknown slot", func(q *Request, _ *Recipe) { q.Inputs[0].Slot = "other" }, CodeInputInvalid},
		{"C5 slot bound twice", func(q *Request, _ *Recipe) {
			q.Inputs = append(q.Inputs, Input{Slot: "deck", Ref: "sha256:" + hexB, Bytes: 1})
		}, CodeInputInvalid},
		{"C5 negative bytes", func(q *Request, _ *Recipe) { q.Inputs[0].Bytes = -1 }, CodeInputInvalid},
		// C6
		{"C6 input bytes over cap", func(q *Request, _ *Recipe) { q.Inputs[0].Bytes = MaxInputBytes + 1 }, CodeInputInvalid},
		{"C6 too many slots", func(q *Request, r *Recipe) {
			r.Inputs = nil
			q.Inputs = nil
			for i := 0; i <= MaxInputs; i++ {
				s := "s" + strings.Repeat("x", i)
				r.Inputs = append(r.Inputs, s)
				q.Inputs = append(q.Inputs, Input{Slot: s, Ref: "sha256:" + hexA, Bytes: 1})
			}
		}, CodeInputInvalid},
		// C7
		{"C7 recipe memory over cap", func(_ *Request, r *Recipe) { r.MemoryMiB = MaxMemoryMiB + 1 }, CodeResources},
		{"C7 request memory over cap", func(q *Request, _ *Recipe) { q.MemoryMiB = MaxMemoryMiB + 1 }, CodeResources},
		{"C7 recipe without memory", func(_ *Request, r *Recipe) { r.MemoryMiB = 0 }, CodeResources},
		{"C7 wall clock over max", func(_ *Request, r *Recipe) { r.WallClockMS = MaxWallClockMS + 1 }, CodeResources},
		{"C7 request wall clock over max", func(q *Request, _ *Recipe) { q.WallClockMS = MaxWallClockMS + 1 }, CodeResources},
		{"C7 recipe without wall clock", func(_ *Request, r *Recipe) { r.WallClockMS = 0 }, CodeResources},
		// C8
		{"C8 egress", func(q *Request, _ *Recipe) { q.Egress = true }, CodeEgress},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			q, r := pdfRequest(), pdfRecipe()
			tc.mutate(&q, &r)
			_, err := Validate(q, r)
			if got := codeOf(t, err); got != tc.want {
				t.Fatalf("code = %s, want %s (err %v)", got, tc.want, err)
			}
		})
	}
}

func TestValidateRequestMayLowerResources(t *testing.T) {
	q := pdfRequest()
	q.MemoryMiB, q.WallClockMS = 512, 60_000
	plan, err := Validate(q, pdfRecipe())
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if plan.MemoryMiB != 512 || plan.WallClockMS != 60_000 {
		t.Fatalf("resources = %d MiB %d ms, want the request's", plan.MemoryMiB, plan.WallClockMS)
	}
}

func TestValidateDoesNotAliasTheRecipe(t *testing.T) {
	r := pdfRecipe()
	plan, err := Validate(pdfRequest(), r)
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	r.Argv[1] = "--changed"
	r.OutputMediaTypes[0] = "text/html"
	if plan.Argv[1] != "--headless" || plan.Recipe.OutputMediaTypes[0] != "application/pdf" {
		t.Fatalf("plan aliases the caller's recipe")
	}
}

// C9
func TestRefuseEngineeringInputs(t *testing.T) {
	if err := RefuseEngineeringInputs(EngineeringInputs{}); err != nil {
		t.Fatalf("empty inputs refused: %v", err)
	}
	cases := map[string]EngineeringInputs{
		"source ref":       {SourceArtifactRef: "artifact:x"},
		"subject digest":   {ExpectedSubjectDigest: "sha256:" + hexA},
		"verifier bundle":  {VerifierBundleDir: "/x"},
		"verifier binding": {VerifierBinding: "{}"},
		"adoption":         {AdoptionDir: "/x"},
		"carried patch":    {CarriedPatchDir: "/x"},
		"egress":           {Egress: true},
		"delegate":         {ClaudeDelegate: true},
		"platform workdir": {WorkingDir: "/workspace/platform"},
		"workspace subdir": {WorkingDir: "/workspace/x"},
	}
	for name, in := range cases {
		t.Run(name, func(t *testing.T) {
			if got := codeOf(t, RefuseEngineeringInputs(in)); got != CodeEngineeringInput {
				t.Fatalf("code = %s", got)
			}
		})
	}
	if err := RefuseEngineeringInputs(EngineeringInputs{WorkingDir: OutDir}); err != nil {
		t.Fatalf("the output dir as working dir refused: %v", err)
	}
}

func outputs() []Output {
	return []Output{
		{Name: "deck.pdf", Bytes: 1000, SHA256: hexA, MediaType: "application/pdf"},
		{Name: "notes.pdf", Bytes: 2000, SHA256: hexB, MediaType: "application/pdf"},
	}
}

func TestCheckOutputsManifest(t *testing.T) {
	plan, err := Validate(pdfRequest(), pdfRecipe())
	if err != nil {
		t.Fatal(err)
	}
	m, err := CheckOutputs(plan, outputs())
	if err != nil {
		t.Fatalf("CheckOutputs: %v", err)
	}
	if m.TotalBytes != 3000 || len(m.Outputs) != 2 || m.Outputs[0].Name != "deck.pdf" {
		t.Fatalf("manifest = %+v", m)
	}
	if !strings.HasPrefix(m.Digest, "sha256:") || len(m.Digest) != len("sha256:")+64 {
		t.Fatalf("digest = %q", m.Digest)
	}
	if m.RequestID != "req-1" || m.Package != "slides" || m.Recipe != "pdf_export" || len(m.Inputs) != 1 {
		t.Fatalf("manifest does not bind the request: %+v", m)
	}
	// C12: order does not change the digest.
	rev := outputs()
	rev[0], rev[1] = rev[1], rev[0]
	m2, err := CheckOutputs(plan, rev)
	if err != nil || m2.Digest != m.Digest {
		t.Fatalf("reordered digest %q (err %v), want %q", m2.Digest, err, m.Digest)
	}
	// A different request over the same bytes is a different manifest.
	other := pdfRequest()
	other.ID = "req-2"
	plan2, _ := Validate(other, pdfRecipe())
	if m3, _ := CheckOutputs(plan2, outputs()); m3.Digest == m.Digest {
		t.Fatalf("manifest digest does not bind the request id")
	}
	// A changed byte count is a different manifest.
	changed := outputs()
	changed[1].Bytes++
	if m4, _ := CheckOutputs(plan, changed); m4.Digest == m.Digest {
		t.Fatalf("manifest digest does not bind the outputs")
	}
}

func TestCheckOutputsRefusals(t *testing.T) {
	cases := []struct {
		name   string
		mutate func([]Output) []Output
	}{
		// C10
		{"slash", func(o []Output) []Output { o[0].Name = "a/b.pdf"; return o }},
		{"backslash", func(o []Output) []Output { o[0].Name = `a\b.pdf`; return o }},
		{"dotdot", func(o []Output) []Output { o[0].Name = ".."; return o }},
		{"leading dot", func(o []Output) []Output { o[0].Name = ".hidden"; return o }},
		{"empty name", func(o []Output) []Output { o[0].Name = ""; return o }},
		{"control char", func(o []Output) []Output { o[0].Name = "a\nb"; return o }},
		{"duplicate", func(o []Output) []Output { o[1].Name = o[0].Name; return o }},
		// C11
		{"none", func(o []Output) []Output { return nil }},
		{"too many", func(o []Output) []Output {
			for i := 0; i < 4; i++ {
				o = append(o, Output{Name: "x" + strings.Repeat("y", i) + ".pdf", Bytes: 1, SHA256: hexA, MediaType: "application/pdf"})
			}
			return o
		}},
		{"bytes over cap", func(o []Output) []Output { o[0].Bytes = 64 << 20; return o }},
		{"negative bytes", func(o []Output) []Output { o[0].Bytes = -5; return o }},
		{"media type", func(o []Output) []Output { o[0].MediaType = "text/html"; return o }},
		{"bad digest", func(o []Output) []Output { o[0].SHA256 = "abc"; return o }},
	}
	plan, err := Validate(pdfRequest(), pdfRecipe())
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := CheckOutputs(plan, tc.mutate(outputs()))
			if got := codeOf(t, err); got != CodeOutputInvalid {
				t.Fatalf("code = %s (err %v)", got, err)
			}
		})
	}
}

func TestCheckOutputsRefusesAnUnvalidatedPlan(t *testing.T) {
	if got := codeOf(t, func() error { _, err := CheckOutputs(Plan{}, outputs()); return err }()); got != CodeRequestInvalid {
		t.Fatalf("code = %s", got)
	}
}
