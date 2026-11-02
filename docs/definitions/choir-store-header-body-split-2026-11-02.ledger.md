# Store Header/Body Split — Ledger

Append-only. One terse entry per pass.

- 2026-11-01T19:00-0500 drafted at 3529f400d after five reread fixes on 2026-11-01 that were one defect (filter fields after the body).
- 2026-11-01T19:37-0500 executable on the owner's word ("Make it a throughline now, superseding"); supersedes the guest isolation goal; now.status working, slice S0 (page reads per statement, three layouts).
- 2026-11-01T19:42-0500 S0: split beats body-last and today on every header read (census 19 vs 214 vs 737 MiB; retire slice 0.23 vs 2.68 vs 9.8); body reads equal. Evidence docs/evidence/store-layout-reads-2026-11-01.md. Next S1.
