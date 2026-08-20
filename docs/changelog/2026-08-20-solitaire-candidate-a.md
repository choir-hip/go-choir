# Changelog: 2026-08-20 — Solitaire Headless Engine (Candidate A)

## Added
- `internal/solitaire`: Headless Klondike Solitaire engine, deck shuffler, and move validator.
- `solitaire_games` & `solitaire_moves`: Additive database schema for game state and move persistence.
- REST API handlers mounted at `/api/solitaire/games`.
- Unit test suite verifying standard game play and move validation.
