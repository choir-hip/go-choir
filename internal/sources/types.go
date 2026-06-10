package sources

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"
)

type SourceType string

const (
	SourceTypeRSS        SourceType = "rss"
	SourceTypeTelegram   SourceType = "telegram"
	SourceTypeGDELT      SourceType = "gdelt"
	SourceTypePolymarket SourceType = "polymarket"
)

type Source struct {
	ID                  string     `json:"id"`
	Type                SourceType `json:"type"`
	Name                string     `json:"name"`
	URL                 string     `json:"url"`
	Verticals           []string   `json:"verticals"`
	Languages           []string   `json:"languages,omitempty"`
	Regions             []string   `json:"regions,omitempty"`
	Jurisdictions       []string   `json:"jurisdictions,omitempty"`
	Tier                string     `json:"tier,omitempty"`
	PollIntervalSeconds int        `json:"poll_interval_seconds"`
	MaxItemsPerPoll     int        `json:"max_items_per_poll,omitempty"`
	RateLimit           string     `json:"rate_limit,omitempty"`
	ConditionalMode     string     `json:"conditional_request_mode,omitempty"`
	UserAgent           string     `json:"user_agent,omitempty"`
	TOSClass            string     `json:"tos_class,omitempty"`
	RobotsPolicy        string     `json:"robots_policy,omitempty"`
	AuthPolicy          string     `json:"auth_policy,omitempty"`
	StoreBodyPolicy     string     `json:"store_body_policy,omitempty"`
	RetentionDays       int        `json:"retention_days,omitempty"`
	Official            bool       `json:"official,omitempty"`
	SourceStanding      string     `json:"source_standing,omitempty"`
	Status              string     `json:"status,omitempty"`
	LastPolled          time.Time  `json:"last_polled,omitempty"`
	LastETag            string     `json:"last_etag,omitempty"`
	LastModified        string     `json:"last_modified,omitempty"`
	LastAuxCursor       string     `json:"last_aux_cursor,omitempty"`
}

func (s Source) AllowsConditionalGET() bool {
	switch strings.TrimSpace(s.ConditionalMode) {
	case "none":
		return false
	default:
		return true
	}
}

func (s Source) EffectiveMaxItemsPerPoll(fallback int) int {
	if s.MaxItemsPerPoll > 0 {
		return s.MaxItemsPerPoll
	}
	if fallback > 0 {
		return fallback
	}
	return 100
}

type Item struct {
	ID                 string     `json:"id"`
	SourceID           string     `json:"source_id"`
	SourceType         SourceType `json:"source_type,omitempty"`
	FetchID            string     `json:"fetch_id,omitempty"`
	OriginalID         string     `json:"original_id"`
	Title              string     `json:"title"`
	Body               string     `json:"body"`
	URL                string     `json:"url"`
	CanonicalURL       string     `json:"canonical_url,omitempty"`
	Published          time.Time  `json:"published"`
	FetchedAt          time.Time  `json:"fetched_at"`
	Verticals          []string   `json:"verticals"`
	Language           string     `json:"language,omitempty"`
	Region             string     `json:"region,omitempty"`
	ContentHash        string     `json:"content_hash,omitempty"`
	BodyKind           string     `json:"body_kind,omitempty"`
	BodyLength         int        `json:"body_length,omitempty"`
	ReaderSnapshot     bool       `json:"reader_snapshot,omitempty"`
	SourceTOSClass     string     `json:"source_tos_class,omitempty"`
	SourceRobotsPolicy string     `json:"source_robots_policy,omitempty"`
	SourceAuthPolicy   string     `json:"source_auth_policy,omitempty"`
	StoreBodyPolicy    string     `json:"store_body_policy,omitempty"`
	RawJSON            string     `json:"raw_json,omitempty"`
	EvidenceLevel      string     `json:"evidence_level,omitempty"`
	VintagePolicy      string     `json:"vintage_policy,omitempty"`
	LookaheadStatus    string     `json:"lookahead_status,omitempty"`
	ReleaseDate        string     `json:"release_date,omitempty"`
}

const (
	BodyKindEmpty          = "empty"
	BodyKindFeedSummary    = "feed_summary"
	BodyKindMetadataPacket = "metadata_packet"
	BodyKindSocialPost     = "social_post"
	BodyKindSourceBody     = "source_body"
	BodyKindReaderSnapshot = "reader_snapshot"
)

func NormalizeItemBodyClassification(item Item) Item {
	body := strings.TrimSpace(item.Body)
	item.BodyLength = len([]rune(body))
	item.BodyKind = strings.TrimSpace(item.BodyKind)
	if item.BodyKind == "" {
		item.BodyKind = BodyKindForSourceType(item.SourceType, body)
	}
	item.ReaderSnapshot = item.ReaderSnapshot || item.BodyKind == BodyKindReaderSnapshot
	return item
}

func BodyKindForSourceType(sourceType SourceType, body string) string {
	if strings.TrimSpace(body) == "" {
		return BodyKindEmpty
	}
	switch sourceType {
	case SourceTypeRSS:
		return BodyKindFeedSummary
	case SourceTypeGDELT:
		return BodyKindMetadataPacket
	case SourceTypeTelegram:
		return BodyKindSocialPost
	default:
		return BodyKindSourceBody
	}
}

type Registry struct {
	UserAgent string   `json:"user_agent"`
	Sources   []Source `json:"sources"`
}

type FetchRecord struct {
	FetchID          string     `json:"fetch_id"`
	SourceID         string     `json:"source_id"`
	SourceType       SourceType `json:"source_type"`
	RequestURL       string     `json:"request_url"`
	CanonicalURL     string     `json:"canonical_url,omitempty"`
	StatusCode       int        `json:"status_code,omitempty"`
	Status           string     `json:"status"`
	StartedAt        time.Time  `json:"started_at"`
	EndedAt          time.Time  `json:"ended_at"`
	ResponseETag     string     `json:"response_etag,omitempty"`
	ResponseModified string     `json:"response_modified,omitempty"`
	ContentHash      string     `json:"content_hash,omitempty"`
	RawSnapshotRef   string     `json:"raw_snapshot_ref,omitempty"`
	ErrorClass       string     `json:"error_class,omitempty"`
	Error            string     `json:"error,omitempty"`
	ItemCount        int        `json:"item_count"`
}

type PollResult struct {
	Fetch FetchRecord `json:"fetch"`
	Items []Item      `json:"items"`
}

func (s Source) EffectiveUserAgent(registryUserAgent string) string {
	if strings.TrimSpace(s.UserAgent) != "" {
		return strings.TrimSpace(s.UserAgent)
	}
	return strings.TrimSpace(registryUserAgent)
}

func NormalizeURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	parsed.Fragment = ""
	return strings.TrimRight(parsed.String(), "/")
}

func ContentHash(parts ...string) string {
	h := sha256.New()
	for _, part := range parts {
		h.Write([]byte(strings.TrimSpace(part)))
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))
}

func StableItemID(source Source, originalID, itemURL, title, body string) string {
	sourceID := strings.TrimSpace(source.ID)
	identity := strings.TrimSpace(originalID)
	if identity == "" {
		identity = NormalizeURL(itemURL)
	}
	if identity == "" {
		identity = ContentHash(title, body)
	}
	sum := sha256.Sum256([]byte(sourceID + "|" + string(source.Type) + "|" + identity))
	return fmt.Sprintf("srcitem_%s", hex.EncodeToString(sum[:12]))
}

func NewFetchRecord(source Source, requestURL string, started time.Time) FetchRecord {
	requestURL = strings.TrimSpace(requestURL)
	sum := sha256.Sum256([]byte(source.ID + "|" + requestURL + "|" + started.UTC().Format(time.RFC3339Nano)))
	return FetchRecord{
		FetchID:      fmt.Sprintf("fetch_%s", hex.EncodeToString(sum[:12])),
		SourceID:     source.ID,
		SourceType:   source.Type,
		RequestURL:   requestURL,
		CanonicalURL: NormalizeURL(requestURL),
		Status:       "started",
		StartedAt:    started.UTC(),
	}
}

func FinishFetch(fetch FetchRecord, statusCode int, body []byte, err error) FetchRecord {
	fetch.EndedAt = time.Now().UTC()
	fetch.StatusCode = statusCode
	if len(body) > 0 {
		fetch.ContentHash = ContentHash(string(body))
		fetch.RawSnapshotRef = "sha256:" + fetch.ContentHash
	}
	if err != nil {
		fetch.Status = "error"
		fetch.ErrorClass = errorClass(err)
		fetch.Error = err.Error()
		return fetch
	}
	if statusCode == 304 {
		fetch.Status = "not_modified"
		return fetch
	}
	if statusCode >= 200 && statusCode < 300 {
		fetch.Status = "ok"
		return fetch
	}
	fetch.Status = "http_error"
	return fetch
}

func MarshalRaw(v any) string {
	data, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	return string(data)
}

func errorClass(err error) string {
	if err == nil {
		return ""
	}
	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "timeout"), strings.Contains(msg, "deadline"):
		return "timeout"
	case strings.Contains(msg, "parse"):
		return "parse_error"
	case strings.Contains(msg, "status"):
		return "http_error"
	default:
		return "error"
	}
}
