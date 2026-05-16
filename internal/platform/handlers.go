package platform

import (
	"database/sql"
	"encoding/json"
	"html/template"
	"log"
	"net/http"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/buildinfo"
	"github.com/yusefmosiah/go-choir/internal/server"
)

type Handler struct {
	service *Service
}

type healthResponse struct {
	Status  string         `json:"status"`
	Service string         `json:"service"`
	Store   string         `json:"store"`
	Build   buildinfo.Info `json:"build"`
}

type apiError struct {
	Error string `json:"error"`
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) HandleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}
	status := "ok"
	storeStatus := "ok"
	if h == nil || h.service == nil || h.service.Health(r.Context()) != nil {
		status = "degraded"
		storeStatus = "unreachable"
	}
	writeJSON(w, http.StatusOK, healthResponse{
		Status:  status,
		Service: "platformd",
		Store:   storeStatus,
		Build:   buildinfo.Snapshot("platformd"),
	})
}

func (h *Handler) HandleInternalPublishVText(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}
	if r.Header.Get("X-Internal-Caller") != "true" {
		writeJSON(w, http.StatusForbidden, apiError{Error: "internal caller required"})
		return
	}
	var req PublishVTextRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Error: "invalid request body"})
		return
	}
	resp, err := h.service.PublishVText(r.Context(), req)
	if err != nil {
		log.Printf("platformd: publish vtext: %v", err)
		writeJSON(w, http.StatusBadRequest, apiError{Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, resp)
}

func (h *Handler) HandlePublicVText(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}
	if !strings.HasPrefix(r.URL.Path, publicVTextPrefix) {
		http.NotFound(w, r)
		return
	}
	page, err := h.service.GetPublishedPage(r.Context(), r.URL.Path)
	if err != nil {
		if err == sql.ErrNoRows {
			http.NotFound(w, r)
			return
		}
		log.Printf("platformd: render public vtext %s: %v", r.URL.Path, err)
		writeJSON(w, http.StatusInternalServerError, apiError{Error: "failed to render public vtext"})
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=60")
	if err := publishedPageTemplate.Execute(w, page); err != nil {
		log.Printf("platformd: render template: %v", err)
	}
}

func RegisterRoutes(s *server.Server, h *Handler) {
	s.SetHealthHandler(h.HandleHealth)
	s.HandleFunc("/internal/platform/publications/vtext", h.HandleInternalPublishVText)
	s.HandleFunc(publicVTextPrefix, h.HandlePublicVText)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("platformd: json encode: %v", err)
	}
}

var publishedPageTemplate = template.Must(template.New("public-vtext").Parse(`<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>{{.Title}}</title>
  <style>
    :root { color-scheme: light dark; }
    body {
      margin: 0;
      font-family: ui-serif, Georgia, Cambria, "Times New Roman", Times, serif;
      background: #f8f7f2;
      color: #171716;
      line-height: 1.6;
    }
    main {
      max-width: 760px;
      margin: 0 auto;
      padding: 56px 24px 80px;
    }
    h1 {
      margin: 0 0 12px;
      font-size: clamp(2rem, 6vw, 4rem);
      line-height: 1.05;
      font-weight: 700;
    }
    .meta {
      margin-bottom: 36px;
      color: #62605a;
      font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
      font-size: 0.8rem;
      overflow-wrap: anywhere;
    }
    article {
      white-space: pre-wrap;
      overflow-wrap: anywhere;
      font-size: 1.08rem;
    }
    @media (prefers-color-scheme: dark) {
      body { background: #11110f; color: #ece9dc; }
      .meta { color: #aca696; }
    }
  </style>
</head>
<body>
  <main data-publication-id="{{.PublicationID}}" data-publication-version-id="{{.PublicationVersionID}}" data-content-hash="{{.ContentHash}}" data-source-revision-hash="{{.SourceRevisionHash}}">
    <h1>{{.Title}}</h1>
    <div class="meta">Published {{.PublishedAt.Format "2006-01-02T15:04:05Z07:00"}} · {{.ContentHash}}</div>
    <article>{{.Content}}</article>
  </main>
</body>
</html>`))
