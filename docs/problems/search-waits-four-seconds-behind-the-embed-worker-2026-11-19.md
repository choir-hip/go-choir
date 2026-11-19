# A corpus search waits four seconds behind the embed worker and comes back without its vector half

2026-11-19. Class: a design defect of the vector deploy, live on staging
since 13:10Z. It fails the way the design says (the answer is the page-text
half alone), so nothing is wrong in an answer; every answer is late and
most lack the half the deploy was for.

## Problem

A ranked corpus search embeds its query on the host embedder before it can
read the vector index, under one 4 s deadline. The embed worker fills the
index through the same embedder. Since the worker began embedding
(13:10Z), the median ranked search on Node B takes 4,000 ms and more than
half report `vector_state=unavailable`.

## Evidence

Read from corpusd's and the embedder's journals on Node B, 2026-11-19.

- `op=capture_search shape=ranked`, by ten minutes: 12:10Z to 12:30Z (the
  worker deployed but stuck in its first statement, embedding nothing)
  median 53 to 99 ms, 21 of 92 unavailable. 13:10Z to 14:35Z (the worker
  embedding) median 4,000 ms in eight of nine windows, 197 of 303
  unavailable.
- `op=vector_embed shape=query`: worker idle (12:05Z to 12:57Z) 178 calls,
  median 97 ms, 34 at the deadline. Worker busy (from 13:12Z) 292 calls,
  median 4,000 ms, 156 at the deadline.
- The embedder (`llama-server --threads 4 --parallel 2`): two slots, and the
  tasks in both slots are decoded together. A pair of passages of about 300
  tokens takes 2.3 s to 4.2 s. A query of 19 tokens that entered a slot at
  the same moment as another of 23 was cancelled by its caller 2.2 s and
  3.3 s later and released at 3.7 s, not yet answered.
- The embedder process uses about 875% of a CPU (nine of twelve threads)
  at `Nice=10`, `CPUWeight=10` and embeds 0.43 passages a second, about 130
  tokens a second. That is the loaded rate the reference page planned for
  (127 to 349 on the build VM). Node B's load average is 16.
- The research desk makes several corpus searches a run (8 and 18 in two
  runs read today), so a run pays the four seconds several times.

## Belief

Two causes, one mechanism. The query shares the server's two slots with the
worker's passages: it waits for a slot (up to one decode, 2 s to 4 s), and
then is decoded in the same batch as a passage in the other slot and ends
when that does. A query alone on this CPU takes about 100 ms. The 19% of
queries at the deadline while the worker was idle are not explained by the
worker; the likely cause is several queries at once on two slots at a low
CPU weight (a research run sends its searches together), not confirmed.

The design chose a low CPU weight for the unit so that guests win
(`nix/embedder.nix`, "four threads: a size, not a limit"). That is right
for the worker's bulk and wrong for a query, which is a guest's own
request with a deadline.

## Options

1. **Two units.** The query path gets its own embedder (its own port,
   normal CPU weight, more slots of a short context); the worker and the
   import keep the low-weight one. A query never shares a decode with a
   passage. Cost: a second process holding the model (about 0.4 GiB; Node
   B has 19 GiB available) and a host change.
2. The worker holds off while a query waits. The query still waits out the
   decode in flight (2 s to 4 s), so this halves the wait at best.
3. Wait for the import. With the Mac's file in, the worker's gap is the
   pages and stories since the export (hours of work, then a trickle), but
   the paper captures and publishes all day, so the collision stays for
   some share of searches for good.

Option 1 is being built (`lane/embed-query`). It lands only with its plan
and timing read on Node B first, as the fourth failure's record says.

## Remaining error

- The idle-worker 19% is unexplained until the query unit is measured alone.
- Whether the vector half's answers are better than the page-text half's
  has not been measured on the deployed search at all: the index is nearly
  empty (story vectors 252 of 34,882 at 13:20Z).
- The embedder's 875% CPU with `--threads 4` is more threads than the flag
  names; which pool spins has not been read.
