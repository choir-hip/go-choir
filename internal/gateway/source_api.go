package gateway

// POST /provider/v1/source-api is the source analogue of /provider/v1/search:
// a guest asks for one named endpoint of one keyed source API, and the
// gateway builds the upstream request and adds the key on the host, so no
// source key ever reaches a guest
// (docs/problems/keyed-source-apis-2026-10-20.md).
//
// The guest names a provider, an endpoint and typed parameters, never a URL,
// host, header or path, so the route cannot carry a key to a host the guest
// picked. Each provider has a daily budget, shared by every computer.

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"
)

const (
	// sourceAPIMaxBody is the most of an upstream body the gateway returns.
	sourceAPIMaxBody = 2 << 20
	// sourceAPIMaxRequest bounds the guest's request body.
	sourceAPIMaxRequest = 64 << 10
	sourceAPITimeout    = 30 * time.Second
	sourceAPIUserAgent  = "ChoirSourceAPI/0.1 (+https://choir.news)"
	// EnvSourceAPIDaily overrides the per-provider daily budgets, as
	// "fred=2000,newsapi_ai=50".
	EnvSourceAPIDaily = "CHOIR_SOURCE_API_DAILY"
)

// SourceAPIRequest is the guest's request.
type SourceAPIRequest struct {
	Provider string                     `json:"provider"`
	Endpoint string                     `json:"endpoint"`
	Params   map[string]json.RawMessage `json:"params"`
}

// SourceAPIResponse is the gateway's answer once the upstream was reached.
// Body holds a whole JSON answer; Text holds anything else, or a cut one.
type SourceAPIResponse struct {
	Provider       string          `json:"provider"`
	Endpoint       string          `json:"endpoint"`
	UpstreamStatus int             `json:"upstream_status"`
	Body           json.RawMessage `json:"body,omitempty"`
	Text           string          `json:"text,omitempty"`
	Truncated      bool            `json:"truncated,omitempty"`
	Error          string          `json:"error,omitempty"`
}

type sourceParamKind int

const (
	paramInt sourceParamKind = iota
	paramText
	paramID
	paramIDList
	paramEnum
	paramDate
	paramDateTime
)

type sourceParamPlace int

const (
	inQuery sourceParamPlace = iota
	inPath
	inBody
)

type sourceParam struct {
	name     string // the guest's name
	upstream string // the upstream name; empty means name
	kind     sourceParamKind
	place    sourceParamPlace
	required bool
	min, max int            // paramInt range; paramText and paramIDList max length/count
	pattern  *regexp.Regexp // paramID and paramIDList items
	enum     []string
}

type keyPlace int

const (
	keyHeader keyPlace = iota
	keyQuery
	keyBody
)

type sourceEndpoint struct {
	method string
	// path is the upstream path; {name} is a required path parameter and
	// {name?} an optional one (dropped with its leading slash when absent).
	path     string
	params   []sourceParam
	fixed    map[string]any // fixed query values (GET) or body fields (POST)
	keyPlace *keyPlace      // overrides the provider's key place
	keyName  string
	// free endpoints cost the provider nothing and are not counted.
	free bool
}

type sourceProvider struct {
	name      string
	envKey    string
	base      string
	keyPlace  keyPlace
	keyName   string
	keyPrefix string
	daily     int
	endpoints map[string]sourceEndpoint
}

var (
	reSeriesID  = regexp.MustCompile(`^[A-Za-z0-9_]{1,64}$`)
	reVideoID   = regexp.MustCompile(`^[A-Za-z0-9_-]{11}$`)
	reChannelID = regexp.MustCompile(`^UC[A-Za-z0-9_-]{22}$`)
	reHandle    = regexp.MustCompile(`^@[A-Za-z0-9._-]{3,100}$`)
	rePlaylist  = regexp.MustCompile(`^[A-Za-z0-9_-]{2,64}$`)
	rePageToken = regexp.MustCompile(`^[A-Za-z0-9_-]{1,128}$`)
	reCourt     = regexp.MustCompile(`^[a-z0-9]{2,20}( [a-z0-9]{2,20}){0,19}$`)
	reLang3     = regexp.MustCompile(`^[a-z]{3}$`)
)

func keyPlaceRef(p keyPlace) *keyPlace { return &p }

// sourceProviders is the whole surface: nothing outside this table can be
// asked for.
func sourceProviders() map[string]*sourceProvider {
	return map[string]*sourceProvider{
		"fred": {
			name: "fred", envKey: "FRED_API_KEY", base: "https://api.stlouisfed.org",
			keyPlace: keyQuery, keyName: "api_key", daily: 2000,
			endpoints: map[string]sourceEndpoint{
				"series": {method: http.MethodGet, path: "/fred/series", fixed: map[string]any{"file_type": "json"},
					params: []sourceParam{{name: "series_id", kind: paramID, pattern: reSeriesID, required: true}}},
				"series_observations": {method: http.MethodGet, path: "/fred/series/observations", fixed: map[string]any{"file_type": "json"},
					params: []sourceParam{
						{name: "series_id", kind: paramID, pattern: reSeriesID, required: true},
						{name: "observation_start", kind: paramDate},
						{name: "observation_end", kind: paramDate},
						{name: "limit", kind: paramInt, min: 1, max: 1000},
						{name: "sort_order", kind: paramEnum, enum: []string{"asc", "desc"}},
						{name: "units", kind: paramEnum, enum: []string{"lin", "chg", "ch1", "pch", "pc1", "pca", "cch", "cca", "log"}},
					}},
				"series_search": {method: http.MethodGet, path: "/fred/series/search", fixed: map[string]any{"file_type": "json"},
					params: []sourceParam{
						{name: "search_text", kind: paramText, max: 200, required: true},
						{name: "limit", kind: paramInt, min: 1, max: 100},
						{name: "order_by", kind: paramEnum, enum: []string{"search_rank", "popularity", "last_updated", "observation_end"}},
					}},
			},
		},
		"congress": {
			name: "congress", envKey: "API_DATA_GOV_KEY", base: "https://api.congress.gov",
			keyPlace: keyHeader, keyName: "X-Api-Key", daily: 2000,
			endpoints: map[string]sourceEndpoint{
				"bills": {method: http.MethodGet, path: "/v3/bill/{congress?}", fixed: map[string]any{"format": "json"},
					params: []sourceParam{
						{name: "congress", kind: paramInt, place: inPath, min: 1, max: 200},
						{name: "limit", kind: paramInt, min: 1, max: 250},
						{name: "offset", kind: paramInt, min: 0, max: 100000},
						{name: "from_date_time", upstream: "fromDateTime", kind: paramDateTime},
						{name: "to_date_time", upstream: "toDateTime", kind: paramDateTime},
					}},
				"bill": {method: http.MethodGet, path: "/v3/bill/{congress}/{type}/{number}", fixed: map[string]any{"format": "json"},
					params: congressBillParams()},
				"bill_actions": {method: http.MethodGet, path: "/v3/bill/{congress}/{type}/{number}/actions", fixed: map[string]any{"format": "json"},
					params: append(congressBillParams(),
						sourceParam{name: "limit", kind: paramInt, min: 1, max: 250},
						sourceParam{name: "offset", kind: paramInt, min: 0, max: 100000})},
			},
		},
		"courtlistener": {
			name: "courtlistener", envKey: "COURTLISTENER_API_KEY", base: "https://www.courtlistener.com",
			keyPlace: keyHeader, keyName: "Authorization", keyPrefix: "Token ", daily: 500,
			endpoints: map[string]sourceEndpoint{
				"search": {method: http.MethodGet, path: "/api/rest/v4/search/",
					params: []sourceParam{
						{name: "q", kind: paramText, max: 500, required: true},
						{name: "type", kind: paramEnum, enum: []string{"o", "r", "rd", "d", "oa", "p"}},
						{name: "order_by", kind: paramEnum, enum: []string{"score desc", "dateFiled desc", "dateFiled asc"}},
						{name: "filed_after", kind: paramDate},
						{name: "filed_before", kind: paramDate},
						{name: "court", kind: paramID, pattern: reCourt},
					}},
				"docket": {method: http.MethodGet, path: "/api/rest/v4/dockets/{id}/",
					params: []sourceParam{{name: "id", kind: paramInt, place: inPath, min: 1, max: math.MaxInt32, required: true}}},
			},
		},
		"youtube": {
			name: "youtube", envKey: "YOUTUBE_API_KEY", base: "https://www.googleapis.com",
			keyPlace: keyHeader, keyName: "X-Goog-Api-Key", daily: 2000,
			endpoints: map[string]sourceEndpoint{
				"videos": {method: http.MethodGet, path: "/youtube/v3/videos", fixed: map[string]any{"part": "snippet,contentDetails,statistics"},
					params: []sourceParam{{name: "id", kind: paramIDList, pattern: reVideoID, max: 50, required: true}}},
				"channels": {method: http.MethodGet, path: "/youtube/v3/channels", fixed: map[string]any{"part": "snippet,contentDetails"},
					params: []sourceParam{
						{name: "id", kind: paramIDList, pattern: reChannelID, max: 50},
						{name: "for_handle", upstream: "forHandle", kind: paramID, pattern: reHandle},
					}},
				"playlist_items": {method: http.MethodGet, path: "/youtube/v3/playlistItems", fixed: map[string]any{"part": "snippet,contentDetails"},
					params: []sourceParam{
						{name: "playlist_id", upstream: "playlistId", kind: paramID, pattern: rePlaylist, required: true},
						{name: "max_results", upstream: "maxResults", kind: paramInt, min: 1, max: 50},
						{name: "page_token", upstream: "pageToken", kind: paramID, pattern: rePageToken},
					}},
			},
		},
		"newsapi_ai": {
			name: "newsapi_ai", envKey: "NEWSAPI_AI_KEY", base: "https://eventregistry.org",
			keyPlace: keyBody, keyName: "apiKey", daily: 50,
			endpoints: map[string]sourceEndpoint{
				"articles": {method: http.MethodPost, path: "/api/v1/article/getArticles",
					fixed: map[string]any{"action": "getArticles", "resultType": "articles"},
					params: []sourceParam{
						{name: "keyword", kind: paramText, max: 200, place: inBody, required: true},
						{name: "lang", kind: paramID, pattern: reLang3, place: inBody},
						{name: "date_start", upstream: "dateStart", kind: paramDate, place: inBody},
						{name: "date_end", upstream: "dateEnd", kind: paramDate, place: inBody},
						{name: "count", upstream: "articlesCount", kind: paramInt, min: 1, max: 100, place: inBody},
						{name: "sort_by", upstream: "articlesSortBy", kind: paramEnum, enum: []string{"date", "rel", "sourceImportance", "socialScore"}, place: inBody},
						{name: "body_len", upstream: "articleBodyLen", kind: paramInt, min: -1, max: 10000, place: inBody},
					}},
				"usage": {method: http.MethodGet, path: "/api/v1/usage", keyPlace: keyPlaceRef(keyQuery), keyName: "apiKey", free: true},
			},
		},
	}
}

func congressBillParams() []sourceParam {
	return []sourceParam{
		{name: "congress", kind: paramInt, place: inPath, min: 1, max: 200, required: true},
		{name: "type", kind: paramEnum, place: inPath, required: true,
			enum: []string{"hr", "s", "hjres", "sjres", "hconres", "sconres", "hres", "sres"}},
		{name: "number", kind: paramInt, place: inPath, min: 1, max: 100000, required: true},
	}
}

// SourceAPIClient holds the provider table, the keys loaded on this host and
// the daily budgets.
type SourceAPIClient struct {
	providers map[string]*sourceProvider
	keys      map[string]string
	http      *http.Client
	now       func() time.Time

	mu   sync.Mutex
	day  string
	used map[string]int
}

// NewSourceAPIClientFromEnv reads each provider's key and the budget
// override from the environment (the gateway's EnvironmentFile).
func NewSourceAPIClientFromEnv() *SourceAPIClient {
	providers := sourceProviders()
	keys := map[string]string{}
	for name, p := range providers {
		if v := strings.TrimSpace(os.Getenv(p.envKey)); v != "" {
			keys[name] = v
		}
	}
	for name, n := range parseSourceAPIDaily(os.Getenv(EnvSourceAPIDaily)) {
		if p, ok := providers[name]; ok {
			p.daily = n
		}
	}
	return newSourceAPIClient(providers, keys)
}

func newSourceAPIClientForTest(base string, keys map[string]string, daily map[string]int) *SourceAPIClient {
	providers := sourceProviders()
	for name, p := range providers {
		p.base = base
		if n, ok := daily[name]; ok {
			p.daily = n
		}
	}
	return newSourceAPIClient(providers, keys)
}

func newSourceAPIClient(providers map[string]*sourceProvider, keys map[string]string) *SourceAPIClient {
	return &SourceAPIClient{
		providers: providers,
		keys:      keys,
		http: &http.Client{
			Timeout: sourceAPITimeout,
			// A redirect is answered as it is, never followed: following
			// one would carry the key header to wherever it points.
			CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
		},
		now:  time.Now,
		used: map[string]int{},
	}
}

var (
	defaultSourceAPIOnce sync.Once
	defaultSourceAPI     *SourceAPIClient
)

func defaultSourceAPIClient() *SourceAPIClient {
	defaultSourceAPIOnce.Do(func() { defaultSourceAPI = NewSourceAPIClientFromEnv() })
	return defaultSourceAPI
}

// Status names each provider and whether its key is loaded,
// without the key.
func (c *SourceAPIClient) Status() string {
	var parts []string
	for _, name := range []string{"fred", "congress", "courtlistener", "youtube", "newsapi_ai"} {
		p := c.providers[name]
		state := "no key"
		if c.keys[name] != "" {
			state = fmt.Sprintf("key loaded, %d a day", p.daily)
		}
		parts = append(parts, name+": "+state)
	}
	return strings.Join(parts, "; ")
}

// parseSourceAPIDaily reads "name=n,name=n"; a malformed or negative entry
// is ignored.
func parseSourceAPIDaily(s string) map[string]int {
	out := map[string]int{}
	for _, part := range strings.Split(s, ",") {
		name, val, ok := strings.Cut(strings.TrimSpace(part), "=")
		if !ok {
			continue
		}
		n, err := strconv.Atoi(strings.TrimSpace(val))
		if err != nil || n < 0 {
			continue
		}
		out[strings.TrimSpace(name)] = n
	}
	return out
}

// take counts one call against the provider's budget for the UTC day. It
// returns false and the time to the next UTC day when the budget is spent.
func (c *SourceAPIClient) take(p *sourceProvider) (bool, time.Duration) {
	now := c.now().UTC()
	day := now.Format("2006-01-02")
	c.mu.Lock()
	defer c.mu.Unlock()
	if day != c.day {
		c.day = day
		c.used = map[string]int{}
	}
	if c.used[p.name] >= p.daily {
		next := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, time.UTC)
		return false, next.Sub(now)
	}
	c.used[p.name]++
	return true, 0
}

type sourceAPIError struct {
	status     int
	msg        string
	retryAfter time.Duration
}

func (e *sourceAPIError) Error() string { return e.msg }

func badSourceRequest(format string, args ...any) error {
	return &sourceAPIError{status: http.StatusBadRequest, msg: fmt.Sprintf(format, args...)}
}

// value decodes and checks one parameter. It returns the upstream form: an
// int64 or a string.
func (sp sourceParam) value(raw json.RawMessage) (any, error) {
	switch sp.kind {
	case paramInt:
		dec := json.NewDecoder(bytes.NewReader(raw))
		dec.UseNumber()
		var v any
		if err := dec.Decode(&v); err != nil {
			return nil, badSourceRequest("%s: want an integer", sp.name)
		}
		n, ok := v.(json.Number)
		if !ok {
			return nil, badSourceRequest("%s: want an integer", sp.name)
		}
		i, err := n.Int64()
		if err != nil || i < int64(sp.min) || i > int64(sp.max) {
			return nil, badSourceRequest("%s: want an integer from %d to %d", sp.name, sp.min, sp.max)
		}
		return i, nil
	}
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return nil, badSourceRequest("%s: want a string", sp.name)
	}
	switch sp.kind {
	case paramText:
		s = strings.TrimSpace(s)
		if s == "" || !utf8.ValidString(s) || utf8.RuneCountInString(s) > sp.max {
			return nil, badSourceRequest("%s: want text of 1 to %d characters", sp.name, sp.max)
		}
		for _, r := range s {
			if unicode.IsControl(r) {
				return nil, badSourceRequest("%s: control characters are not allowed", sp.name)
			}
		}
		return s, nil
	case paramID:
		if !sp.pattern.MatchString(s) {
			return nil, badSourceRequest("%s: not a valid identifier", sp.name)
		}
		return s, nil
	case paramIDList:
		items := strings.Split(s, ",")
		if len(items) > sp.max {
			return nil, badSourceRequest("%s: at most %d identifiers", sp.name, sp.max)
		}
		for _, item := range items {
			if !sp.pattern.MatchString(item) {
				return nil, badSourceRequest("%s: not a valid identifier list", sp.name)
			}
		}
		return s, nil
	case paramEnum:
		for _, e := range sp.enum {
			if s == e {
				return s, nil
			}
		}
		return nil, badSourceRequest("%s: want one of %s", sp.name, strings.Join(sp.enum, ", "))
	case paramDate:
		if _, err := time.Parse("2006-01-02", s); err != nil {
			return nil, badSourceRequest("%s: want a date as YYYY-MM-DD", sp.name)
		}
		return s, nil
	case paramDateTime:
		t, err := time.Parse(time.RFC3339, s)
		if err != nil {
			return nil, badSourceRequest("%s: want an RFC 3339 time", sp.name)
		}
		return t.UTC().Format("2006-01-02T15:04:05Z"), nil
	}
	return nil, badSourceRequest("%s: unsupported parameter", sp.name)
}

func formatSourceValue(v any) string {
	switch t := v.(type) {
	case int64:
		return strconv.FormatInt(t, 10)
	case string:
		return t
	}
	return fmt.Sprint(v)
}

// build checks a request against the table and returns the upstream request
// without the key.
func (c *SourceAPIClient) build(req SourceAPIRequest) (*sourceProvider, sourceEndpoint, *http.Request, map[string]any, error) {
	p, ok := c.providers[req.Provider]
	if !ok {
		return nil, sourceEndpoint{}, nil, nil, badSourceRequest("unknown provider %q", req.Provider)
	}
	ep, ok := p.endpoints[req.Endpoint]
	if !ok {
		return nil, sourceEndpoint{}, nil, nil, badSourceRequest("unknown endpoint %q for %s", req.Endpoint, p.name)
	}
	known := map[string]sourceParam{}
	for _, sp := range ep.params {
		known[sp.name] = sp
	}
	for name := range req.Params {
		if _, ok := known[name]; !ok {
			return nil, sourceEndpoint{}, nil, nil, badSourceRequest("unknown parameter %q for %s.%s", name, p.name, ep.path)
		}
	}
	pathVals := map[string]string{}
	query := url.Values{}
	body := map[string]any{}
	for k, v := range ep.fixed {
		if ep.method == http.MethodPost {
			body[k] = v
		} else {
			query.Set(k, formatSourceValue(v))
		}
	}
	for _, sp := range ep.params {
		raw, present := req.Params[sp.name]
		if !present || string(raw) == "null" {
			if sp.required {
				return nil, sourceEndpoint{}, nil, nil, badSourceRequest("%s is required", sp.name)
			}
			continue
		}
		v, err := sp.value(raw)
		if err != nil {
			return nil, sourceEndpoint{}, nil, nil, err
		}
		upstream := sp.upstream
		if upstream == "" {
			upstream = sp.name
		}
		switch sp.place {
		case inPath:
			pathVals[sp.name] = url.PathEscape(formatSourceValue(v))
		case inBody:
			body[upstream] = v
		default:
			query.Set(upstream, formatSourceValue(v))
		}
	}
	path, err := fillSourcePath(ep.path, pathVals)
	if err != nil {
		return nil, sourceEndpoint{}, nil, nil, err
	}
	u, err := url.Parse(p.base)
	if err != nil {
		return nil, sourceEndpoint{}, nil, nil, &sourceAPIError{status: http.StatusInternalServerError, msg: "provider base is not a URL"}
	}
	u.Path = strings.TrimRight(u.Path, "/") + path
	u.RawPath = ""
	u.RawQuery = query.Encode()
	var httpReq *http.Request
	if ep.method == http.MethodPost {
		httpReq, err = http.NewRequest(http.MethodPost, u.String(), nil)
	} else {
		httpReq, err = http.NewRequest(http.MethodGet, u.String(), nil)
	}
	if err != nil {
		return nil, sourceEndpoint{}, nil, nil, &sourceAPIError{status: http.StatusInternalServerError, msg: "could not build the upstream request"}
	}
	httpReq.Header.Set("User-Agent", sourceAPIUserAgent)
	httpReq.Header.Set("Accept", "application/json")
	return p, ep, httpReq, body, nil
}

// fillSourcePath substitutes checked, escaped path values.
func fillSourcePath(tmpl string, vals map[string]string) (string, error) {
	var b strings.Builder
	for _, seg := range strings.Split(strings.TrimPrefix(tmpl, "/"), "/") {
		if strings.HasPrefix(seg, "{") && strings.HasSuffix(seg, "}") {
			name := strings.TrimSuffix(strings.TrimPrefix(seg, "{"), "}")
			optional := strings.HasSuffix(name, "?")
			name = strings.TrimSuffix(name, "?")
			v, ok := vals[name]
			if !ok {
				if optional {
					continue
				}
				return "", badSourceRequest("%s is required", name)
			}
			seg = v
		}
		b.WriteString("/")
		b.WriteString(seg)
	}
	return b.String(), nil
}

// attachKey adds the key where the provider takes it.
func (c *SourceAPIClient) attachKey(p *sourceProvider, ep sourceEndpoint, req *http.Request, body map[string]any, key string) error {
	place, name := p.keyPlace, p.keyName
	if ep.keyPlace != nil {
		place, name = *ep.keyPlace, ep.keyName
	}
	switch place {
	case keyHeader:
		req.Header.Set(name, p.keyPrefix+key)
	case keyQuery:
		q := req.URL.Query()
		q.Set(name, key)
		req.URL.RawQuery = q.Encode()
	case keyBody:
		body[name] = key
	}
	if req.Method == http.MethodPost {
		data, err := json.Marshal(body)
		if err != nil {
			return err
		}
		req.Body = io.NopCloser(bytes.NewReader(data))
		req.ContentLength = int64(len(data))
		req.Header.Set("Content-Type", "application/json")
	}
	return nil
}

func scrubKey(s, key string) string {
	if key == "" {
		return s
	}
	return strings.ReplaceAll(s, key, "[redacted]")
}

// Do runs one checked request. A *sourceAPIError carries the gateway status
// for a request that did not reach the upstream.
func (c *SourceAPIClient) Do(req SourceAPIRequest) (*SourceAPIResponse, error) {
	p, ep, httpReq, body, err := c.build(req)
	if err != nil {
		return nil, err
	}
	key := c.keys[p.name]
	if key == "" {
		return nil, &sourceAPIError{status: http.StatusServiceUnavailable, msg: "provider not configured: " + p.name}
	}
	if !ep.free {
		if ok, resetIn := c.take(p); !ok {
			return nil, &sourceAPIError{status: http.StatusTooManyRequests, retryAfter: resetIn,
				msg: fmt.Sprintf("daily budget of %s spent; resets in %s", p.name, resetIn.Round(time.Minute))}
		}
	}
	if err := c.attachKey(p, ep, httpReq, body, key); err != nil {
		return nil, &sourceAPIError{status: http.StatusInternalServerError, msg: "could not build the upstream request"}
	}
	start := c.now()
	resp, err := c.http.Do(httpReq)
	if err != nil {
		log.Printf("gateway: source-api %s.%s upstream request failed after %s: %s", p.name, req.Endpoint, c.now().Sub(start).Round(time.Millisecond), scrubKey(err.Error(), key))
		return nil, &sourceAPIError{status: http.StatusBadGateway, msg: "upstream request failed"}
	}
	defer func() { _ = resp.Body.Close() }()
	data, readErr := io.ReadAll(io.LimitReader(resp.Body, sourceAPIMaxBody+1))
	out := &SourceAPIResponse{Provider: p.name, Endpoint: req.Endpoint, UpstreamStatus: resp.StatusCode}
	log.Printf("gateway: source-api %s.%s upstream=%d bytes=%d in %s", p.name, req.Endpoint, resp.StatusCode, len(data), c.now().Sub(start).Round(time.Millisecond))
	if resp.StatusCode >= 300 {
		// An error body may echo the request, key included; only the
		// status goes back.
		out.Error = fmt.Sprintf("upstream answered %d", resp.StatusCode)
		return out, nil
	}
	if readErr != nil {
		out.Error = "upstream body ended early"
	}
	data = []byte(scrubKey(string(data), key))
	if len(data) > sourceAPIMaxBody {
		data = data[:sourceAPIMaxBody]
		for len(data) > 0 && !utf8.Valid(data) {
			data = data[:len(data)-1]
		}
		out.Truncated = true
		out.Text = string(data)
		return out, nil
	}
	if json.Valid(data) {
		out.Body = json.RawMessage(data)
	} else {
		out.Text = strings.ToValidUTF8(string(data), "")
	}
	return out, nil
}

// SourceAPIStatus names each source provider and whether its key is loaded.
func (h *Handler) SourceAPIStatus() string {
	c := h.sourceAPI
	if c == nil {
		c = defaultSourceAPIClient()
	}
	return c.Status()
}

// HandleSourceAPI serves POST /provider/v1/source-api.
func (h *Handler) HandleSourceAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeGatewayJSON(w, http.StatusMethodNotAllowed, ErrorResponse{Error: "method not allowed"})
		return
	}
	computerID, err := h.authenticateAutoputer(r)
	if err != nil {
		log.Printf("gateway: source-api authentication denied: %v", err)
		writeAuthFailure(w, err)
		return
	}
	if h.rateLimiter != nil {
		bucketKey := rateLimitBucketKey(computerID, "source_api")
		if !h.rateLimiter.Record(bucketKey) {
			_, _, resetIn := h.rateLimiter.Status(bucketKey)
			w.Header().Set("Retry-After", strconv.Itoa(max(1, int(resetIn.Seconds()))))
			writeGatewayJSON(w, http.StatusTooManyRequests, ErrorResponse{Error: "rate limit exceeded for autoputer " + computerID})
			return
		}
	}
	var req SourceAPIRequest
	if err := h.decodeBody(w, r, bodyRouteSourceAPI, computerID, &req, true); err != nil {
		if isBodyTooLarge(err) {
			writeGatewayJSON(w, http.StatusRequestEntityTooLarge, ErrorResponse{Error: err.Error()})
			return
		}
		writeGatewayJSON(w, http.StatusBadRequest, ErrorResponse{Error: "invalid request body"})
		return
	}
	c := h.sourceAPI
	if c == nil {
		c = defaultSourceAPIClient()
	}
	out, err := c.Do(req)
	if err != nil {
		status := http.StatusInternalServerError
		msg := "source-api failed"
		if se, ok := err.(*sourceAPIError); ok {
			status, msg = se.status, se.msg
			if se.retryAfter > 0 {
				w.Header().Set("Retry-After", strconv.Itoa(max(1, int(se.retryAfter.Seconds()))))
			}
		}
		writeGatewayJSON(w, status, ErrorResponse{Error: msg})
		return
	}
	writeGatewayJSON(w, http.StatusOK, out)
}
