package main

import (
	"database/sql"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/mmcdole/gofeed"
	_ "modernc.org/sqlite"
)

type Source struct {
	ID   string
	Type string
	URL  string
	Name string
}

// 34 validated sources across all verticals and regions
var sources = []Source{
	// Global wire / multilingual
	{"bbc_world",        "rss", "https://feeds.bbci.co.uk/news/world/rss.xml",                  "BBC World"},
	{"aljazeera",        "rss", "https://www.aljazeera.com/xml/rss/all.xml",                     "Al Jazeera"},
	{"france24_en",      "rss", "https://www.france24.com/en/rss",                               "France 24 EN"},
	{"dw_world",         "rss", "https://rss.dw.com/rdf/rss-en-world",                           "Deutsche Welle World"},
	{"rfi_en",           "rss", "https://www.rfi.fr/en/rss",                                     "RFI English"},

	// Asia
	{"nikkei",           "rss", "https://asia.nikkei.com/rss/feed/nar",                          "Nikkei Asia"},
	{"scmp",             "rss", "https://www.scmp.com/rss/91/feed",                              "SCMP China"},
	{"the_hindu",        "rss", "https://www.thehindu.com/feeder/default.rss",                   "The Hindu"},
	{"straits_times",    "rss", "https://www.straitstimes.com/news/world/rss.xml",               "Straits Times"},
	{"vietnam_news",     "rss", "https://vietnamnews.vn/rss/world.rss",                          "Vietnam News"},
	{"bangkok_post",     "rss", "https://www.bangkokpost.com/rss/data/topstories.xml",           "Bangkok Post"},
	{"dawn_pk",          "rss", "https://www.dawn.com/feeds/home",                               "Dawn Pakistan"},

	// Middle East / West Asia
	{"arab_news",        "rss", "https://www.arabnews.com/rss.xml",                              "Arab News"},
	{"middle_east_eye",  "rss", "https://www.middleeasteye.net/rss",                             "Middle East Eye"},
	{"al_monitor",       "rss", "https://www.al-monitor.com/rss",                                "Al-Monitor"},

	// Africa
	{"the_africa_report","rss", "https://www.theafricareport.com/feed/",                         "The Africa Report"},
	{"premium_times_ng", "rss", "https://www.premiumtimesng.com/feed",                           "Premium Times Nigeria"},
	{"nation_africa",    "rss", "https://nation.africa/kenya/rss.xml",                           "Nation Africa"},
	{"rfi_afrique",      "rss", "https://www.rfi.fr/fr/rss",                                     "RFI Afrique (FR)"},

	// Latin America
	{"mercopress",       "rss", "https://en.mercopress.com/rss",                                 "MercoPress"},
	{"folha_sp",         "rss", "https://feeds.folha.uol.com.br/mundo/rss091.xml",               "Folha de S.Paulo (PT)"},
	{"la_nacion_ar",     "rss", "https://www.lanacion.com.ar/arcio/rss/",                        "La Nación Argentina (ES)"},

	// AI / Tech
	{"the_verge",        "rss", "https://www.theverge.com/rss/index.xml",                        "The Verge"},
	{"techcrunch",       "rss", "https://techcrunch.com/feed/",                                  "TechCrunch"},
	{"mit_tech_review",  "rss", "https://www.technologyreview.com/feed/",                        "MIT Tech Review"},
	{"wired",            "rss", "https://www.wired.com/feed/rss",                                "Wired"},
	{"rest_of_world",    "rss", "https://restofworld.org/feed/",                                 "Rest of World"},

	// Public health
	{"stat_news",        "rss", "https://www.statnews.com/feed/",                                "STAT News"},

	// Economics / Markets
	{"ft_world",         "rss", "https://www.ft.com/world?format=rss",                           "Financial Times World"},
	{"economist",        "rss", "https://www.economist.com/latest/rss.xml",                      "The Economist"},

	// Conflict / Security
	{"icg_alerts",       "rss", "https://www.crisisgroup.org/rss.xml",                           "ICG Crisis Alerts"},
	{"bellingcat",       "rss", "https://www.bellingcat.com/feed/",                              "Bellingcat"},

	// Culture / Scenius
	{"aeon",             "rss", "https://aeon.co/feed.rss",                                      "Aeon Magazine"},
	{"the_baffler",      "rss", "https://thebaffler.com/feed",                                   "The Baffler"},

	// Telegram — conflict monitoring
	{"conflict_monitor", "telegram", "https://t.me/s/ConflictMonitor",                           "Conflict Monitor"},
	{"rybar_en",         "telegram", "https://t.me/s/rybar_en",                                     "Rybar English (TG)"},
	{"intel_slava",      "telegram", "https://t.me/s/intelslava",                                  "Intel Slava (TG)"},

	// West Africa / Sahel (French)
	{"jeune_afrique",    "rss", "https://www.jeuneafrique.com/feed/",                              "Jeune Afrique (FR)"},
	{"le_monde_afrique", "rss", "https://www.lemonde.fr/afrique/rss_full.xml",                    "Le Monde Afrique (FR)"},

	// South Asia
	{"thedailystar_bd",  "rss", "https://www.thedailystar.net/rss.xml",                           "Daily Star Bangladesh"},
	{"tribune_pk",       "rss", "https://tribune.com.pk/feed/home",                               "Tribune Pakistan"},

	// Latin America (Spanish/Portuguese investigative)
	{"el_pais_am",       "rss", "https://feeds.elpais.com/mrss-s/pages/ep/site/elpais.com/section/america/portada", "El País América (ES)"},
	{"agencia_publica",  "rss", "https://apublica.org/feed/",                                     "Agência Pública (PT)"},
	{"ciper_chile",      "rss", "https://www.ciperchile.cl/feed/",                                "CIPER Chile (ES)"},

	// Science / Research
	{"nature_news",      "rss", "https://www.nature.com/nature.rss",                              "Nature News"},
	{"science_daily",    "rss", "https://www.sciencedaily.com/rss/all.xml",                       "Science Daily"},

	// Ideas / Analysis
	{"project_syndicate","rss", "https://www.project-syndicate.org/rss",                         "Project Syndicate"},
	{"public_books",     "rss", "https://www.publicbooks.org/feed/",                              "Public Books"},
}

func initDB(db *sql.DB) error {
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS items (
		id TEXT PRIMARY KEY,
		source_id TEXT,
		title TEXT,
		body TEXT,
		url TEXT,
		fetched_at TEXT
	)`)
	if err != nil { return err }
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS issues (
		id TEXT PRIMARY KEY,
		timestamp TEXT,
		content TEXT
	)`)
	return err
}

func pollRSS(db *sql.DB, s Source) int {
	fp := gofeed.NewParser()
	fp.UserAgent = "ChoirVanguard/1.0 (+https://choir.news)"
	feed, err := fp.ParseURL(s.URL)
	if err != nil {
		log.Printf("  [%s] RSS error: %v", s.ID, err)
		return 0
	}
	count := 0
	for _, item := range feed.Items {
		id := fmt.Sprintf("%s:%s", s.ID, item.GUID)
		if item.GUID == "" {
			id = fmt.Sprintf("%s:%s", s.ID, item.Link)
		}
		_, err := db.Exec("INSERT OR IGNORE INTO items (id, source_id, title, body, url, fetched_at) VALUES (?, ?, ?, ?, ?, ?)",
			id, s.ID, item.Title, item.Description, item.Link, time.Now().Format(time.RFC3339))
		if err == nil {
			count++
		}
	}
	return count
}

func pollTelegram(db *sql.DB, s Source) int {
	client := &http.Client{Timeout: 10 * time.Second}
	req, _ := http.NewRequest("GET", s.URL, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (ChoirVanguard; +https://choir.news)")
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("  [%s] Telegram error: %v", s.ID, err)
		return 0
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	content := string(body)
	messages := strings.Split(content, "tgme_widget_message_text")
	count := 0
	for i, msg := range messages {
		if i == 0 { continue }
		end := strings.Index(msg, "</div>")
		if end == -1 { continue }
		text := msg[:end]
		id := fmt.Sprintf("%s:%d", s.ID, time.Now().UnixNano()+int64(i))
		_, err := db.Exec("INSERT OR IGNORE INTO items (id, source_id, title, body, url, fetched_at) VALUES (?, ?, ?, ?, ?, ?)",
			id, s.ID, "Telegram Update", text, s.URL, time.Now().Format(time.RFC3339))
		if err == nil {
			count++
		}
	}
	return count
}

func runIngest(db *sql.DB) int {
	total := 0
	for _, s := range sources {
		var n int
		if s.Type == "rss" {
			n = pollRSS(db, s)
		} else if s.Type == "telegram" {
			n = pollTelegram(db, s)
		}
		if n > 0 {
			log.Printf("  [%s] +%d new items", s.ID, n)
		}
		total += n
	}
	return total
}

func main() {
	log.Println("Choir Global Wire — Ingestion Engine starting")

	db, err := sql.Open("sqlite", "vanguard.db")
	if err != nil { log.Fatal(err) }
	defer db.Close()

	if err := initDB(db); err != nil { log.Fatal(err) }

	// Run immediately, then every 15 minutes
	for {
		log.Printf("── Ingest cycle starting at %s", time.Now().Format("15:04:05"))
		n := runIngest(db)
		log.Printf("── Ingest cycle done: %d new items total", n)

		var total int
		db.QueryRow("SELECT COUNT(*) FROM items").Scan(&total)
		log.Printf("── Database: %d total items", total)

		log.Println("── Sleeping 15 minutes until next cycle...")
		time.Sleep(15 * time.Minute)
	}
}
