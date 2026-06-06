package sources

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/sourcefetch"
)

type GDELTFetcher struct {
	Client    *http.Client
	UserAgent string
}

func NewGDELTFetcher(userAgent string) *GDELTFetcher {
	return &GDELTFetcher{
		Client:    sourcefetch.Client(60 * time.Second),
		UserAgent: userAgent,
	}
}

func (f *GDELTFetcher) Poll(ctx context.Context, source *Source) (PollResult, error) {
	started := time.Now().UTC()
	fetch := NewFetchRecord(*source, source.URL, started)
	if err := sourcefetch.ValidateURL(source.URL); err != nil {
		fetch = FinishFetch(fetch, 0, nil, err)
		return PollResult{Fetch: fetch}, err
	}
	// 1. Get the last update URL
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, source.URL, nil)
	if err != nil {
		fetch = FinishFetch(fetch, 0, nil, err)
		return PollResult{Fetch: fetch}, err
	}
	req.Header.Set("User-Agent", f.UserAgent)
	resp, err := f.Client.Do(req)
	if err != nil {
		fetch = FinishFetch(fetch, 0, nil, err)
		return PollResult{Fetch: fetch}, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fetch = FinishFetch(fetch, resp.StatusCode, nil, err)
		return PollResult{Fetch: fetch}, err
	}
	if resp.StatusCode != http.StatusOK {
		err := fmt.Errorf("gdelt returned status: %d", resp.StatusCode)
		fetch = FinishFetch(fetch, resp.StatusCode, body, err)
		return PollResult{Fetch: fetch}, err
	}

	// GDELT lastupdate.txt format:
	// 123456 abc http://...export.CSV.zip
	// 123456 abc http://...mentions.CSV.zip
	// 123456 abc http://...gkg.CSV.zip
	lines := strings.Split(string(body), "\n")
	if len(lines) < 3 {
		err := fmt.Errorf("unexpected GDELT lastupdate format")
		fetch = FinishFetch(fetch, resp.StatusCode, body, err)
		return PollResult{Fetch: fetch}, err
	}

	// We want the GKG (Global Knowledge Graph) for high-signal themes
	var gkgURL string
	for _, line := range lines {
		if strings.Contains(line, "gkg.csv.zip") {
			parts := strings.Fields(line)
			if len(parts) >= 3 {
				gkgURL = parts[2]
			}
		}
	}

	if gkgURL == "" {
		err := fmt.Errorf("GKG URL not found in GDELT lastupdate")
		fetch = FinishFetch(fetch, resp.StatusCode, body, err)
		return PollResult{Fetch: fetch}, err
	}

	// 2. Download and Unzip GKG file
	items, gkgBody, err := f.fetchGKG(ctx, gkgURL, source, fetch.FetchID)
	if err != nil {
		fetch = FinishFetch(fetch, resp.StatusCode, body, err)
		return PollResult{Fetch: fetch}, err
	}

	source.LastPolled = time.Now()
	fetch.CanonicalURL = NormalizeURL(gkgURL)
	fetch = FinishFetch(fetch, resp.StatusCode, append(body, gkgBody...), nil)
	fetch.ItemCount = len(items)
	return PollResult{Fetch: fetch, Items: items}, nil
}

func (f *GDELTFetcher) fetchGKG(ctx context.Context, url string, source *Source, fetchID string) ([]Item, []byte, error) {
	if err := sourcefetch.ValidateURL(url); err != nil {
		return nil, nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, nil, err
	}
	req.Header.Set("User-Agent", f.UserAgent)
	resp, err := f.Client.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()

	zipData, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, zipData, fmt.Errorf("gdelt gkg returned status: %d", resp.StatusCode)
	}

	r, err := zip.NewReader(bytes.NewReader(zipData), int64(len(zipData)))
	if err != nil {
		return nil, zipData, err
	}

	var items []Item
	for _, file := range r.File {
		if strings.HasSuffix(file.Name, ".csv") {
			rc, err := file.Open()
			if err != nil {
				return nil, zipData, err
			}
			defer rc.Close()

			reader := csv.NewReader(rc)
			reader.Comma = '\t'
			reader.LazyQuotes = true

			for {
				record, err := reader.Read()
				if err == io.EOF {
					break
				}
				if err != nil {
					continue
				}

				// GKG v2 columns:
				// 0: GKGRECORDID, 1: DATE, 2: SOURCECOLLECTIONID, 3: SOURCECOMMONNAME, 4: DOCUMENTIDENTIFIER,
				// 5: COUNTS, 6: V2COUNTS, 7: THEMES, 8: V2THEMES, 9: LOCATIONS, 10: V2LOCATIONS,
				// 11: PERSONS, 12: V2PERSONS, 13: ORGANIZATIONS, 14: V2ORGANIZATIONS, 15: V2TONE...
				if len(record) < 16 {
					continue
				}

				published, _ := time.Parse("20060102150405", record[1])

				item := Item{
					ID:            StableItemID(*source, record[0], record[4], record[3], record[7]),
					SourceID:      source.ID,
					SourceType:    source.Type,
					FetchID:       fetchID,
					OriginalID:    record[0],
					Title:         fmt.Sprintf("GDELT Event: %s", record[3]),
					Body:          fmt.Sprintf("Themes: %s\nOrganizations: %s\nLocations: %s", record[7], record[13], record[9]),
					URL:           record[4],
					CanonicalURL:  NormalizeURL(record[4]),
					Published:     published.UTC(),
					FetchedAt:     time.Now().UTC(),
					Verticals:     source.Verticals,
					Language:      firstString(source.Languages),
					Region:        firstString(source.Regions),
					EvidenceLevel: "source_feed",
				}
				item.ContentHash = ContentHash(item.Title, item.Body, item.CanonicalURL)
				items = append(items, item)

				if len(items) >= 100 { // Limit for toy version
					break
				}
			}
		}
	}

	return items, zipData, nil
}
