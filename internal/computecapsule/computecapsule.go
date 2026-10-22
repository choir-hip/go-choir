// Package computecapsule is the contract of a compute capsule (SDC ARCH-09):
// a disposable capsule an app desk opens to turn content-addressed inputs
// into content-addressed output artifacts by running one recipe its app
// package declared. It has no source tree, no effect bundle and no egress;
// the engineering capsule stays the only path to the platform (I26).
//
// This package is pure: it validates a request against its recipe, renders
// the recipe's argv, refuses engineering inputs on a compute spawn, and
// checks a finished job's outputs into a manifest. Nothing calls it yet; the
// capsule executor (S1), the request runtime (S2) and the desk verb (S3)
// wire it. Failure modes C1-C12 in
// docs/design/app-desk-compute-capsule-2026-10-21.md §6.
package computecapsule

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"path"
	"sort"
	"strings"
)

const (
	// InDir and OutDir are the capsule paths of the inputs (read-only, one
	// file per slot) and of the outputs.
	InDir  = "/in"
	OutDir = "/out"

	// MaxInputs bounds the inputs of one request and the slots of a recipe.
	MaxInputs = 16
	// MaxInputBytes bounds the total bytes copied into one capsule.
	MaxInputBytes int64 = 256 << 20
	// MaxMemoryMiB is the memory request ceiling until S3 reads the
	// requesting desk's app floor.
	MaxMemoryMiB = 2048
	// MaxWallClockMS bounds one job: fifteen minutes.
	MaxWallClockMS int64 = 15 * 60 * 1000
	// MaxOutputs and MaxOutputBytes bound what a recipe may declare.
	MaxOutputs           = 64
	MaxOutputBytes int64 = 512 << 20

	maxArgs    = 64
	maxIDLen   = 128
	maxNameLen = 128
)

// Code names a refusal.
type Code string

const (
	CodeRequestInvalid   Code = "request_invalid"
	CodeDeskNotApp       Code = "desk_not_app"
	CodeRecipeNotOwned   Code = "recipe_not_owned"
	CodeRecipeInvalid    Code = "recipe_invalid"
	CodeInputInvalid     Code = "input_invalid"
	CodeResources        Code = "resources"
	CodeEgress           Code = "egress"
	CodeEngineeringInput Code = "engineering_input"
	CodeOutputInvalid    Code = "output_invalid"
)

// Refusal is a typed refusal.
type Refusal struct {
	Code   Code
	Detail string
}

func (r *Refusal) Error() string {
	return "compute capsule refused (" + string(r.Code) + "): " + r.Detail
}

func refuse(code Code, format string, args ...any) error {
	return &Refusal{Code: code, Detail: fmt.Sprintf(format, args...)}
}

// systemDesks may not open a compute capsule: engineering has its own
// capsule, research is read-only, management mutates nothing.
var systemDesks = map[string]bool{"management": true, "engineering": true, "research": true}

// forbiddenTools are interpreters a recipe may not name as argv[0]: a
// recipe runs one declared tool, never a shell (I19).
var forbiddenTools = map[string]bool{
	"sh": true, "bash": true, "dash": true, "zsh": true, "ash": true, "ksh": true,
	"busybox": true, "env": true, "xargs": true,
}

// Recipe is what an app package declares: one tool invocation with input
// slots and output bounds. The desk names it; it never writes argv.
type Recipe struct {
	Package string `json:"package"`
	Name    string `json:"name"`
	// Tools are the absolute paths the recipe may execute; Argv[0] is one.
	Tools []string `json:"tools"`
	// Argv is the template. An argument holding a brace must be exactly
	// {out} or {in:<slot>}.
	Argv             []string `json:"argv"`
	Inputs           []string `json:"inputs"`
	OutputMediaTypes []string `json:"output_media_types"`
	MaxOutputs       int      `json:"max_outputs"`
	MaxOutputBytes   int64    `json:"max_output_bytes"`
	MemoryMiB        int      `json:"memory_mib"`
	WallClockMS      int64    `json:"wall_clock_ms"`
}

// Input binds a content-addressed artifact to a recipe slot.
type Input struct {
	Slot  string `json:"slot"`
	Ref   string `json:"ref"`
	Bytes int64  `json:"bytes"`
}

// Request is an app desk's compute request. MemoryMiB and WallClockMS may
// lower the recipe's (0 keeps it).
type Request struct {
	ID          string  `json:"id"`
	Desk        string  `json:"desk"`
	Package     string  `json:"package"`
	Recipe      string  `json:"recipe"`
	Inputs      []Input `json:"inputs"`
	MemoryMiB   int     `json:"memory_mib,omitempty"`
	WallClockMS int64   `json:"wall_clock_ms,omitempty"`
	Egress      bool    `json:"egress,omitempty"`
}

// Plan is a validated request: what a spawn runs.
type Plan struct {
	RequestID   string
	Desk        string
	CapsuleID   string
	Recipe      Recipe
	Inputs      []Input // sorted by slot
	Argv        []string
	MemoryMiB   int
	WallClockMS int64
}

// Validate checks a request against the recipe it names and renders the
// plan. Every refusal is a *Refusal.
func Validate(req Request, recipe Recipe) (Plan, error) {
	if !validID(req.ID, maxIDLen) {
		return Plan{}, refuse(CodeRequestInvalid, "request id %q is invalid", req.ID)
	}
	desk := strings.ToLower(strings.TrimSpace(req.Desk))
	if desk == "" || systemDesks[desk] {
		return Plan{}, refuse(CodeDeskNotApp, "desk %q is not an app desk", req.Desk)
	}
	if req.Package == "" || req.Package != recipe.Package || req.Recipe != recipe.Name {
		return Plan{}, refuse(CodeRecipeNotOwned, "recipe %s/%s is not package %q's %q", recipe.Package, recipe.Name, req.Package, req.Recipe)
	}
	if req.Egress {
		return Plan{}, refuse(CodeEgress, "a compute capsule has no egress")
	}
	if len(req.Inputs) > MaxInputs {
		return Plan{}, refuse(CodeInputInvalid, "%d inputs, more than %d", len(req.Inputs), MaxInputs)
	}
	if err := validateRecipe(recipe); err != nil {
		return Plan{}, err
	}
	inputs, err := bindInputs(req.Inputs, recipe.Inputs)
	if err != nil {
		return Plan{}, err
	}
	memory, wall := recipe.MemoryMiB, recipe.WallClockMS
	if req.MemoryMiB < 0 || req.MemoryMiB > MaxMemoryMiB || req.WallClockMS < 0 || req.WallClockMS > MaxWallClockMS {
		return Plan{}, refuse(CodeResources, "request asks %d MiB and %d ms, over %d MiB or %d ms", req.MemoryMiB, req.WallClockMS, MaxMemoryMiB, MaxWallClockMS)
	}
	if req.MemoryMiB > 0 && req.MemoryMiB < memory {
		memory = req.MemoryMiB
	}
	if req.WallClockMS > 0 && req.WallClockMS < wall {
		wall = req.WallClockMS
	}
	sum := sha256.Sum256([]byte(req.ID))
	recipe = cloneRecipe(recipe)
	return Plan{
		RequestID:   req.ID,
		Desk:        req.Desk,
		CapsuleID:   "compute-" + hex.EncodeToString(sum[:12]),
		Recipe:      recipe,
		Inputs:      inputs,
		Argv:        renderArgv(recipe.Argv),
		MemoryMiB:   memory,
		WallClockMS: wall,
	}, nil
}

func validateRecipe(r Recipe) error {
	if !validName(r.Package) || !validName(r.Name) {
		return refuse(CodeRecipeInvalid, "recipe package %q or name %q is invalid", r.Package, r.Name)
	}
	tools := map[string]bool{}
	for _, t := range r.Tools {
		if !path.IsAbs(t) || path.Clean(t) != t || strings.Contains(t, "..") {
			return refuse(CodeRecipeInvalid, "tool %q is not a clean absolute path", t)
		}
		if forbiddenTools[path.Base(t)] {
			return refuse(CodeRecipeInvalid, "tool %q is an interpreter", t)
		}
		tools[t] = true
	}
	if len(r.Argv) == 0 || len(r.Argv) > maxArgs {
		return refuse(CodeRecipeInvalid, "argv has %d arguments (1 to %d)", len(r.Argv), maxArgs)
	}
	if !tools[r.Argv[0]] {
		return refuse(CodeRecipeInvalid, "argv[0] %q is not a declared tool", r.Argv[0])
	}
	if len(r.Inputs) > MaxInputs {
		return refuse(CodeInputInvalid, "recipe declares %d slots, more than %d", len(r.Inputs), MaxInputs)
	}
	slots := map[string]bool{}
	for _, s := range r.Inputs {
		if !validName(s) || slots[s] {
			return refuse(CodeRecipeInvalid, "slot %q is invalid or repeated", s)
		}
		slots[s] = true
	}
	for i, a := range r.Argv[1:] {
		if !strings.ContainsAny(a, "{}") {
			continue
		}
		if a == "{out}" {
			continue
		}
		if s, ok := strings.CutPrefix(a, "{in:"); ok {
			if s, ok = strings.CutSuffix(s, "}"); ok && slots[s] {
				continue
			}
		}
		return refuse(CodeRecipeInvalid, "argv[%d] %q is not {out} or {in:<declared slot>}", i+1, a)
	}
	if r.MaxOutputs <= 0 || r.MaxOutputs > MaxOutputs || r.MaxOutputBytes <= 0 || r.MaxOutputBytes > MaxOutputBytes {
		return refuse(CodeRecipeInvalid, "output bounds %d files %d bytes outside 1..%d and 1..%d", r.MaxOutputs, r.MaxOutputBytes, MaxOutputs, MaxOutputBytes)
	}
	if len(r.OutputMediaTypes) == 0 {
		return refuse(CodeRecipeInvalid, "recipe declares no output media type")
	}
	for _, m := range r.OutputMediaTypes {
		if !validMediaType(m) {
			return refuse(CodeRecipeInvalid, "media type %q is malformed", m)
		}
	}
	if r.MemoryMiB <= 0 || r.MemoryMiB > MaxMemoryMiB || r.WallClockMS <= 0 || r.WallClockMS > MaxWallClockMS {
		return refuse(CodeResources, "recipe needs %d MiB and %d ms, outside 1..%d MiB and 1..%d ms", r.MemoryMiB, r.WallClockMS, MaxMemoryMiB, MaxWallClockMS)
	}
	return nil
}

func bindInputs(in []Input, slots []string) ([]Input, error) {
	declared := map[string]bool{}
	for _, s := range slots {
		declared[s] = true
	}
	bound := map[string]bool{}
	var total int64
	out := make([]Input, 0, len(in))
	for _, i := range in {
		if !declared[i.Slot] {
			return nil, refuse(CodeInputInvalid, "slot %q is not the recipe's", i.Slot)
		}
		if bound[i.Slot] {
			return nil, refuse(CodeInputInvalid, "slot %q is bound twice", i.Slot)
		}
		hexPart, ok := strings.CutPrefix(i.Ref, "sha256:")
		if !ok || !isHex64(hexPart) {
			return nil, refuse(CodeInputInvalid, "input ref %q is not sha256:<64 lowercase hex>", i.Ref)
		}
		if i.Bytes < 0 || i.Bytes > MaxInputBytes-total {
			return nil, refuse(CodeInputInvalid, "input bytes %d negative or over the %d total", i.Bytes, MaxInputBytes)
		}
		total += i.Bytes
		bound[i.Slot] = true
		out = append(out, i)
	}
	for _, s := range slots {
		if !bound[s] {
			return nil, refuse(CodeInputInvalid, "slot %q is unbound", s)
		}
	}
	sort.Slice(out, func(a, b int) bool { return out[a].Slot < out[b].Slot })
	return out, nil
}

func renderArgv(tmpl []string) []string {
	argv := make([]string, len(tmpl))
	for i, a := range tmpl {
		switch {
		case a == "{out}":
			argv[i] = OutDir
		case strings.HasPrefix(a, "{in:"):
			argv[i] = InDir + "/" + strings.TrimSuffix(strings.TrimPrefix(a, "{in:"), "}")
		default:
			argv[i] = a
		}
	}
	return argv
}

// EngineeringInputs mirrors the engineering-only fields of capsule.SpawnSpec
// and its working directory. A compute spawn must leave every one empty.
type EngineeringInputs struct {
	SourceArtifactRef     string
	ExpectedSubjectDigest string
	VerifierBundleDir     string
	VerifierBinding       string
	AdoptionDir           string
	CarriedPatchDir       string
	Egress                bool
	ClaudeDelegate        bool
	WorkingDir            string
}

// RefuseEngineeringInputs refuses a compute spawn that carries any
// engineering input or works under /workspace (C9).
func RefuseEngineeringInputs(in EngineeringInputs) error {
	var set []string
	for name, v := range map[string]bool{
		"source artifact ref":     in.SourceArtifactRef != "",
		"expected subject digest": in.ExpectedSubjectDigest != "",
		"verifier bundle":         in.VerifierBundleDir != "",
		"verifier binding":        in.VerifierBinding != "",
		"adoption":                in.AdoptionDir != "",
		"carried patch":           in.CarriedPatchDir != "",
		"egress":                  in.Egress,
		"claude delegate":         in.ClaudeDelegate,
		"workspace working dir":   in.WorkingDir == "/workspace" || strings.HasPrefix(path.Clean("/"+in.WorkingDir), "/workspace/"),
	} {
		if v {
			set = append(set, name)
		}
	}
	if len(set) > 0 {
		sort.Strings(set)
		return refuse(CodeEngineeringInput, "a compute spawn carries %s", strings.Join(set, ", "))
	}
	return nil
}

// Output is one file a finished job left in /out.
type Output struct {
	Name      string `json:"name"`
	Bytes     int64  `json:"bytes"`
	SHA256    string `json:"sha256"`
	MediaType string `json:"media_type"`
}

// Manifest is the checked record of a finished job: the request, its inputs
// and its outputs, sorted, under one digest.
type Manifest struct {
	RequestID  string   `json:"request_id"`
	Desk       string   `json:"desk"`
	Package    string   `json:"package"`
	Recipe     string   `json:"recipe"`
	Inputs     []Input  `json:"inputs"`
	Outputs    []Output `json:"outputs"`
	TotalBytes int64    `json:"total_bytes"`
	Digest     string   `json:"-"`
}

// CheckOutputs checks a finished job's outputs against its plan's recipe
// (C10, C11) and returns the manifest, whose digest is independent of the
// outputs' order (C12).
func CheckOutputs(plan Plan, outs []Output) (Manifest, error) {
	if plan.RequestID == "" || plan.CapsuleID == "" {
		return Manifest{}, refuse(CodeRequestInvalid, "outputs checked against a plan Validate did not return")
	}
	r := plan.Recipe
	if len(outs) == 0 || len(outs) > r.MaxOutputs {
		return Manifest{}, refuse(CodeOutputInvalid, "%d outputs, want 1 to %d", len(outs), r.MaxOutputs)
	}
	types := map[string]bool{}
	for _, m := range r.OutputMediaTypes {
		types[m] = true
	}
	seen := map[string]bool{}
	var total int64
	sorted := make([]Output, 0, len(outs))
	for _, o := range outs {
		if !validOutputName(o.Name) || seen[o.Name] {
			return Manifest{}, refuse(CodeOutputInvalid, "output name %q is invalid or repeated", o.Name)
		}
		if !isHex64(o.SHA256) {
			return Manifest{}, refuse(CodeOutputInvalid, "output %q digest %q is not 64 lowercase hex", o.Name, o.SHA256)
		}
		if !types[o.MediaType] {
			return Manifest{}, refuse(CodeOutputInvalid, "output %q media type %q is not the recipe's", o.Name, o.MediaType)
		}
		if o.Bytes < 0 || o.Bytes > r.MaxOutputBytes-total {
			return Manifest{}, refuse(CodeOutputInvalid, "output %q bytes %d negative or over the recipe's %d total", o.Name, o.Bytes, r.MaxOutputBytes)
		}
		total += o.Bytes
		seen[o.Name] = true
		sorted = append(sorted, o)
	}
	sort.Slice(sorted, func(a, b int) bool { return sorted[a].Name < sorted[b].Name })
	m := Manifest{
		RequestID: plan.RequestID, Desk: plan.Desk, Package: r.Package, Recipe: r.Name,
		Inputs: append([]Input(nil), plan.Inputs...), Outputs: sorted, TotalBytes: total,
	}
	data, err := json.Marshal(m)
	if err != nil {
		return Manifest{}, err
	}
	sum := sha256.Sum256(data)
	m.Digest = "sha256:" + hex.EncodeToString(sum[:])
	return m, nil
}

func cloneRecipe(r Recipe) Recipe {
	r.Tools = append([]string(nil), r.Tools...)
	r.Argv = append([]string(nil), r.Argv...)
	r.Inputs = append([]string(nil), r.Inputs...)
	r.OutputMediaTypes = append([]string(nil), r.OutputMediaTypes...)
	return r
}

func validID(s string, max int) bool {
	if s == "" || len(s) > max {
		return false
	}
	for _, c := range s {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '-' || c == '_' || c == ':' || c == '.') {
			return false
		}
	}
	return true
}

// validName is a package, recipe or slot name: lowercase letters, digits
// and underscores, starting with a letter.
func validName(s string) bool {
	if s == "" || len(s) > maxNameLen || !(s[0] >= 'a' && s[0] <= 'z') {
		return false
	}
	for _, c := range s {
		if !(c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '_') {
			return false
		}
	}
	return true
}

func validOutputName(s string) bool {
	if s == "" || len(s) > maxNameLen || s[0] == '.' || strings.ContainsAny(s, `/\`) {
		return false
	}
	for _, c := range s {
		if c < 0x20 || c == 0x7f {
			return false
		}
	}
	return true
}

func validMediaType(s string) bool {
	top, sub, ok := strings.Cut(s, "/")
	if !ok || top == "" || sub == "" || strings.ContainsAny(s, " ;\t") || strings.Count(s, "/") != 1 {
		return false
	}
	return strings.ToLower(s) == s
}

func isHex64(s string) bool {
	if len(s) != 64 {
		return false
	}
	for _, c := range s {
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f') {
			return false
		}
	}
	return true
}
