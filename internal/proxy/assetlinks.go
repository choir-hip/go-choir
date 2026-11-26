package proxy

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/yusefmosiah/go-choir/internal/appids"
)

// assetlinksPath is where Android verifies the package and signing certificate
// that may use this site's passkeys. It is public, needs no
// authentication, must answer 200 with no redirect, and is served as JSON.
const assetlinksPath = "/.well-known/assetlinks.json"

type assetlinksTarget struct {
	Namespace              string   `json:"namespace"`
	PackageName            string   `json:"package_name"`
	SHA256CertFingerprints []string `json:"sha256_cert_fingerprints"`
}

type assetlinksStatement struct {
	Relation []string          `json:"relation"`
	Target   assetlinksTarget `json:"target"`
}

// assetlinksBody is built once from the canonical Android package/certificate
// record. The relation order and field order are part of the exact served file.
var assetlinksBody = func() []byte {
	statements := make([]assetlinksStatement, 0, len(appids.AndroidPasskeyApps))
	for _, app := range appids.AndroidPasskeyApps {
		if _, err := appids.AndroidAPKKeyHashOrigin(app); err != nil { // same bytes, same validation as the accepted origin
			panic(err)
		}
		statements = append(statements, assetlinksStatement{
			Relation: []string{"delegate_permission/common.get_login_creds"},
			Target: assetlinksTarget{
				Namespace:              "android_app",
				PackageName:            app.Package,
				SHA256CertFingerprints: []string{app.SHA256Fingerprint()},
			},
		})
	}
	b, err := json.MarshalIndent(statements, "", "  ")
	if err != nil {
		panic(err)
	}
	return append(b, '\n')
}()

// HandleAndroidAssetLinks serves the file at its exact path, ahead of the
// catch-all computer-surface route, so no sign-in check, redirect or HTML shell
// can wrap it.
func (h *Handler) HandleAndroidAssetLinks(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "public, max-age=300")
	w.Header().Set("Content-Length", strconv.Itoa(len(assetlinksBody)))
	w.WriteHeader(http.StatusOK)
	if r.Method == http.MethodGet {
		_, _ = w.Write(assetlinksBody)
	}
}
