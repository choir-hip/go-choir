# Persistent desks overnight 2026-10-11

Written early Sunday, October eleventh, about four hours into the overnight mission that started at nine twenty-five Saturday evening your time. It covers the persistent desks work and the search changes you asked for during the night.

The research desk is now one persistent desk per document that remembers its work between runs, and the AI news demo now publishes its first grounded revision in under four minutes.

## Where things stand

Two of the four stations are on staging and proven on a test computer. The first makes research one desk per document, so Texture's follow-ups go back to the desk that did the work. The second keeps a desk's Go session alive between its runs, and the desk is told whether its session is new or continuing. Along the way the demo traces exposed three problems that had nothing to do with desks, and all three are fixed. Texture could not see the block ids its edits need, so it guessed them by staging real writes. One rate-limit reply benched a search provider for a whole day. And desk workers were never cleaned up after a run. Keenable is live as a new search provider. The ChatGPT search pull request needs two fixes before it can go in, and I was not allowed to integrate it, so it waits for you. The third station, the Texture root desk, has a design and a precedent but no code.

## The first demo: right desk, thin result

The first proof run asked for last week's AI news and then sent one follow-up question. The routing worked: one research agent served the request, the follow-up and four rounds. Earlier demos had opened two research agents per request.

The result was thin: about a thousand words and nine source links, against the twenty-six items in the multi-Haiku baseline. The traces showed two causes, and neither was the research model. Five of the seven search providers were benched for a day, left over from the rate-limit storm the night before. And Texture spent eighty-seven cells on its turns, thirty-four of them failed, guessing block ids it could not see. One of those guesses would have replaced a paragraph of the document with the word "probe" if it had been right.

## The fixes

I documented each problem before fixing it. Texture's cell now carries the document outline with every block id, and the edit help gives the whole edit shape and says every staged turn is a real write. A Texture cell now sees only the reports its turn can actually accept. A rate limit now benches a search provider for a minute and doubles, and only a real quota message keeps the day-long bench. Desk sessions are kept per agent, reclaimed after thirty minutes idle, and capped, which also ends the worker leak.

You asked for Keenable. I added it to the gateway, tested one live call with your local key, and added a narrow mode to the credentials script that writes only the named key on Node B. The full script would also have copied your local ChatGPT login over the gateway's own refreshed one, which could have broken ChatGPT inference. Node B gained exactly one line, and the gateway restarted cleanly.

## The second demo

Same prompt, same follow-up, everything above deployed. The first grounded revision came at three minutes fifty-one seconds, down from six minutes, and then a revision every one to three minutes. Texture's turns fell to sixty-six cells with seven errors and one id guess. Keenable served beside Parallel and Tavily, and the gateway returned no rate-limit errors. One research desk ran eight rounds. After the first round, every run opened with "continuing session" and the right counts, so session memory works on the real product path.

The document reached about seventeen hundred words with twenty dated items in the window. That is still short of the baseline's twenty-six. Part of the gap is a difference in standards: the Choir brief keeps a section of contested reports and will not assert items it could not trace to a primary or wire source, such as GPT-6 in ChatGPT, which the baseline states as fact. The desk also does not yet reuse its earlier variables. It reads its updates again and searches anew.

## What needs you

The ChatGPT search pull request is good work with one blocking problem. It builds a second login refresher on the same ChatGPT auth file that inference uses. Refresh tokens rotate, so the two can break each other. The fix is one shared login object per file and an atomic file write. Separately, a query with no sources should count as an empty result, not a provider failure. I wrote this up on the pull request. I tried to apply it to main and the session's safety check refused, so applying it is your call or a session with that permission.

The older owner decisions still stand: the sixty-calls-a-minute gateway budget, Serper credits, and the deploy shape. Brave, Exa, SerpAPI and Serper come back from their day-long benches tonight at about eight thirty-eight Eastern; SearXNG stays out until October fifteenth.

## What I would do next

The one thing worth remembering: the research gap was search capacity and an editor that could not see its own document, not the research model.

Next I would make research one desk per computer rather than per document. Persistent management already shows how: an agent outside any single document's trajectory, with its work items on each requesting trajectory. Then the Texture root and its master document, built on the same pattern.

## Names and receipts

Goal files: docs/definitions/choir-persistent-desks-metamission-2026-10-11.md, choir-pd1-persistent-research-desk-2026-10-11.md (receipts S1c, S1d), choir-pd2-desk-sessions-outlive-runs-2026-10-11.md (receipt S1d-PD2), choir-pd3-texture-root-desk-2026-10-11.md (design draft, management precedent), choir-pd4-computer-controls-computer-2026-10-11.md.

Problem docs: docs/problems/desk-workers-leak-until-restart-2026-10-11.md, texture-probes-canonical-write-to-learn-schema-2026-10-11.md (with amendment), search-rate-limit-benches-provider-for-a-day-2026-10-11.md.

Code commits on main: 2ac20789, 8b350515, 16d7c21a (PD1); fcb424b9 (PD2 sessions); beb075cc (Texture frame boundary); 118f3fb6 (ApplyTexture help); 909c4442 (outline in cell); b7668dc3 (rate-limit bench); b8a06609 (Keenable); 7abfca38 (credentials upsert); 7edae7ed (harness retry).

Deploys: 16d7c21a for S1c; c713d63a for S1d, CI green, staging health reported c713d63a at 03:07Z. Keenable key upserted on Node B at about 03:08Z; key-name diff showed one added line.

Demo runs on test computer demo-arxiv-2026-10-10c: S1c document 630ce665, research agent research:630ce665; S1d document 596395c7, research agent research:596395c7. Baseline: docs/evidence/ai-news-haiku-baseline-2026-10-10/report.md.

ChatGPT search review: https://github.com/choir-hip/go-choir/pull/52#issuecomment-6104781006

Mutation classes: red for PD1, PD2, the Texture changes, the search policy and Keenable; yellow for prompts, help text and scripts. Rollback is a revert of each commit; per-run desk sessions return with the PD2 revert.
