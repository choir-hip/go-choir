package proxy

import (
	"encoding/json"
	"net/http"
)

// appleAppSiteAssociationPath is where Apple fetches (through its CDN) the
// file that ties this site to the phone app for passkeys. It is public, needs
// no authentication, must answer 200 with no redirect, and is served as JSON.
const appleAppSiteAssociationPath = "/.well-known/apple-app-site-association"

// appleWebCredentialsApps are the apps allowed to use choir.news passkeys
// (webcredentials): team S47GERGJZ3 and the three bundle identifiers of the
// phone app (release, lab, lab one). Adding an app is a change here.
var appleWebCredentialsApps = []string{
	"S47GERGJZ3.news.choir.autoradio",
	"S47GERGJZ3.news.choir.autoradio.lab",
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
	w.WriteHeader(http.StatusOK)
	if r.Method == http.MethodGet {
		_, _ = w.Write(appleAppSiteAssociationBody)
	}
}
