# Reference pages: dependencies newer than training data

Convention (owner, 2026-11-10): a dependency that is newer than a coding
agent's training data (a model, an API, a framework, a runtime) gets one
page here, named in capitals after the dependency (`SYSTEM1.md`, `EXACT.md`).

Each page states:

- what the dependency is, and the version or date observed;
- how we call it, with file:line;
- its limits and prices, each figure marked **verified** (how, when) or
  **relayed-unverified**;
- the rules we follow;
- the mistakes already made;
- sources, with fetch dates.

Upkeep: a page is updated in the same change that changes how we use the
dependency. An agent reads the page before touching the dependency and never
answers a question about it from memory. A figure that cannot be re-checked
stays marked as relayed, never promoted by repetition.

Pages:

- [SYSTEM1.md](SYSTEM1.md): System 1 decision models (Jev today, via OpenRouter): rules, wire shape, surprises, patterns, our measurements; eight evidence subpages in [system1/](system1/).
- [ON-DEVICE-MODELS.md](ON-DEVICE-MODELS.md): models and runtimes for the
  Autoradio phone app; sources fetched 2026-11-11 (section 8); only Silero
  and Parakeet EOU are measured on a phone by us.
- [EMBEDDINGGEMMA.md](EMBEDDINGGEMMA.md): EmbeddingGemma 2 (Core AI port by Everpaper, macOS 27 only as declared; files, prompts, Matryoshka, licence), the GGUF alternative, the on-device vector store pick (brute force first, sqlite-vec later) and BM25 + RRF hybrid ranking; sources fetched 2026-11-17, nothing measured on a phone.
- [EXACT.md](EXACT.md): the Exact app-shell runtime (pinned commit, model, surprises, decision guide), with subpages in [exact/](exact/): ui-model, native-modules, deck, audio, build-and-ship, testing-and-debugging, internals, gaps, auth-session. The runnable example is [mobile/exact-spike/](../../mobile/exact-spike/README.md).
- [CERAMIC.md](CERAMIC.md): Ceramic, a lexical web search service (English pages, keyword queries, 50 words, 20 results): the call, query rules, errors, limits, price, and what its documents do not say.
- [RIVER.md](RIVER.md): River, the host for DeepSeek V4.1 Flash 1017: the one model we use, Chat Completions versus Responses, reasoning values, caching, the gateway variables and the procedure to register the key on Node B (not yet done on 2026-11-14).
- [AUTORADIO-TURNS.md](AUTORADIO-TURNS.md): `POST /api/autoradio/turns`, the Autoradio desk's fast answer: request, server-sent event shapes, records, pool order, measuring (slice 1).
- [AUTORADIO-PROGRAMME.md](AUTORADIO-PROGRAMME.md): `POST /api/autoradio/programme`, the DJ's stretch with one forecast claim per content segment (station L1 of the learning engine): request, events, validators, records, replay.
- [AUTORADIO-SHEET.md](AUTORADIO-SHEET.md): the server run sheet, `GET /api/autoradio/sheet`, `POST /api/autoradio/sheet/ops`, `GET /api/autoradio/sheet/stream`: shapes, dropped reasons, bounds, records, replay (slice D1).
- [AUTORADIO-EDITION-PICKS.md](AUTORADIO-EDITION-PICKS.md): the phone's edition picks as shared Exact code: the old Swift file's carry-over rule (proof from the account's notebook), and the state table, account by account.
- [PODCAST-CLIP-FINGERPRINT.md](PODCAST-CLIP-FINGERPRINT.md): the podcast clip fingerprint (8 log-mel bands, 10 frames a second, 12 s head and tail), the locate algorithm the phone runs, the clip record fields, how the worker makes and repairs them; conformance fixture in `mobile/fixtures/`; built in a lane, not deployed.
- [CAPTURE-FULLTEXT.md](CAPTURE-FULLTEXT.md): the full-text index of captured pages (BM25 over passages, SQLite FTS5 in corpusd): what text a capture holds, the four engines measured on a synthetic corpus and why FTS5, the passage table the vector index will key on, resource projections at 250,000 and a million pages, the catch-up and its restart rules, and the follow-up that removes the title term table's search role.
- [LANE.md](LANE.md): `lane`, copy-on-write git worktrees (two seconds, about 100 MB): how a worktree is made here, the rules, what we do not use.
- [ORDINARY-PROMPT-PATH.md](ORDINARY-PROMPT-PATH.md): the policy for a prompt, in the owner's words (v0 raw, v1 from model weights, v2 at research's first message, one way to open a Texture, Texture woken by messages), where each clause lives in the code, and the tests that pin it.
