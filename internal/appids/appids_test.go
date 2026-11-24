package appids

import "testing"

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
