# Research desk: search, read and the interim result in one cell (design, read-only)

Rulings applied: 1 (no batching or delay between desks), 3 (no trimming; assume caching on),
4 (one cell), 5 (progressive disclosure), 9 (budget). Code read at the checkout
`.claude/worktrees/followup-token` (main 5dc98bcf5); paths below are relative to it.
Labels: **M** measured (command or script named), **I** inferred, **H** hypothesis.

Data: the 83 paper research runs (17:30 to 19:54 UTC, 2026-11-09) whose event streams are in
`scratchpad/eff/research-data/ev/`, re-analysed by `scratchpad/followup/data/an.py`, `an2.py`
(outputs: `runs_an2.json`, `seqs.txt`). 77 runs have a tool loop; 67 completed with at least
one packet sent. All numbers are the paper unless stated. `$SCR/paper.sh '/api/runs?limit=500'`
gave the run list; events are `GET /api/runs/<id>/events` (already fetched by the audit).

## 0. Answer in short

- The model does not write "search, wait, read, send" cells for two reasons the prompt makes
  worse. (1) A cell is authored in one model call, before any page has been read, so the
  model cannot write a packet that quotes text it has not seen. The only cell that can send
  an interim in the same cell as the read is one whose packet is assembled by code from
  what the reads returned. (2) The research overlay tells it to work "one cell, read its
  result, then the next cell" (`internal/runtimeprompts/overlays/rlm_research_runtime.yaml`,
  Workbench section), and nothing shows a composed cell.
- Everything a composed cell needs already exists except two things: a way to pick pages to
  read in code, and a way to build an evidence packet from captures in code. Search is
  already blocking via `SearchWait`, a research cell has no deadline, a cell may run 8 host
  calls at once (1024 per cell), and goroutines work.
- Measured today: 19.7 calls and 438k input tokens per completed run, 78% of calls have under
  250 output tokens and carry 74% of input, and a finding waits a mean 47 s (median 35 s, p90
  95 s) between its read finishing and its packet reaching Texture.
- Recommendation, in order: (a) prompt rewrite plus one worked cell, with the four small
  defect fixes of section 5; (b) a parallel read helper and typed result accessors; (c) one
  host verb `choir.Gather` that searches, picks and reads, and returns capture-backed
  excerpts, plus a code-built evidence packet that the model's own cell stages in the same
  cell. Option (d), a host-sent packet on every read, needs an owner ruling (authorship and
  the cell-end commit, section 3).

## 1. Today's verbs and flow

### 1.1 Instructions the model reads

Three texts reach the research model.

1. The 874-character wake prompt (M: `loop.submitted prompt_length 874` in every run; text from
   `internal/agentcore` `recordsInboxPromptFor`, asserted in
   `internal/agentcore/research_first_turn_prompt_test.go`): "read them with choir.Records() ...
   Send an interim packet (no work_disposition) as soon as you have read one new source you
   can cite, and another each time you have read more; the final Report comes last. Before
   your final Report, record what this story is about: read choir.StoryAnalysis() ...".
2. The research prompt `internal/promptstore/defaults/research.yaml` (version 12; written by
   8d53b58b5). The sentences that decide cell shape:
   - "Search, then read the sources behind the results. A snippet or your own memory is a
     lead, not evidence."
   - "Read widely and choose well. A search returns up to 40 results ... read at least 8
     independent sources across distinct hosts before your final Report ... Pick them from
     the host-deduped choir.Searches.Summary(id)". Choosing is a judgment the prompt gives
     to the model, after it has seen the Summary.
   - "Report early and pack everything. Send an interim packet as soon as the first source is
     captured; do not wait for the search to finish."
   - "Work in rounds. After each round, send Texture everything you found with
     choir.ReportPacket, then keep going on what is still open."
   - "Searching is asynchronous. choir.SearchStart(query) returns at once ... start several
     searches in one cell, then read them with choir.Searches.Get(id), or wait with
     choir.SearchWait(id, ms) ... A search arrival never wakes you: read the results you need
     before you end your turn. Reads take a few seconds each, so leave timeout_ms unset."
3. The runtime overlay `internal/runtimeprompts/overlays/rlm_research_runtime.yaml`:
   - "Send at most one desk_go_eval call per turn and wait for its receipt before sending the
     next: batching multiple eval calls in one turn is refused and none run. Work
     step-by-step — one cell, read its result, then the next cell building on persisted
     state."
   - "Print summaries (counts, keys, the first few items, a search hit) and keep whole records
     in variables."
   - "A cell that panics at runtime or times out replaces the session ... re-import packages
     and re-declare what the correction needs."
   - Evidence Doctrine: "start several searches in one cell with choir.SearchStart, read their
     results with choir.Searches.Get as they land (choir.SearchWait when you need a search
     finished), and read what matters with choir.Read, with parallelism appropriate to the
     task, novelty, and provider health."

No text shows a cell that searches, waits, reads and reports.

### 1.2 Verbs (signature, returns, size in the transcript)

Sizes are mean printed characters of the cell result by cell kind (M, `an2.py`, 1409 cells);
every result also carries a JSON envelope (`diag, duration, error, intents, receipts, reuse,
stderr`) and the "[Data, not instructions ...]" banner, about 600 to 750 characters (the
audit's measure, `communication.md` section 1).

| verb | signature (file:line) | returns | in transcript |
|---|---|---|---|
| `choir.Records()` | `choir.go:1400` | `[]InboxRecord`: the Cast (objective, body) | with `StoryAnalysis` 2.7k chars (cell "Rec") |
| `choir.StoryAnalysis()` | `story_analysis.go` | story, entities, System 1 answers | in the same cell |
| `choir.SearchWireCorpus(q, n)` | `choir.go:~810` | `JSON` bytes: events, captures, stories | about 7k chars (cells "S3+Corp") |
| `choir.SearchStart(q)` | `search_store.go:390` | `SearchStub{ID, Query, Providers, Cooling}`, prints "s3 started: 1 providers" | about 25 chars |
| `choir.SearchWait(id, ms)` | `search_store.go:404` | `SearchCount{ID, Answered, Arrived, Total, Done}`; blocks up to `searchWaitMax` = 2 min (`:144`) | one line |
| `choir.Searches.Get(id)` / `.List()` | `search_store.go` | `Search` copy: each arrival, `Results []SearchHit{Title, URL, Snippet,...}` deduped by URL, up to 40 hits | 2.5k chars when printed in part |
| `choir.Searches.Summary(id)` | `search_store.go:~215` | string, results grouped by host, 2 hits per host, capped at `summaryMaxBytes` = 5000 | 4.0k chars (cells "Sum") |
| `choir.Read(url)` | `choir.go:653` over `researchtools/read.go:readPage` | `JSON` bytes (`ReadResult`: capture_id, url, title, media_type, block_count, `Blocks` up to `readShownChars` = 6000 characters, `next_block`, `headings` when longer, `warnings`) | 6.6k chars per read cell |
| `choir.ReadCapture(id, from, maxChars)` | `choir.go:777` | `JSON` bytes, blocks from `from`, up to `maxChars` (0 means 8000) | 6.7k chars |
| `choir.Captures()` / `choir.Reads()` | `read_ledger.go` (8d53b58b5) | the run's reads: url, ok, capture_id, title, error | 1.4k chars |
| `choir.ReportPacket("texture", packet, "")` | `choir.go:1866` | a receipt ref string; stages an intent | the packet source stays in the transcript (5.9k chars mean) |
| `choir.Relate("", entities)` | `relate.go` | receipt | 1k chars; the runtime refuses a final Report before it |
| `choir.System1(questions)` | `system1.go` | distribution per closed question (at most 12 questions) | about 2 per run |
| `choir.CellOut(handle)` | `cell_output.go:94` | the whole text of a printed output cut at 8000 characters | 4k chars |
| `choir.Research(children)` | `choir.go:591` | child findings | rare |

There is no separate "capture" verb: `Read` is the capture (it writes a shared
`choir.web_capture` object through corpusd and returns the id); `ReadCapture` reopens one by id.
`Read` and `ReadCapture` return `JSON`, a `[]byte` type (`choir.go:570`), not a struct; that
causes the compile errors of section 5.

### 1.3 Which verbs are async, and why a "wait" step exists

- `SearchStart` is async by design (`search_store.go:18-30`): a host call cannot outlive its
  cell, but a provider's frame can land any time, so searches were made streaming across N
  providers, with "arrivals never wake the desk". That was built for 4+ providers answering at
  different speeds.
- Today one provider answers: keenable served 759 of 767 searches, the other seven are cooling
  in every start line (M, audit `research.md` section 1). Measured search latency: p50 0.27 s,
  p90 0.55 s, p99 1.06 s, max 4.3 s (M: `desk.egress verb=search_done`, n=759).
- So the "wait" is not a wait: it costs a model call (median 2.4 s, mean 5.2 s, p90 12 s) plus a
  resend of the whole transcript, to cover a search that already finished.
- `Read` is synchronous and blocking. Measured read latency: p50 0.31 s, p90 1.57 s, p99 11.0 s,
  max 30 s; 155 of 727 reads (21%) returned an error (M: `desk.cell.egress verb=read`).

### 1.4 What the host does at the first capture

Commit 8d53b58b5 ("every read is visible to the desk, the packet is checked against its
captures, and the first capture triggers an interim report", 2026-11-08) added
`internal/yaegikernel/read_ledger.go`:

- `ChoirScope.Read` calls `recordRead` (`choir.go:653-657`); the ledger keeps every read with
  its capture or its error.
- `readNoteLines` (`read_ledger.go:153-180`) builds the cell-result notes. After the first
  successful capture, if no packet was sent this run and the nudge has not fired, and the desk's
  manifest has both `ReportPacket` and `Read`, it appends one line once per run: "you have N
  captured sources and have sent no packet; send an interim packet now (choir.ReportPacket
  without work_disposition) so Texture can revise while you read on".
- `ReportPacket` (`choir.go:1866-1881`) calls `notePacketSent`, which warns (never refuses) about
  captures the packet leaves out. ce980de72 keyed the nudge on manifest verbs, not the desk name.
- It is only a line in the next cell result. It adds no verb, no automatic send, and the model
  still has to author a packet in a later model call.

Measured effect (M, `an2.py`): the nudge appeared in 75 of 77 runs. The first packet came in the
very next cell in 26 runs; in the rest after 2 to 15 cells (mean 3.9 cells, 36.5 s, median 24 s).
So it works as a reminder, not as a send.

### 1.5 Two facts about cells that the design must respect

- A staged act is committed only when the cell returns without error
  (`internal/agentcore/tools_desk.go:866-904`: `reduction.commit` runs only when
  `result.Error == ""`; an error calls `reduction.abort`, which records the cell's fate and
  commits nothing). `ReportPacket` stages, so it reaches Texture at cell end, not at the call.
  A panic or compile error anywhere in a cell drops the packet it staged. M: run `020c24e6`
  call 6, a `reflect: slice index out of range` panic in the cell that held the first interim
  packet; the cell event shows `intents: 0` and `reuse: unsafe_to_reuse`, and the next cell
  reports "new session".
- A cell has no deadline for research (`deskCellHostDeadline` returns 0 unless the desk row sets
  `cell_deadline`; only management does, `tools_desk.go:62-83`). `timeout_ms` is optional.

## 2. What actually happens in the traces

### 2.1 Population (M, `an2.py`, 67 completed runs with a packet)

- 19.7 calls, 438k input tokens, 2.46 packets per run (41 runs sent 2, 21 sent 3, 5 sent 4).
- 78% of all calls (1157 of 1477) have under 250 output tokens, and carry 74% of the input.
- Per call: model time mean 5.2 s (median 2.4 s, p90 12.1 s); cell time mean 2.8 s (median 1.4 s,
  p90 6.8 s). Over the completed runs the model is 54% of wall time, the cells 27%, the rest
  queueing and host overhead.
- First successful read finishes at a mean 55 s (median 44 s) after the run starts, at call 5.
  First interim is at a mean 92 s (median 83.5 s), at call 9. The gap from the first read to the
  first interim is a mean 37.5 s (median 26 s, max 143 s).
- For every packet (n=135) the oldest read it was waiting on finished a mean 46.7 s earlier
  (median 34.9 s, p90 94.9 s); first packets 39 s, later packets 53 s. Packets are a mean 62.6 s
  apart (median 50.6 s). This is the "interim reaches Texture as found" gap the owner ruling
  cares about.
- A packet-writing call costs 5 to 25 s of model output time alone (packets are 0.7k to 3.1k
  output tokens), which a composed cell does not remove (see 3, option d).
- Typical cell sequence by signature (count over the 77 runs): single Read 132, Records or
  Analysis 117, single ReadCapture 97, interim packet 77, final packet 63, wait+Summary 51,
  CellOut 50, Captures-only 42, Relate 35, Summary alone 35, Get alone 31.

### 2.2 Cell by cell, seven runs (M; full per-call tables in `scratchpad/followup/data/seqs.txt`)

Columns: call number, input tokens, output tokens, model seconds, cell seconds, what the cell did.

Run `4ad561f5` (20 calls, 435k input, 135 s; first read 18 s, packets at 63, 102, 117 s):

    1 in  9185 out 186  2.6s/ 5.7s  Records + Captures
    2 in 10171 out 107  2.1s/ 2.7s  Read(1) + wire corpus
    3 in 12740 out  93  3.2s/ 0.9s  ReadCapture(1) + Captures
    4 in 14926 out  67  1.5s/ 0.8s  ReadCapture + CellOut + Captures
    5 in 17166 out  29  1.2s/ 0.8s  CellOut
    6 in 18934 out  39  1.4s/ 2.5s  ReadCapture + Captures
    7 in 20372 out 1469 9.3s/ 0.8s  PACKET + Records ... ERR (compile error; interim lost)
    8 in 20482 out 154  1.8s/ 0.5s  Records
    9 in 20826 out 1311 4.7s/ 3.3s  PACKET sent to Texture (first interim: 63 s)
    10..15 Records, searches (3), Summary, CellOut, Relate, Read     (six micro-steps)
    16 in 28358 out 1527            PACKET sent (102 s)
    17 ReadCapture; 18 PACKET-final sent (117 s); 19 Records; 20 text only

Run `a609c196` (17 calls, 381k; first read 81 s; packets at 124, 184, 228 s):

    1 Records+Captures; 2 searches x4 + corpus; 3 wait+Summary+ReadCapture
    4 wait+Summary ERR; 5 wait+Summary; 6 search x1 + wait + Summary   (calls 3-6: 4 cells to pick a URL)
    7 in 16183 out 300  Read(1)                         11.3 s cell
    8 in 20450 out 1615 8.6s/19.2s PACKET sent (124 s)   <- 43 s after the Read finished
    9 ReadCapture; 10 search+wait+Summary; 11 Read; 12 search+wait+Summary
    13 in 31905 out 2144 6.6s/12.0s PACKET sent (184 s)
    14 search+wait+Summary; 15 Records+Relate; 16 PACKET-final+Captures sent (228 s); 17 text

Run `696ef178` (11 calls, 202k; first read finishes 66 s after a 33 s single-Read cell; packets at
87 and 133 s): calls 1-4 frame, search, wait+Summary, Summary; call 5 one Read (33 s); call 6
packet (16.8 s of model time to write 969 tokens), sent 21 s after the read.

Run `2f11bcd2` (15 calls, 299k): first read 55 s, packet at 74 s (call 5, cell 13.0 s), then 148 s
and 178 s. Run `aebdede8` (21 calls, 494k, 416 s): model stalls of 39 s, 49 s, 87 s and 33 s at
calls 3, 12, 14 and 16 (provider retries; the model is the long tail), first interim 121 s.
Run `3e5346fd` (24 calls, 517k): two failed cells at calls 1 and 4, three CellOut/Captures
micro-cells in a row, first packet at call 11. Run `56606cf6` (32 calls, 793k): six failed cells,
seven consecutive "Get" micro-cells (calls 8-12 and 16, 22), first packet at call 20.

Summary table (M):

| run | calls | input | out | micro calls (<250 out) | first read (s) | packets at (s) | first packet at call | failed cells |
|---|---|---|---|---|---|---|---|---|
| 696ef178 | 11 | 202k | 5.0k | 8 | 66 | 87, 133 | 6 | 0 |
| 2f11bcd2 | 15 | 299k | 6.6k | 12 | 55 | 74, 148, 178 | 5 | 0 |
| a609c196 | 17 | 381k | 9.5k | 12 | 81 | 124, 184, 228 | 8 | 1 |
| 4ad561f5 | 20 | 435k | 7.9k | 15 | 18 | 63, 102, 117 | 9 | 1 |
| aebdede8 | 21 | 494k | 6.7k | 17 | 111 | 121, 362, 405 | 8 | 1 |
| 3e5346fd | 24 | 517k | 8.8k | 18 | 43 | 93, 173, 184 | 11 | 3 |
| 56606cf6 | 32 | 793k | 6.6k | 28 | 35 | 171, 257 | 20 | 6 |

### 2.3 Calls if each cell did search, wait, read, send

Counting rounds as the number of packets that reached Texture, a composed design needs one cell
per packet plus the first composed round, with each analytical packet sharing its cell with the
next round's search (I): calls = packets + 1 gives 3.5 calls a run; allowing one or two more for
a retry or a Relate-only call gives 4.5 to 5.5. Against 19.7 now.

Tokens, with the transcript grown the same (22.5k mean growth is real content, less about
150 tokens of envelope per cell removed) (I, formula in `an2.py` notes, 67 runs):

| design | calls | raw input | cache-weighted input (read 0.1, write 1.25) |
|---|---|---|---|
| today | 19.7 | 438k | 69.9k |
| composed, packets+1 calls | 3.5 | 67k (6.5x fewer) | 30.0k (2.3x fewer) |
| composed, packets+2 | 4.5 | 87k (5.1x) | 32.1k |
| composed, packets+3 | 5.5 | 107k (4.1x) | 34.3k |

With caching on, the saving is much smaller than the raw count suggests, because the content
that is new each call (reads, packets) is written to the cache either way. The gains left are
about half of the cache-weighted tokens, latency (15 fewer sequential model calls at a mean 5 s),
and fewer failed cells. I did not measure cache prices for Haiku 5.5 on Bedrock; 0.1 and 1.25 are
the usual Anthropic ratios.

### 2.4 What stops the model writing such cells today

Ordered by how directly the evidence ties to it.

1. **Prompt text.** The overlay quote in 1.1(3) ("one cell, read its result, then the next cell")
   and "Send at most one desk_go_eval call per turn" teach micro-steps. The prompt's
   "Send an interim packet as soon as the first source is captured" instructs the model to stop
   after the first read and report. No worked composed cell exists anywhere in the three texts.
2. **Information order inside a cell (the real constraint).** The model writes a cell before
   it has read the pages. It can compose search, wait, read of URLs it chose in a previous turn
   from the Summary (110 cells do read non-literal URLs; 80 cells contain both a search and
   a read), but not "read the best result and quote it": it needs to see block text first. So
   the per-round loop in the traces is search cell, pick cell, read cell, packet cell: four model
   calls where code could do three of the four.
3. **Verbs that cannot be composed without Go plumbing.** `Read` returns `[]byte`. To pick a
   field the model writes a struct and `json.Unmarshal` (every composed cell in the traces does).
   The first mistake costs a cell: "cannot convert \"capture_id\" to int" (run 020c24e6), and
   `undefined method: CaptureID` (runs b7dd2c4a, 8da78c22). The kernel hint exists
   (`surface.go:421`) but the cell has already failed.
4. **Errors that destroy the cell and the session.** 47 cells ended in a runtime panic (40 string
   slice bounds, 8 nil field), and in 45 of those the next cell reports a new session (M:
   the string "new session" in the next result). A panic also drops any packet the cell staged.
   A model that has been burned once splits the work. The overlay says a panic replaces the
   session.
5. **Output-size limits.** A printed output over 8000 characters is cut to 6000 head plus 1500
   tail (`cell_output.go:23-30`), kept for `CellOut` (8 kept). Composed cells that print five
   reads cross that line (cells "R3" average 8.1k chars) and cost a `CellOut` call (50 cells,
   15 errors). `readShownChars` already holds one read to 6000 characters, so two full reads
   exceed the bound.
6. **Cell time limits: not the blocker.** No research cell deadline; reads p99 11 s; 8 host
   calls concurrently and 1024 per cell (`desk_session_worker.go:644-657`); goroutines work in
   the yaegi fork (`desk_cell_fanout.go`, 64 goroutines x 50 rounds probe). The sequential
   reads in a cell are the cost today (run 020c24e6 call 5: three reads took 11.6 s serially).
7. **Intent commit semantics** (1.5): the staged send is only as safe as the whole cell.

## 3. Design options

All four keep the transcript append-only and the cache prefix unchanged (the prompt change is
a new prefix, once). None truncates or rewrites a prior result.

### (a) Prompt and one worked cell (smallest; prompt plus overlay text)

Change: rewrite the overlay's "one cell, read its result, then the next cell" and the prompt's
"as soon as the first source is captured" into: "a cell may search, wait, read and report; the
cell ends when everything it staged is sent". Add one worked cell (about 40 lines) that:
starts the round's searches, waits each, takes the first hit per new host from
`choir.Searches.Get`, reads up to six in goroutines, decodes with a small struct, prints a
bounded digest. Packets are still model-authored in the next call because quotes need text.

- Calls per run: 19.7 to about 10 to 12 (I): search and read collapse into one cell per round;
  the model still sees reads before writing the analytical packet. I have no run that does this
  today, so this is a guess; the probe is section 6.
- Tokens: raw about 40 to 50% of today; with cache about 0.6x of today's cache-weighted input (I).
- Latency to first interim: the packet is still written after the read; about 40 to 50 s
  (I; today 92 s), because search+read is one call (about 10 s) plus the packet call (5 to 25 s).
- Quality risk: low. The model picks pages by rule (first hit per host), not by reading
  snippets. It may read fewer thoughtfully chosen pages; the 8-host floor in the prompt still
  holds.
- Size: green or yellow (prompt text); a prompt-store version bump (`research.yaml` version 13).
  A hand-written 60-line cell is where the panics come from (section 5), so (a) alone will
  reproduce the 9.3% failed-cell rate in bigger cells; ship the section 5 fixes with it.
- Must not change: every read shown to the desk (the digest prints title, capture id, error and
  the blocks it chose to show); no trimming; ledger and pack warning unchanged.

### (b) Blocking helpers (small kernel change)

What exists: `SearchWait` blocks up to 2 minutes; `Read` blocks; 8 concurrent host calls; no
cell deadline. What is missing, and each is additive on the cell surface:

- `choir.ReadAll(urls []string) []Page`: reads up to 8 URLs concurrently (the cell limit), each
  with its own 20 s guard, recording each in the ledger exactly as `Read` does. `Page` is a
  struct (`CaptureID, URL, Title, BlockCount, Blocks []Block, NextBlock, Err`), so no JSON plumbing.
- Accessors on the existing `JSON` type (`yaegikernel.JSON`): `.CaptureID()`, `.Title()`,
  `.Blocks()`. The existing `Read` result keeps printing as JSON bytes and keeps working.
- `choir.Search(query) SearchResult`: SearchStart + SearchWait(15 s) + Get in one call; keeps the
  async verbs for the many-provider future.
- A safe slice helper (`choir.Cut(s, a, b)`) and tolerant `SearchWait` for an id from an ended
  session (returns the empty count and a stated reason, not a Go panic).
- Cell reads happen in parallel: p50 0.3 s, p90 1.6 s, p99 11 s, so a six-page round is bounded
  by its slowest page (cap 30 s observed max) instead of the sum.

- Calls per run: about 8 to 10 (I); the model still picks URLs.
- Tokens: raw 0.4x to 0.5x; cache-weighted 0.5x to 0.6x of today.
- Latency to first interim: about 35 to 45 s (I).
- Quality risk: low.
- Size: orange (new verbs, kernel + `surface.go` docs + `deskprotocol` version bump, as 18.1.0 did
  for the ledger verbs, ce980de72), about 300 lines plus tests.
- Delay of a send in a composed cell: the packet reaches Texture when the cell returns
  (1.5). If one slow page (max 30 s) is in the cell, an earlier finding in the same cell waits
  for it. The helper must give each page a 20 s guard and return its failure as a result, and
  the example must stage the interim before the slow tail, but the staged act still waits for
  cell end. This is the one place the "never delayed" constraint is bent by at most the cell
  time (median 2 s, p90 about 7 s). Say so in the doc and decide whether to accept it (section 7).

### (c) One host verb: search, pick, read, return excerpts (medium; the main design)

`choir.Gather(queries []string, opts GatherOpts) GatherResult`:

1. Start all queries (existing `SearchStart` path), wait for done or 15 s.
2. Pool the hits; drop URLs the run has already been shown (section 4) and hosts already
   captured this run unless the query asks for them; pick up to `N` (default 6) by: round-robin
   over queries, one page per host, primary-source hosts first (hosts named in the Cast
   objective), skip known paywalled or failing hosts from the run's own ledger.
3. Read the picks concurrently through the same `readPage` path and ledger as `Read`.
4. For each capture return `{capture_id, url, title, block_count, lead, matched, failure}`:
   `lead` is the first blocks to about 1.5k characters; `matched` is the top blocks ranked
   against terms from the Cast objective, its questions and the queries (BM25 over block text;
   all in the worker or host, no model call), at most about 4k characters, with block ids.
   Everything else stays in the stored capture, reachable with `ReadCapture(capture_id, from, max)`.
5. `GatherResult.Digest()` prints one bounded string that lists every page read (read ledger
   lines, including failures), and `GatherResult.EvidencePacket(summary)` returns a
   `coagent_source_packet.v1` value built from the captures: one source per page
   (`target.uri`, `target.title`, `target.capture_id`), a `text_quote` selector with the matched
   block text copied verbatim (so `quotematch` passes by construction), `excerpt`, `evidence_state:
   available`, `confidence: low`; claims are attributed quotes ("<host> reports: <quote>")
   with stance "background", so Texture sees leads, not conclusions. The packet validates under
   `coagentpacket.Validate`: `summary` plus sources is enough (`packet.go:126-134`).

The model's own cell then stages it:

```go
res := choir.Gather([]string{q1, q2, q3}, choir.GatherOpts{N: 6})
if len(res.Pages) > 0 {
    choir.ReportPacket("texture", res.EvidencePacket("Interim: captured, not yet analysed"), "")
}
fmt.Println(res.Digest())
```

The next model call sees the digest and writes the analytical packet (claims, status, what
changed) in the same cell that gathers the next round. That is where the model's judgment
still lives.

- Calls per run: 3.5 to 5.5 (section 2.3).
- Tokens: raw 67k to 107k (4x to 6.5x fewer); cache-weighted 30k to 34k (2.1x to 2.3x fewer).
- Latency to first interim at Texture: call 1 writes the cell (about 3 to 5 s, with the Cast in
  the first message), the cell runs (search 0.3 s, reads in parallel 1 to 3 s typical, 30 s
  worst), the interim leaves at cell end: about 10 to 15 s typical, against 92 s now (I). Each
  later finding goes out in the cell that read it, not 47 s later.
- Quality risk: medium and real. (1) Code-picked pages can be worse than model-picked pages;
  mitigate with the host-dedupe (Summary shows more hosts), a second call `Gather` for the
  follow-up thread, and keep the model free to `Read` any URL. (2) Raw-quote interims reach
  Texture before analysis; Texture revises on leads. The packet must say "captured, not yet
  analysed" in `summary`, claims stay `background`, and the model's analytical packet follows
  next cell. (3) `matched` is lexical: it can miss a relevant block; the lead and the
  headings digest stay, and the whole page is one `ReadCapture` away. I would run System 1 on
  the matched blocks as a gate only after the lexical version is measured.
- Size: orange; about 600 to 900 lines (host handler or worker-side orchestration, BM25 over
  blocks, packet builder, digest, tests), a `deskprotocol` bump, one prompt version.
- Must not change: every read shown to the desk (digest lists all picks and failures, and the
  ledger/`choir.Captures()` stay); no trimming (the capture is complete and re-openable; the
  transcript is never rewritten); evidence citations (`capture_id` plus verbatim block quote).

### (d) The host sends an automatic evidence interim on each read

Make `ChoirScope.Read` (or `ReadAll` and `Gather`) stage an `EvidencePacket` for each successful
capture automatically, so even a cell that does nothing else delivers each finding.

- Calls per run: same as (c) or one fewer (no explicit send).
- Latency to first interim: same as (c).
- Tokens: same as (c).
- Quality risk: highest. It sends one packet per read (strictly the ruling: nothing batched)
  and so one Texture wake per page; with six pages in a cell that is six wakes inside a second
  (Texture's warm-cache append cost is being handled separately, but I did not check whether
  Texture coalesces wakes in the same instant). It also moves authorship of a desk act from the
  model's cell to a verb side effect. The staged packet is still lost if the cell later errors.
- Size: small on top of (c), but it touches a red surface (reducer provenance for packets the
  model did not write) and removes the model's choice of what to say.
- My recommendation: do not do (d) as the default. Let the cell stage the evidence packet
  (c), so authorship, the "stage then commit at cell end" rule and the failure behaviour stay as
  they are; the prompt makes the stage line mandatory in the gather cell. If the owner wants a
  host-guaranteed floor, add it later as one packet per cell, still committed at cell end.

### Which option first

(a)+(b)'s small fixes ship now with the section 5 fixes, because they cut the 9.3% failed cells
that any composed cell would multiply. (c) is the change that meets the owner's ruling (search,
read and the interim in one cell, each finding going out in the cell that found it). Do (c) on
d@e first (section 6).

## 4. What a read and a search return in the first place

Rule used: return less up front is allowed; trimming a result later (or re-rendering an earlier
transcript entry) is forbidden because it breaks the cache. Everything below changes only what
the verb returns the first time and leaves the full data in a stored object the model can open.

### 4.1 Read

Today: `readPage` returns the blocks from the top of the page until 6000 characters
(`researchtools/read.go:25, 188-197`), plus a heading digest only when truncated. Mean read cell
6.6k to 6.8k chars; reads are 25% of research input (audit M). My measure of what the model
quotes (M, `an` scripts: 1427 `text_quote` selectors in sent packets; 1365 found verbatim in an
earlier cell output of the same run):

- 818 (57%) came from a first `Read` output, at a median offset of 2.5k characters into the
  output, p90 5.5k: the model does use the early blocks.
- 462 (32%) came from a `ReadCapture` cell, i.e. the first read did not show the block, and a
  further model call was needed (about 150 such cells in the 77 runs).
- 28 from `CellOut` re-reads and 57 from other cells.

So "show matched blocks" is not only a size saving; a third of quotes cost an extra call because
`Read` showed the page top, not the relevant blocks.

Proposal: `Read` (and the page entries of `Gather`) return `capture_id`, title, block count, a
`lead` (about 1.5k characters), the `matched` blocks (about 4k characters, ranked against the
Cast terms and the queries, each with its block id, in page order), the headings digest always,
and a line "omitted: N blocks; ReadCapture(capture_id, from, max) reads them; FindInCapture(id,
"terms", k) ranks them". `ReadCapture` and `readShownChars` semantics for a model that asks for a
range stay unchanged. The stored capture (corpusd, raw bytes plus block JSON) is already the
"stored artifact reference". Estimated saving: reads from 6.8k to about 5k characters on average
(I) plus the avoided `ReadCapture` calls: about 8 to 12% of research input (I); not a replacement
for the composed cell.

Needs an owner call: that "every read is shown to the desk" (8d53b58b5's intent) means each
read's identity and matched content, not every block of every page. The commit title speaks
about visibility of reads (failed reads were invisible), not about showing 6000 characters of
nav boilerplate.

### 4.2 Search digest that avoids URLs already delivered

Today `Searches.Summary(id)` prints up to 40 hits grouped by host, 2 per host, capped at
5000 bytes, every time (mean 4.0k chars; 51 "wait+Summary" and 35 "Summary" cells, 23 + 23 + 21 + 13
more with searches). Measured repeats (M, a script over the printed search outputs: URLs in
`Summary`/`Get`/corpus cells, seen earlier in the same run's printed output): 6593 URL mentions
in Summary/Get cells, 1850 repeats = 28.1% of mentions and 32.2% of characters; including corpus
cells 29.2%. The audit's 41% is higher than my measure; I did not reproduce its method, so treat
both as "about a third".

Proposal: the worker keeps a `shown` set of URL keys (it already has `searchHitKey` and the
per-search `searchTold` counter) and `Summary(id)` / `Gather` print only URLs not yet shown and
not yet captured, with a last line "N more results already shown earlier". This changes what the
next verb prints; earlier transcript bytes are untouched. Saving about 30% of search output
(search outputs are 16% of input), about 5% of research input (I). Caveat: the set lives in the
worker session and is lost when the session is replaced (a panic), so the first Summary after a
restart repeats; the fix for that is not to panic (section 5) or to rebuild the set from the
ledger on a session start. `Gather` should also skip hosts the run already captured unless asked.

## 5. Failed cells: 131 of 1409 (9.3%)

Classes (M: `an2.py`, error text from each result; cell counts):

| class | count | example | fix |
|---|---|---|---|
| undefined field or method on a verb result | 43 | `14:67: undefined method: CaptureID ... r.CaptureID` on the `[]byte` result of `choir.Read` (b7dd2c4a call 5; 8da78c22 call 5); `s1.Id` on a `SearchStub` (8da78c22 call 3); `.Host`, `.Ok`, `.Json`, `.PYes`, `.TopChoice` on guessed names | typed accessors on `JSON` (`.CaptureID()`, `.Title()`, `.Blocks()`); `choir.Help("Read")` printing the Go signature and a decode example; the error already carries a hint in 35 of 43 cases, but the cell has failed by then: put the field list in the verb doc the model reads once (progressive disclosure, ruling 5) |
| runtime panic: string slice out of bounds | 40 | `out, _ := choir.CellOut("af5baa28:1"); out[len(out)-3000:]` (18 CellOut cases: the handle was stale, so `out` was empty); `string(r1)[:2000]` of a Read result shorter than 2000 (15 cases); `out[11000:14600]` | `choir.Cut(s, a, b)` that clamps; a `CellOut` that returns an error string the model sees as the result, not an empty string plus an ignored error; the kernel could rewrite `x[a:b]` on strings in cell source to the clamped form (the panic wipes the session and drops staged acts, so a clamp is worth a source rewrite) |
| other (compile) | 26 | `func dump(cap string, from int) {` mixed with statements: "expected '(', found dump" (9 cells: b7dd2c4a calls 7 and 25, and a packet cell with `addClaim`); `choir.SearchWait(id, ...)` given a `SearchStub` instead of `.ID` (2 cells); `con, _ := choir.Context()` "returns 1 values" (4 cells, same class as `choir.Records()`) | document "cells are fragments: declare helpers as `name := func(...) {...}`" in one line; make `Records()` and `Context()` return `(value, error)` like the other verbs, or print the signature in the error; a `SearchWait` that accepts a `SearchStub` |
| ReportPacket source did not compile | 8 plus 11 more inside the packet cells | JS-style trailing comma and nested literal errors in a 2k-token map literal: `_.go:9:499: unexpected comma; expecting ]` (d26c0367 call 14, c6aaf0a2 call 11) | the model must write the packet in JSON in a backquoted string (the error hint says so, `surface.go packetLiteralHint`) and `EvidencePacket()` builds the sources and quotes in code; 19 of ~153 packet cells (12%) failed, each costs a rewrite of 1.3k to 2.2k output tokens |
| nil result field (reflect zero Value) | 8 | `sa := choir.StoryAnalysis(); println(sa.Story.Title)` when the preload is empty (3e5346fd call 16; 9bb25379 call 21); `recs` loop on an empty list (3e5346fd call 1) | return a zero struct, not a nil pointer, and a `Valid` flag; or document that `StoryAnalysis()` is empty on a non-story desk |
| JSON bytes indexed like a map | 1 | `c2["capture_id"]` on `JSON` (020c24e6 call 8) | accessors as above |
| model-forced panic | 1 | `panic("restart needed")` | none |

Consequence chain (M for the links, I for the cost): 47 panic cells, 45 followed by "new session";
the Searches store, the read ledger and the model's variables are in the worker session, so the
next cell's `SearchWait("s5")` finds "this session's searches: s1..s4" or nothing (the audit
counted 36 such calls). Error turns cost 2.54M input tokens, 7.7% of research input (audit M).

## 6. Proof plan on d@e

Rules: d@e is `computer-59c902c5d46caecc0a180a8557164e72`; never the paper or the owner's
computers; adversarial drills only on d@e (CLAUDE.md Operating Tempo rule 8). This plan needs
live model calls and a deploy, so it is for the implementer; I have made none.

Conditions: one fixed prompt (a Cast from the paper's own recent research run, e.g. the Paraguay
cabinet story objective of run 020c24e6, pasted as the same Texture Cast on a d@e test document),
the same model, caching as configured. Three arms on d@e, each run three times to cover variance
(model latency has a 20 to 87 s tail):

- arm 0: current build, current prompt (baseline);
- arm 1: (a)+(b)-fixes (prompt v13, `ReadAll`, `Cut`, accessors);
- arm 2: (c) `Gather` + `EvidencePacket` + the matched-block `Read`.

Measure per run from the events (all by `de.sh '/api/runs/<id>/events'`, no new tooling):

1. calls and input tokens: `loop.progress phase=tool_loop` count; sum of `input_tokens`; and
   `cached_input_tokens` where reported;
2. time to first interim at Texture: the `ts` of the `tool.result` whose cell has `intents >= 1`
   and a `ReportPacket`; compare to the first `desk.cell.egress verb=read is_error=false`
   `finished_at`; each later interim likewise (oldest unreported read to packet commit);
3. failed cells: `desk.cell` with non-empty `error`;
4. that Texture got every interim: for each `ReportPacket` receipt in the run's cells
   (`receipts: rlm:report:<seq>`), find the Texture-side record with that receipt/seq in the
   Texture agent's run (`/api/runs?...` role texture, same channel) or the commitment ledger, and
   require the count and order to match; plus that every `capture_id` in a Texture-visible
   source is in `choir.Captures()` of the run; zero captured reads missing from the sent
   packets without a stated reason (the pack warning in the cell result is zero);
5. quality: the final packet's distinct hosts (target.uri) and the number of verbatim-valid
   quotes (quotematch result), against arm 0; the published-or-not outcome is out of scope on
   d@e.

Acceptance: arm 2 median calls <= 6, first interim <= 20 s after the first read finishes in 8
of 9 runs, 0 failed cells caused by verb-result misuse, every interim visible at Texture,
distinct hosts in the final packet not below arm 0 minus one. If arm 2 passes, repeat once more
with an adversarial Cast on d@e (a page that 403s, a search that returns repeats) before a ring
release. Hypothesis to confirm: the composed arm does not shift the long-tail model stalls
(arm 0 shows 39, 49, 87 s single calls); it only has fewer calls to be hit by them.

## 7. Open questions for the owner or the next investigation

1. **Commit at cell end.** The ruling says each finding reaches Texture when found. In any
   one-cell design a staged packet is committed when the cell returns, so a finding can wait
   for the cell's slowest page. Accept the cell time (median 2 s, p90 about 7 s, max 30 s with
   the 20 s per-page guard), or add a mid-cell commit (`choir.Flush()`), which changes the
   reducer path in `tools_desk.go:866-904` (red surface). I recommend the first for now.
2. **A cell that errors loses its staged act.** A panic after a good `Gather` loses the interim
   even though the pages are captured. Making the new verbs panic-free and removing model string
   slicing (section 5) is the mitigation; a "commit what was staged before the error" rule would
   be the structural fix, red surface.
3. **Per-read or per-cell evidence packet** (option d): one packet per read honours "never batch"
   literally but multiplies Texture wakes. I did not check whether Texture already coalesces
   packets that arrive together.
4. **Where the Cast terms come from** for `matched`: the worker has `choir.Records()` text in
   the frame (`s.inbox`); the Cast objective is available in-cell. Confirm it is also available
   to a host handler if `Gather` moves host-side.
5. **The 9186-token first call** (audit open question 1) is not explained; a progressive
   disclosure prompt (ruling 5) will shrink it, and that interacts with the example cell in (a):
   the example is a few hundred tokens of prefix, cached.
6. **The 41% vs 28% repeated-URL figure** differ between the audit and this report; the probe is
   to run both scripts over the same cell outputs.
7. **Quality of code-picked pages** is the main untested claim in this design. The only evidence
   is indirect: 38 of 67 runs already cite 2 or more distinct hosts in the first packet (audit),
   and 96% of quotes come from pages the model had read in the first place. The d@e comparison
   in section 6 is the test.

## Files

- Report: `/private/tmp/claude-501/-Users-wiz-Dev-go-choir/f298736e-cabb-51ff-9cea-34588a4989cb/scratchpad/followup/research-one-cell.md`
- Data and scripts: `.../scratchpad/followup/data/an.py`, `an2.py`, `runs_an2.json`, `seqs.txt`
- Code read: `internal/promptstore/defaults/research.yaml`,
  `internal/runtimeprompts/overlays/rlm_research_runtime.yaml`,
  `internal/yaegikernel/{read_ledger,search_store,choir,surface,cell_output,desk_session_worker,intent}.go`,
  `internal/researchtools/read.go`, `internal/agentcore/tools_desk.go`,
  `internal/coagentpacket/packet.go`.
