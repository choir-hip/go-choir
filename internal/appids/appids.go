// Package appids is the one list of the phone app builds that may use
// choir.news passkeys. The association file (internal/proxy) serves it and
// the passkey finish routes (internal/auth) accept only the bundle ids on it,
// so a name the server accepts is always an app that can exist. A client names
// its app by one of these and never supplies free text. Adding an app is a
// change here, and a build signed with an App ID that carries Associated
// Domains.
package appids

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
