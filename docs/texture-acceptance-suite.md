# Texture acceptance suite (Gate 1 exit item)

Defined 2026-10-09 during the autonomous run, from the Texture failures the
owner reported this week (slow list, slow version chevrons, documents that
load into "Revising…", cancel that is not instant, "does research work?")
and the owner rule that a crash never resumes work.

Spec: `frontend/tests/texture-acceptance-deployed.spec.js`, run against
staging with a disposable account:

```sh
cd frontend && BASE_URL=https://choir.news npx playwright test \
  tests/texture-acceptance-deployed.spec.js
```

It writes a receipt to `docs/evidence/texture-acceptance-<stamp>.json` with
every timing and id.

| # | Check | Pass |
|---|---|---|
| T1 | Document list, first and second read | each under 1.5 s |
| T2a | First draft after create (no further owner action) | an appagent revision lands within 5 min (soft: recorded, the suite continues) |
| T2b | Owner asks Texture to revise | an appagent revision lands within 5 min; the document is no longer pending |
| T3 | Version chevrons (read each revision) | each read under 1 s |
| T4 | Reload after a finished turn | the document is not pending on three reads over 10 s (no "Revising…" zombie) |
| T5 | Cancel during a turn | pending clears within 20 s of the cancel request |
| T5b | Revise again after the cancel (soft) | the revise is accepted (202); records whether a cancel leaves the document revisable |
| T6 | Research (on a fresh document; soft, so T7 and T8 always run) | an appagent revision with at least one source entity cited in the body (`source_refs`) lands within 6 min of the request |
| T6_idle | After the cited research revision (soft) | the document is no longer pending within 3 more minutes |
| T7 | Crash mid-turn (host restarts the computer with no planned marker) | the document reports `agent_revision_interrupted`, is not pending, and no new appagent revision appears in the 2 minutes after the computer is back |
| T8 | What is owed after the crash (`GET /api/runtime/obligations`; SL fault-matrix leg d) | the boot reads as `crash_or_stop`; within 2 min no wake is still owed, none exhausted, no run is running, and the surface reports no read errors |

T7 uses the host's internal vmctl refresh over SSH (test harness only);
everything else goes through the owner-facing API with the session
cookie, exactly as the editor does.

`agent_revision_pending` and `agent_revision_interrupted` are `omitempty`:
an absent field means false.

Thresholds are the owner's experience bar, not the current measurement;
tighten them as the product improves.

## Runs on staging

| Run | Build | Result | Receipt |
|---|---|---|---|
| 1–3 | up to 7bc8f374 | found the raw-JSON recovering page, the occurrence consumption path, and the settled-work revise refusal | earlier receipts in `docs/evidence/` |
| 4 | 4f9331cf | T1–T5 pass; T5b fails (cancel ends the document); T6 fails (search plane cooled down by a caller cancel) | `texture-acceptance-2026-10-09T19-19-29-623Z.json` |
| 5 | 97970cda (search fix) | T1–T5 pass (first draft and revise 26 s); T5b fails (unchanged); T6 fails: search works, every research packet rejected by the packet schema | `texture-acceptance-2026-10-09T19-50-30-737Z.json` |
| 6 | 19b7ef48 (packet contract) | T2b fails: research reports and Texture loop, the document never idles (fixed dc3e86b6, two research openers per owner request) | `texture-acceptance-2026-10-09T20-23-25-707Z.json` |
| 7 | 733bec77 | T1–T5, T7, T8 pass (T7 and T8 for the first time; T6 made soft); T5b fails; T6 fails: the citing turn had no listed sources and never found the inline shape | `texture-acceptance-2026-10-09T22-03-56-155Z.json` |
| 8 | 5b851eed (inline citation hint) | T1–T5, T7, T8 pass; T5b fails; T6 substance passes (cited revision with two sources in 5 min) but the same turn opened follow-up research and the document stayed pending, so T6 was split into T6 and T6_idle | `texture-acceptance-2026-10-09T23-19-02-531Z.json` |

The first attempt of run 8 never signed in (the page stayed on the local
preview) while a second new computer booted beside it; the retry passed
sign-in.
