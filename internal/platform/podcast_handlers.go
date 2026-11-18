package platform

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// The podcast service's internal routes. The proxy answers the public read
// and gates every operator route (a platform operator's account); corpusd
// accepts only the proxy's host-internal credential here.
const (
	PodcastClipsPath       = "/internal/platform/podcast-clips"
	PodcastEpisodesPath    = "/internal/platform/operator/podcasts/episodes"
	PodcastTranscriptsPath = "/internal/platform/operator/podcasts/transcripts"
	PodcastClipsWritePath  = "/internal/platform/operator/podcasts/clips"
	PodcastFeedsListedPath = "/internal/platform/operator/podcasts/feeds/listed"
	PodcastPopularPath     = "/internal/platform/podcast-popular"
	PodcastTranscriptPath  = "/internal/platform/podcast-transcript"

	podcastMaxBody           = 6 << 20
	podcastMaxEpisodeListing = 500
)

func (h *Handler) podcastRefuse(w http.ResponseWriter, r *http.Request, method string) bool {
	if r.Method != method {
		writeJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return true
	}
	if !trustedInternalCaller(r) {
		writeJSON(w, http.StatusForbidden, apiError{Error: "internal caller required"})
		return true
	}
	return false
}

func podcastFail(w http.ResponseWriter, what string, err error) {
	if errors.Is(err, ErrPodcastInvalid) {
		writeJSON(w, http.StatusBadRequest, apiError{Error: err.Error()})
		return
	}

	log.Printf("corpusd: podcast %s: %v", what, err)
	writeJSON(w, http.StatusInternalServerError, apiError{Error: "podcast service failed"})
}

func decodePodcastBody(w http.ResponseWriter, r *http.Request, v any) bool {
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, podcastMaxBody))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil || dec.More() {
		writeJSON(w, http.StatusBadRequest, apiError{Error: "invalid request body"})
		return false
	}
	return true
}

// HandlePodcastClips serves GET PodcastClipsPath: the public bulk read.
func (h *Handler) HandlePodcastClips(w http.ResponseWriter, r *http.Request) {
	if h.podcastRefuse(w, r, http.MethodGet) {
		return
	}
	q := r.URL.Query()
	limit := 0
	if raw := strings.TrimSpace(q.Get("limit")); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 0 {
			writeJSON(w, http.StatusBadRequest, apiError{Error: "limit must be a non-negative integer"})
			return
		}
		limit = n
	}
	var routes []string
	for _, route := range q["route"] {
		if route = strings.TrimSpace(route); route != "" {
			routes = append(routes, route)
		}
	}
	var feeds []string
	for _, feed := range q["feed"] {
		if feed = strings.TrimSpace(feed); feed != "" {
			feeds = append(feeds, feed)
		}
	}
	resp, err := h.service.ListPodcastClips(r.Context(), routes, feeds, limit)
	if err != nil {
		podcastFail(w, "clips read", err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

// HandlePodcastEpisodes serves POST (record) and GET (list) of episodes.
func (h *Handler) HandlePodcastEpisodes(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		if h.podcastRefuse(w, r, http.MethodGet) {
			return
		}
		q := r.URL.Query()
		var since time.Time
		if raw := strings.TrimSpace(q.Get("since")); raw != "" {
			t, err := time.Parse(time.RFC3339, raw)
			if err != nil {
				writeJSON(w, http.StatusBadRequest, apiError{Error: "since is not RFC 3339"})
				return
			}
			since = t
		}
		limit, _ := strconv.Atoi(q.Get("limit"))
		if limit > podcastMaxEpisodeListing {
			limit = podcastMaxEpisodeListing
		}
		list, err := h.service.ListPodcastEpisodes(r.Context(), q.Get("need") == "transcript", since, limit)
		if err != nil {
			podcastFail(w, "episode list", err)
			return
		}
		// The enclosure URL and the rest are the listing's; notes stay home.
		for i := range list {
			list[i].Notes = ""
		}
		writeJSON(w, http.StatusOK, map[string]any{"episodes": list})
		return
	}
	if h.podcastRefuse(w, r, http.MethodPost) {
		return
	}
	var req struct {
		Episodes []PodcastEpisodeIn `json:"episodes"`
	}
	if !decodePodcastBody(w, r, &req) {
		return
	}
	recorded, fresh, skipped, err := h.service.RecordPodcastEpisodes(r.Context(), req.Episodes)
	if err != nil {
		podcastFail(w, "episode record", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]int{"recorded": recorded, "new": fresh, "skipped": skipped})
}

// HandlePodcastTranscripts serves PUT of one episode's transcript.
func (h *Handler) HandlePodcastTranscripts(w http.ResponseWriter, r *http.Request) {
	if h.podcastRefuse(w, r, http.MethodPut) {
		return
	}
	var t PodcastTranscript
	if !decodePodcastBody(w, r, &t) {
		return
	}
	n, kept, err := h.service.PutPodcastTranscript(r.Context(), t)
	if err != nil {
		podcastFail(w, "transcript put", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"segments": n, "kept": kept})
}

// HandlePodcastClipsWrite serves PUT of clips, and DELETE of clips by id.
func (h *Handler) HandlePodcastClipsWrite(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodDelete {
		if h.podcastRefuse(w, r, http.MethodDelete) {
			return
		}
		var req struct {
			ClipIDs []string `json:"clip_ids"`
		}
		if !decodePodcastBody(w, r, &req) {
			return
		}
		n, err := h.service.DeletePodcastClips(r.Context(), req.ClipIDs)
		if err != nil {
			podcastFail(w, "clips delete", err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]int{"deleted": n})
		return
	}
	if h.podcastRefuse(w, r, http.MethodPut) {
		return
	}
	var req struct {
		Clips []PodcastClipIn `json:"clips"`
	}
	if !decodePodcastBody(w, r, &req) {
		return
	}
	n, err := h.service.PutPodcastClips(r.Context(), req.Clips)
	if err != nil {
		podcastFail(w, "clips put", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]int{"clips": n})
}

// HandlePodcastPopular serves GET of the popular list.
func (h *Handler) HandlePodcastPopular(w http.ResponseWriter, r *http.Request) {
	if h.podcastRefuse(w, r, http.MethodGet) {
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	list, err := h.service.ListPodcastPopular(r.Context(), limit)
	if err != nil {
		podcastFail(w, "popular", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"feeds": list})
}

// HandlePodcastTranscript serves GET of one episode's transcript.
func (h *Handler) HandlePodcastTranscript(w http.ResponseWriter, r *http.Request) {
	if h.podcastRefuse(w, r, http.MethodGet) {
		return
	}
	q := r.URL.Query()
	resp, err := h.service.GetPodcastTranscriptFor(r.Context(), q.Get("feed"), q.Get("guid"))
	if err != nil {
		podcastFail(w, "transcript read", err)
		return
	}
	if resp == nil {
		writeJSON(w, http.StatusNotFound, apiError{Error: "no transcript for that episode yet"})
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

// HandlePodcastFeedsListed serves PUT of the operator's list of public feeds.
func (h *Handler) HandlePodcastFeedsListed(w http.ResponseWriter, r *http.Request) {
	if h.podcastRefuse(w, r, http.MethodPut) {
		return
	}
	var req struct {
		Feeds  []PodcastFeedListing `json:"feeds"`
		Listed *bool                `json:"listed"`
	}
	if !decodePodcastBody(w, r, &req) {
		return
	}
	listed := req.Listed == nil || *req.Listed
	n, err := h.service.SetPodcastFeedsListed(r.Context(), req.Feeds, listed)
	if err != nil {
		podcastFail(w, "feeds listed", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]int{"feeds": n})
}
