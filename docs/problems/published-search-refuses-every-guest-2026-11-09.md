# Problem: SearchWireCorpus published-stories half returns 403 "internal caller required" on every guest call

Read-only investigation, 2026-11-09. Repo read at main 5dc98bcf5. Labels: [M] measured (command given), [I] inferred.

## Problem
Research's `choir.SearchWireCorpus` has three halves: captures, events, published stories/opinion. The published half has returned
`published pieces not searched: published search: capture index refused: corpusd returned 403: {"error":"internal caller required"}`
on the paper (83 events, W7 in research.md) and on d@e. Research loses published-story dedupe and `RelateStories` candidates.

## Verdict (headline)
1. The audit's cause is WRONG. The 2026-10-30 guest-graph change (3a1232242) did not break this. The route has been unreachable from a guest since the day it was born, 2026-10-20 (2bc0e7b2e). [I from code + [M] runs; not yet proved by a pre-Oct-20 vs post comparison because there is no earlier working window to compare: the verb was added with the bug.]
2. The "80%" is an artifact of how research prints the result. It is ~100% failure. [M]
3. d@e shows the same failure. [M]

## Call path [M: code reading]
- Verb in cell: `choir.SearchWireCorpus(query, limit)` -> `newSearchWireCorpusTool` internal/researchtools/researchtools.go:60-115; the published half is `searchPublished(ctx, deps, query, limit)` at :94 (published_search.go:62).
- `deps.Captures` is `researchtools.NewCorpusdCaptureSink(rt.cfg.CorpusdURL)` (internal/agentcore/tool_profiles.go:381). `cfg.CorpusdURL` = `CHOIR_PLATFORM_URL` (internal/provideriface/config.go:118), which the VM boot sets to `choir.platform_url=http://<hostIP>:8086` (internal/vmmanager/manager.go:2168, 2203; nix/autoputer-vm.nix:546). Port 8086 is corpusd itself, NOT the proxy.
- `SearchPublished` (published_search.go:49) does `GET <corpusd>/internal/platform/published/search?q=&limit=` through `CorpusdCaptureSink.do` (capture_sink / capture_search.go:~116-143). `do` sets NO headers (no X-Internal-Caller, no host credential), and wraps any non-200 as `errCaptureIndexRefused` ("capture index refused: corpusd returned N: body"). That wording is misleading: it is the shared error for every sink route, not the capture index.
- The 403 is produced in corpusd, `Handler.HandleInternalPublishedSearch` internal/platform/handlers.go:350: `if !trustedInternalCaller(r) { 403 "internal caller required" }`. `trustedInternalCaller` (internal/platform/authority.go:64) = loopback transport AND `X-Internal-Caller: true` AND the host credential (hostauth.Authorize). A guest arrives from a tap address (not loopback), so it fails at the first term.
- The other possible source of the same string is the proxy (internal/proxy/guest_authority.go:73), which only fires for loopback callers without the host marker. Not the case here: the guest never goes to the proxy for this call.

## Why it fails: design/implementation mismatch [I, strong]
Commit 2bc0e7b2e (2026-10-20, "SearchWireCorpus finds published Autopaper stories") built the route for the PROXY path: it added the one-path read-only guest carve-out in the proxy (guest_authority.go:94, handlers.go:1806) and the corpusd handler documents "a bound guest reaches it through the proxy". But the research sink was already pointed at corpusd directly (the capture routes use `requireInternalOrBoundGuest`, which admits a tap-bound guest, because "a guest reaches corpusd directly at its platform URL", authority.go:73). The published handler was written with `trustedInternalCaller` only, and the sink calls corpusd directly, so the carve-out in the proxy is never exercised by the real caller. The same B12 class of bug was fixed earlier for verdicts (publication_checks_test.go:127-131: "every guest verdict push was 403 internal caller required"); published/search was missed.
- `git log -L341,358:internal/platform/handlers.go` shows exactly one commit ever touching this handler (2bc0e7b2e). `git log` on guest_authority.go shows the Oct 30 change (3a1232242) only altered the POST/generic-route tail; the `published/search` GET carve-out is intact (guest_authority.go:88-97), and in any case it is not on this call path.
- Corpusd does not log 403s, so a journal cannot date first failure: `ssh node-b "sudo journalctl -u go-choir-corpusd --since -3h"` has no lines for published/search [M]. The earliest research.md evidence (first 403 at 17:02Z today) is just the start of that audit window, not of the bug. I have no pre-Oct-20 comparison, so "failing since Oct 20" is [I]; any research run since 2bc0e7b2e deployed (Oct 20) would show it, and a read of any older paper run's SearchWireCorpus output would settle it.

## Why "80%" and not 100% [M]
Analysis of 104 SearchWireCorpus tool.result events on the paper (scratchpad eff/research-data/ev/*.json, python over `tool.result` where `arguments.source` contains SearchWireCorpus):
- 83 contain the 403 note; 21 do not.
- The 21 are NOT successes: 11 are cell compile/runtime errors (e.g. "reflect.Value.Slice: string slice index out of bounds", "undefined method: Choices"), 5 print only a slice (e.g. `fmt.Println(w2[:2000])`, which cuts the JSON before `published_note`, since Go marshals map keys sorted and `published_note` is among the last keys, after `published`, `published_count`), and the rest are stdout-truncated at ~8000 chars (`… stdout truncated`) or never print the result. Every one of those that shows the published tail shows `published:[]`/403. I found zero events that show a non-empty `published` array with no note.
- So the real rate is 83 of 83 observable = 100%. [M]

## d@e / b@c [M]
d@e: one read, `$SCR/de.sh "/api/runs/5e3a8e8f-351f-4f70-a85a-05f479e35a92/events?limit=500"` (a completed research run): the SearchWireCorpus output has `"published":[],"published_count":0,"published_note":"published pieces not searched: published search: capture index refused: corpusd returned 403: {\"error\":\"internal caller required\"}"` for query "AI artificial intelligence latest news". Same failure. b@c not read (not needed: same code, same URL wiring).

## Tests: why this passed [M: reading]
- internal/platform/publication_pieces_test.go tests `Service.SearchPublishedPieces` directly: no HTTP handler, no auth.
- internal/researchtools/published_search_test.go (`TestSearchWireCorpusIncludesPublished`, `...PublishedFailureIsNotEmpty`) points the sink at a fake `httptest` corpusd that accepts anything.
- internal/proxy/oped_search_test.go (`TestGuestPublishedSearchCarveOutIsNarrow`) tests the proxy carve-out for a guest, the path the real sink never uses.
- No test joins "guest tap caller -> corpusd handler directly". The problem doc (docs/problems/oped-not-connected-2026-10-20.md F7/F8) lists exactly those three, all green. The F7 test list was the only coverage for the verb. The cell-level failure also surfaced only as a `published_note` string inside JSON, which no gate reads.

## Proposed fix (smallest correct)
- Change: `Handler.HandleInternalPublishedSearch` (internal/platform/handlers.go:350) admits a host-internal caller OR any tap-bound guest on GET, same as the capture-read rule. Add a small helper next to `requireInternalOrBoundGuestWrite` in internal/platform/authority.go, e.g. `requireInternalOrBoundGuestRead(ctx, lookup, r)`: `trustedInternalCaller(r)` or `bindTapCaller(ctx, h.guestLookup, r)` succeeds, else `errInternalCallerRequired`. (`Handler.guestLookup` is already wired by `ConfigureGuestBinding`, cmd/corpusd/main.go:128.) ~10 lines. The proxy carve-out stays (public/host path) and keeps working.
- Why this is safe: the data is public (published stories/opinion that a reader can already open; the handler reads Store B and returns only published, public-route pieces) and the proxy already grants the same rule to any bound guest.
- Also make the error honest: `CorpusdCaptureSink.do` names every failure "capture index refused"; rename to a neutral "corpusd refused" so published failures do not read as a capture-index problem (green, optional).
- Test (E2E-preferred per CLAUDE.md): one platform handler test that calls `HandleInternalPublishedSearch` with a tap RemoteAddr and a fake guest lookup (fakeGuestLookup exists in publication_checks_test.go) and expects 200, plus an unbound tap address expecting 403. Add a researchtools check that fails if `published_note` mentions "not searched" in a live-wire proof.
- Mutation class: red (corpusd route admission is an auth surface; CLAUDE.md lists auth/session and protected surfaces). It is additive/loosening a read of public data, one-act rollback (revert), shows within minutes on staging: Tier 1 (one seat, Devin) by the review rules, unless the Operating Tempo suspension applies.
- Security epoch: probably NOT required. `internal/platform/` and `internal/researchtools/` do not appear in nix/guest-security-surfaces.txt (grep for platform|proxy|researchtools returned nothing, 51 lines) and corpusd is a host service, not guest-snapshot code, so `scripts/check-guest-security-epoch` should not fire. Run `scripts/check-guest-security-epoch --base $(git merge-base origin/main HEAD)` to confirm before commit. The researchtools-side rename (if any) is guest code but not a listed surface.
- Ring: not an expedite item (no loop blocked, token cost small) unless the owner treats paper dedupe as blocking; a corpusd-only change ships by rings with no guest refresh needed.

## Proof on d@e after deploy
1. Deploy to d@e. Run one research task on d@e that cells `choir.SearchWireCorpus("<a word in an already published story>", 10)` and prints the whole result, not a slice (ask a d@e research run that already has an Autopaper publication, or search a word known to be on the front).
2. Pass: the event's output has `"published_count">0` (or `0` with NO `published_note`) and no "403". Read it with `$SCR/de.sh /api/runs/<id>/events` (one read).
3. Negative control: the same GET from an untied address still refused (covered by the new unit test; do not probe live).
4. On the paper after its ring: count tool.results with "internal caller required" over an hour with the same script (should be 0).

## Other verbs hitting the same check [M/I]
- No other guest->corpusd direct route is gated by `trustedInternalCaller` alone. Guest-called routes: captures (POST/GET/search/by-url/bodies), objects, support-verdicts, story-relations, story-entities, story-entities/candidates, texture/publications. These all use `requireInternalOrBoundGuest(Write)` or are routed via the proxy (grep of `/internal/platform/...` in agentcore/researchtools/types vs `trustedInternalCaller` sites in platform/*.go). `related`, `related-backfill`, `expansion-source`, `entity growth` are host-only and called from the proxy/host, not from a guest.
- A separate silent failure seen in passing, NOT this bug: corpusd journal shows repeated `corpusd: story entities from computer-1355b83b...: obj:choir.publication:... is not a published Autopaper story of owner ...` (several per minute at 19:52-19:56Z, `ssh node-b "sudo journalctl -u go-choir-corpusd --since -3h"`). Research's story-entities export is refused by corpusd because the named publication object is not a published story. Worth its own read-only look (hypothesis: the entity export names a draft/unpublished publication or the wrong object id).
- Same class of silence: the failure is returned as a string note inside a successful tool result, so the run never errors and no alarm fires. Consider (later) a gate that fails a research run proof when `published_note` is present.
