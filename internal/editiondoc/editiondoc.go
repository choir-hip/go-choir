// Package editiondoc names the listener's edition document: the Texture
// titled "My edition" that the Autoradio onboarding interview writes
// (docs/design/autoradio-app-ui-2026-11-10.md, section 2, "What the server
// lacks"). The marker is the title, as IsGuideTitle is for the Style Guide
// (the styleguide package): no new document role or schema, so the document stays
// an ordinary Texture the owner reads, edits and forks.
package editiondoc

import "strings"

// Title is the title of the edition document.
const Title = "My edition"

// IsEditionTitle reports whether a document title names the edition: the
// title compared case-insensitively with runs of white space folded.
func IsEditionTitle(title string) bool {
	return strings.EqualFold(strings.Join(strings.Fields(title), " "), Title)
}
