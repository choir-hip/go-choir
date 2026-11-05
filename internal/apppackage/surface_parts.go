package apppackage

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
)

// The components of the behaviour surface (trust draft 4.2, MA7.3). The
// surface hash is one sha256; to compute how much of a package's record a new
// version keeps, the same document is hashed in parts so the change between
// two versions is computed, never declared.
const (
	PartPrompt    = "prompt"    // each desk's name and prompt file
	PartCode      = "code"      // each desk's code and tool files
	PartSchedules = "schedules" // each desk's schedules
	PartAccepts   = "accepts"   // what each desk says it accepts
	PartKinds     = "kinds"     // owned kinds with schemas, and the kinds it reads
	PartGrants    = "grants"    // the capability ceiling, with its arguments (egress hosts among them)
	PartApp       = "app"       // the window's entry, abi, SDK range, aliases, and the files it opens
	PartAssets    = "assets"    // every other file of the release tree
)

// SurfaceParts hashes the components of the behaviour surface. The parts
// partition the same canonical document BehaviourSurface hashes, so equal
// surface hashes mean equal parts.
func (m Manifest) SurfaceParts(src Source) (map[string]string, error) {
	doc, err := m.surfaceDocument(src)
	if err != nil {
		return nil, err
	}
	type promptPart struct {
		Name   string      `json:"name"`
		Prompt surfaceFile `json:"prompt"`
	}
	type codePart struct {
		Name  string        `json:"name"`
		Code  *surfaceFile  `json:"code"`
		Tools []surfaceFile `json:"tools"`
	}
	type schedPart struct {
		Name      string     `json:"name"`
		Schedules []Schedule `json:"schedules"`
	}
	type acceptPart struct {
		Name    string          `json:"name"`
		Accepts json.RawMessage `json:"accepts"`
	}
	var prompts []promptPart
	var codes []codePart
	var scheds []schedPart
	var accepts []acceptPart
	owned := map[string]bool{} // files a desk part already covers
	for _, d := range doc.Desks {
		prompts = append(prompts, promptPart{d.Name, d.Prompt})
		codes = append(codes, codePart{d.Name, d.Code, d.Tools})
		scheds = append(scheds, schedPart{d.Name, d.Schedules})
		accepts = append(accepts, acceptPart{d.Name, d.Accepts})
		owned[d.Prompt.Path] = true
		if d.Code != nil {
			owned[d.Code.Path] = true
		}
		for _, t := range d.Tools {
			owned[t.Path] = true
		}
	}
	var assets []surfaceFile
	for _, f := range doc.Files {
		if !owned[f.Path] {
			assets = append(assets, f)
		}
	}
	parts := map[string]any{
		PartPrompt:    prompts,
		PartCode:      codes,
		PartSchedules: scheds,
		PartAccepts:   accepts,
		PartKinds: struct {
			Kinds []surfaceKind
			Uses  []string
		}{doc.Kinds, doc.UsesKinds},
		PartGrants: doc.Capabilities,
		PartApp: struct {
			App   *surfaceApp
			Opens []surfaceOpens
		}{doc.App, doc.Opens},
		PartAssets: assets,
	}
	out := make(map[string]string, len(parts))
	names := make([]string, 0, len(parts))
	for k := range parts {
		names = append(names, k)
	}
	sort.Strings(names)
	for _, k := range names {
		raw, err := json.Marshal(parts[k])
		if err != nil {
			return nil, err
		}
		sum := sha256.Sum256(raw)
		out[k] = hex.EncodeToString(sum[:])
	}
	return out, nil
}
