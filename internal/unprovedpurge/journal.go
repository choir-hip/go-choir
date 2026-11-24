package unprovedpurge

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"
)

// FileJournal appends one JSON line to a file before each irreversible step of a run (Journal), each fsynced: the
// accounts (hashed), the status of every planned service, the mail directories removed so far, and the row counts.
// A run that dies leaves the line of the step it was about to take; LastIncomplete reads it on the next run.
type FileJournal struct {
	f   *os.File
	key []byte
}

// OpenJournal opens (and creates) the receipt file and reads the key file, if one is named, BEFORE anything is
// deleted: a path or key that cannot be used stops the run while nothing has happened yet.
func OpenJournal(path, keyFile string) (*FileJournal, error) {
	if strings.TrimSpace(path) == "" {
		return nil, errors.New("a receipt path is required")
	}
	var key []byte
	if keyFile != "" {
		k, err := os.ReadFile(keyFile)
		if err != nil {
			return nil, fmt.Errorf("receipt key file: %w", err)
		}
		if len(k) == 0 {
			return nil, errors.New("receipt key file is empty")
		}
		key = k
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, fmt.Errorf("receipt file: %w", err)
	}
	return &FileJournal{f: f, key: key}, nil
}

func (j *FileJournal) Close() error { return j.f.Close() }

type journalLine struct {
	At       string           `json:"at"`
	Stage    string           `json:"stage"`
	Digest   string           `json:"digest"`
	Services []journalService `json:"services"`
	MailDirs int              `json:"mail_dirs"`
	Counts   map[string]int64 `json:"rows,omitempty"`
	IDs      []string         `json:"id_sha256,omitempty"`
	Emails   []string         `json:"email_hmac_sha256,omitempty"`
}

type journalService struct {
	Name   string `json:"name"`
	Status string `json:"status"` // pending, committed, failed
	Error  string `json:"error,omitempty"`
}

// Entry writes the line for a stage. The status of a service is "committed" or "failed" once it ran, else "pending".
func (j *FileJournal) Entry(stage string, res *Result) error {
	l := journalLine{At: time.Now().UTC().Format(time.RFC3339), Stage: stage, MailDirs: res.MailDirs}
	if res.Plan != nil {
		l.Digest = res.Plan.Digest
		l.Counts = map[string]int64{}
		for _, c := range res.Plan.Counts {
			l.Counts[c.Service+"."+c.Table+"."+c.Column] = c.Rows
		}
		ran := map[string]ServiceResult{}
		for _, r := range res.Services {
			ran[r.Name] = r
		}
		for _, name := range res.Plan.Services {
			st := journalService{Name: name, Status: "pending"}
			if r, ok := ran[name]; ok {
				st.Status = "failed"
				if r.Committed {
					st.Status = "committed"
				}
				if r.Err != nil {
					st.Error = r.Err.Error()
				}
			}
			l.Services = append(l.Services, st)
		}
		for _, c := range res.Plan.Purge {
			l.IDs = append(l.IDs, Hash(c.ID)) // the same form the dry run prints, so a line can be matched to the list approved
			if j.key != nil {
				l.Emails = append(l.Emails, EmailMAC(j.key, c.Email))
			}
		}
		sort.Strings(l.IDs)
		sort.Strings(l.Emails)
	}
	line, err := json.Marshal(l)
	if err != nil {
		return err
	}
	if _, err := j.f.Write(append(line, '\n')); err != nil {
		return err
	}
	return j.f.Sync()
}

// LastIncomplete reads a receipt file: if its last line is a step short of "done", it says which, so the next run
// knows the one before it died or stopped half way. A missing, empty or blank file is no earlier run. A last line
// that cannot be parsed (a write cut short by a crash) is reported with the last line that can.
func LastIncomplete(path string) (string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	text := strings.TrimSpace(string(b))
	if text == "" {
		return "", nil
	}
	lines := strings.Split(text, "\n")
	torn := false
	for i := len(lines) - 1; i >= 0; i-- {
		var last journalLine
		if err := json.Unmarshal([]byte(lines[i]), &last); err != nil {
			torn = true
			continue
		}
		if last.Stage == "done" && !torn {
			return "", nil
		}
		note := ""
		if torn {
			note = " (a later line is cut short)"
		}
		return fmt.Sprintf("the last run (digest %s, %s) ended at %q%s: some services may have committed (%v); finish it with a fresh dry run, keeping go-choir-auth and go-choir-maild stopped", last.Digest, last.At, last.Stage, note, last.Services), nil
	}
	return "an earlier receipt in " + path + " has no line that can be read; check it before going on", nil
}

// OpenSQLiteFile opens an existing SQLite file for the command: read-only for a dry run, with the write lock taken
// at begin (BEGIN IMMEDIATE) and secure_delete on for a real run. A path that is not an existing file is refused
// (a mistyped one would make an empty database).
func OpenSQLiteFile(path string, write, immediate bool) (*sql.DB, error) {
	if st, err := os.Stat(path); err != nil || st.IsDir() {
		return nil, fmt.Errorf("%s is not an existing database file (a mistyped path would make an empty one)", path)
	}
	dsn := "file:" + path + "?_pragma=busy_timeout(60000)&_pragma=foreign_keys(on)"
	if write {
		dsn += "&_pragma=secure_delete(on)"
	} else {
		dsn += "&mode=ro"
	}
	if immediate {
		dsn += "&_txlock=immediate"
	}
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	return db, nil
}
