package maild

import "github.com/yusefmosiah/go-choir/internal/frontrank"

// FrontRow is one row of corpusd's internal front list as the daily digest
// reads it. The rank rule itself lives in internal/frontrank, where the front
// render record reads the same code (docs/problems/autopaper-daily-digest-
// phase1-2026-10-20.md, problem 1: the substrate fix is one rank both read).
type FrontRow = frontrank.Row

// rankFrontRows is the page's rank order; see frontrank.Rank.
func rankFrontRows(rows []FrontRow) []FrontRow { return frontrank.Rank(rows) }
