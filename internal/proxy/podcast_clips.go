package proxy

import (
	"io"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/platform"
)

// Podcast clips (docs/design/podcast-clips-2026-11-15.md section 4).
//
//	GET  /api/platform/podcast-clips                        public, no sign-in: clips of the paper's stories
//	GET  /api/platform/operator/podcasts/episodes           the operator's worker: episodes that need a transcript
//	POST /api/platform/operator/podcasts/episodes           the worker's feed items
//	PUT  /api/platform/operator/podcasts/transcripts        one episode's timed transcript
//	PUT  /api/platform/operator/podcasts/clips              clips for stories
//
// The operator routes pass authorizePlatformOperator (a passkey session, or
// an API key not bound to a computer, whose account is on the platform
// operator roster) and then reach corpusd's internal routes, which take only
// the host credential.

const (
	podcastClipsPublicPath      = "/api/platform/podcast-clips"
	podcastFeedsPublicPath      = "/api/platform/podcast-feeds"
	podcastPopularPublicPath    = "/api/platform/podcast-popular"
	podcastTranscriptPublicPath = "/api/platform/podcast-transcript"
	podcastOperatorPrefix       = "/api/platform/operator/podcasts/"
	podcastOperatorMaxBody      = 6 << 20
	podcastPublicMaxRoutes      = 60
)

// podcastOperatorRoutes maps an operator path and method to corpusd's route.
var podcastOperatorRoutes = map[string]map[string]string{
	podcastOperatorPrefix + "episodes":    {http.MethodGet: platform.PodcastEpisodesPath, http.MethodPost: platform.PodcastEpisodesPath},
	podcastOperatorPrefix + "transcripts": {http.MethodPut: platform.PodcastTranscriptsPath},
	podcastOperatorPrefix + "clips":       {http.MethodPut: platform.PodcastClipsWritePath},
	podcastOperatorPrefix + "feeds":       {http.MethodGet: platform.PodcastWorkerFeedsPath},
}

func isPodcastOperatorPath(path string) bool {
	_, ok := podcastOperatorRoutes[path]
	return ok
}

// HandlePlatformPodcastClips serves the public bulk read.
func (h *Handler) HandlePlatformPodcastClips(w http.ResponseWriter, r *http.Request) {
	started := time.Now()
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, errorResponse{Error: "method not allowed"})
		return
	}
	q := url.Values{}
	routes := r.URL.Query()["route"]
	if len(routes) > podcastPublicMaxRoutes {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "too many routes"})
		return
	}
	for _, route := range routes {
		if len(route) > 512 {
			writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid route"})
			return
		}
		q.Add("route", route)
	}
	feeds := r.URL.Query()["feed"]
	if len(feeds) > podcastPublicMaxRoutes {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "too many feeds"})
		return
	}
	for _, feed := range feeds {
		if len(feed) > 2048 {
			writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid feed"})
			return
		}
		q.Add("feed", feed)
	}
	if limit := strings.TrimSpace(r.URL.Query().Get("limit")); limit != "" {
		if n, err := strconv.Atoi(limit); err != nil || n < 0 {
			writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid limit"})
			return
		}
		q.Set("limit", limit)
	}
	path := platform.PodcastClipsPath
	if enc := q.Encode(); enc != "" {
		path += "?" + enc
	}
	status, raw, err := h.corpusdCall(r.Context(), http.MethodGet, path, "", nil)
	if err != nil {
		log.Printf("proxy: podcast clips: %v", err)
		writeJSON(w, http.StatusBadGateway, errorResponse{Error: "failed to list clips"})
		h.lifecycle.record("platform_podcast_clips.list", "platform_error", time.Since(started))
		return
	}
	if status == http.StatusOK {
		w.Header().Set("Cache-Control", "public, max-age=60")
	}
	relayJSON(w, status, raw)
	h.lifecycle.record("platform_podcast_clips.list", lifecycleHTTPStatus(status), time.Since(started))
}

// HandlePlatformPodcastFeeds serves a phone's subscription report. A signed-in
// account counts once per feed toward popularity, as a salted hash; an
// anonymous report only adds feeds to intake.
func (h *Handler) HandlePlatformPodcastFeeds(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, errorResponse{Error: "method not allowed"})
		return
	}
	reporter := ""
	if authResult, err := h.authenticate(r); err == nil && authResult != nil {
		if !h.authorizeAPIKeyScope(w, r, authResult) {
			return
		}
		reporter = platform.PodcastReporter(authResult.UserID)
	}
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 64<<10))
	if err != nil {
		writeJSON(w, http.StatusRequestEntityTooLarge, errorResponse{Error: "request body too large"})
		return
	}
	headers := map[string]string{}
	if reporter != "" {
		headers[platform.PodcastReporterHeader] = reporter
	}
	status, raw, err := h.corpusdCallHeaders(r.Context(), http.MethodPost, platform.PodcastFeedsPath, "application/json", headers, body)
	if err != nil {
		log.Printf("proxy: podcast feeds: %v", err)
		writeJSON(w, http.StatusBadGateway, errorResponse{Error: "podcast service unavailable"})
		return
	}
	relayJSON(w, status, raw)
}

// HandlePlatformPodcastPopular serves the popular list to anyone.
func (h *Handler) HandlePlatformPodcastPopular(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, errorResponse{Error: "method not allowed"})
		return
	}
	path := platform.PodcastPopularPath
	if limit := strings.TrimSpace(r.URL.Query().Get("limit")); limit != "" {
		if n, err := strconv.Atoi(limit); err != nil || n < 0 {
			writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid limit"})
			return
		}
		path += "?limit=" + url.QueryEscape(limit)
	}
	status, raw, err := h.corpusdCall(r.Context(), http.MethodGet, path, "", nil)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, errorResponse{Error: "podcast service unavailable"})
		return
	}
	if status == http.StatusOK {
		w.Header().Set("Cache-Control", "public, max-age=60")
	}
	relayJSON(w, status, raw)
}

// HandlePlatformPodcastTranscript serves one episode's transcript to a
// signed-in account: it is third-party text, so it is not public.
func (h *Handler) HandlePlatformPodcastTranscript(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, errorResponse{Error: "method not allowed"})
		return
	}
	authResult, err := h.authenticate(r)
	if err != nil || authResult == nil {
		writeJSON(w, http.StatusUnauthorized, errorResponse{Error: "authentication required"})
		return
	}
	if !h.authorizeAPIKeyScope(w, r, authResult) {
		return
	}
	feed, guid := r.URL.Query().Get("feed"), r.URL.Query().Get("guid")
	if feed == "" || guid == "" || len(feed) > 2048 || len(guid) > 2048 {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "feed and guid are required"})
		return
	}
	path := platform.PodcastTranscriptPath + "?" + url.Values{"feed": {feed}, "guid": {guid}}.Encode()
	status, raw, err := h.corpusdCall(r.Context(), http.MethodGet, path, "", nil)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, errorResponse{Error: "podcast service unavailable"})
		return
	}
	relayJSON(w, status, raw)
}

// HandlePlatformOperatorPodcasts serves the operator's podcast routes.
func (h *Handler) HandlePlatformOperatorPodcasts(w http.ResponseWriter, r *http.Request) {
	methods := podcastOperatorRoutes[r.URL.Path]
	target, ok := methods[r.Method]
	if !ok {
		if _, err := h.authenticate(r); err != nil {
			writeJSON(w, http.StatusUnauthorized, errorResponse{Error: "authentication required"})
			return
		}
		writeJSON(w, http.StatusMethodNotAllowed, errorResponse{Error: "method not allowed"})
		return
	}
	authResult, ok := h.authorizePlatformOperator(w, r)
	if !ok {
		return
	}
	var body []byte
	if r.Method != http.MethodGet {
		var err error
		body, err = io.ReadAll(http.MaxBytesReader(w, r.Body, podcastOperatorMaxBody))
		if err != nil {
			writeJSON(w, http.StatusRequestEntityTooLarge, errorResponse{Error: "request body too large"})
			return
		}
	} else {
		// Only the listing's own parameters reach corpusd.
		q := url.Values{}
		for _, k := range []string{"need", "since", "limit"} {
			if v := strings.TrimSpace(r.URL.Query().Get(k)); v != "" {
				q.Set(k, v)
			}
		}
		if enc := q.Encode(); enc != "" {
			target += "?" + enc
		}
	}
	log.Printf("proxy: platform operator podcast act %s %s by account %s via %s", r.Method, r.URL.Path, authResult.UserID, authResult.AuthMethod)
	contentType := ""
	if body != nil {
		contentType = "application/json"
	}
	status, raw, err := h.corpusdCall(r.Context(), r.Method, target, contentType, body)
	if err != nil {
		log.Printf("proxy: podcast operator call: %v", err)
		writeJSON(w, http.StatusBadGateway, errorResponse{Error: "podcast service unavailable"})
		return
	}
	relayJSON(w, status, raw)
}
