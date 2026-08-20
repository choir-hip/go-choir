# Solitaire Headless Engine & REST API (Candidate A)

**Package:** `internal/solitaire`  
**API Surface:** `/api/solitaire/games`  
**Storage:** SQLite/Dolt tables `solitaire_games` and `solitaire_moves`  

## Overview
Provides a deterministic headless Klondike Solitaire game engine. Supports standard 7-column tableau dealing, stock drawing, waste recycling, tableau-to-tableau cascading, and foundation stacking.

## REST Endpoints
- `POST /api/solitaire/games`: Initialize and deal a new game with optional `{"seed": uint64}`.
- `GET /api/solitaire/games/{id}`: Query current game state.
- `POST /api/solitaire/games/{id}/moves`: Apply a game move (`draw_stock`, `recycle_waste`, `waste_to_tableau`, `waste_to_foundation`, `tableau_to_foundation`, `tableau_to_tableau`).
- `GET /api/solitaire/games/{id}/history`: Retrieve ordered move history.

## Known Characteristics (Candidate A)
Candidate A includes standard Klondike tableau stacking rules and rank-ascending foundation rules.
