package appids

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

// A name the server accepts is an app that can exist: the free-team lab app
// (QSGU6VRJL6) cannot carry Associated Domains, so it is not accepted, and the
// association file names exactly the accepted apps on their one team.
func TestAcceptedAppsAreTheOnesTheAssociationFileNames(t *testing.T) {
	for _, bundle := range []string{"news.choir.autoradio", "news.choir.autoradio.lab.one"} {
		if !Known(bundle) {
			t.Fatalf("%q should be accepted", bundle)
		}
	}
	for _, bundle := range []string{"news.choir.autoradio.lab", "", "evil.example", "S47GERGJZ3.news.choir.autoradio"} {
		if Known(bundle) {
			t.Fatalf("%q must not be accepted", bundle)
		}
	}
	got := WebCredentialApps()
	if len(got) != 2 || got[0] != "S47GERGJZ3.news.choir.autoradio" || got[1] != "S47GERGJZ3.news.choir.autoradio.lab.one" {
		t.Fatalf("association apps = %v", got)
	}
}

// The phone app reads its own list (mobile/AutoradioLab/passkey-apps.json) to say where
// passkeys cannot work; it must be the list the server accepts and serves.
func TestTheAppsOwnListIsTheServersList(t *testing.T) {
	raw, err := os.ReadFile("../../mobile/AutoradioLab/passkey-apps.json")
	if err != nil {
		t.Fatal(err)
	}
	var f struct {
		Apps    []struct{ Team, Bundle string } `json:"apps"`
		Android []string                       `json:"android"`
	}
	if err := json.Unmarshal(raw, &f); err != nil {
		t.Fatal(err)
	}
	if len(f.Apps) != len(PasskeyApps) {
		t.Fatalf("app list has %d apps, server has %d", len(f.Apps), len(PasskeyApps))
	}
	for i, a := range f.Apps {
		if a.Team != PasskeyApps[i].Team || a.Bundle != PasskeyApps[i].Bundle {
			t.Fatalf("app %d: %+v, server %+v", i, a, PasskeyApps[i])
		}
	}
	if len(f.Android) != len(AndroidPasskeyApps) {
		t.Fatalf("Android list has %d apps, server has %d", len(f.Android), len(AndroidPasskeyApps))
	}
	for i, pkg := range f.Android {
		if pkg != AndroidPasskeyApps[i].Package {
			t.Fatalf("Android app %d: %q, server %q", i, pkg, AndroidPasskeyApps[i].Package)
		}
	}
}

// The Android identity is the physical A0 artifact, not a second hand-typed
// package list: app.json's lab-one variant and the apksigner evidence must
// agree with the one record that drives assetlinks and the accepted origin.
func TestAndroidPasskeyIdentityMatchesManifestAndA0Evidence(t *testing.T) {
	if len(AndroidPasskeyApps) != 1 {
		t.Fatalf("Android passkey apps = %d, want 1", len(AndroidPasskeyApps))
	}
	got := AndroidPasskeyApps[0]
	if got.Package != "news.choir.autoradio.lab.one" {
		t.Fatalf("Android package = %q", got.Package)
	}
	if got.CertSHA256 != "dfe9767c642a2f864b297049bde97c0836cb58d5ce516ffe7a7da11f6fec9ea7" {
		t.Fatalf("Android certificate = %q", got.CertSHA256)
	}

	raw, err := os.ReadFile("../../mobile/AutoradioLab/app.json")
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		App struct{ ID string } `json:"app"`
		Variants map[string]struct {
			IDSuffix string            `json:"idSuffix"`
			Env      map[string]string `json:"env"`
		} `json:"variants"`
	}
	if err := json.Unmarshal(raw, &manifest); err != nil {
		t.Fatal(err)
	}
	lab := manifest.Variants["lab-one"]
	if lab.IDSuffix != ".lab.one" || manifest.App.ID+lab.IDSuffix != got.Package || lab.Env["APP_ID"] != got.Package {
		t.Fatalf("lab-one manifest = %q + %q / %q, want %q", manifest.App.ID, lab.IDSuffix, lab.Env["APP_ID"], got.Package)
	}

	evidence, err := os.ReadFile("../../docs/evidence/android-a0-2026-11-26/artifact.txt")
	if err != nil {
		t.Fatal(err)
	}
	var fingerprint string
	for _, line := range strings.Split(string(evidence), "\n") {
		if value, ok := strings.CutPrefix(line, "signing_certificate_sha256="); ok {
			fingerprint = value
		}
	}
	if fingerprint != got.CertSHA256 {
		t.Fatalf("A0 signing certificate = %q, canonical %q", fingerprint, got.CertSHA256)
	}
	origin, err := AndroidAPKKeyHashOrigin(got)
	if err != nil {
		t.Fatal(err)
	}
	if want := "android:apk-key-hash:3-l2fGQqL4ZLKXBJvel8CDbLWNXOUW_-en2hH2_snqc"; origin != want {
		t.Fatalf("Android origin = %q, want %q", origin, want)
	}
	if want := "DF:E9:76:7C:64:2A:2F:86:4B:29:70:49:BD:E9:7C:08:36:CB:58:D5:CE:51:6F:FE:7A:7D:A1:1F:6F:EC:9E:A7"; got.SHA256Fingerprint() != want {
		t.Fatalf("assetlinks fingerprint = %q, want %q", got.SHA256Fingerprint(), want)
	}
}

func TestAndroidAPKKeyHashOriginRefusesAnInvalidCertificate(t *testing.T) {
	for _, cert := range []string{"", "abc", strings.Repeat("0", 63), strings.Repeat("g", 64)} {
		if origin, err := AndroidAPKKeyHashOrigin(AndroidApp{Package: "news.choir.autoradio.lab.one", CertSHA256: cert}); err == nil || origin != "" {
			t.Fatalf("certificate %q produced origin %q, error %v", cert, origin, err)
		}
	}
}

// The Android record adds an association and an accepted origin. It does not
// turn the finish route's client-supplied app label into an authorization or
// add a production Android package to the fixed Apple list.
func TestAndroidRecordDoesNotChangeAppLabelAuthorization(t *testing.T) {
	if !Known("news.choir.autoradio.lab.one") || Known("news.choir.autoradio") != true || Known("news.choir.autoradio.lab") {
		t.Fatal("the fixed app-label list changed")
	}
	got := WebCredentialApps()
	if len(got) != 2 || got[0] != "S47GERGJZ3.news.choir.autoradio" || got[1] != "S47GERGJZ3.news.choir.autoradio.lab.one" {
		t.Fatalf("Apple association apps = %v", got)
	}
}
