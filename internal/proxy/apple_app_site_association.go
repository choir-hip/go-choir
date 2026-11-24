package proxy

import (
	"encoding/json"
	"net/http"
	"strconv"
)

// appleAppSiteAssociationPath is where Apple fetches (through its CDN) the
// file that ties this site to the phone app for passkeys. It is public, needs
// no authentication, must answer 200 with no redirect, and is served as JSON.
const appleAppSiteAssociationPath = "/.well-known/apple-app-site-association"

// appleWebCredentialsApps are the apps allowed to use choir.news passkeys
// (webcredentials), each as <team>.<bundle id>, team S47GERGJZ3:
//   - news.choir.autoradio: the App Store app; its App ID has
//     Associated Domains enabled in the developer account.
//   - news.choir.autoradio.lab.one: the development app on the same team. It
//     matches only in a build signed with an explicit App ID that carries
//     Associated Domains; a wildcard profile does not.
//
// The first development app, news.choir.autoradio.lab, is not listed: it is
// signed by a free team (mobile/README.md), which cannot carry Associated
// Domains, so no entry could ever match it. Apple caches this file for about
// a day; remove an id here when its app is retired.
var appleWebCredentialsApps = []string{
	"S47GERGJZ3.news.choir.autoradio",
	"S47GERGJZ3.news.choir.autoradio.lab.one",
}

// appleAppSiteAssociationBody is the served file, built once from the list.
var appleAppSiteAssociationBody = func() []byte {
	b, err := json.Marshal(map[string]any{"webcredentials": map[string]any{"apps": appleWebCredentialsApps}})
	if err != nil {
		panic(err)
	}
	return append(b, '\n')
}()

// HandleAppleAppSiteAssociation serves the file. It is registered at the exact
// path, ahead of the catch-all computer-surface route, so no sign-in check,
// redirect or HTML shell can wrap it.
func (h *Handler) HandleAppleAppSiteAssociation(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "public, max-age=300")
	w.Header().Set("Content-Length", strconv.Itoa(len(appleAppSiteAssociationBody)))
	w.WriteHeader(http.StatusOK)
	if r.Method == http.MethodGet {
		_, _ = w.Write(appleAppSiteAssociationBody)
	}
}
