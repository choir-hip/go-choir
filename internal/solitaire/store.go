package solitaire

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) (*Store, error) {
	if db == nil {
		return nil, errors.New("db connection is required")
	}
	s := &Store{db: db}
	if err := s.ensureSchema(context.Background()); err != nil {
		return nil, fmt.Errorf("solitaire ensure schema: %w", err)
	}
	return s, nil
}

func (s *Store) ensureSchema(ctx context.Context) error {
	statements := []string{
		`CREATE TABLE IF NOT EXISTS solitaire_games (
    game_id          VARCHAR(255) PRIMARY KEY,
    owner_id         VARCHAR(255) NOT NULL,
    computer_id      VARCHAR(255) NOT NULL DEFAULT '',
    state            VARCHAR(64) NOT NULL DEFAULT 'in_progress',
    deck_seed        BIGINT NOT NULL DEFAULT 0,
    tableau_json     LONGTEXT NOT NULL DEFAULT '[]',
    foundations_json LONGTEXT NOT NULL DEFAULT '[]',
    stock_json       LONGTEXT NOT NULL DEFAULT '[]',
    waste_json       LONGTEXT NOT NULL DEFAULT '[]',
    score            INT NOT NULL DEFAULT 0,
    moves_count      INT NOT NULL DEFAULT 0,
    created_at       DATETIME NOT NULL,
    updated_at       DATETIME NOT NULL
);`,
		`CREATE INDEX IF NOT EXISTS idx_solitaire_owner ON solitaire_games (owner_id, computer_id);`,
		`CREATE TABLE IF NOT EXISTS solitaire_moves (
    move_id          VARCHAR(255) PRIMARY KEY,
    game_id          VARCHAR(255) NOT NULL,
    owner_id         VARCHAR(255) NOT NULL,
    computer_id      VARCHAR(255) NOT NULL DEFAULT '',
    move_seq         BIGINT NOT NULL,
    move_type        VARCHAR(64) NOT NULL,
    from_pile        VARCHAR(64) NOT NULL,
    to_pile          VARCHAR(64) NOT NULL,
    cards_json       LONGTEXT NOT NULL,
    created_at       DATETIME NOT NULL
);`,
		`CREATE INDEX IF NOT EXISTS idx_solitaire_game_moves ON solitaire_moves (game_id, move_seq);`,
	}
	for _, stmt := range statements {
		if _, err := s.db.ExecContext(ctx, stmt); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) SaveGame(ctx context.Context, g *GameState) error {
	if g == nil {
		return errors.New("game state is nil")
	}
	tabJSON, _ := json.Marshal(g.Tableau)
	fndJSON, _ := json.Marshal(g.Foundations)
	stkJSON, _ := json.Marshal(g.Stock)
	wstJSON, _ := json.Marshal(g.Waste)

	query := `
INSERT INTO solitaire_games (
    game_id, owner_id, computer_id, state, deck_seed,
    tableau_json, foundations_json, stock_json, waste_json,
    score, moves_count, created_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON DUPLICATE KEY UPDATE
    state = VALUES(state),
    tableau_json = VALUES(tableau_json),
    foundations_json = VALUES(foundations_json),
    stock_json = VALUES(stock_json),
    waste_json = VALUES(waste_json),
    score = VALUES(score),
    moves_count = VALUES(moves_count),
    updated_at = VALUES(updated_at);
`
	// Also fallback to SQLite syntax if ON DUPLICATE KEY is not supported
	_, err := s.db.ExecContext(ctx, query,
		g.GameID, g.OwnerID, g.ComputerID, string(g.Status), int64(g.DeckSeed),
		string(tabJSON), string(fndJSON), string(stkJSON), string(wstJSON),
		g.Score, g.MovesCount, g.CreatedAt.Format(time.RFC3339Nano), g.UpdatedAt.Format(time.RFC3339Nano),
	)
	if err != nil {
		// SQLite compatible upsert
		sqliteQuery := `
INSERT OR REPLACE INTO solitaire_games (
    game_id, owner_id, computer_id, state, deck_seed,
    tableau_json, foundations_json, stock_json, waste_json,
    score, moves_count, created_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);
`
		_, err = s.db.ExecContext(ctx, sqliteQuery,
			g.GameID, g.OwnerID, g.ComputerID, string(g.Status), int64(g.DeckSeed),
			string(tabJSON), string(fndJSON), string(stkJSON), string(wstJSON),
			g.Score, g.MovesCount, g.CreatedAt.Format(time.RFC3339Nano), g.UpdatedAt.Format(time.RFC3339Nano),
		)
	}
	return err
}

func (s *Store) GetGame(ctx context.Context, ownerID, computerID, gameID string) (*GameState, error) {
	row := s.db.QueryRowContext(ctx, `
SELECT game_id, owner_id, computer_id, state, deck_seed,
       tableau_json, foundations_json, stock_json, waste_json,
       score, moves_count, created_at, updated_at
FROM solitaire_games
WHERE game_id = ? AND owner_id = ? AND computer_id = ?
`, gameID, ownerID, computerID)

	var g GameState
	var statusStr, tabJSON, fndJSON, stkJSON, wstJSON, createdAtStr, updatedAtStr string
	var seed int64

	err := row.Scan(
		&g.GameID, &g.OwnerID, &g.ComputerID, &statusStr, &seed,
		&tabJSON, &fndJSON, &stkJSON, &wstJSON,
		&g.Score, &g.MovesCount, &createdAtStr, &updatedAtStr,
	)
	if err != nil {
		return nil, err
	}

	g.Status = GameStatus(statusStr)
	g.DeckSeed = uint64(seed)
	_ = json.Unmarshal([]byte(tabJSON), &g.Tableau)
	_ = json.Unmarshal([]byte(fndJSON), &g.Foundations)
	_ = json.Unmarshal([]byte(stkJSON), &g.Stock)
	_ = json.Unmarshal([]byte(wstJSON), &g.Waste)
	g.CreatedAt, _ = time.Parse(time.RFC3339Nano, createdAtStr)
	g.UpdatedAt, _ = time.Parse(time.RFC3339Nano, updatedAtStr)

	return &g, nil
}

func (s *Store) SaveMove(ctx context.Context, m MoveRecord) error {
	cardsJSON, _ := json.Marshal(m.Cards)
	query := `
INSERT INTO solitaire_moves (
    move_id, game_id, owner_id, computer_id, move_seq,
    move_type, from_pile, to_pile, cards_json, created_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
`
	_, err := s.db.ExecContext(ctx, query,
		m.MoveID, m.GameID, m.OwnerID, m.ComputerID, m.MoveSeq,
		string(m.MoveType), m.FromPile, m.ToPile, string(cardsJSON),
		m.CreatedAt.Format(time.RFC3339Nano),
	)
	return err
}

func (s *Store) ListMoves(ctx context.Context, ownerID, computerID, gameID string) ([]MoveRecord, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT move_id, game_id, owner_id, computer_id, move_seq,
       move_type, from_pile, to_pile, cards_json, created_at
FROM solitaire_moves
WHERE game_id = ? AND owner_id = ? AND computer_id = ?
ORDER BY move_seq ASC
`, gameID, ownerID, computerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var moves []MoveRecord
	for rows.Next() {
		var m MoveRecord
		var typeStr, cardsJSON, createdAtStr string
		if err := rows.Scan(
			&m.MoveID, &m.GameID, &m.OwnerID, &m.ComputerID, &m.MoveSeq,
			&typeStr, &m.FromPile, &m.ToPile, &cardsJSON, &createdAtStr,
		); err != nil {
			return nil, err
		}
		m.MoveType = MoveType(typeStr)
		_ = json.Unmarshal([]byte(cardsJSON), &m.Cards)
		m.CreatedAt, _ = time.Parse(time.RFC3339Nano, createdAtStr)
		moves = append(moves, m)
	}
	return moves, rows.Err()
}
