package appids

import (
	"encoding/json"
	"os"
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
		Apps []struct{ Team, Bundle string } `json:"apps"`
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
}
