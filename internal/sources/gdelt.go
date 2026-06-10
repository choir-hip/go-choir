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

func parseGDELTLastUpdate(body string) (gkgURL, mentionsURL string) {
	for _, line := range strings.Split(body, "\n") {
		lower := strings.ToLower(strings.TrimSpace(line))
		if lower == "" {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) < 3 {
			continue
		}
		url := parts[len(parts)-1]
		switch {
		case strings.Contains(lower, "gkg.csv.zip"):
			gkgURL = url
		case strings.Contains(lower, "mentions.csv.zip"):
			mentionsURL = url
		}
	}
	return gkgURL, mentionsURL
}

func (f *GDELTFetcher) Poll(ctx context.Context, source *Source) (PollResult, error) {
	started := time.Now().UTC()
	fetch := NewFetchRecord(*source, source.URL, started)
	if err := sourcefetch.ValidateURL(source.URL); err != nil {
		fetch = FinishFetch(fetch, 0, nil, err)
		return PollResult{Fetch: fetch}, err
	}
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

	gkgURL, mentionsURL := parseGDELTLastUpdate(string(body))
	if gkgURL == "" && mentionsURL == "" {
		err := fmt.Errorf("no GDELT GKG or mentions URL in lastupdate")
		fetch = FinishFetch(fetch, resp.StatusCode, body, err)
		return PollResult{Fetch: fetch}, err
	}

	skipGKG := gkgURL != "" && gkgURL == strings.TrimSpace(source.LastETag)
	skipMentions := mentionsURL != "" && mentionsURL == strings.TrimSpace(source.LastModified)
	if skipGKG && skipMentions {
		fetch = FinishFetch(fetch, http.StatusNotModified, body, nil)
		source.LastPolled = time.Now().UTC()
		return PollResult{Fetch: fetch}, nil
	}

	var items []Item
	var payload []byte
	payload = append(payload, body...)

	if gkgURL != "" && !skipGKG {
		gkgItems, gkgBody, err := f.fetchGKG(ctx, gkgURL, source, fetch.FetchID)
		if err != nil {
			fetch = FinishFetch(fetch, resp.StatusCode, body, err)
			return PollResult{Fetch: fetch}, err
		}
		items = append(items, gkgItems...)
		payload = append(payload, gkgBody...)
		source.LastETag = gkgURL
	}

	if mentionsURL != "" && !skipMentions {
		mentionItems, mentionBody, err := f.fetchMentions(ctx, mentionsURL, source, fetch.FetchID)
		if err != nil {
			fetch = FinishFetch(fetch, resp.StatusCode, body, err)
			return PollResult{Fetch: fetch}, err
		}
		items = append(items, mentionItems...)
		payload = append(payload, mentionBody...)
		source.LastModified = mentionsURL
	}

	source.LastPolled = time.Now().UTC()
	canonical := gkgURL
	if canonical == "" {
		canonical = mentionsURL
	}
	fetch.CanonicalURL = NormalizeURL(canonical)
	fetch = FinishFetch(fetch, resp.StatusCode, payload, nil)
	fetch.ItemCount = len(items)
	return PollResult{Fetch: fetch, Items: items}, nil
}

func (f *GDELTFetcher) fetchGKG(ctx context.Context, url string, source *Source, fetchID string) ([]Item, []byte, error) {
	zipData, err := f.downloadZip(ctx, url)
	if err != nil {
		return nil, nil, err
	}

	var items []Item
	maxItems := source.EffectiveMaxItemsPerPoll(500)
	for _, file := range zipData.files {
		if !strings.HasSuffix(file.Name, ".csv") {
			continue
		}
		rc, err := file.Open()
		if err != nil {
			return nil, zipData.raw, err
		}
		reader := csv.NewReader(rc)
		reader.Comma = '\t'
		reader.LazyQuotes = true

		for {
			record, err := reader.Read()
			if err == io.EOF {
				rc.Close()
				break
			}
			if err != nil {
				continue
			}
			if len(record) < 16 {
				continue
			}
			published, _ := time.Parse("20060102150405", record[1])
			body := fmt.Sprintf("Themes: %s\nOrganizations: %s\nLocations: %s", record[7], record[13], record[9])
			item := Item{
				ID:            StableItemID(*source, record[0], record[4], record[3], record[7]),
				SourceID:      source.ID,
				SourceType:    source.Type,
				FetchID:       fetchID,
				OriginalID:    record[0],
				Title:         fmt.Sprintf("GDELT Event: %s", record[3]),
				Body:          body,
				URL:           record[4],
				CanonicalURL:  NormalizeURL(record[4]),
				Published:     published.UTC(),
				FetchedAt:     time.Now().UTC(),
				Verticals:     source.Verticals,
				Language:      firstString(source.Languages),
				Region:        firstString(source.Regions),
				BodyKind:      BodyKindMetadataPacket,
				BodyLength:    len([]rune(strings.TrimSpace(body))),
				EvidenceLevel: "source_feed",
			}
			item.ContentHash = ContentHash(item.Title, item.Body, item.CanonicalURL)
			items = append(items, item)
			if len(items) >= maxItems {
				rc.Close()
				break
			}
		}
		if len(items) >= maxItems {
			break
		}
	}
	return items, zipData.raw, nil
}

func (f *GDELTFetcher) fetchMentions(ctx context.Context, url string, source *Source, fetchID string) ([]Item, []byte, error) {
	zipData, err := f.downloadZip(ctx, url)
	if err != nil {
		return nil, nil, err
	}

	var items []Item
	maxItems := source.EffectiveMaxItemsPerPoll(500)
	remaining := maxItems
	for _, file := range zipData.files {
		if !strings.HasSuffix(file.Name, ".csv") || remaining <= 0 {
			continue
		}
		rc, err := file.Open()
		if err != nil {
			return nil, zipData.raw, err
		}
		reader := csv.NewReader(rc)
		reader.Comma = '\t'
		reader.LazyQuotes = true

		for {
			record, err := reader.Read()
			if err == io.EOF {
				rc.Close()
				break
			}
			if err != nil || len(record) < 6 {
				continue
			}
			published, _ := time.Parse("20060102150405", record[1])
			sourceName := record[4]
			docID := record[5]
			body := fmt.Sprintf("Mention: %s\nDocument: %s", sourceName, docID)
			item := Item{
				ID:            StableItemID(*source, "mentions:"+record[0], docID, sourceName, record[2]),
				SourceID:      source.ID,
				SourceType:    source.Type,
				FetchID:       fetchID,
				OriginalID:    record[0],
				Title:         fmt.Sprintf("GDELT Mention: %s", sourceName),
				Body:          body,
				URL:           docID,
				CanonicalURL:  NormalizeURL(docID),
				Published:     published.UTC(),
				FetchedAt:     time.Now().UTC(),
				Verticals:     source.Verticals,
				Language:      firstString(source.Languages),
				Region:        firstString(source.Regions),
				BodyKind:      BodyKindMetadataPacket,
				BodyLength:    len([]rune(strings.TrimSpace(body))),
				EvidenceLevel: "source_feed",
			}
			item.ContentHash = ContentHash(item.Title, item.Body, item.CanonicalURL)
			items = append(items, item)
			remaining--
			if remaining <= 0 {
				rc.Close()
				break
			}
		}
	}
	return items, zipData.raw, nil
}

type gdeltZipPayload struct {
	files []*zip.File
	raw   []byte
}

func (f *GDELTFetcher) downloadZip(ctx context.Context, url string) (gdeltZipPayload, error) {
	if err := sourcefetch.ValidateURL(url); err != nil {
		return gdeltZipPayload{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return gdeltZipPayload{}, err
	}
	req.Header.Set("User-Agent", f.UserAgent)
	resp, err := f.Client.Do(req)
	if err != nil {
		return gdeltZipPayload{}, err
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return gdeltZipPayload{}, err
	}
	if resp.StatusCode != http.StatusOK {
		return gdeltZipPayload{raw: raw}, fmt.Errorf("gdelt zip returned status: %d", resp.StatusCode)
	}
	reader, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		return gdeltZipPayload{raw: raw}, err
	}
	return gdeltZipPayload{files: reader.File, raw: raw}, nil
}
