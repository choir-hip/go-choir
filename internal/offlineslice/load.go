package offlineslice

import (
	"bufio"
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"
)

func openCSV(path string) (*csv.Reader, *os.File, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, err
	}
	r := csv.NewReader(bufio.NewReaderSize(f, 1<<20))
	r.FieldsPerRecord = -1
	r.LazyQuotes = true
	return r, f, nil
}

func parseTime(s string) time.Time {
	t, err := time.Parse(time.RFC3339Nano, s)
	if err != nil {
		return time.Time{}
	}
	return t.UTC()
}

// ReadCaptures reads captures.csv (id, fetched, published, host, url_key,
// canonical_key, language, source_id, title, text_hash).
func ReadCaptures(path string) ([]Capture, error) {
	r, f, err := openCSV(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var out []Capture
	for {
		rec, err := r.Read()
		if err == io.EOF {
			return out, nil
		}
		if err != nil {
			return nil, err
		}
		if len(rec) < 10 {
			return nil, fmt.Errorf("captures: %d fields", len(rec))
		}
		out = append(out, Capture{ID: rec[0], Fetched: parseTime(rec[1]), Published: rec[2], Host: rec[3], URLKey: rec[4], CanonKey: rec[5],
			Language: rec[6], SourceID: rec[7], Title: rec[8], TextHash: strings.TrimPrefix(rec[9], "sha256:")})
	}
}

// ReadVectors reads vectors.csv (id, "[x,y,...]") into unit vectors by capture id.
func ReadVectors(path string) (map[string][]float32, error) {
	r, f, err := openCSV(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	out := map[string][]float32{}
	for {
		rec, err := r.Read()
		if err == io.EOF {
			return out, nil
		}
		if err != nil {
			return nil, err
		}
		if len(rec) < 2 {
			continue
		}
		parts := strings.Split(strings.Trim(rec[1], "[]"), ",")
		v := make([]float32, len(parts))
		for i, p := range parts {
			x, err := strconv.ParseFloat(strings.TrimSpace(p), 32)
			if err != nil {
				return nil, fmt.Errorf("vector of %s: %w", rec[0], err)
			}
			v[i] = float32(x)
		}
		out[rec[0]] = normalize(v)
	}
}

// ReadEntities reads entities.csv (canonical_id, created, updated, tombstone, metadata).
func ReadEntities(path string) ([]Entity, error) {
	r, f, err := openCSV(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var out []Entity
	for {
		rec, err := r.Read()
		if err == io.EOF {
			return out, nil
		}
		if err != nil {
			return nil, err
		}
		if len(rec) < 5 {
			continue
		}
		var m struct {
			EntityID string   `json:"entity_id"`
			Name     string   `json:"name"`
			Type     string   `json:"type"`
			Aliases  []string `json:"aliases"`
		}
		if json.Unmarshal([]byte(rec[4]), &m) != nil || m.EntityID == "" || m.Name == "" {
			continue
		}
		out = append(out, Entity{Canonical: rec[0], EntityID: m.EntityID, Name: m.Name, Type: m.Type, Aliases: m.Aliases,
			Created: parseTime(rec[1]), Updated: parseTime(rec[2]), Tombstone: rec[3] == "t"})
	}
}

// ReadStops reads stopwords.csv (canonical_id, created, updated, tombstone, body).
func ReadStops(path string) ([]Stop, error) {
	r, f, err := openCSV(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var out []Stop
	for {
		rec, err := r.Read()
		if err == io.EOF {
			return out, nil
		}
		if err != nil {
			return nil, err
		}
		if len(rec) < 5 || rec[3] == "t" {
			continue
		}
		var b struct {
			Key         string    `json:"key"`
			Active      bool      `json:"active"`
			ActivatedAt time.Time `json:"activated_at"`
			RevokedAt   time.Time `json:"revoked_at"`
		}
		if json.Unmarshal([]byte(rec[4]), &b) != nil || b.Key == "" {
			continue
		}
		s := Stop{Key: b.Key, Active: b.Active, Created: parseTime(rec[1])}
		if b.ActivatedAt.Year() > 1 {
			s.Activated = b.ActivatedAt.UTC()
		}
		if b.RevokedAt.Year() > 1 {
			s.Revoked = b.RevokedAt.UTC()
		}
		out = append(out, s)
	}
}

// ReadLeads reads bodies.bin (frames "H <hash> <n>\n" + n bytes; n = -1 for a
// missing file) and returns each present body's lead by text hash.
func ReadLeads(path string) (leads map[string]string, missing int, err error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, 0, err
	}
	defer f.Close()
	br := bufio.NewReaderSize(f, 1<<20)
	leads = map[string]string{}
	for {
		line, err := br.ReadBytes('\n')
		if err == io.EOF && len(line) == 0 {
			return leads, missing, nil
		}
		if err != nil {
			return nil, 0, err
		}
		fs := bytes.Fields(line)
		if len(fs) != 3 || string(fs[0]) != "H" {
			return nil, 0, fmt.Errorf("bodies: bad frame header %q", line)
		}
		n, err := strconv.Atoi(string(fs[2]))
		if err != nil {
			return nil, 0, err
		}
		if n < 0 {
			missing++
			continue
		}
		buf := make([]byte, n)
		if _, err := io.ReadFull(br, buf); err != nil {
			return nil, 0, err
		}
		leads[string(fs[1])] = LeadOf(buf)
	}
}
