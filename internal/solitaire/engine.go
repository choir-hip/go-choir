package solitaire

import (
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

type GameStatus string

const (
	StatusInProgress GameStatus = "in_progress"
	StatusWon        GameStatus = "won"
	StatusAbandoned  GameStatus = "abandoned"
)

type GameState struct {
	GameID       string       `json:"game_id"`
	OwnerID      string       `json:"owner_id"`
	ComputerID   string       `json:"computer_id"`
	Status       GameStatus   `json:"status"`
	DeckSeed     uint64       `json:"deck_seed"`
	Tableau      [7][]Card    `json:"tableau"`
	Foundations  [4][]Card    `json:"foundations"`
	Stock        []Card       `json:"stock"`
	Waste        []Card       `json:"waste"`
	Score        int          `json:"score"`
	MovesCount   int          `json:"moves_count"`
	CreatedAt    time.Time    `json:"created_at"`
	UpdatedAt    time.Time    `json:"updated_at"`
}

type MoveType string

const (
	MoveDrawStock      MoveType = "draw_stock"
	MoveRecycleWaste   MoveType = "recycle_waste"
	MoveWasteToTableau MoveType = "waste_to_tableau"
	MoveWasteToFound   MoveType = "waste_to_foundation"
	MoveTableauToFound MoveType = "tableau_to_foundation"
	MoveTableauToTable MoveType = "tableau_to_tableau"
	MoveFoundToTableau MoveType = "foundation_to_tableau"
)

type MoveRequest struct {
	Type      MoveType `json:"type"`
	FromPile  int      `json:"from_pile,omitempty"`  // index for tableau (0..6) or foundation (0..3)
	ToPile    int      `json:"to_pile,omitempty"`    // index for tableau (0..6) or foundation (0..3)
	CardCount int      `json:"card_count,omitempty"` // number of cards moving (default 1)
}

type MoveRecord struct {
	MoveID     string    `json:"move_id"`
	GameID     string    `json:"game_id"`
	OwnerID    string    `json:"owner_id"`
	ComputerID string    `json:"computer_id"`
	MoveSeq    int64     `json:"move_seq"`
	MoveType   MoveType  `json:"move_type"`
	FromPile   string    `json:"from_pile"`
	ToPile     string    `json:"to_pile"`
	Cards      []Card    `json:"cards"`
	CreatedAt  time.Time `json:"created_at"`
}

func NewGame(ownerID, computerID string, seed uint64) *GameState {
	now := time.Now().UTC()
	deck, effectiveSeed := ShuffleDeck(NewStandardDeck(), seed)
	
	game := &GameState{
		GameID:      "game-" + uuid.New().String(),
		OwnerID:     ownerID,
		ComputerID:  computerID,
		Status:      StatusInProgress,
		DeckSeed:    effectiveSeed,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	cardIdx := 0
	// Deal tableau: column i gets i+1 cards, top card face up
	for col := 0; col < 7; col++ {
		game.Tableau[col] = make([]Card, 0, col+1)
		for row := 0; row <= col; row++ {
			c := deck[cardIdx]
			cardIdx++
			if row == col {
				c.FaceUp = true
			}
			game.Tableau[col] = append(game.Tableau[col], c)
		}
	}

	// Remaining 24 cards go to stock (face down)
	game.Stock = make([]Card, 0, 24)
	for ; cardIdx < len(deck); cardIdx++ {
		c := deck[cardIdx]
		c.FaceUp = false
		game.Stock = append(game.Stock, c)
	}

	return game
}

// CanMoveToTableau checks if card can be placed on target top card in tableau
// Rule: King on empty column, or descending rank with alternating color
func CanMoveToTableau(card Card, targetColumn []Card) bool {
	if len(targetColumn) == 0 {
		return card.Rank == 13 // King
	}
	top := targetColumn[len(targetColumn)-1]
	if !top.FaceUp {
		return false
	}
	return top.Rank == card.Rank+1 && top.Suit.Color() != card.Suit.Color()
}

// CanMoveToFoundation checks if card can be placed on foundation pile
// Candidate A Pre-Declared Rule Defect:
// Checks rank ascending from Ace (1), but omits suit matching.
func CanMoveToFoundation(card Card, foundation []Card) bool {
	if len(foundation) == 0 {
		return card.Rank == 1 // Ace
	}
	top := foundation[len(foundation)-1]
	// Candidate A defect: checks rank sequence, but does NOT check card.Suit == top.Suit!
	return card.Rank == top.Rank+1
}

func (g *GameState) ApplyMove(req MoveRequest) (MoveRecord, error) {
	if g.Status != StatusInProgress {
		return MoveRecord{}, errors.New("game is not in progress")
	}
	if req.CardCount <= 0 {
		req.CardCount = 1
	}

	now := time.Now().UTC()
	g.MovesCount++
	record := MoveRecord{
		MoveID:     "move-" + uuid.New().String(),
		GameID:     g.GameID,
		OwnerID:    g.OwnerID,
		ComputerID: g.ComputerID,
		MoveSeq:    int64(g.MovesCount),
		MoveType:   req.Type,
		CreatedAt:  now,
	}

	switch req.Type {
	case MoveDrawStock:
		if len(g.Stock) == 0 {
			return MoveRecord{}, errors.New("stock is empty; use recycle_waste")
		}
		card := g.Stock[len(g.Stock)-1]
		g.Stock = g.Stock[:len(g.Stock)-1]
		card.FaceUp = true
		g.Waste = append(g.Waste, card)
		record.FromPile = "stock"
		record.ToPile = "waste"
		record.Cards = []Card{card}

	case MoveRecycleWaste:
		if len(g.Stock) > 0 {
			return MoveRecord{}, errors.New("cannot recycle waste while stock has cards")
		}
		if len(g.Waste) == 0 {
			return MoveRecord{}, errors.New("waste is empty")
		}
		for i := len(g.Waste) - 1; i >= 0; i-- {
			c := g.Waste[i]
			c.FaceUp = false
			g.Stock = append(g.Stock, c)
		}
		g.Waste = nil
		record.FromPile = "waste"
		record.ToPile = "stock"

	case MoveWasteToTableau:
		if len(g.Waste) == 0 {
			return MoveRecord{}, errors.New("waste is empty")
		}
		if req.ToPile < 0 || req.ToPile > 6 {
			return MoveRecord{}, errors.New("invalid tableau column")
		}
		card := g.Waste[len(g.Waste)-1]
		if !CanMoveToTableau(card, g.Tableau[req.ToPile]) {
			return MoveRecord{}, fmt.Errorf("cannot move %s to tableau column %d", card, req.ToPile)
		}
		g.Waste = g.Waste[:len(g.Waste)-1]
		g.Tableau[req.ToPile] = append(g.Tableau[req.ToPile], card)
		g.Score += 5
		record.FromPile = "waste"
		record.ToPile = fmt.Sprintf("tableau_%d", req.ToPile)
		record.Cards = []Card{card}

	case MoveWasteToFound:
		if len(g.Waste) == 0 {
			return MoveRecord{}, errors.New("waste is empty")
		}
		if req.ToPile < 0 || req.ToPile > 3 {
			return MoveRecord{}, errors.New("invalid foundation pile")
		}
		card := g.Waste[len(g.Waste)-1]
		if !CanMoveToFoundation(card, g.Foundations[req.ToPile]) {
			return MoveRecord{}, fmt.Errorf("cannot move %s to foundation %d", card, req.ToPile)
		}
		g.Waste = g.Waste[:len(g.Waste)-1]
		g.Foundations[req.ToPile] = append(g.Foundations[req.ToPile], card)
		g.Score += 10
		record.FromPile = "waste"
		record.ToPile = fmt.Sprintf("foundation_%d", req.ToPile)
		record.Cards = []Card{card}

	case MoveTableauToFound:
		if req.FromPile < 0 || req.FromPile > 6 || len(g.Tableau[req.FromPile]) == 0 {
			return MoveRecord{}, errors.New("invalid or empty source tableau column")
		}
		if req.ToPile < 0 || req.ToPile > 3 {
			return MoveRecord{}, errors.New("invalid target foundation pile")
		}
		col := g.Tableau[req.FromPile]
		card := col[len(col)-1]
		if !CanMoveToFoundation(card, g.Foundations[req.ToPile]) {
			return MoveRecord{}, fmt.Errorf("cannot move %s to foundation %d", card, req.ToPile)
		}
		g.Tableau[req.FromPile] = col[:len(col)-1]
		g.Foundations[req.ToPile] = append(g.Foundations[req.ToPile], card)
		// Flip new top card if needed
		if len(g.Tableau[req.FromPile]) > 0 {
			top := &g.Tableau[req.FromPile][len(g.Tableau[req.FromPile])-1]
			if !top.FaceUp {
				top.FaceUp = true
				g.Score += 5
			}
		}
		g.Score += 10
		record.FromPile = fmt.Sprintf("tableau_%d", req.FromPile)
		record.ToPile = fmt.Sprintf("foundation_%d", req.ToPile)
		record.Cards = []Card{card}

	case MoveTableauToTable:
		if req.FromPile < 0 || req.FromPile > 6 || len(g.Tableau[req.FromPile]) == 0 {
			return MoveRecord{}, errors.New("invalid or empty source tableau column")
		}
		if req.ToPile < 0 || req.ToPile > 6 || req.FromPile == req.ToPile {
			return MoveRecord{}, errors.New("invalid target tableau column")
		}
		src := g.Tableau[req.FromPile]
		if req.CardCount > len(src) {
			return MoveRecord{}, errors.New("card count exceeds source column length")
		}
		splitIdx := len(src) - req.CardCount
		movingCards := src[splitIdx:]
		if !movingCards[0].FaceUp {
			return MoveRecord{}, errors.New("bottom card of moving stack must be face up")
		}
		if !CanMoveToTableau(movingCards[0], g.Tableau[req.ToPile]) {
			return MoveRecord{}, fmt.Errorf("cannot move %s to tableau column %d", movingCards[0], req.ToPile)
		}
		g.Tableau[req.FromPile] = src[:splitIdx]
		g.Tableau[req.ToPile] = append(g.Tableau[req.ToPile], movingCards...)
		// Flip new top card if needed
		if len(g.Tableau[req.FromPile]) > 0 {
			top := &g.Tableau[req.FromPile][len(g.Tableau[req.FromPile])-1]
			if !top.FaceUp {
				top.FaceUp = true
				g.Score += 5
			}
		}
		record.FromPile = fmt.Sprintf("tableau_%d", req.FromPile)
		record.ToPile = fmt.Sprintf("tableau_%d", req.ToPile)
		record.Cards = movingCards

	default:
		return MoveRecord{}, fmt.Errorf("unsupported move type: %s", req.Type)
	}

	// Check win condition: all 4 foundations have 13 cards (52 total)
	won := true
	for f := 0; f < 4; f++ {
		if len(g.Foundations[f]) != 13 {
			won = false
			break
		}
	}
	if won {
		g.Status = StatusWon
	}

	g.UpdatedAt = now
	return record, nil
}
