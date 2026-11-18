package types

import (
	"crypto/sha256"
	"encoding/hex"
	"regexp"
	"strings"
)

// Takes (docs/design/autoradio-learned-document-2026-11-17.md section 11): a
// listener's recorded voice, published so other listeners' DJs can play it.
// These are the names the proxy, corpusd and a computer's programme route
// share, so the three cannot disagree on a handle or an id.

// TakeIDRe is the grammar of a take id: "take-" and 16 hex digits.
var TakeIDRe = regexp.MustCompile(`^take-[0-9a-f]{16}$`)

// The length a take's own audio header may state, in seconds. The claim is 3
// to 60; the header may sit outside it because Apple's AAC encoder adds about
// 0.11 s of priming to the container (60.000 s of audio has an mvhd of
// 60.117 s) and a recorder stops a moment after its timer. A take is stored
// with its header's length, so publish and the programme route's candidate
// check share this one pair: an honest full-length take is never refused by
// either.
const (
	TakeHeaderMinSeconds = 2.5
	TakeHeaderMaxSeconds = 62.0
)

// TakePublisherRe is the grammar of a publisher handle.
var TakePublisherRe = regexp.MustCompile(`^voice-[0-9a-f]{6}$`)

// TakePublisherHandle is a signed-in user's byline on a take: a short stable
// handle derived from the user id, never the id or the email.
func TakePublisherHandle(userID string) string {
	sum := sha256.Sum256([]byte("choir-voice-author:" + strings.TrimSpace(userID)))
	return "voice-" + hex.EncodeToString(sum[:])[:6]
}

// TakePublisherKey is the hash of a user id that groups a publisher's takes
// and their record. It is never published.
func TakePublisherKey(userID string) string {
	sum := sha256.Sum256([]byte("choir-voice-key:" + strings.TrimSpace(userID)))
	return hex.EncodeToString(sum[:])
}
