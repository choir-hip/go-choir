package sources

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// Per-item language (lane Q). Ways it can fail, written before the code:
//   - an English item is called another language because of a few shared
//     words, so English stories are held or relabelled;
//   - a French or German item keeps its source's "en" label;
//   - a short or unsure text is guessed anyway, replacing a correct source
//     label with noise;
//   - an English story with a quoted foreign sentence is called foreign;
//   - a story wholly in another script is not noticed.
func TestGuessLanguage(t *testing.T) {
	cases := []struct{ name, text, want string }{
		{"english", "The council said that it will vote on the new bridge plan after the budget review is done.", "en"},
		{"french", "Le conseil municipal a annoncé que le pont sera fermé pendant les travaux, et les habitants sont invités à utiliser la nouvelle route.", "fr"},
		{"german", "Der Stadtrat hat beschlossen, dass die Brücke nicht geschlossen wird und die Anwohner sich auf eine neue Route einstellen müssen.", "de"},
		{"spanish", "El consejo anunció que el puente será cerrado por las obras y que los vecinos deben usar la nueva ruta para llegar al centro.", "es"},
		{"indonesian", "Dewan kota mengumumkan bahwa jembatan akan ditutup selama pekerjaan perbaikan, dan warga diminta untuk menggunakan jalan baru yang dibuka dari pusat kota.", "id"},
		{"hausa", "Majalisar birnin ta sanar da cewa za a rufe gadar saboda aikin gyara, kuma mazauna yankin suna da damar amfani da sabuwar hanya daga cikin gari domin zuwa wurin aiki, amma ba a bayyana lokaci ba.", "ha"},
		{"too short", "Le pont est fermé", ""},
		{"empty", "", ""},
		{"other script", "Правительство объявило новый план по ремонту моста через реку в следующем году", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := GuessLanguage(tc.text); got != tc.want {
				t.Fatalf("GuessLanguage(%q) = %q, want %q", tc.text, got, tc.want)
			}
		})
	}
}

func TestLooksNotEnglish(t *testing.T) {
	english := "The council said that it will vote on the new bridge plan after the budget review is done, and residents have been told that the road will stay open for the whole of the winter while the work is carried out."
	french := "Le conseil municipal a annoncé que le pont sera fermé pendant les travaux, et les habitants sont invités à utiliser la nouvelle route pour se rendre au centre, selon un communiqué publié par la mairie dans la matinée."
	cases := []struct {
		name, text string
		want       bool
	}{
		{"english", english, false},
		{"french", french, true},
		{"english with a quoted french sentence", english + ` The mayor added: "Le pont sera fermé pendant les travaux." Officials said the date has not been set.`, false},
		{"indonesian", strings.Repeat("Dewan kota mengumumkan bahwa jembatan akan ditutup selama pekerjaan perbaikan, dan warga diminta untuk menggunakan jalan baru yang dibuka dari pusat kota. ", 2), true},
		{"hausa", strings.Repeat("Majalisar birnin ta sanar da cewa za a rufe gadar saboda aikin gyara, kuma mazauna yankin suna da damar amfani da sabuwar hanya daga cikin gari domin zuwa wurin aiki. ", 2), true},
		{"russian", "Правительство объявило новый план по ремонту моста через реку в следующем году и сообщило о сроках работ", true},
		{"chinese", "市议会宣布大桥将在施工期间关闭，居民被要求使用新的道路前往市中心", true},
		{"short french headline", "Le pont est fermé", false},
		{"numbers only", "2026 11 10 42 17", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := LooksNotEnglish(tc.text); got != tc.want {
				t.Fatalf("LooksNotEnglish = %v, want %v", got, tc.want)
			}
		})
	}
}

// A feed labelled English that carries a French item stamps French on that
// item, and keeps the source's label on an item whose text cannot say.
func TestRSSPollerItemLanguageComesFromItsText(t *testing.T) {
	allowPrivateSourceFetchForTest(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`<?xml version="1.0"?>
<rss version="2.0"><channel><title>Mixed</title>
<item><title>Le conseil annonce la fermeture du pont</title><link>https://example.test/a</link>
<description>Le conseil municipal a annoncé que le pont sera fermé pendant les travaux, et les habitants sont invités à utiliser la nouvelle route.</description></item>
<item><title>Bridge</title><link>https://example.test/b</link><description>Closed.</description></item>
</channel></rss>`))
	}))
	defer server.Close()
	source := &Source{ID: "rss:mixed", Type: SourceTypeRSS, URL: server.URL, Languages: []string{"en"}}
	res, err := NewRSSPoller("ChoirTest/1.0").Poll(context.Background(), source)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Items) != 2 {
		t.Fatalf("items = %d", len(res.Items))
	}
	if got := res.Items[0].Language; got != "fr" {
		t.Errorf("French item language = %q, want fr", got)
	}
	if got := res.Items[1].Language; got != "en" {
		t.Errorf("unsure item language = %q, want the source's en", got)
	}
	if !strings.Contains(res.Items[0].Body, "pont") {
		t.Errorf("body lost: %q", res.Items[0].Body)
	}
}
