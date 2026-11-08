package appdesk

import (
	"bytes"
	"embed"
	_ "embed"
	"encoding/json"
	"fmt"

	"github.com/yusefmosiah/go-choir/internal/deskmanifest"
)

// First-party package desks (plan MA4.7, doctrine I26).
//
// There are no special desks: an in-tree first-party package carries its desk
// like any package, registered by the one path (an install event folded by
// Derive). What is first-party is only who publishes it. A few such desks take
// a slug the registry reserves (deskmanifest.FirstPartyAppSlug): the Mail desk
// is app:mail, the desk of the in-tree Mail package. The reservation holds for
// everyone else: a package desk under a reserved slug registers only when its
// manifest is exactly the one this image ships for that package (same digest),
// so a third-party package named mail, or any other manifest under that name,
// is refused.

//go:embed firstparty.json
var firstPartyJSON []byte

var firstPartyDesks = mustFirstParty(firstPartyJSON)

func mustFirstParty(raw []byte) map[string]Required {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	var f requiredFile
	if err := dec.Decode(&f); err != nil {
		panic(fmt.Sprintf("appdesk: first-party desks: %v", err))
	}
	out := map[string]Required{}
	for _, r := range f.Packages {
		if r.Manifest.Package != r.Package {
			panic(fmt.Sprintf("appdesk: first-party package %q carries the manifest of %q", r.Package, r.Manifest.Package))
		}
		if !deskmanifest.FirstPartyAppSlug(r.Package) {
			panic(fmt.Sprintf("appdesk: first-party package %q is not a reserved first-party slug", r.Package))
		}
		out[r.Package] = r
	}
	return out
}

// FirstPartyDesk returns the desk manifest the image ships for an in-tree
// first-party package under a reserved slug, and its version.
func FirstPartyDesk(pkg string) (Required, bool) {
	r, ok := firstPartyDesks[pkg]
	return r, ok
}

// firstParty reports whether m is exactly the image's first-party desk
// manifest for its package: the only manifest a reserved slug admits.
func firstParty(m Manifest) bool {
	r, ok := firstPartyDesks[m.Package]
	return ok && Digest(r.Manifest) == Digest(m)
}

//go:embed firstparty_prompts/*.md
var firstPartyPrompts embed.FS

// FirstPartyPrompt is the prompt of an in-tree first-party package's desk,
// shipped with the image beside its manifest. A desk's prompt is behaviour, so
// a first-party desk runs on this text and never on a release's own prompt
// file (the package's prompt.md is a copy for the Library, not the source).
func FirstPartyPrompt(pkg string) (string, bool) {
	if _, ok := firstPartyDesks[pkg]; !ok {
		return "", false
	}
	raw, err := firstPartyPrompts.ReadFile("firstparty_prompts/" + pkg + ".md")
	if err != nil {
		return "", false
	}
	return string(raw), true
}
