# Retrieval over the Dolt object graph: research, first benchmark, and the ladder before embeddings (2026-10-14)

Owner ruling, about 15:10 UTC (recorded in
[search-efficiency-and-reuse-2026-10-14.md](search-efficiency-and-reuse-2026-10-14.md)):
"take our retrieval and query patterns over the object graph as a serious
research project. We basically should table the vector embeddings until we
reach diminishing marginal returns with our Dolt versioned SQL object
graph." This note is that project's first pass: what we query today, what we
will need to query, what SQL and Dolt can and cannot do for each need
(measured on Dolt 2.1.9, the version Store B runs), a ladder of improvements,
and the measurable point at which embeddings would earn their place.

Mutation class of this note and its harness: **green/yellow** (docs; a
benchmark program under `scripts/ograph-bench/` that refuses non-loopback
databases). Nothing on Node B or staging was queried or changed; the only
Node B contact was read-only (`systemctl show` for the Dolt binary path,
`du` of the corpus directory, `journalctl` of corpusd).

Companions: [claims-events-entities-ontology-2026-10-14.md](claims-events-entities-ontology-2026-10-14.md)
(the ledger layer this retrieval serves),
[search-is-duplicated-and-discarded-2026-10-14.md](../problems/search-is-duplicated-and-discarded-2026-10-14.md),
[l2a-merge-recall-on-the-paper-2026-10-14.md](../problems/l2a-merge-recall-on-the-paper-2026-10-14.md).

## 0. Findings in one page

1. **Today nothing is retrieved in SQL.** Every corpus, story and citation
   lookup loads a window (the newest 1,000 captures, or every Texture
   document with its body) and matches words in Go. At today's ~4,500
   captures a day the corpus window is about 5 hours. Four hot lookups are
   JSON-path scans that no index can serve as written. No retrieval call is
   timed anywhere (§1).
2. **At 1M captures and 10M edges, today's shapes cost seconds**
   (measured on local Dolt 2.1.9, the version Store B runs): the corpus-search
   window 5.3 s p50, a URL lookup through `JSON_EXTRACT` 5.0 s, a title
   `LIKE` 16.7 s, a "this edge kind today" count 14.7 s (§4 A).
3. **Typed columns and composite indexes take the structured needs to
   about 1-30 ms**: URL/known-item 0.6 ms, the corpus window 15-28 ms
   *with a time bound*, same-event candidates by shared entities and time
   5.5 ms, settlement due 10 ms, alias exact 0.5 ms, typo-tolerant alias 69
   ms, story → captures → claims 3.3 ms (§4 B, E).
4. **Dolt's planner decides whether an index helps, and it is easy to get
   wrong.** `ORDER BY ... DESC LIMIT` sorts the whole range (an index alone
   made the window *slower*, 14.7 s); one added index flipped an edge
   lookup from 1.5 ms to 12 s, and index hints and `ANALYZE` did not help; a
   plain `JOIN` hash-joined 5M claims (11.8 s) where a CTE took 3.3 ms.
   Every shipped retrieval query needs a plan-shape test (§4 G).
5. **Dolt FULLTEXT is fit for single words on short text only.** On 2.1.9:
   natural-language mode only (no phrases, no `+`/`-`, no prefix), no
   ngram/CJK, case-sensitive under the default collation, tokens under 3
   characters dropped ("AI", "EU", "US" are unsearchable, and 46.8% of
   real research queries contain such a token), duplicate rows, and the
   index is used only when `MATCH` is the whole `WHERE`. Building it on 1M
   titles took 642 s and 14 GB of transient garbage (§3.1, §4 C).
6. **Our own postings table beats FULLTEXT for titles** (any token length,
   CJK bigrams, phrases by AND + verify; 0.3 ms per word, ~4 s of writes a
   day) **but body text does not belong in Dolt at all**: a day's 36k blocks
   cost 282 s through FULLTEXT and 197 s through postings, versus 1.4 s in
   SQLite FTS5 with native phrase search (§4 D). Body text goes in a derived,
   rebuildable FTS5 file beside corpusd; Dolt keeps every key and the
   authority.
7. **Versioning is cheap where we need it.** A day of writes commits in
   0.29 s; `AS OF` a tag is as fast as head. "What we knew at T" should be
   an indexed timestamp filter over append-only rows, with `DOLT_TAG`
   witnesses at publication and settlement boundaries; rebuildable
   projections go in `dolt_ignore`d tables (§3.2 N10, §4 F).
8. **Embeddings wait for a measured plateau.** The ladder (§5) ends where
   recall on a frozen labelled set (§6) stops moving and the remaining
   misses are semantic-only (no shared token, entity, alias or gloss).
   Cross-lingual matching on named entities needs no vectors: Wikidata ids
   and multilingual aliases are language-neutral keys.

**Recommended first three changes:**

1. **Instrument and freeze the yardstick** (rung 0, yellow): time and count
   every retrieval call on guest `/health` and corpusd, log corpus-search
   queries with result ids, and build `retrieval-eval-v1` from the 7,810
   real queries, the L2a pairs and an entity sample (§6).
2. **Fix today's shapes without new tables** (rung 1, orange): Store B
   composite `(object_kind, owner_id, computer_id, updated_at)` plus a time
   bound on every "newest N"; functional or generated-column indexes for the
   JSON-path lookups (`capture_label.capture_id`, `public_route.route_path`,
   `publication_check.publication_version_id`,
   `publication_proposal.proposal_id`) with queries rewritten to the literal
   indexed expression; `(to_id, kind)` edge index; plan-shape tests.
   Rollback: drop the indexes, revert the query text.
3. **SQL corpus search over a typed capture projection** (rungs 2-3,
   orange): `capture_index` written with each capture, URL/host/source/time
   lookups and read-through by canonical URL, and a `dolt_ignore`d title
   postings table with our tokenizer, so corpus-first sees all history
   instead of five hours. The FTS5 body index (rung 4) follows once the
   eval set shows what titles alone miss.

## 1. Today's query patterns

Inventory of every read of `og_objects`/`og_edges`, captures, stories and
Texture documents made for retrieval or lookup (code at `origin/main`
49919474c). **No retrieval query does text matching in SQL**: there is no
`MATCH`, no `FULLTEXT`, and `LIKE` appears only on two non-graph tables. All
matching is Go-side, after loading a bounded window or a whole kind.

### 1.1 Schemas and indexes

| Plane | Table | Indexes today |
|---|---|---|
| Guest (embedded Dolt, `internal/objectgraph/dolt_store.go`) | `og_objects` | PK `canonical_id`; `(object_kind, owner_id)`; `(updated_at)`; `(owner_id, computer_id, object_kind)`; `(object_kind, owner_id, computer_id)`; generated columns `og_meta_run_id`, `og_meta_trajectory_id`, `og_meta_channel_id`, each indexed `(object_kind, col, updated_at)` |
| Guest | `og_edges` | PK `edge_id`; `(from_id)`; `(to_id)` |
| Store B (corpusd, `internal/platform/store.go:131-160`) | `og_objects` | PK; `(object_kind, owner_id)`; `(updated_at)`. **No `computer_id` index, no generated columns.** Bodies over 1 KiB live on disk (`body_ref`) |
| Store B | `og_edges` | PK; `(from_id)`; `(to_id)` |

Every guest `DoltStore` call holds `engineMu`, so all guest reads and writes
are serialized on one engine (`dolt_store.go`, `engine_mutex.go`).

### 1.2 Store B (corpusd) reads

| # | Call site | Caller, frequency | Predicate / order / limit | Index use | Matching |
|---|---|---|---|---|---|
| B1 | `ObjectGraphStore.ListObjects` (`platform/objectgraph_store.go:205-224`) via `GET /internal/platform/objects` | `searchCaptures` (`researchtools/corpus_search.go:153`, N=1000, every `search_wire_corpus` call: 122 of 136 paper research runs); `TextureCaptures.newestCaptureOf` (`texture_captures.go:64`, N=1000, per URL citation handle per frame build, no cross-handle cache); `TraceCapturesInWindow` (N=500 per `GET /api/captures`) | `object_kind=? AND owner_id=? AND computer_id=? ORDER BY updated_at DESC LIMIT ?`, all columns | `(object_kind, owner_id)` range over **all the owner's captures on every computer**; `computer_id` residual; sort | Go: word-prefix match on title, label headline, outlet/host/source, language (`corpusTermIn`); URL equality in Go for citations |
| B2 | `ReadCapture` (`platform/captures.go:244-329`) | `choir.ReadCapture`, Texture capture reads, publish (`captureBlockDoc`) | PK + two disk blob reads | PK | — |
| B3 | `sourcegraph.WriteCaptures` (`sourcegraph/captures.go:134-245`) | every polled item and every `choir.Read` | PK `GetObject(autopaper:capture:<canonical_url>#<doc_hash>)`, then `INSERT IGNORE` | PK | — (identity includes `doc_hash`, so a changed page is a new capture: 34% of window captures are re-captures) |
| B4 | `choir.Read` (`researchtools/read.go`) | research | **no lookup before fetch**; dedupe only after the fetch through B3 | — | 17% of reads re-fetch a URL already read |
| B5 | Front page (`publication_front.go:165-253`) | each front load | `object_kind='choir.publication' AND tombstone=FALSE ORDER BY updated_at DESC, canonical_id DESC LIMIT ?`, then 4-5 queries per publication (edge lists by `from_id`, PK reads, blob reads) | partial; sort | Go filter on state/mode/owner |
| B6 | Publication resolve (`service_publication_read.go:22-123`) | each public story load | `GetObjectByMetadata('choir.public_route','$.route_path')`: `JSON_UNQUOTE(JSON_EXTRACT(CAST(metadata AS JSON), ?)) = ?` | **scan of the kind** | then 10-20 edge/PK reads |
| B7 | Publication checks (`publication_checks.go:181`) | per publication read | `ListObjectsByMetadata('choir.publication_check','$.publication_version_id')` | **JSON scan** | — |
| B8 | `SearchPublished` (`service_publication_read.go:557-695`) | `/api/platform/retrieval/search` | newest 100 publications, ~8 queries each | — | Go `strings.Count`, top 20 |
| B9 | `UpdateProposalDeliveryState` (`publication_graph.go:982`) | write path | JSON scan on `$.proposal_id` | **scan** | — |

### 1.3 Guest (embedded Dolt) reads

| # | Call site | Caller, frequency | Predicate | Index use | Matching |
|---|---|---|---|---|---|
| G1 | `SearchDocuments` (`store/texture.go:554-641`), the story half of `SearchWireCorpus` | every corpus search | `ListObjectsPage(kind=texture_document)` loop: every owner's documents **with bodies**, page 256, `ORDER BY canonical_id`; then one PK read of each candidate's head revision | kind prefix only; re-ranges and sorts per page | Go `strings.Contains` on titles, then revision text |
| G2 | `RecentCaptureLabels` (`store/capture_label.go:169-193`) | every corpus search, limit 2000 | `kind, owner, computer ORDER BY updated_at DESC LIMIT ?` | `(object_kind, owner_id, computer_id)`; sort | Go |
| G3 | `ListStoryDocuments` (`store/stories.go:41-80`) | `choir.Stories` (management, per cell that calls it); intake shortlist once per poll with new captures (~270 captures/hour at 28 sources) | same all-documents scan as G1, then Go filters, then **one PK lifecycle read per document** (N+1) | as G1 | Go |
| G4 | Intake shortlist for Jev (`agentcore/intake_labels.go:87-153`) | per poll | G3, then Go: updated within 72 h, newest 30, plus up to 10 by title-token overlap | — | Go token overlap |
| G5 | `LatestCaptureLabel` / `CaptureLabels` (`capture_label.go:134-165`) | per labelled capture; per arrival item in every management cell load; per disposal | `ListObjectsByMetadataPage(capture_label,'$.capture_id')` | **JSON scan of every label row per call** (the Page variant never uses generated columns) | — |
| G6 | `GET /api/texture/documents` (`textureowner/handler.go:149-185`) | each page load | two full document scans + per-document revision stats (`CountObjectsByEdgeTo`, edge join) + PK read | partial | Go sort, cap 50 |
| G7 | `GetObjectByMetadata`, `ListObjectsByOwnerAndBody`, `ListJSONBodyFieldsByKindOwner` | various | JSON extraction from metadata or **from the body** per row | scan within kind/owner | — |

`internal/yaegikernel/search_store.go` is the per-session in-memory store of
web search results; it does not read the graph. `objectgraph.HTTPStore` has
no production constructor.

### 1.4 Known latencies

There is **no timing** on `searchCaptures`, `SearchDocuments`,
`ListStoryDocuments`, the front page or resolve, and corpusd logs no request
latency (checked: corpusd's journal for the last 2 h holds only lifecycle and
artifact-GC lines). What exists:

- JSON_EXTRACT full scans "~1.4 s/call under drain" (`dolt_store.go:150`;
  [texture-latency-layered-evidence-2026-10-01](../problems/texture-latency-layered-evidence-2026-10-01.md)):
  guest Dolt reads 7 ms idle, 1-3.7 s under desk drain; `engineMu` held ~97%
  of wall time over 5 minutes; `ListObjectsByMetadata` ~1.4 s/call,
  `ListObjects` ~1.6 s/call, `ReadObjectSnapshotFiltered` ~580 ms/call.
- `GET /api/texture/documents` 2.67 s cold, 0.15-0.17 s warm
  ([texture-list-cold-latency-2026-10-09](../problems/texture-list-cold-latency-2026-10-09.md)).
- Replay page query 3.6 s with zero rows (16-28 s from the guest), ~0.02 s
  with an index ([replay-page-query-scans-whole-chain-2026-10-09](../problems/replay-page-query-scans-whole-chain-2026-10-09.md)):
  the same Dolt scan pattern on Store A.
- Scale: Store B held 8.76M `og_objects` and 4.38M `og_edges` when it was
  reset on 10-09 ([world-wire-store-b-inventory-2026-10-09](../evidence/world-wire-store-b-inventory-2026-10-09.md));
  the corpus directory on Node B is 22 GB today (`du`, read-only). Captures
  arrive at ~4,500/day (1.4 GB/day of bodies on disk, outside Dolt).

Measured on synthetic data (§4): the corpus-search window query B1 costs
**5.3 s p50 (14 s p95)** at 1M captures, and the JSON-path URL lookup is a
full scan (**5.0 s p50, 18 s p95**).

## 2. Retrieval needs of the next months

What the designs ask for, with the shape of the query. The query mix comes
from the 7,810 gateway search requests of 10-12 to 10-14 (read-only extract
in `~/.config/choir-qa/traces/search/`, outside the repo), which are the
questions research asks today and which corpus-first retrieval must answer.

| Real web queries (n = 7,810) | Share |
|---|---|
| contain a quoted phrase | 18.3% |
| use `site:` | 11.4% |
| contain a two-character token (AI, EU, US, UK, 5G, a number) | **46.8%** |
| contain "AI" | 14.4% |
| contain a multi-word proper name | 58.8% |
| contain a year or date word | 46.5% |
| non-Latin script (CJK 1.3%) | 1.5% |
| URL-like | 1.8% |
| tokens per query | median 8, p90 12 |

| Need (design source) | Query shape | Keys |
|---|---|---|
| N1 Corpus-first known-item lookup before web search (search report §3; ontology E5) | exact URL; canonical URL; HN item id; host (`site:`); title words; quoted phrase in body; recent first | `canonical_url`, `host`, `source_id`/`item_id`, `fetched_at`, title tokens, body phrases |
| N2 Read-through (search report §3.3) | newest capture of a canonical URL younger than a bound | `(url_hash, fetched_at)` |
| N3 Earlier searches (`choir.search_result` pointers, ontology §9.4) | same normalized query key within TTL; URLs returned for a query; queries that returned a URL | `query_key`, `(url_hash)`, `created_at` |
| N4 Same-event lookup for merging (L2a; ontology §9.1, E2-E3) | candidate events for a capture: shared participant entities within a time window, same place, overlapping propositions; recent events; `part_of` siblings (topic vs event) | `(entity_id, start_at)`, `(event_id)`, `(proposition_id)`, place entity |
| N5 Claims by speaker / proposition / time (ontology §4, §6) | a speaker's claims in a window; all claims asserting a proposition, by stance and outlet; claims of an event | `(speaker, fetched_at)`, `(proposition)`, `(event, fetched_at)` |
| N6 Entity resolution (ontology §12.1 step 4) | mention surface form → candidates: exact alias (any language), Wikidata QID, prefix, typo-tolerant, transliteration | normalized alias, QID, alias trigrams |
| N7 Story → sources → claims navigation (event dossier, ontology §9.2) | story → covered events → captures → claims, grouped by independence group | edges `(to_id, kind)`, `(from_id, kind)`, claims by capture |
| N8 Settlement queries (ontology §6) | open forecasts/commitments with deadline in a window, scheduled selection only; claims needing evidence | `(settled, modality, deadline)` |
| N9 Front page / depth aggregates (ontology §9.3) | per event: independent outlets, languages, claims per hour, contradicts density, cited share of propositions | aggregates over claims × capture × outlet, per event |
| N10 History / AS OF for track records (ontology §4.1, §6) | what the graph said at T (precommitment condition: "no settling evidence in the graph at `fetched_at`"); a claim's revisions; settlement revisions | append-only timestamps; `AS OF`; `dolt_history_*` |

## 3. What SQL and Dolt can do for each need

### 3.1 What Dolt 2.1.9 does, measured here

Probed directly on `dolt 2.1.9` (the binary `go-choir-corpus-dolt.service`
runs on Node B; read from `systemctl show`). Dolt's own docs mark FULLTEXT
as "Only basic functionality supported"
([data description](https://www.dolthub.com/docs/sql-reference/sql-support/data-description/);
design: [implementing full-text indexes](https://www.dolthub.com/blog/2023-08-14-implementing-fulltext-indexes/),
five hidden tables per index).

| Capability | Result on 2.1.9 | Consequence |
|---|---|---|
| `MATCH ... AGAINST` natural language mode | works; returns a relevance score | usable for word lookup |
| `IN BOOLEAN MODE` (phrases, `+`/`-`, `word*`) | **error: not supported yet** | no phrase search, no required terms, no prefix search |
| `WITH QUERY EXPANSION` | not supported | — |
| `WITH PARSER ngram` | **syntax error** | no CJK segmentation: a CJK title is one token; `人工智能` does not match `中国发布新的人工智能模型` |
| Case | under the database default collation `utf8mb4_0900_bin`, matching is **case-sensitive** (`power` matches, `Power`/`POWER` do not; `украины` misses `Украины`) | index a `utf8mb4_0900_ai_ci` column; with it, `russian` and `украины` match |
| Short tokens | tokens under 3 characters are dropped (`IQ`, `14` miss; `bit`, `000` match); stopwords (`the`, `in`) dropped | **"AI", "EU", "US", "UK" are unsearchable**, and 46.8% of real queries carry such a token |
| Duplicate rows | a row matching two query tokens is returned twice (`600,000` → row 1 twice) | wrap in `DISTINCT` |
| Planner | the FULLTEXT index is used **only when `MATCH` is the whole `WHERE`**; `MATCH ... AND fetched_at >= ?` (or `AND text LIKE ?`) becomes a full table scan evaluating `MATCH` per row; a derived table gets the filter pushed back in. A **CTE holding only the `MATCH`, joined back by primary key**, keeps the index (`LookupJoin`; `EXPLAIN PLAN` in §4) | every combined query must use the CTE shape |
| FULLTEXT on a generated column over JSON metadata | works | — |
| Functional indexes `((JSON_UNQUOTE(JSON_EXTRACT(metadata,'$.x'))))`, `((LOWER(title)))` | **work** (hidden generated column; `EXPLAIN` shows `IndexedTableAccess`), on `LONGTEXT` metadata | the JSON-path scans of B6, B7, B9, G5 can be indexed without a schema change, **provided the query uses the same literal expression**: today's code passes the path as a bind parameter and wraps `CAST(metadata AS JSON)`, which no index can match |
| `dolt_ignore` | tables matching a pattern are never staged or committed (verified: `proj_%` absent from `dolt_status` after `DOLT_ADD -A`) | rebuildable projections can live in Store B without growing history |
| `AS OF` / `dolt_history_<table>` | work (§4) | time travel at commit granularity |

### 3.2 Need by need

**N1, N2 known-item and read-through.** A typed projection of capture
metadata, one row per capture, written in the same transaction as the
`og_objects` row by the corpusd capture route (or as stored generated
columns on `og_objects`; both measured):

```sql
capture_index(canonical_id PK, owner_id, computer_id, url_hash CHAR(32),
  canonical_url, host, source_id, item_id, lang, fetched_at, published_at,
  title, title_ci COLLATE utf8mb4_0900_ai_ci)
  INDEX (url_hash), INDEX (computer_id, fetched_at), INDEX (source_id, fetched_at),
  INDEX (host, fetched_at)
```

URL, HN item, `site:` and "newest of this source" become index seeks (§4:
URL lookup 0.6-0.7 ms instead of 5 s). Read-through is `WHERE url_hash=? AND canonical_url=? ORDER BY
fetched_at DESC LIMIT 1`. B1's window query is **not** fixed by an index alone:
with the composite `(object_kind, owner_id, computer_id, updated_at)` it got
slower (14.7 s), because Dolt sorts the whole equality range for `ORDER BY
... DESC LIMIT`; with a time bound (`updated_at >= now - 24h`, widened when
short) it takes 28 ms (§4 G2).

**Title words.** Dolt FULLTEXT over `title_ci` answers single words in
milliseconds (§4) but drops 2-character tokens, has no phrases and no CJK.
Our own **inverted index table** removes all three limits:

```sql
title_postings(term VARCHAR(64) COLLATE utf8mb4_0900_bin, doc_id, ord, first_pos,
  PRIMARY KEY (term, doc_id, ord))          -- dolt_ignore'd; rebuildable
```

The tokenizer is ours (Go; the harness prototype in
`scripts/ograph-bench/postings.go` does the first three): lowercase, every token length
kept, CJK runs emitted as overlapping bigrams (the ngram parser Dolt lacks),
optional transliteration of Cyrillic. A multi-word query is an `IN` list with
`GROUP BY doc HAVING COUNT(*) = k`; a phrase is that plus a `LIKE` check on
the joined row. Titles are ~12 postings each: 4,500 captures/day ≈ 54k
posting rows/day, about 4 s of writes at the measured 1,118 titles/s
(10.8k postings/s); word lookups 0.3 ms, three-word phrases 9 ms p50
(measured on the newest 100k titles; diacritic folding and Cyrillic
transliteration are proposed, not prototyped).

**Body text (quoted phrases).** This is where Dolt is the wrong engine, and
the measurement is unambiguous: inserting one day of block text (36,000
blocks) took **282 s through Dolt FULLTEXT** (128 blocks/s) and **197 s
through our own postings table** (183 blocks/s, 9.1k postings/s), against
**1.4 s in SQLite FTS5** (25,700 blocks/s, `unicode61`; 4.9 s with the
`trigram` tokenizer), which also has native phrase queries and BM25. Dolt
FULLTEXT's five hidden tables are versioned like any table, so body text
would also enter Dolt history. Body-text search therefore belongs in a
**derived FTS5 file beside corpusd**: keyed by `capture_id#block_ord`,
rebuildable from the CAS blocks blobs, reporting its newest indexed capture
with every answer (index lag visible), never an authority. That is the
companion report's §3.1 choice, now with a measured reason; it does not
conflict with the ruling, because the authority and every structured key
stay in Dolt and no vectors are involved.

**N3 earlier searches.** `search_result` pointer objects in `og_objects`
plus a typed `search_hits(search_id, query_key, url_hash, rank, provider,
created_at)` with `(query_key, created_at)` and `(url_hash)` indexes;
hit titles go into `title_postings`. Exact repeats are an index seek; "what
did we already find for this subject" is the title-postings query over
search hits and captures together.

**N4 same-event lookup.** With `event_participants(entity_id, start_at,
event_id, role)` keyed `(entity_id, start_at, event_id)`, the candidate
query is one indexed range per entity, aggregated:

```sql
SELECT event_id, COUNT(*) n_shared FROM event_participants  -- "shared" is reserved
 WHERE entity_id IN (:capture_entities) AND start_at BETWEEN :t-72h AND :t+72h
 GROUP BY event_id ORDER BY n_shared DESC LIMIT 20
```

(5.5 ms p50, 11 ms p95 at ~1.6M participant rows). Proposition overlap is the
same shape over `claims(proposition_id)`. Place is a participant with role
`place`. Topic-versus-event (`part_of` siblings) is one PK hop on
`world_events.parent_event_id`. Ranking is a small feature score in Go
(shared entities weighted by inverse frequency, time distance, shared
place, title-token Jaccard), all available without embeddings. Because
entity ids are language-neutral (Wikidata QIDs, multilingual aliases),
**cross-lingual same-event matching on named entities needs no vectors**.

**N5 claims.** `claims` as typed rows (the extraction blob stays the
authority; the row is its index, written by the same corpusd route):
`(speaker_entity_id, fetched_at)`, `(proposition_id)`, `(event_id,
fetched_at)`, `(capture_id)`, `(settled, modality, deadline)`. Measured in
§4: a typical speaker's last 30 days 0.9 ms; settlement due 10 ms; a
proposition's support by outlet 16 ms p50. The heavy cases (a speaker with
200k claims: 0.3-3 s; their box counts 1.6 s) need the same time bound as B1
and materialized counts (N9).

**N6 entity resolution.** `entity_aliases(alias_norm, entity_id, lang,
alias)` with primary key `(alias_norm, entity_id, lang)` serves exact and
prefix matches; `alias_trigrams(gram, alias_norm, entity_id)` serves typos
and transliteration variants by counting shared trigrams. Aliases live in a
side table so new surface forms do not version the entity row (ontology
§4.2). Measured: exact 0.5 ms, prefix 0.4 ms, trigram typo match 69 ms
p50 over ~3M trigram rows.

**N7 navigation.** Edges need `(to_id, kind)` and `(from_id, kind)`
composites (today only single-column), plus `claims(capture_id)`.
Measured: story → captures → claims is 3.3 ms **as a CTE** and 11.8 s as a
plain `JOIN` (the planner hash-joins all 5M claims; §4 G4).

**N8 settlement.** `(settled, modality, deadline)` makes "forecasts and
commitments due in this window, scheduled only" a range scan.
Measured: 10 ms for a day's due window over 5M claims.

**N9 aggregates.** Counts per event (outlets, languages, claims per hour,
contradictions) are GROUP BYs over `(event_id, ...)` indexes; at front-page
rates they should be **materialized projection rows** (per event, updated on
each judgment batch) in `dolt_ignore`d tables, as the ontology already
prescribes for memberships and counts (§5.3), not computed per page load.

**N10 history.** Two mechanisms, and the design should lean on the first:

1. **Append-only rows with their own times.** Captures, extractions, claims,
   judgments, settlements and search pointers are append-only and carry
   `fetched_at`/`created_at`, and supersession is a new row. "What did the
   graph hold at T" is then `WHERE created_at <= T` plus "not superseded by
   a row created at or before T": an indexed query on the current head that
   needs no Dolt history at all, and works in the `dolt_ignore`d
   projections too.
2. **Dolt `AS OF` and `dolt_history_*`** for what is mutable (canonical
   entity/event/proposition rows: labels, glosses, intervals) and as a
   **witness**: a Dolt commit hash at T is a content-addressed proof of the
   whole graph's state. Store B commits on a 45 s debounce
   (`doltbatch.DefaultDebounce`), so `AS OF '<timestamp>'` has 45 s
   resolution; scoring should pin **named tags** at settlement and
   publication boundaries (`DOLT_TAG`), cheap and exact, rather than rely on
   timestamps.

Costs: history grows with every changed row and index page; the 218 GB
incident ([platform-dolt-oldgen-218g-dead-history-2026-08-26](../evidence/platform-dolt-oldgen-218g-dead-history-2026-08-26.md))
was per-mutation commits, now debounced. Measured here: a synthetic day of writes (4.5k captures, 36k blocks, 22k
claims, 40k edges) committed in 0.29 s, and the repo size moved within GC
noise; FULLTEXT is the exception (its build on 1M titles grew the repo to
20 GB before GC). Rules
that follow: projections, postings and aggregates go in `dolt_ignore`d tables
(rebuildable, no history); body text never enters Dolt; FULLTEXT, if used at
all, only on short columns.

## 4. Benchmark

### 4.1 Plan

Harness: `scripts/ograph-bench/` (Go; `gen` writes CSVs, `run` times
queries, `postings` builds and queries our own inverted index), with
`schema.sql`, `indexes.sql`, `load.sh` (chunked import with `dolt gc` per
chunk), `stage.sh` (one index at a time, timed, with sizes), `asof.sh` (one
synthetic day of writes, commit, `AS OF`), `suites.sh`. `run` refuses any
DSN that is not loopback. Everything ran against a **local `dolt sql-server`
2.1.9** on an 8-core, 16 GB laptop (Apple M1), one connection, warm
cache, sequential queries; Node B is a 6-core Xeon E-2236 with 31 GB, so
absolute numbers will differ, ratios should hold.

Data (synthetic, `seed 20261014`): 1.0M captures over 222 days (~4,500/day)
with Store B-shaped metadata JSON (70% English; de, fr, es, ru, uk, zh, ja,
ar), titles from a 40k-word Zipf vocabulary with 2-letter tokens and CJK;
0.5M other objects; 10.0M edges; 5.0M claims (Zipf speakers over 500k
entities, Zipf-shared propositions, 13% forecasts/commitments with
deadlines); 500k entities with ~2M multilingual aliases and 200k trigram-
indexed aliases; 300k events with ~1.5M participants. Block text: 20k
captures (~160k blocks) at full scale, and 5k captures (40k blocks) in a
small repo used for the FULLTEXT write measurements.

What was **estimated rather than measured**: Node B absolute latency;
behaviour under concurrent writers; block FULLTEXT at full scale (extrapolated
from the measured per-block rates); 10M-chunk scale; real-text term
distributions (synthetic Zipf, not news).

### 4.2 Results

All latencies in milliseconds, p50 / p95, one connection, warm. "Measured"
means run on this laptop; nothing here was run on Node B.

**Scale reached.** Full scale for captures (1.0M; 1.5M `og_objects`), edges
(10.0M), claims (5.0M), entities (500k; ~2.0M aliases, ~3M trigram rows),
events (300k; ~1.6M participants). Block text: 160k blocks in the large repo
(no FULLTEXT on them); FULLTEXT and postings on block text were measured in
a small repo (40k-76k blocks) while the large import ran beside it, so those
numbers carry CPU contention and are, if anything, pessimistic.

**A. Today's query shapes (Store B indexes only, 1M captures, 10M edges).**

| Query | p50 | p95 |
|---|---|---|
| B1 corpus-search window: newest 1,000 captures of (owner, computer), all columns | **5,341** | 14,249 |
| capture by canonical URL through `JSON_EXTRACT` (what a URL lookup costs today) | **4,977** | 18,302 |
| title `LIKE '%word%'` through `JSON_EXTRACT` over all captures | 16,736 | 17,241 |
| object by primary key | 0.31 | 0.48 |
| edges from an id | 1.99 | 3.29 |
| edges to an id with a kind | 1.54 | 2.23 |
| count of one edge kind in the last day | 14,690 | 24,096 |

**B. After the proposed indexes (same data).**

| Query | p50 | p95 | vs A |
|---|---|---|---|
| URL via typed `capture_index(url_hash)` | 0.69 | 0.89 | ~7,000× |
| URL via stored generated column on `og_objects` | 0.59 | 0.74 | |
| `doc_hash` via a **functional index** on the JSON path | 0.61 | 0.92 | |
| B1 with composite `(kind, owner, computer, updated_at)`, unbounded | **14,707** | 29,289 | worse |
| B1 typed `(computer_id, fetched_at)`, unbounded | 6,183 | 6,805 | no gain |
| B1 composite **plus `updated_at >= now-24h`** | **27.9** | 31.2 | 190× |
| B1 typed plus `fetched_at >= now-24h` | 15.2 | 18.4 | 350× |
| captures of one source in the last day | 21.6 | 32.5 | |
| edges to id + kind, composite `(to_id, kind)` | 1.67 | 2.83 | (see the planner trap below) |
| edge kind in the last day, `(kind, created_at, to_id)` | 5.6 | 8.2 | 2,600× |

**C. Title search, 1M titles.**

| Query | p50 | p95 | rows |
|---|---|---|---|
| Dolt FULLTEXT on `title_ci`, rare word | 9.0 | 12.4 | 20 |
| … mid-frequency word | 12.9 | 16.5 | 50 |
| … common word, LIMIT 50 | 8.0 | 11.3 | 50 |
| … mid word ranked by score, top 20 | 99.6 | 264 | 20 |
| … two words ranked, top 20 | 46.5 | 97.7 | 20 |
| … `MATCH AND fetched_at >= now-7d` (index dropped by planner) | **13,521** | 16,669 | 18 |
| … same as a CTE joined back by PK | 31.8 | 73.9 | 18 |
| typed `title_ci LIKE '%word%'` scan | 3,073 | 3,409 | 16 |
| own postings table (100k titles, 968k postings): word | 0.32 | 0.41 | 45 |
| own postings: 3-word phrase (AND of terms + `LIKE` verify) | 9.2 | 355 | 1.6 |

**D. Block (body) text, small repo.**

| | Dolt FULLTEXT | own postings table in Dolt | SQLite FTS5 (`unicode61`) | FTS5 `trigram` |
|---|---|---|---|---|
| build or insert of 36,000 blocks (one day) | **282 s** insert (113 s to build over 40k) | **197 s** (9.1k postings/s) | **1.4 s** | 4.9 s |
| word query | 33.9 / 76.5 | 0.38 / 0.58 | 0.47 / 23.7 | 2.9 / 24.3 |
| 3-word phrase | 1,407 / 2,975 as CTE on all words; 8.3 / 924 on the longest word + `LIKE`; `MATCH AND LIKE` 3,200 | 33 / 435 | **0.36 / 1.1** (native phrase) | 5.7 / 8.3 |
| no index: `LIKE '%phrase%'` scan of 76k blocks | 776 (≈100 s extrapolated to 10M blocks) | | | |
| repo or file size | 1.9 GB before GC, 318 MB after, for 20k captures with 40k blocks | | 122 MB file for both FTS5 tables | |

**E. Ledger (5M claims, 500k entities, 300k events).** The synthetic Zipf
is harsher than news: the top three speakers have 221k, 170k and 138k claims
and the largest proposition 170k; typical speakers have a handful.

| Query | p50 | p95 |
|---|---|---|
| a typical speaker's claims, last 30 days, newest 100 | 0.87 | 1.71 |
| a top speaker's claims, last 30 days, newest 100 | 332 | 3,044 |
| claims asserting a proposition | 20.8 | 1,772 |
| a proposition's affirming claims grouped by outlet (join to captures) | 16.3 | 2,169 |
| settlement due: open scheduled forecasts/commitments with deadline in the last day | 10.2 | 10.7 |
| a top speaker's box counts (`GROUP BY modality, settled`) | 1,630 | 4,316 |
| alias exact | 0.50 | 0.71 |
| alias prefix | 0.40 | 0.59 |
| alias typo via shared trigrams, top 10 | 69 | 148 |
| **same-event candidates**: events sharing the capture's entities within ±72 h, ranked | **5.5** | 11.4 |
| story → captures → claims, plain `JOIN` | **11,805** | 37,347 |
| … same as `IN (subquery)` | 267,990 (one run) | |
| … same as a CTE | **3.3** | 5.6 |
| an entity's events, last 90 days | 0.62 | 2.81 |

**F. Writes, history, size (large repo, every index above present).**

| One synthetic day | Time |
|---|---|
| 4,500 `og_objects` rows (with the generated column and functional index) | 4.4 s |
| 4,500 `capture_index` rows (with title FULLTEXT) | 9.3 s |
| 36,000 `capture_blocks` rows (no FULLTEXT) | 4.3 s |
| 22,000 claims (5 secondary indexes) | 6.0 s |
| 40,000 edges (5 indexes) | 22.2 s |
| settle 1,000 forecasts (update) | 0.7 s |
| `DOLT_COMMIT` of the day | **0.29 s** |
| repo size change for the day | within GC noise (9.3 → 9.1 GB with server auto-GC) |

| History query | p50 | p95 |
|---|---|---|
| a top speaker's claim count at head | 22.5 | 50.9 |
| same `AS OF 'c1'` (the tag before the day) | 8.6 | 21.1 |
| a claim's rows in `dolt_history_claims` (4 commits) | 0.56 | 2.27 |

| Build step (large repo) | Time | Size after |
|---|---|---|
| chunked import of everything, with GC per chunk | 465 s (edges ~25 min separately) | 3.1 GB |
| Store B's shipped indexes | 167 s | 4.1 GB |
| stored generated column over 1.5M objects | 58.6 s | |
| functional index on a JSON path, 1.5M objects | 98.4 s | |
| edge composites on 10M edges | 53-66 s each | |
| **FULLTEXT on 1M titles** | **642 s**, repo grew to **20 GB** before GC | +0.5 GB after GC |
| each claims index (5M rows) | 26-36 s | |
| everything | | **7.6 GB** after GC |

**G. Dolt planner behaviour that decides whether an index helps.** All seen
with `EXPLAIN PLAN` on 2.1.9:

1. FULLTEXT is used only when `MATCH` is the whole `WHERE`; otherwise a full
   scan (C, D). Fix: CTE with only the `MATCH`, joined back by PK.
2. `ORDER BY t DESC LIMIT n` over an equality prefix is a **TopN over the
   whole range**, not a reverse index walk (B1 unbounded: 6-15 s on 700k
   rows). Fix: always bound the time (`t >= now - window`, widen if short).
3. Index choice is heuristic, and **`USE INDEX` / `FORCE INDEX` are ignored
   and `ANALYZE TABLE` did not change the plan**. Adding `(kind, created_at)`
   made `to_id = ? AND kind = ?` pick it over `(to_id, kind)`: **1.5 ms
   became over 12 s**. Appending `to_id` (`(kind, created_at, to_id)`)
   restored the right plan.
4. Join shape: a plain `JOIN` became a `HashJoin` over all 5M claims (11.8
   s), `IN (subquery)` a `MergeJoin` over two full indexes (268 s), a CTE a
   `LookupJoin` (3.3 ms).

So every retrieval query that ships needs a **plan-shape test**: a fixture
repo and an assertion on `EXPLAIN PLAN` (index name, no `Table` full scan,
no `HashJoin` on a large table), because one added index can silently turn
a 1 ms query into a 12 s one.

**Raw outputs** (not in the repo): session scratchpad `res-*.txt`,
`stage-big.log`, `load3.log`; the generator seed and flags above regenerate
the data.

## 5. The ladder, in order of value per effort

Each rung is measured on the §6 set before the next is built; a rung that
moves nothing is reverted or kept only for latency.

| Rung | Change | Class | Effort | What it buys (measured or expected) |
|---|---|---|---|---|
| 0 | **Instrument**: time and count every retrieval call (B1, B5, B6, G1, G3, G5, corpus search) on guest `/health` and corpusd logs; log corpus-search query, result ids and hit/miss; build `retrieval-eval-v1` (§6) | yellow | days | the baseline; today no retrieval call is timed |
| 1 | **Indexes on existing shapes, no new tables**: Store B `(object_kind, owner_id, computer_id, updated_at)`; edges `(to_id, kind)`, `(from_id, kind)`; functional indexes on hot JSON paths (`capture_label.capture_id`, `public_route.route_path`, `publication_check.publication_version_id`, `publication_proposal.proposal_id`, capture `canonical_url`) with queries rewritten to the literal indexed expression; `GetObjectByMetadata`/`…Page` use the generated columns; **every "newest N" query gets a time bound**; **plan-shape tests** (`EXPLAIN PLAN` assertions on a fixture repo) for each hot query, since one new index can flip a plan (§4 G3) | orange | small | B1 window 5.3 s → 28 ms (composite **plus a time bound**); edge kind/time 14.7 s → 5.6 ms; JSON-path lookups from a scan to ~1 ms; removes the "~1.4 s/call under drain" class |
| 2 | **Typed capture projection** (`capture_index`, written with the capture) and **SQL corpus search**: URL, read-through, HN item, `site:` host, source/time windows; title search over all captures instead of the newest 1,000 | orange | ~1 week | known-item lookups become seeks; window grows from ~5 hours to all history; read-through saves ~17% of fetches |
| 3 | **Our own postings table** for short text (titles, search-hit titles, claim glosses, entity labels) with our tokenizer (all token lengths, CJK bigrams, case and diacritic folding), `dolt_ignore`d; Dolt FULLTEXT not used | orange | ~1 week | "AI"/"EU" queries (46.8% carry a short token), CJK, phrase-on-title; ~4 s/day of writes for titles; word 0.3 ms |
| 4 | **Body-text FTS5 sidecar** (derived, rebuildable, lag reported) for quoted phrases and body words; `SearchWireCorpus` merges it with rung 2-3 results | orange | ~1 week | the 18.3% quoted-phrase queries and the 31% reformulation chains; FTS5 writes a day's blocks in ~1.4 s vs ~280 s through Dolt |
| 5 | **Ledger tables** with the E1-E3 slices: `claims`, `entity_aliases` (+ trigrams), `world_events`, `event_participants`, judgment projections; the same-event candidate query feeds Jev's shortlist instead of "newest 30 + 10 by title overlap" | red (with E1-E3) | with the slices | same-event recall without embeddings via shared entities and time; same-event candidates 5.5 ms, settlement due 10 ms, navigation 3.3 ms (CTE) |
| 6 | **Ranking without vectors**: BM25 from FTS5; a feature score for structured candidates (IDF-weighted shared entities, time distance, place, outlet independence, title-token Jaccard); query rewriting from the eval misses (acronym and alias expansion through `entity_aliases`, transliteration) | orange | days per feature | precision@10 on R1/R2; each feature kept only if it moves the set |
| 7 | **History discipline**: append-only rows with times for "as of T"; `DOLT_TAG` at publication and settlement boundaries; projections and postings in `dolt_ignore` | orange | small | exact, cheap precommitment witnesses; history grows only with authority rows |

**Where embeddings would add what SQL and full text cannot.** Rungs 0-7
cover exact, lexical, entity, temporal and structural matches. What they
cannot reach is a relevant item that shares **no token, no entity id, no
alias and no gloss** with the query: paraphrase ("power outage" vs
"blackout" vs "lost electricity") and cross-lingual text whose entities are
unlinked. Claims carry English glosses and entities carry multilingual
aliases, which narrows that residual further. The measurable criterion, on
`retrieval-eval-v1`, held-out split:

1. **Plateau.** The last two rungs each raised recall@20 by **less than 2
   points** on R1 and on R2 (and same-event merge recall on the paper's
   windows is below the E3 bar of 0.85 or flat), and
2. **Residual is semantic.** At least **30% of the remaining misses** (or at
   least 5 points of recall) are classified **semantic-only**, while misses
   that share a token or an entity (which are indexing bugs, not model
   gaps) are under 10% of misses, and
3. **Offline check.** An offline run of a small multilingual embedding model
   over the frozen set's candidates (no production index) recovers at least
   half of those semantic-only misses at the same candidate budget.

When all three hold, embeddings enter as a **derived shortlist generator**
for glosses, labels and titles (ontology §9.4), never as authority, and
the same set measures them. Until then, the next point of recall is
cheaper in SQL.

## 6. A labelled retrieval evaluation set

Improvements are measured against a frozen set, not guessed. It follows the
L2a and E0 practice: labels live outside the repo, desks never see them, a
registered judge labels blind (two shuffled calls), a human reads a sample,
and a tuning split is kept apart from a held-out split.

**Sources of queries (all exist today, read-only):**

1. **Research web searches** (R1, corpus-first). The 7,810 gateway requests
   of 10-12 to 10-14 and the 136 paper research runs with their 17,835
   events (`~/.config/choir-qa/traces/search/`). Sample 400 queries
   stratified by shape: quoted phrase, `site:`, HN hunt, proper name, short
   token (AI/EU), non-Latin, plain. **Relevance:** the page the run went on
   to read (`read` egress after the search), the capture it cited, and for
   reformulation chains (Jaccard ≥ 0.5 within 10 min, 31% of searches) the
   page the chain ended on. Quoted-phrase queries also get automatic
   judgments: every held capture block containing the phrase is relevant.
   This set measures "would corpus-first have answered without the web",
   so a judged relevant item must exist in the corpus at query time: the
   snapshot of the corpus is frozen with the set (a Dolt tag on a local copy
   built from public captures, or the fixture repo).
2. **Same-event merges** (R2). The L2a fixture (27 captures, 8 events), the
   after-window's 46 judge pairs and its 31 false-merge candidates read by
   hand (`l2a-merge-recall-on-the-paper`), and E0's held-out sample of the
   425 window captures with re-captures removed. Each capture is a query;
   candidates are events or stories; labels are same-event, same-topic
   (`part_of` sibling), different. 150-250 queries.
3. **Entity resolution** (R3). 300 mentions sampled from captures across
   languages (at least 30% non-English, 10% Cyrillic, 10% CJK), each labelled
   with a Wikidata QID or "no entity"; includes transliterations, acronyms,
   and typos.
4. **Claims and settlement** (R4), once E1 writes claims: 100 questions of
   the forms "what did S say about X since T", "who affirms or denies P",
   "which forecasts are due this week", judged against the extraction rows.

**Metrics, per class and overall:** recall@10 and @20 (corpus-first and
merging are recall problems: the downstream model or management decides),
MRR, nDCG@10 for ranking rungs, false-candidate rate for R2, p50/p95 latency
at the production row counts, write cost per capture, and index bytes. Every
system's top-20 is **pooled** for judgment (TREC-style), so a new rung's
finds are judged rather than counted as misses. Each miss is classified:
not in corpus; lexical (shares a ≥ 3-character token or a short token after
normalization); entity (shares an entity id); temporal (right entity, wrong
window); **semantic-only** (paraphrase or cross-lingual with no shared token,
entity or gloss). That classification is what tells us when embeddings are
the next lever (§5).

**Freezing and reuse:** version the set (`retrieval-eval-v1`), store it with
the scorer outside the repo like the L2a labels, keep 30% held out, and add
new queries only as a new version, so numbers across rungs stay comparable.

## 7. Heresy and conjecture delta

- Discovered: corpus search and Texture citation resolution load the
  newest 1,000 captures and filter in Go (B1); Store B has no `computer_id`
  index, so that window ranges over every computer of the owner;
  `GetObjectByMetadata` and `ListObjectsByMetadataPage` never use the guest's
  generated columns and pass JSON paths as bind parameters, so no index can
  serve them (G5, B6, B7, B9); `ListStoryDocuments` and `SearchDocuments`
  read every owner's Texture documents with bodies and then do N+1 PK reads
  (G1, G3); no retrieval call is timed anywhere.
- Introduced: none (docs and a local-only harness).
- Repaired: none.
- Conjecture (R-1): structured retrieval over typed Dolt tables plus a
  derived lexical index answers the corpus-first, same-event,
  entity-resolution, claim and settlement needs at single-digit
  milliseconds per query at 1M-capture scale, and embeddings add recall
  only on a residual of paraphrase and cross-lingual misses with no shared
  entity or token. Falsified if, on the labelled set of §6, the best
  non-vector rung leaves more than a third of its misses in the
  "shares an entity or a token" class (an indexing failure we have not
  fixed) **and** cannot be fixed by the next rung, or if the semantic-only
  residual is already above the §5 threshold at rung 3.

## Sources

- Dolt: [data description (FULLTEXT "Only basic functionality supported")](https://www.dolthub.com/docs/sql-reference/sql-support/data-description/);
  [implementing full-text indexes](https://www.dolthub.com/blog/2023-08-14-implementing-fulltext-indexes/);
  [functional indexes](https://www.dolthub.com/blog/2026-04-29-announcing-functional-indexes-in-dolt/).
- MySQL reference for comparison: [ngram parser](https://dev.mysql.com/doc/refman/8.4/en/fulltext-search-ngram.html),
  [full-text restrictions](https://dev.mysql.com/doc/en/fulltext-restrictions.html).
