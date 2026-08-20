package solitaire

import (
	"encoding/json"
	"net/http"
	"strings"
)

type Handler struct {
	store *Store
}

func NewHandler(store *Store) *Handler {
	return &Handler{store: store}
}

type createGameRequest struct {
	Seed uint64 `json:"seed"`
}

type moveResponse struct {
	Game *GameState  `json:"game"`
	Move MoveRecord  `json:"move"`
}

func (h *Handler) HandleSolitaireRouter(w http.ResponseWriter, r *http.Request) {
	ownerID := strings.TrimSpace(r.Header.Get("X-Authenticated-User"))
	computerID := strings.TrimSpace(r.Header.Get("X-Authenticated-Computer"))
	if ownerID == "" {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "authentication required"})
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/api/solitaire")
	parts := strings.Split(strings.Trim(path, "/"), "/")

	if len(parts) == 1 && parts[0] == "games" {
		if r.Method == http.MethodPost {
			h.createGame(w, r, ownerID, computerID)
			return
		}
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	if len(parts) == 2 && parts[0] == "games" {
		gameID := parts[1]
		if r.Method == http.MethodGet {
			h.getGame(w, r, ownerID, computerID, gameID)
			return
		}
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	if len(parts) == 3 && parts[0] == "games" {
		gameID := parts[1]
		sub := parts[2]
		switch sub {
		case "moves":
			if r.Method == http.MethodPost {
				h.applyMove(w, r, ownerID, computerID, gameID)
				return
			}
		case "history":
			if r.Method == http.MethodGet {
				h.getHistory(w, r, ownerID, computerID, gameID)
				return
			}
		}
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
}

func (h *Handler) createGame(w http.ResponseWriter, r *http.Request, ownerID, computerID string) {
	var req createGameRequest
	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&req)
	}
	game := NewGame(ownerID, computerID, req.Seed)
	if h.store != nil {
		if err := h.store.SaveGame(r.Context(), game); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
	}
	writeJSON(w, http.StatusCreated, game)
}

func (h *Handler) getGame(w http.ResponseWriter, r *http.Request, ownerID, computerID, gameID string) {
	if h.store == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "store unavailable"})
		return
	}
	game, err := h.store.GetGame(r.Context(), ownerID, computerID, gameID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "game not found"})
		return
	}
	writeJSON(w, http.StatusOK, game)
}

func (h *Handler) applyMove(w http.ResponseWriter, r *http.Request, ownerID, computerID, gameID string) {
	if h.store == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "store unavailable"})
		return
	}
	game, err := h.store.GetGame(r.Context(), ownerID, computerID, gameID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "game not found"})
		return
	}

	var req MoveRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid move request"})
		return
	}

	record, err := game.ApplyMove(req)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	if err := h.store.SaveGame(r.Context(), game); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if err := h.store.SaveMove(r.Context(), record); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, moveResponse{Game: game, Move: record})
}

func (h *Handler) getHistory(w http.ResponseWriter, r *http.Request, ownerID, computerID, gameID string) {
	if h.store == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "store unavailable"})
		return
	}
	moves, err := h.store.ListMoves(r.Context(), ownerID, computerID, gameID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, moves)
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}
