// Package appids is the one list of the phone app builds that may use
// choir.news passkeys. The association file (internal/proxy) serves it and
// the passkey finish routes (internal/auth) accept only the bundle ids on it,
// so a name the server accepts is always an app that can exist. A client names
// its app by one of these and never supplies free text. Adding an app is a
// change here, and a build signed with an App ID that carries Associated
// Domains.
package appids

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strings"
)

// App is one signed build: the Apple team that signs it and its bundle id. The
// association file names an app by "<team>.<bundle id>".
type App struct {
	Team   string
	Bundle string
}

// AppID is the signed application identifier, "<team>.<bundle id>".
func (a App) AppID() string { return a.Team + "." + a.Bundle }

// PasskeyApps are the builds that can use choir.news passkeys, all on team
// S47GERGJZ3.
//   - news.choir.autoradio: the App Store app; its App ID has Associated
//     Domains enabled in the developer account.
//   - news.choir.autoradio.lab.one: the development app on the same team. It
//     matches only in a build signed with an explicit App ID that carries
//     Associated Domains; a wildcard profile does not.
//
// The first development app, news.choir.autoradio.lab, is NOT listed: it is
// signed by a free team (QSGU6VRJL6, mobile/README.md), which cannot carry
// Associated Domains, so no entry could ever match it and no passkey can be
// made in it. Apple caches the association file for about a day; remove an
// entry when its app is retired.
var PasskeyApps = []App{
	{Team: "S47GERGJZ3", Bundle: "news.choir.autoradio"},
	{Team: "S47GERGJZ3", Bundle: "news.choir.autoradio.lab.one"},
}

// AndroidApp is one installed Android package and the SHA-256 of the
// certificate that signs it. The fingerprint is public; no private key belongs
// in this repository.
type AndroidApp struct {
	Package    string
	CertSHA256 string
}

// AndroidPasskeyApps is the canonical Android association list. It currently
// names only the A0 physical-device build, news.choir.autoradio.lab.one, signed
// with this Mac's Android debug certificate. The finish route's `app` field
// remains a label from PasskeyApps; this record adds no label authorization.
var AndroidPasskeyApps = []AndroidApp{
	{Package: "news.choir.autoradio.lab.one", CertSHA256: "dfe9767c642a2f864b297049bde97c0836cb58d5ce516ffe7a7da11f6fec9ea7"},
}

// SHA256Fingerprint is the colon-separated uppercase form Digital Asset Links
// uses. AndroidAPKKeyHashOrigin validates the same bytes before deriving an
// origin.
func (a AndroidApp) SHA256Fingerprint() string {
	s := strings.ToUpper(a.CertSHA256)
	var out strings.Builder
	for i := 0; i+1 < len(s); i += 2 {
		if i != 0 {
			out.WriteByte(':')
		}
		out.WriteString(s[i : i+2])
	}
	return out.String()
}

// AndroidAPKKeyHashOrigin is the origin Credential Manager writes into
// clientDataJSON: android:apk-key-hash:<base64url(certificate SHA-256 bytes)>.
// The fingerprint is already the SHA-256 digest; it is encoded, not hashed a
// second time. A malformed certificate is an error rather than a partially
// trusted origin.
func AndroidAPKKeyHashOrigin(a AndroidApp) (string, error) {
	if a.Package == "" {
		return "", fmt.Errorf("Android passkey app has no package")
	}
	raw, err := hex.DecodeString(a.CertSHA256)
	if err != nil {
		return "", fmt.Errorf("Android passkey certificate is not hex: %w", err)
	}
	if len(raw) != sha256.Size {
		return "", fmt.Errorf("Android passkey certificate has %d bytes, want %d", len(raw), sha256.Size)
	}
	return "android:apk-key-hash:" + base64.RawURLEncoding.EncodeToString(raw), nil
}

// Known reports whether bundle is the bundle id of one of PasskeyApps.
func Known(bundle string) bool {
	for _, a := range PasskeyApps {
		if a.Bundle == bundle {
			return true
		}
	}
	return false
}

// WebCredentialApps are the `webcredentials.apps` entries of the
// apple-app-site-association file.
func WebCredentialApps() []string {
	out := make([]string, 0, len(PasskeyApps))
	for _, a := range PasskeyApps {
		out = append(out, a.AppID())
	}
	return out
}
