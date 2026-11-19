# A corpus search takes about two seconds on the paper (2026-11-19)

## Problem

One `search_wire_corpus` call on the newspaper's computer takes a median of
2.1 seconds, and one in ten takes more than 5.4. The research desk makes
about two such calls a run, so it pays about four seconds a run today. The
corpus brief (lane `corpus-brief`, not landed) makes up to six, one after
another, under an eight second deadline before the run's first model call.
At these times the brief would answer about three threads of six, report
the rest as unanswered, and hold every research run for the full eight
seconds. Seven review rounds of the brief did not find this, because none
of them measured the search.

## Evidence

Read from Node B on 2026-11-19, 01:30Z to 02:30Z.

The guest's own line (`corpus search computer=… captures_ms=…`), 39 calls:
minimum 602 ms, quartile 1,476, median 2,096, third quartile 3,397, ninth
decile 5,423, maximum 18,399. `text_indexed=214195 computer_pages=214195`.

corpusd's journal (`corpusd retrieval op=… ms=…`), the same hour:

| operation | calls | mean ms | max ms | over 1 s |
| --- | --- | --- | --- | --- |
| `capture_search shape=terms` (titles and labels) | 35 | 1,315 | 7,208 | 20 |
| `capture_fulltext_search shape=passages` (page text) | 35 | 751 | 1,311 | 8 |
| `capture_by_url shape=url_key` | 20 | 0.4 | 1.2 | 0 |

The title-and-label half is the slower one, though it searches far less
text than the page-text half.

## Belief

The time is in corpusd, not in the guest or the network: the two operations
sum to the guest's median. The cause inside each is not known. Guesses to
test, none confirmed: a scan or a sort over every posting of a common word;
a count over the whole scope on each call; the ranking step reading more
rows than it keeps.

## What is being done

- A read-only investigation reproduces the term search on the build machine
  at about 200,000 pages and reads the query plans (2026-11-19).
- The corpus brief does not land as built. It will run its threads'
  searches at the same time under the one deadline, so the wait is the
  slowest search and not the sum, and it lands only after its latency is
  measured against these numbers.

## Remaining error

- The cause of the term search's time, and whether six searches at once
  slow each other on corpusd.
- The maximum of 18.4 seconds on the guest side has no matching corpusd
  line over 7.2 seconds in the same hour; the rest of that call's time is
  unexplained.
- These are one hour's numbers from one computer.

## Update, 2026-11-19T05:25Z (station L1; read-only)

Three hours of the paper's console (02:15Z to 05:22Z), 280 `corpus search`
lines: median `captures_ms` 1,706, ninth decile 4,745, maximum 15,603. The
earlier hour's median was 2,096 and its ninth decile 5,423, so the time has
not moved. The corpus brief reached the paper at about 05:24Z. Its first
three `research brief` lines: 3.4 s with four threads answered, 4.1 s with
four answered, and one `outcome=failed reason="the 8s deadline passed"` with
`unanswered=1` (one query, no thread known, nothing found). One failed in
three so far; three lines is not a rate. The failed brief is the case the
earlier "brief claimed absence wrongly" defect was about: the desk is told
the search did not answer, not that nothing is held. The clustering
assessment of this and two other corpus-search defects is in
[recurring agent failures read from traces](recurring-agent-failures-read-from-traces-2026-11-19.md).
