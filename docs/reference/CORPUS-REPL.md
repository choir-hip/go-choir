# The corpus as values (research cell)

Owner, 2026-11-24: "we should definitely be able to have the whole corpus and new captures at a desks fingertips (repl)". Code: `internal/yaegikernel/corpus_repl.go` (verbs and types), `internal/researchtools/corpus_repl.go` (host tools), `internal/platform/corpus_repl.go` (corpusd routes `/internal/platform/captures/{page,near,edges,texts}`). Grant `corpus.read` in `internal/deskmanifest/grants.go`, held by the research desk only.

## Verbs and bounds

| Verb | Returns | Bound |
| --- | --- | --- |
| `CorpusCaptures(since, until, source, kind, cursor, limit)` | `CorpusPage{Captures []CorpusRow, Next}`; newest fetched first, then those with no fetch time; kind `source` (a poll) or `read` (a research Read) | 1,000 rows by default, 5,000 at most; cut at 2 MiB as written with `Next` set |
| `CorpusTexts(ids, maxChars)` | `CorpusTexts{Texts []CorpusText, Unreached}`; blocks with ids, in the order asked | 200 ids; 20,000 chars each by default, 200,000 at most; cut at 2 MiB as written, the rest in `Unreached` |
| `CorpusNear(of, k, since, until)` | `CorpusNear{Captures []NearCapture, Stories []NearStory, Cut}`, cosine `Score` | k 20 by default, 500 at most of each; `of` is a capture id, a `pub-` id or a text; cut at 2 MiB |
| `CorpusEdges(ref)` | `CorpusEdges`: see below; `Cut` names a shortened list | 500 per list; cut at 2 MiB |

A byte bound is on the answer as corpusd writes it (JSON escapes: `<`, `>` and `&` are six bytes), under the desk frame of 4 MiB. It is a bound on an answer, not on how many calls a desk makes. The host refuses an answer that still reaches its read bound; it never reads one short. A page is a walk of what existed when it started: a capture indexed late with an old fetch time lands behind the cursor and is missed.

No generative model is called. A text query (`CorpusNear` with a text) is one call to the host's embedder, bounded by the text's size (8,192 bytes from a cell) and not by a rate; with a story as `of` the first passage's vector is used, and stories are one row each (best passage), so fewer than k can come back.

## Edges that exist

`publication --has_version--> version --cites_capture--> capture`; `publication <--related_story--> publication` (symmetric, relation `same_incident`, `later_development` or `same_subject`; the edge has no direction, so a successor cannot be told from a predecessor; a relation outside these three is ignored); captures of the same canonical URL. The entity and event layers are retired and not exposed.

## Custody

Scope is the run's own owner and computer (from the run context). The routes are admitted as the neighbours route is: an internal caller, or the guest vmctl binds to the owner and computer the body names (the gate is tested on all four routes; store tests pin each scope predicate). Owner-private, tombstoned and superseded captures are never listed or served: every capture statement, text included, carries the one eligibility predicate of the vector search. A story, version or title appears only if the version is published and public itself; the title is the latest version's, shown only when that version is public. A ref that is absent, another computer's, private, withdrawn, unpublished or restricted gets one answer.

Not exposed: Texture documents and their `succeeds` refs (the Texture custodian's), the search-lead conversation links, the entity and event layers. The older `GET /internal/platform/captures/<id>` serves any capture id to any bound guest ([problem](../problems/capture-read-serves-any-capture-id-to-any-guest-2026-11-24.md)); these verbs do not use it.

**The web child does not hold these verbs.** A page it reads can steer it, `Read` reaches any host with a URL of 8 KB, and with the corpus enumerable (5,000 rows a page, 200 texts a call) that is a way to carry it out. The parent holds the verbs and passes the child the ids or texts it chooses (owner, 2026-11-24, on the review's finding).

## A capture desk

Add `G("corpus.read")` to its manifest row. Nothing else changes. The grant is privileged (first-party desks only); decide there whether that desk may reach the open web: if it can, the same exfiltration argument applies to it.
