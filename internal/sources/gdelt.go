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
)

type GDELTFetcher struct {
	Client    *http.Client
	UserAgent string
}

func NewGDELTFetcher(userAgent string) *GDELTFetcher {
	return &GDELTFetcher{
		Client: &http.Client{
			Timeout: 60 * time.Second,
		},
		UserAgent: userAgent,
	}
}

func (f *GDELTFetcher) Poll(ctx context.Context, source *Source) ([]Item, error) {
	// 1. Get the last update URL
	resp, err := f.Client.Get(source.URL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	// GDELT lastupdate.txt format:
	// 123456 abc http://...export.CSV.zip
	// 123456 abc http://...mentions.CSV.zip
	// 123456 abc http://...gkg.CSV.zip
	lines := strings.Split(string(body), "\n")
	if len(lines) < 3 {
		return nil, fmt.Errorf("unexpected GDELT lastupdate format")
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
		return nil, fmt.Errorf("GKG URL not found in GDELT lastupdate")
	}

	// 2. Download and Unzip GKG file
	items, err := f.fetchGKG(ctx, gkgURL, source)
	if err != nil {
		return nil, err
	}

	source.LastPolled = time.Now()
	return items, nil
}

func (f *GDELTFetcher) fetchGKG(ctx context.Context, url string, source *Source) ([]Item, error) {
	resp, err := f.Client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	zipData, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	r, err := zip.NewReader(bytes.NewReader(zipData), int64(len(zipData)))
	if err != nil {
		return nil, err
	}

	var items []Item
	for _, file := range r.File {
		if strings.HasSuffix(file.Name, ".csv") {
			rc, err := file.Open()
			if err != nil {
				return nil, err
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
					ID:         fmt.Sprintf("gdelt:%s", record[0]),
					SourceID:   source.ID,
					OriginalID: record[0],
					Title:      fmt.Sprintf("GDELT Event: %s", record[3]),
					Body:       fmt.Sprintf("Themes: %s\nOrganizations: %s\nLocations: %s", record[7], record[13], record[9]),
					URL:        record[4],
					Published:  published,
					FetchedAt:  time.Now(),
					Verticals:  source.Verticals,
				}
				items = append(items, item)

				if len(items) >= 100 { // Limit for toy version
					break
				}
			}
		}
	}

	return items, nil
}
