package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

var foundingDate = time.Date(2026, 5, 23, 0, 0, 0, 0, time.UTC)

type SourceRef struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	URL      string `json:"url"`
	SourceID string `json:"source"`
}

type IssueJSON struct {
	ID          string      `json:"id"`
	Timestamp   string      `json:"timestamp"`
	Content     string      `json:"content"`
	Sources     []SourceRef `json:"sources"`
	IssueNumber int         `json:"issue_number"`
	Volume      int         `json:"volume"`
}

type ArchiveEntry struct {
	ID          string `json:"id"`
	Timestamp   string `json:"timestamp"`
	Headline    string `json:"headline"`
	IssueNumber int    `json:"issue_number"`
}

func openDB() (*sql.DB, error) {
	return sql.Open("sqlite", "vanguard.db")
}

func calcIssueNumber(ts string) (int, int) {
	t, err := time.Parse("2006-01-02 15:04:05", ts)
	if err != nil {
		return 1, 1
	}
	minutesSinceFounding := int(t.Sub(foundingDate).Minutes())
	issueNum := (minutesSinceFounding / 15) + 1
	if issueNum < 1 {
		issueNum = 1
	}
	volume := (minutesSinceFounding/(60*24*7) + 1)
	if volume < 1 {
		volume = 1
	}
	return volume, issueNum
}

func extractFirstHeadline(content string) string {
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "## ") {
			return strings.TrimPrefix(line, "## ")
		}
	}
	return "Choir Global Wire"
}

func populateSources(db *sql.DB, issue *IssueJSON) {
	rows, err := db.Query("SELECT title, url, source_id FROM items ORDER BY fetched_at DESC LIMIT 60")
	if err != nil {
		return
	}
	defer rows.Close()
	i := 1
	for rows.Next() {
		var title, url, sourceID string
		rows.Scan(&title, &url, &sourceID)
		issue.Sources = append(issue.Sources, SourceRef{
			ID:       fmt.Sprintf("S%d", i),
			Title:    title,
			URL:      url,
			SourceID: sourceID,
		})
		i++
	}
}

func handleAPILatest(w http.ResponseWriter, r *http.Request) {
	db, err := openDB()
	if err != nil {
		http.Error(w, `{"error":"db"}`, 500)
		return
	}
	defer db.Close()

	var issue IssueJSON
	err = db.QueryRow("SELECT id, timestamp, content FROM issues ORDER BY timestamp DESC LIMIT 1").
		Scan(&issue.ID, &issue.Timestamp, &issue.Content)
	if err == sql.ErrNoRows {
		issue.Content = "No issues yet."
		issue.Timestamp = time.Now().Format("2006-01-02 15:04:05")
	} else if err != nil {
		http.Error(w, `{"error":"query"}`, 500)
		return
	}

	issue.Volume, issue.IssueNumber = calcIssueNumber(issue.Timestamp)
	populateSources(db, &issue)

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-cache")
	json.NewEncoder(w).Encode(issue)
}

func handleAPIIssue(w http.ResponseWriter, r *http.Request) {
	issueID := strings.TrimPrefix(r.URL.Path, "/api/issue/")
	if issueID == "" {
		http.Error(w, `{"error":"missing id"}`, 400)
		return
	}
	db, err := openDB()
	if err != nil {
		http.Error(w, `{"error":"db"}`, 500)
		return
	}
	defer db.Close()

	var issue IssueJSON
	err = db.QueryRow("SELECT id, timestamp, content FROM issues WHERE id=?", issueID).
		Scan(&issue.ID, &issue.Timestamp, &issue.Content)
	if err == sql.ErrNoRows {
		http.Error(w, `{"error":"not found"}`, 404)
		return
	} else if err != nil {
		http.Error(w, `{"error":"query"}`, 500)
		return
	}

	issue.Volume, issue.IssueNumber = calcIssueNumber(issue.Timestamp)
	populateSources(db, &issue)

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "max-age=3600")
	json.NewEncoder(w).Encode(issue)
}

func handleAPIArchive(w http.ResponseWriter, r *http.Request) {
	db, err := openDB()
	if err != nil {
		http.Error(w, `{"error":"db"}`, 500)
		return
	}
	defer db.Close()

	rows, err := db.Query("SELECT id, timestamp, content FROM issues ORDER BY timestamp DESC")
	if err != nil {
		http.Error(w, `{"error":"query"}`, 500)
		return
	}
	defer rows.Close()

	var entries []ArchiveEntry
	for rows.Next() {
		var id, ts, content string
		rows.Scan(&id, &ts, &content)
		_, issueNum := calcIssueNumber(ts)
		entries = append(entries, ArchiveEntry{
			ID:          id,
			Timestamp:   ts,
			Headline:    extractFirstHeadline(content),
			IssueNumber: issueNum,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-cache")
	json.NewEncoder(w).Encode(entries)
}

func main() {
	log.Println("Starting Choir Global Wire server on :8080")

	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))
	http.HandleFunc("/api/latest", handleAPILatest)
	http.HandleFunc("/api/issue/", handleAPIIssue)
	http.HandleFunc("/api/archive", handleAPIArchive)
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/about" {
			http.ServeFile(w, r, "static/about.html")
			return
		}
		http.ServeFile(w, r, "static/index.html")
	})

	log.Fatal(http.ListenAndServe(":8080", nil))
}
