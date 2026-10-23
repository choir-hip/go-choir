# The overnight groundwork pass

Written on Friday, October twenty-third, at about half past eleven UTC. It covers the overnight stretch from the restart at about five UTC to now, about six and a half hours of the launch first pass, as run by FP0.

Seven deploys landed overnight, each proved on staging. The paper's long stall at every refresh is gone. The steady 503s on its health check now have a measured cause and a fix in review.

Where things stand. Progress on the first-pass checklist is sixty-five percent. Last night's forecast for nine in the morning Eastern was sixty-four and a half to sixty-six, so the pass is inside it. The deploys were D9 through D15. Each one went through review sized to its risk, a CI mirror, and a proof on the paper or on the named test computer. Three decisions are yours this morning. First, whether to raise the Synthetic concurrency gate, which I held at three. Second, when to build a new index on the paper, which costs a slow window. Third, the roadmap re-cut, which waits for your review. There are also three proof steps only you can run, because they need a root shell inside a guest.

## Before dawn: the inert batch and the event table

D9 landed at about six twenty-five. It was a batch of switched-off and observe-only work: the outbox scan, the trace log, the tape repack tool, the disk-pressure trigger in observe mode, and the pin hardening that refuses a fallback the owner did not allow. The outbox scan proved itself at once. Its slow key read went from three and a half seconds to zero. D10 then landed the new event table on its own, because it bumps the store schema. Its proof passed on the test computer. It also surfaced a separate problem, which became its own fix later in the night: the test computer's self-development history could not be replayed exactly.

D11 and D12 followed. D11 makes the store's close pin its connection pools, and the paper's planned close came back clean. D12 carried three things. Run events are now numbered. The prompt bar accepts a pasted script with bracketed indexes, and the editor still refuses that text, as intended. And guest DNS now goes through the host's resolver for one named test computer. I proved D12 on that test computer, including a model call through the guest and a Texture desk revision that landed thirty-two seconds after the prompt.

## Morning: finding what stalls the paper

The paper stalled for about a minute and three quarters at every refresh. A Sonnet worker traced it to one read. At boot, the store found its next stream number by scanning every event, under the store's main lock. The fix reads a bounded window of the newest objects instead. It took three rounds. In round two, DeepSeek ran a counterexample that handed out a number already in use, and Devin had missed it. I took the stricter reading and restored the safe fallback. That cost one round, and it was the right call: a reused number would have been silent. The record now says that the next event-table deploy needs a real high-water mark first.

D13 landed that fix at about nine thirty-six. The first passivation after the refresh took three quarters of a second, down from a minute and three quarters. I first wrote that the refresh window had no 503s at all. Then I looked at the minutes after it and found more. I corrected the ledger: the boot stall was gone, but a second cause remained.

That second cause is the most useful finding of the night. The health check shares the store's single write connection. One function, the read of a desk's inbox cursor, held that connection for seconds at a time. Seven of the eight steady 503s matched it. D15 halved how often it runs and added a slow-read log. On its first line, the log named the exact culprit. The management desk's cursor read walks about fifteen thousand rows. It takes about sixteen seconds cold and about three seconds warm. The steady 503 rate after D15 is under three percent. It was eleven percent in the window after D14.

D14, between those two, carried the self-development replay fix. A reviewer found a fail-open beside it: reopening the store turned off a vocabulary write fence, and that can happen on a crash boot. I had that fixed in the same landing rather than deferred.

## What I got wrong, and what I learned

Three things. First, I ran a disk-usage count over the artifact store on Node B, about ten million files, at normal priority for about five minutes before I stopped it. Second, I assumed the tape census tool ran at idle disk priority. Node B's drives use a scheduler that ignores that setting, so the census ran at normal priority. Its read phase overlapped the paper's worst minutes after D14. Third, lowering the Synthetic gate from eight to three at five in the morning traded fast failures for slow queues. A read-only investigation showed Synthetic throughput did not rise. About three of the paper's eight dispatcher slots now sit idle, waiting at the gate. Half of all activations wait over five minutes.

## Your decisions this morning

The Synthetic gate. Synthetic's weekly allowance is near sixty-three percent and falls about eight points an hour, so it lasts until about six in the evening UTC at today's rate. A higher gate would run it out sooner. With no fallback, the Synthetic half of the paper would then fail. The investigation names the options and their costs: more slots, a higher gate, more luna share, fewer cells per story, or scheduling that knows each agent's model pool. I chose none of them.

The index. The fix for the management cursor read is a new index on the run-memory table, about seven times faster locally. Deploying it changes nothing on the paper. The index is built only when an operator asks, and on the paper that build may hold the store for tens of seconds or minutes. DeepSeek found that my first index order would have broken replay eligibility, so round two switches to an order that leaves the schema observation unchanged. It needs your timing for the slow window.

The manual steps. Three proof steps need a root shell in a guest: the resolver config for guest DNS, the copied-token refusal, and the oversized-body test for the gateway caps.

The thing to remember is that the stalls were reads, not load. Each one became visible once a log named it. If I were holding the phone, I would pick the slow window for the index first, then decide the Synthetic gate with the allowance in view.

## Names and receipts

- Metamission: docs/definitions/choir-launch-first-pass-metamission-2026-10-21.md, with its ledger alongside.
- Status page: https://claude.ai/artifact/WArejUzHzD42RnT7SMpP2F (version 36 and later).
- Deploys on main:
  - D9: 663023143
  - D10: a49ae398a
  - D11: 6d71f5def
  - D12: 7ca537e01
  - D13: 7319914be
  - D14: 35e5c82f2
  - D15: ff5cb4cbb
- Problem records:
  - docs/problems/stream-seq-seed-scans-the-event-kind-at-boot-2026-10-23.md
  - docs/problems/selfdev-operations-replay-not-equivalent-2026-10-23.md
  - docs/problems/boot-reads-with-the-seed-shape-2026-10-23.md
  - docs/problems/run-memory-entries-hold-the-engine-2026-10-23.md
  - docs/problems/paper-dispatcher-slots-saturated-2026-10-23.md
- In review: lane/fp0-run-memory-kind-index, round 2 after bunch 160.
- For your review: lane/fp0-roadmap-recut.
- Evidence directories, under /Users/wiz/Dev/lanes/_runs: deployD9 through deployD15, fp4-crash-half, and the bunches up to 160.
- Census: /var/lib/go-choir/taperepack-work on Node B. The listing measured 7.33 million files in 111 minutes, and the run is about 3.4 hours for one computer.
- Synthetic watch log: /Users/wiz/Dev/lanes/_runs/synthetic-overnight/watch.log.
