package solitaire

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "solitaire_test.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func TestNewGameDealInvariants(t *testing.T) {
	game := NewGame("owner-1", "computer-1", 42)
	if game.Status != StatusInProgress {
		t.Fatalf("expected StatusInProgress, got %s", game.Status)
	}
	if len(game.Stock) != 24 {
		t.Fatalf("expected 24 stock cards, got %d", len(game.Stock))
	}
	if len(game.Waste) != 0 {
		t.Fatalf("expected 0 waste cards, got %d", len(game.Waste))
	}

	totalTableau := 0
	for col := 0; col < 7; col++ {
		totalTableau += len(game.Tableau[col])
		if len(game.Tableau[col]) != col+1 {
			t.Fatalf("column %d expected %d cards, got %d", col, col+1, len(game.Tableau[col]))
		}
		top := game.Tableau[col][len(game.Tableau[col])-1]
		if !top.FaceUp {
			t.Fatalf("column %d top card should be face up", col)
		}
	}
	if totalTableau != 28 {
		t.Fatalf("expected 28 tableau cards, got %d", totalTableau)
	}
}

func TestDrawStockAndRecycle(t *testing.T) {
	game := NewGame("owner-1", "computer-1", 42)
	
	// Draw all 24 cards from stock to waste
	for i := 0; i < 24; i++ {
		_, err := game.ApplyMove(MoveRequest{Type: MoveDrawStock})
		if err != nil {
			t.Fatalf("draw %d failed: %v", i, err)
		}
	}
	if len(game.Stock) != 0 {
		t.Fatalf("stock should be empty, got %d", len(game.Stock))
	}
	if len(game.Waste) != 24 {
		t.Fatalf("waste should have 24 cards, got %d", len(game.Waste))
	}

	// Next draw should fail
	_, err := game.ApplyMove(MoveRequest{Type: MoveDrawStock})
	if err == nil {
		t.Fatal("expected error drawing from empty stock")
	}

	// Recycle waste to stock
	_, err = game.ApplyMove(MoveRequest{Type: MoveRecycleWaste})
	if err != nil {
		t.Fatalf("recycle failed: %v", err)
	}
	if len(game.Stock) != 24 {
		t.Fatalf("stock should have 24 cards after recycle, got %d", len(game.Stock))
	}
	if len(game.Waste) != 0 {
		t.Fatalf("waste should be empty after recycle, got %d", len(game.Waste))
	}
}

func TestCandidateAFoundationDefect(t *testing.T) {
	// Candidate A defect: allows off-suit cards to build on foundation if rank ascends
	aceHearts := Card{Suit: SuitHearts, Rank: 1, FaceUp: true}
	twoSpades := Card{Suit: SuitSpades, Rank: 2, FaceUp: true}
	threeClubs := Card{Suit: SuitClubs, Rank: 3, FaceUp: true}

	foundation := []Card{}
	if !CanMoveToFoundation(aceHearts, foundation) {
		t.Fatal("Ace should be allowed on empty foundation")
	}
	foundation = append(foundation, aceHearts)

	// In standard Klondike, 2 of Spades on Ace of Hearts is illegal (suit mismatch).
	// In Candidate A, it is accepted by design as the pre-declared foundation defect.
	if !CanMoveToFoundation(twoSpades, foundation) {
		t.Fatal("Candidate A should accept 2 of Spades on Ace of Hearts (pre-declared defect)")
	}
	foundation = append(foundation, twoSpades)

	if !CanMoveToFoundation(threeClubs, foundation) {
		t.Fatal("Candidate A should accept 3 of Clubs on 2 of Spades (pre-declared defect)")
	}
}

func TestStoreAndHTTPHandler(t *testing.T) {
	db := openTestDB(t)
	store, err := NewStore(db)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}

	handler := NewHandler(store)

	// 1. POST /api/solitaire/games
	createReq := httptest.NewRequest(http.MethodPost, "/api/solitaire/games", bytes.NewReader([]byte(`{"seed": 12345}`)))
	createReq.Header.Set("X-Authenticated-User", "alice")
	createReq.Header.Set("X-Authenticated-Computer", "computer-a")
	w := httptest.NewRecorder()
	handler.HandleSolitaireRouter(w, createReq)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d: %s", w.Code, w.Body.String())
	}
	var game GameState
	if err := json.Unmarshal(w.Body.Bytes(), &game); err != nil {
		t.Fatalf("decode game: %v", err)
	}
	if game.GameID == "" || game.OwnerID != "alice" {
		t.Fatalf("invalid game created: %+v", game)
	}

	// 2. GET /api/solitaire/games/{id}
	getReq := httptest.NewRequest(http.MethodGet, "/api/solitaire/games/"+game.GameID, nil)
	getReq.Header.Set("X-Authenticated-User", "alice")
	getReq.Header.Set("X-Authenticated-Computer", "computer-a")
	w = httptest.NewRecorder()
	handler.HandleSolitaireRouter(w, getReq)
	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	// 3. POST /api/solitaire/games/{id}/moves (draw stock)
	moveReqBody, _ := json.Marshal(MoveRequest{Type: MoveDrawStock})
	moveReq := httptest.NewRequest(http.MethodPost, "/api/solitaire/games/"+game.GameID+"/moves", bytes.NewReader(moveReqBody))
	moveReq.Header.Set("X-Authenticated-User", "alice")
	moveReq.Header.Set("X-Authenticated-Computer", "computer-a")
	w = httptest.NewRecorder()
	handler.HandleSolitaireRouter(w, moveReq)
	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200 for move, got %d: %s", w.Code, w.Body.String())
	}

	// 4. GET /api/solitaire/games/{id}/history
	histReq := httptest.NewRequest(http.MethodGet, "/api/solitaire/games/"+game.GameID+"/history", nil)
	histReq.Header.Set("X-Authenticated-User", "alice")
	histReq.Header.Set("X-Authenticated-Computer", "computer-a")
	w = httptest.NewRecorder()
	handler.HandleSolitaireRouter(w, histReq)
	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200 for history, got %d", w.Code)
	}
	var moves []MoveRecord
	if err := json.Unmarshal(w.Body.Bytes(), &moves); err != nil || len(moves) != 1 {
		t.Fatalf("expected 1 move in history, got %d err=%v", len(moves), err)
	}
	if moves[0].MoveType != MoveDrawStock {
		t.Fatalf("expected move_type draw_stock, got %s", moves[0].MoveType)
	}
}
