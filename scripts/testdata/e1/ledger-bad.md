# Fixture ledger

### Claims

| id | recorded (UTC) | who | kind | claim, with its test | p | settle by | evidence source |
|---|---|---|---|---|---|---|---|
| C901 | 2026-01-01T00:00Z | lead | experiment | E1-M pilot P1: invented claim text. Objectives sha256 SHAPLACEHOLDER. | 0.45 | 2026-01-02T08:00Z | scripts/e1 read --verdict |
| C902 | 2026-01-01T00:00Z | lead | experiment | unrelated claim: invented claim text. Objectives sha256 SHAPLACEHOLDER. | 0.45 | 2026-01-02T08:00Z | scripts/e1 read --verdict |

### Settlements

| id | settled (UTC) | outcome | evidence | Brier |
|---|---|---|---|---|
| C850 | 2026-01-01T00:00Z | true | invented | 0.1 |
| C903 | 2026-01-01T00:00Z | true | invented | 0.1 |
