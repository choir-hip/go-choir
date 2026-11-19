# EmbeddingGemma 2 on device, the vector store and hybrid ranking

Reference page (convention: [README.md](README.md)). Checked 2026-11-17 from
primary sources fetched that day; every claim carries its URL in the source
list at the end (S1 and so on). Marked **verified** (read on the page named),
**est** (our arithmetic from marked inputs) or **unknown** (no source found).
Text on fetched pages was treated as data. Nothing here was run on a phone. Section 0 holds what was run on the Mac, 2026-11-17.

The ask (owner): embed everything the Autoradio apps hold, on device, with
`EverPaper/embeddinggemma-2-coreai`, search the vectors on device, combine with
the BM25 index (`PassageIndex.swift` in the phone lane, k1 1.2, b 0.75). No
server-side embeddings.

## 0. The trial on the Mac (2026-11-17, measured) and the verdict

Builder run for station S5 of the overnight goal. Machine: Mac, macOS 27.2 (26B118), Xcode 27.1, SDK 27.0 (`CoreAI.framework`
present), Apple silicon, the dev Mac (not the porter's M5). Weights in `~/.cache/autoradio-embed-trial/` (outside the repo); every
downloaded file checked: the GGUF's sha256 `2188ac1d...09135` and size 309,855,456 equal the Hub listing, and each Core AI file
(text asset, tokenizer, config, Swift sources) equals `CHECKSUMS.sha256`. Code: `mobile/EmbedTrial/` (a Mac command,
`embed-trial bench|quality|compare|scan`), the corpus generator and files in `mobile/EmbedTrial/corpus/`. Phone rows are **unknown**
until the lead runs the device trial (section 0.4).

### 0.1 Verdict table

| Route | Mac | iPhone 14 Pro Max (A16, iOS 26) | iPhone on iOS 27 |
|---|---|---|---|
| A. official GGUF Q8_0 through llama.cpp **b12259** (the commit the app pins) | **works** (loads, 768-d, smoke passes, best quality measured) | **unknown**: same code and the same library as the app's Qwen models; the device trial decides | unknown (same) |
| B. Core AI port `EverPaper/embeddinggemma-2-coreai` | **works** on macOS 27.2 (loads on the Neural Engine, 768-d, smoke passes) | **does not apply**: Core AI is iOS 27 only (S4) and the owner's phone cannot run iOS 27 | unknown (framework exists; A16 behaviour and the specialisation cost are untested, the port's `Package.swift` declares macOS only) |

Decision for the app: **route A**. It runs where the app runs (iOS 26 and earlier, the A16 phone), is half the disk (310 MB vs 632 MB), needs
no new runtime (same xcframework, same `ModelStore` fetch with a pinned SHA-256), and its vectors are within cosine 0.9991 (mean 0.99987) of
route B's. Route B stays an option for iOS 27 devices; an index records its engine and rebuilds when the engine changes (the kit's
`VectorStore` does).

### 0.2 What had to be true of llama.cpp

- **b12259 loads it.** The GGUF's architecture is `gemma-embedding2` and its tokenizer `gemma4`; b12259 (`351025ec5`, 2026-11-10) has
  `LLM_ARCH_GEMMA_EMBEDDING2` in `src/llama-arch.cpp` and `src/models/gemma-embedding2.cpp`. The runtime does **not** have to move. Run
  through the release `llama-b12259-xcframework.zip` (the macOS slice, the same URL and checksum as `mobile/AutoradioKit/Package.swift`),
  not a source build, so the trial ran the exact binary the Mac command links.
- The file's pooling is mean (`pooling_type 1`), `add_bos` and `add_eos` are true (llama.cpp's tokenizer adds both), the context is not causal.
  The pooled vector is **not unit length**: raw norms are 259 to 1394 (measured over 479 vectors), so the caller normalises
  (llama.cpp's own server does the same). After normalising the norm is 1.00000 (min 0.999999, max 1.000001). Route B's output norms are
  0.9995 to 1.0005 (fp16).
- Prompt strings are the card's own (section 3). They go in the text: `title: <headline> | text: <body>` for documents (the card's
  titled form) and `task: search result | query: <question>` for questions.
- Metal (GPU) is essential, the CPU is not usable for a backlog: see the numbers below.

### 0.3 Numbers (`mobile/EmbedTrial`, corpus of 449 passages: 300 synthetic stories + 149 passages of Alice's Adventures in Wonderland)

Corpus (all ours, no owner data): 300 invented news stories dated 2 to 15 November 2026, many on overlapping subjects (senate votes, bridge
closures, a chip company), 24 of them the answers to 24 questions; and *Alice's Adventures in Wonderland* (Project Gutenberg #11, public
domain, fetched from the PGLAF mirror `gutenberg.pglaf.org/cache/epub/11/pg11.txt`, because gutenberg.org itself answered "down for
maintenance"), cut into 149 passages of about 150 words; 6 questions on it. 30 questions in all: 13 exact-name ones (BM25 should win) and 17
paraphrases written to share no key words with the answer. 54,216 tokens in the corpus.

| | A, GGUF Q8_0, Metal | A, GGUF Q8_0, CPU 4 threads | B, Core AI, Neural Engine |
|---|---|---|---|
| load, first in the process | 517 ms (an earlier run 8.6 s: cold file cache) | 699 ms | 7.6 s to 8.2 s **after** the cache existed (see note) |
| load again in the same process | 253 to 288 ms | 439 ms | prewarm 1.4 s |
| passages / second (449, one at a time) | 23.5 | 1.3 | 26.8 |
| tokens / second | 2,835 | 160 | 3,237 |
| per passage p50 / p95 | 25 ms / 95 ms | 408 ms / 1,800 ms | 25 ms / 67 ms |
| one query (about 20 tokens), p50 / p95 | 22.3 ms / 27.4 ms | 251 ms / 267 ms | 15.0 ms / 15.1 ms |
| process footprint after load / peak | 148 MB / 161 MB (peak resident 484 MB) | 278 MB / 369 MB (peak resident 646 MB) | 1,055 MB / 1,110 MB (peak resident 4.4 GB) |

Notes. (1) The first Core AI run was stopped by us after 2 minutes 20 seconds, still inside its first load (the coordinator needed the
build lock); the next run loaded in 8.2 s, so the one-time specialisation on this Mac took longer than 2 minutes and less than the
roughly 5 minutes between the two runs, not measured more finely. The card says about 140 s. (2) "Cold" file cache was not forced (it
needs `purge`, which needs root). (3) The 4.4 GB peak resident size is the Neural Engine's mapped model on this Mac; the footprint
column is what the system charges the process. (4) Runs went one at a time through the Mac build lock; A's Metal numbers are one
run each (a first run: 21.5 s, 20.9 passages/s, load 8.6 s). (5) A phone is not this Mac: the card puts the Neural Engine in
the M5 at 5 ms a call; ours is 15 ms for a query.

Smoke (same for both routes): dimension 768; a paraphrase pair scores 0.887 and an unrelated pair 0.679 (route A: 0.8872 / 0.6782 Metal, 0.8873 /
0.6788 CPU; route B: 0.8873 / 0.6794).

Routes against each other (same 479 texts: 449 passages and 30 questions): cosine min **0.99906**, mean **0.99987**, 5th percentile 0.99977; at
256 d: min 0.99916, mean 0.99989. They are close; a store still must not mix engines.

Vector store (`FlatStore`, flat float16 file read back with `mmap`, Accelerate `cblas_sgemv` over 4,096-row blocks converted to float32, top 50):

| Vectors | dim | file | scan, f16 blocks p50 / p95 | scan, f32 in memory p50 |
|---|---|---|---|---|
| 330 | 768 | 495 KiB | 0.27 ms / 0.28 ms | 0.02 ms |
| 50,000 (random, unit) | 768 | 75,000 KiB | **16.8 ms / 19.1 ms** | 10.2 ms |
| 330 | 256 | 165 KiB | 0.07 ms | 0.02 ms |
| 50,000 | 256 | 25,000 KiB | **4.9 ms / 5.0 ms** | 6.0 ms |

A scan is not the cost of a question: a query takes 15 to 22 ms to embed, the scan at corpus size is far under a millisecond, and at
50,000 vectors the scan is about as long as embedding the query. This Mac is not a phone: the page's trigger ("a scan on the A16
above about 50 ms at p95") is **untested**; 768-d at 50k is 17 ms here.

Quality, 30 questions, a hit is a right passage in the top 5 (for a book question, any passage holding the phrase), MRR counts the first right one:

| Method | recall@5 all | MRR all | recall@5 exact (13) | recall@5 paraphrase (17) | MRR paraphrase |
|---|---|---|---|---|---|
| BM25 alone (`RetrievalIndex`, no floor, top 50) | 0.767 (23/30) | 0.591 | 1.000 | 0.588 (10/17) | 0.376 |
| vectors, route A, 768 d | **1.000 (30/30)** | **0.957** | 1.000 | **1.000** | 0.953 |
| vectors, route A, 256 d | 1.000 | 0.901 | 1.000 | 1.000 | 0.855 |
| vectors, route B, 768 d | 0.967 (29/30) | 0.956 | 1.000 | 0.941 | 0.951 |
| vectors, route B, 256 d | 1.000 | 0.901 | 1.000 | 1.000 | 0.855 |
| BM25 + vectors A 768, RRF k = 60, top 50 each | 0.933 (28/30) | 0.810 | 1.000 | 0.882 | 0.693 |
| BM25 + vectors A 256, RRF k = 60 | 0.933 | 0.785 | 1.000 | 0.882 | 0.651 |
| BM25 + vectors B 768, RRF k = 60 | 0.933 | 0.810 | 1.000 | 0.882 | 0.693 |

What this says, and what it does not:

- **Vectors beat BM25 on paraphrase** (recall@5 1.00 against 0.59; BM25 ranked the right story 44th, 25th, 19th and 18th on several, and
  did not list two at all) and **tie it on exact names** (both 13/13). Exact-name questions did not need BM25.
- **RRF with k = 60 did worse than vectors alone** here (0.933 against 1.000; MRR 0.81 against 0.96): BM25's weaker list pulls a wrong
  story over the right one where BM25 ranks the right one low and a wrong one high. The page's recipe (section 9) is **not** supported on
  this corpus; the cheap alternatives (vector first, BM25 only as a tiebreak or for a name that the vectors miss; or weight the vector
  list) were **not measured**. Do not weaken the finding: 30 questions and one author; the sets overlap by design.
- Truncating to 256 d keeps recall@5 and costs about 0.05 MRR (0.957 to 0.901).
- The questions and stories are ours and were written by one author with the model's behaviour in mind only loosely; this is a smoke-level
  measure, not a benchmark. The weakest question for every vector route was the Alice question about growing too tall for the house
  (rank 5 for A, 6 for B, where BM25 had 2).
- Route B's one miss (29/30) is that same question at rank 6; route A has it at rank 5. A sixth-place difference between two fp16/Q8 runs is noise.

### 0.4 The phone (what the app now has, and what the lead runs)

Built in this lane (class orange; not built for iOS by the builder, only the Mac targets compile here):

- `AutoradioKit/Embedder.swift`: the `Embedder` protocol (`engine`, `dimension`, `embed(prompts)`), `EmbedPrompt` (the card's strings),
  `VectorStore` (flat float16 file + `rows.json` of ids and content keys, atomic save, drops rows the vector file lacks, starts empty when
  the engine or dimension changes, Accelerate scan), `RankFusion.rrf` and `EmbeddingJob`. Tests: `EmbedderTests` (prompts, truncation,
  incremental embedding by content key, reopen, engine change, cancel and resume, speech wins, a missing model, fusion).
- `AutoradioModels/LlamaEmbedder.swift`: route A, one context on the shared `LlamaWeights`, fetched through `ModelStore.shared.ensure`
  into Application Support (spec `KnownModels.embeddingGemma2`, SHA-256 pinned, in the registry beside the Qwen files: the registered
  count is now one more; the log line is `autoradio models: registered N models, M stages`). Nothing is in the bundle; `check-no-weights.sh`
  now also refuses `*.aimodel`, `*.aimodelc` and `*.mlirb`.
- `EmbeddingJob` embeds only what is new or changed (content key), 4 passages per slice by default, saves after every slice (a kill loses
  one slice), waits and looks again while `busy()` is true (the app passes "a voice line is on or the listener is speaking": the
  `RadioDemand`, section 0.7) and rests 150 ms between slices. Wired in section 0.7. It never starts after a crash by itself.
- `AutoradioModels/EmbedDeviceTrial.swift`, **the device trial**. Launch the app once with `-EmbedTrial 1` (the same way as
  `-ModelProofStages`, e.g. `devicectl device process launch ... --arguments "-EmbedTrial 1"`). With the flag absent it does nothing. It
  downloads the 310 MB file when needed (Wi-Fi), loads it, embeds the 24 fixture stories, asks the 24 questions, scans 50,000 random vectors,
  runs the job and stops and resumes it, and writes `Documents/proof/embed-trial.json` plus `autoradio embed-trial:` lines on stderr. The
  file holds: device and OS, thermal state before and after, low power mode, model download seconds, load ms first and again, memory
  available (`os_proc_available_memory`) before, after load and after the batch, footprint and peak, dimension and raw norms, the smoke
  cosines and PASS, passages per second and tokens per second, query p50 and p95, recall@5 and MRR for BM25, vectors 768, vectors 256 and
  fused, the 50k scan at 768 and 256 d, the job's stop-and-resume line, and for route B whether the OS has `CoreAI.framework` (route B is
  not built into the app). Pull it with `devicectl device copy from` as for `results.json`.

### 0.6 Which way to combine BM25 and vectors (measured 2026-11-17, 475 documents, 50 answerable questions)

Corpus as in 0.3 plus 26 more stories (20 answers to 20 new questions that mix a name with a paraphrase, 6 decoys that share a name with an
answer: two "Fenwick", two "Marlowe", and so on) and 6 questions the corpus cannot answer. 475 documents: 300 + 26 stories, 149 Alice passages. Route A,
Metal. A hit is a right document in the top 5 of a list of 20; the 50 are the first 30 questions of 0.3 ("base") and the 20 new ("extra").
The three strategies are `HybridRank.fuse` in `AutoradioKit/HybridRetrieval.swift`; each is unit-tested (`HybridRetrievalTests`).
"BM25 as shipped" is `RetrievalIndex.candidates` with its relevance floor (two matching terms, at least half of the question's terms).

| Method | recall@5 base 30 | MRR base | recall@5 extra 20 | MRR extra | recall@5 all 50 | MRR all | names (27) recall / MRR | paraphrase (23) recall / MRR |
|---|---|---|---|---|---|---|---|---|
| BM25 as shipped (with its floor) | 0.433 | 0.394 | 0.500 | 0.500 | **0.460** | 0.437 | 0.704 / 0.660 | 0.174 / 0.174 |
| BM25 with no floor | 0.733 | 0.587 | 0.950 | 0.892 | 0.820 | 0.709 | 1.000 / 0.895 | 0.609 / 0.491 |
| (a) vectors only, 768 d | 1.000 | 0.957 | 1.000 | 1.000 | **1.000** | **0.974** | 1.000 / 0.981 | 1.000 / 0.965 |
| (b) RRF k = 60, top 50 of each | 1.000 | 0.915 | 1.000 | 1.000 | 1.000 | 0.949 | 1.000 / 0.954 | 1.000 / 0.943 |
| (c) vectors, plus every passage holding a rare word | 1.000 | 0.957 | 1.000 | 1.000 | **1.000** | **0.974** | 1.000 / 0.981 | 1.000 / 0.965 |
| (a) at 256 d | 1.000 | 0.901 | 1.000 | 1.000 | 1.000 | 0.941 | 1.000 / 0.981 | 1.000 / 0.893 |

Read it plainly: the BM25 the app has today finds the right passage in the top 5 for **46 percent** of these questions (its relevance floor
drops most paraphrases: 17 percent of the paraphrase questions), vectors find **all 50**. (a) and (c) tie to the third decimal; (b) is
worse on MRR (RRF lets BM25's weaker list move wrong items above right ones). **Shipped: (c)**: the same numbers as (a), and a name or number
that the vectors rank low still stays in the list (at most half of it). It is the only path; there is no setting. What the numbers cannot
say: (c)'s guarantee never fired on this set (the vectors already ranked each name first), so the only evidence for (c) over (a) is the
tie plus the unit test of the case; 50 questions, one author, a Mac. A corpus where the right passage holds a rare word the vectors miss
would separate them.

Floor on the cosine: the best cosine of each answerable question was at least **0.716** (5th percentile 0.745, median 0.808); of the 6
unanswerable ones at most **0.627**. The app's floor is **0.67** (`KnownModels.embeddingCosineFloor`): all 50 answerable kept, all 6
unanswerable refused (also by BM25's own floor), with 0.04 of margin each side. Six unanswerable questions is a small sample; treat 0.67 as
a first setting to re-check on the phone's real content, not as a constant of the model (an unrelated sentence pair scores 0.68 in the
card's sentence-similarity prompt, so a floor below 0.65 would let anything through).

### 0.7 What is wired (lane `embed-trial`, class orange)

- `EmbeddingService` (`AutoradioKit/EmbeddingService.swift`): attached at launch where the content store opens (`AutoradioModule.swift`), it sets
  `SavedCorpus.retriever` to a `FusedRetriever` straight away (BM25 alone, nothing said about vectors, until the model is on the phone and 90
  percent of the items hold a vector), waits 15 s after launch, then runs a pass on every store change (3 s settle): it reads the index, makes
  the item list (every live item; an item longer than about 180 words as windows `id~w1`, `id~w2` ...; newest first, undated last: stories,
  book passages, transcripts, mail, documents all come from the index), embeds only what is new or changed (content key), and saves after every
  slice of 4. A killed app resumes from the file.
- The model: fetched once into the model store by `ModelStore.ensure`, **only on Wi-Fi** (`NWPathMonitor`: Wi-Fi or wired, not expensive), SHA-256 checked;
  off Wi-Fi nothing is fetched and the state is `absent` until Wi-Fi comes back. No weights in the repo.
- Waits (the job sleeps and looks again): Low Power Mode, thermal state serious or critical, and `RadioDemand.busy`: a voice line or the programme
  audible (`LiveAudio.voiceIsSpeaking`, `programmeAudible`), speech being made (`ProgrammePlayer.synthesisBusy`, new), remote audio playing
  (`RunSheetPlayer.isRemote` and not paused, copied every 2 s because the player is main-actor state). Not connected: the listener's own speech
  before a turn starts (the job runs while the listener speaks if nothing is on air).
- Files: `<content store root>/vectors/vectors.f16` and `rows.json`; the folder carries the store's protection class
  (`completeUntilFirstUserAuthentication`), the files are written with it, and it is excluded from backup. Sign-out: `SavedCorpus.reset()` runs a hook that
  stops the pass and wipes the vectors; the store's own `wipe()` removes the root.
- Console, counts only: `AUTORADIO embed model=<ok|downloading|absent> items=<done>/<total> rate=<per s>` on change (at most once a second,
  and at the end of a pass) and, per question, `AUTORADIO retrieve retrieved=<n> vec=<n> bm25=<n> ms=<n>`. The prompt and the commands are the
  other lanes'; the seam is `SavedCorpus.retriever`.
- Tests (all through the lock): `HybridRankTests`, `FusedRetrieverTests`, `EmbeddingServiceTests` (download once and only on Wi-Fi, low power and
  thermal pause, speech pauses, sign-out wipes, newest first, backup flag), `EmbedderTests`.
- iOS: `swift build --triple arm64-apple-ios18.0 --sdk iphoneos --target AutoradioModels` compiled `LlamaEmbedder` and `EmbedDeviceTrial` for
  arm64 iOS against the release xcframework's iOS slice (with the package's llama condition temporarily widened to iOS; the committed `Package.swift` is
  unchanged, the Exact app links the static build). Not compiled: the Exact app, `AutoradioModule.swift`, a device or simulator run.

### 0.5 Commands

```
cd mobile/EmbedTrial && python3 -I corpus/make_corpus.py        # corpus and the device fixture (deterministic)
swift build -c release                                          # through the Mac build lock; resolves the llama release xcframework
.build/release/embed-trial bench --route a [--gpu 0|1] [--tag NAME] [--limit N]
.build/release/embed-trial bench --route b --compute ane
.build/release/embed-trial quality a-gpu b-ane ; compare a-gpu b-ane ; scan
```
Weights: `ggml-org/embeddinggemma-2-GGUF` `embeddinggemma-2-Q8_0.gguf`; `EverPaper/embeddinggemma-2-coreai` text asset, tokenizer, config and
Swift sources (the Swift sources are vendored unmodified in `mobile/EmbedTrial/Sources/EmbeddingGemma2CoreAI/`, Apache-2.0).

## 0.8 On the server: feasibility for the corpus, x86 (study of 2026-11-18, nothing measured on x86 yet)

The owner, 2026-11-18: "now may be the time where we should start looking
into doing embeddings and vector search over our corpus. I think we can use
embedding Gemma 2 and run all that in x86. But we should do a feasibility
study for sure." Why: [the corpus search audit](../problems/research-does-not-start-from-the-corpus-2026-11-18.md).
External facts below were fetched on 2026-11-18 from the pages in
"Sources"; the rest is arithmetic on stated assumptions.

**Verdict: feasible on Node B for stories and the lead of each captured
page; not for every page's whole text.** The unknown that decides it is
tokens per second on our CPU, which nobody has published.

- **Hardware.** Node B: Xeon E-2236, 6 cores and 12 threads, AVX2, no
  AVX-512, 31 GB (18 GB available), load about 3.7, no GPU. The build VM is
  the same CPU with 8 vCPU, 3 GB and 20 GB of disk.
- **Where it runs.** A computer's VM has 2 vCPU and 2 or 4 GiB
  (`internal/vmctl/ownership.go`), so the embedder is a host service that
  computers call, like the gateway, under a CPU and memory ceiling. One
  embedder per computer would cost 2 vCPU each on the same host.
- **Runtime without cgo.** llama.cpp's server with the official Q8_0 GGUF
  (310 MB), called over HTTP. The model's architecture is in llama.cpp;
  whether the nixpkgs `llama-cpp` version has it is not checked.
  text-embeddings-inference does not list the architecture. Pure-Go
  inference is for much smaller models.
- **Speed.** No x86 figure exists. One published single-vCPU PyTorch run:
  130 to 155 tokens a second. Our Mac, CPU, four threads: 160. Planning
  range 100 to 300 tokens a second per core.
- **The job** (assumed: 8,000 stories of 250 tokens; 250,000 captured
  pages, the first 512 tokens of each). On three cores: stories 1 to 2
  hours; page leads 31 to 93 hours; all page text (about 0.9 billion
  tokens) 12 to 35 days. Steady state: under a fifth of a core for new
  pages.
- **Storage** for 258,000 vectors: 0.8 GB at 768 float32; 66 MB at 256
  dimensions int8 (the model is trained to be cut to 256; quality 60.4
  against 61.4 on the multilingual benchmark).
- **Index.** Brute force in process is enough to about a million vectors
  at 256 int8. pgvector 0.8 is in nixpkgs and was the owner's stated
  direction when Qdrant was removed (2026-10-30). `modernc.org/sqlite/vec`
  exists without cgo; not verified at our pin.
- **Whose index.** One public index for stories and the paper's captured
  pages, beside corpusd, which stays custodian. A listener's files and mail
  are indexed inside that listener's computer, never in the shared one.
- **Licence.** Apache 2.0 on the Hub with no gate on the page; the card
  adds the Gemma Prohibited Use Policy. Whether that policy binds a hosted
  product is the owner's reading.
- **Alternatives looked at.** Qwen3-Embedding-0.6B (Apache 2.0, scores
  higher, about twice the compute); the Jina v5 small models (non-commercial
  licence, ruled out).
- **Not shown anywhere:** that a French query lands near an English
  document. Our measurement has to show it.
- **What vectors are for here.** Finding held sources worded differently
  from the query; proposing that a story overlaps an existing one; proposing
  that a radio story is about an event already heard. In the last two a
  model judges; similarity only proposes.

**How we will know** (claims C132 and C133 in the calibration ledger):
query = a published story's headline; relevant = the pages that story
cites; compare today's title-only search, BM25 over page text, vectors, and
both; by language. Indexing page text for BM25 is the cheap rival: if it
gets most of the gain, embeddings wait.

## 0.9 Measured for the corpus (2026-11-18): retrieval on Autopaper's own stories, and speed on x86 and the Mac

Result files are in the lead's scratch directory (`embed-eval/`), not in
the repo. Claims C132 and C133 in the calibration ledger.

### Retrieval

200 published stories (all English; no non-English story was found on the
front), query = the headline, relevant = the pages the story cites (6.3 a
story), pool = 1,253 cited pages. **Page text was not available**: no
public route serves it, so each page is its title plus the paragraphs the
story quoted from it. That flatters every method that reads text and it
cannot compare full pages with page leads (1.22 passages a page).

| Method | Recall at 10 | Recall at 40 | MRR |
|---|---|---|---|
| Today's capture search (title and host words) | 0.273 | 0.273 | 0.669 |
| BM25 over title and text | 0.800 | 0.877 | 0.948 |
| Vectors, 768 dimensions | 0.927 | 0.979 | 0.983 |
| Vectors, 256 dimensions int8 | 0.918 | 0.979 | 0.985 |
| Both, reciprocal rank fusion (k 60) | 0.856 | 0.967 | 0.964 |
| Vector order with BM25's top three guaranteed a place | 0.926 | 0.979 | 0.983 |

- Vectors beat BM25 by 12.7 points of recall at 10 (95% interval 9.4 to
  16.1). About half of that is other languages: of 209 cited pages not in
  English (French, Russian, Spanish, Japanese), vectors put 72% in the top
  ten and BM25 33%; today's search 3%. This is the first evidence we hold
  that the model matches an English query to a page in another language.
- Reciprocal rank fusion is worse than vectors alone, as on 2026-11-17.
  Vector order with BM25's best hits kept is the combination to build.
- Cutting to 256 dimensions int8 costs one point.
- With a short keyword query of the kind the research desk writes: BM25
  0.756, vectors 0.857.
- BM25 wins where a page's title is empty or the match is a rare exact
  term.

### Story against story (headline plus standfirst, 200 stories)

The 30 closest pairs, read by hand: eight the same event told twice
(cosine 0.764 to 0.954), two updates (0.872 to 0.927), seventeen related
but different (0.738 to 0.858), three unrelated (0.748 to 0.801). Every
pair at 0.86 or above was a duplicate or an update but one. Below that the
groups overlap. So similarity proposes and a model must judge.

### Speed

| Where | Tokens a second |
|---|---|
| Build VM (Xeon E-2236), one thread, load about 10 | 164 (192 passages; later runs under heavier load: 50 and 97) |
| Build VM, two threads | 276 (1.69 times one) |
| Build VM, one thread, 520 passages (163,818 tokens), second run, load 1.6 rising to 5 | 127 |
| Build VM, four threads, 400 passages, load 3 to 7 | 349 (2.74 times one) |
| Build VM, four threads, batches of four, load 7 to 11 | 301 (batching did not help) |
| The lead's Mac (M1), CPU, four threads | about 1,390 |
| The lead's Mac, Metal | about 3,000 |

The first runs were on a saturated VM. The second run (same day, 520
passages) started at load 1.6 but other work arrived during it, so no figure
here is from a quiet machine; eight threads was skipped at load 11.5.
Vectors from the VM match the Mac's (cosine 0.9999). Query latency on x86:
one thread 145 ms median and 182 ms at the 95th percentile, four threads 56
and 80 ms (50 queries, load 11; the first run gave 120 ms median). Node A is the same CPU as Node B
and was at load 9.6.

### What it means for the plan

- One x86 thread embeds about 0.4 to 0.5 of a 300-token passage a second
  (127 to 164 tokens a second on a shared machine); four threads about 1.1
  passages a second. That is enough for the steady state and slow for a
  first pass over everything.
- First pass over 250,000 pages: at the lead of each page (about 400
  tokens) roughly 9 hours on the Mac with Metal, or 37 to 68 hours on Node
  B on six or three cores. At whole pages the answer depends on a number
  we do not have, the tokens of extracted text per page. At the 3,600
  earlier assumed: 83 hours on the Mac with Metal, 14 days on six cores of
  Node B.
- Steady state at whole pages (4,500 pages a day at 3,600 tokens) is about
  190 tokens a second: one and a half x86 threads at the lower measured
  rate, all day.
- Not measured: full page against lead (needs the real page text), any
  non-English story, a quiet x86 machine.

## 0.11 The server (Node B, 2026-11-19; built, not yet deployed)

Station V1 of the night's goal. Code: `nix/embedder.nix`, `nix/postgresql.nix`, the
caller in `internal/platform/vector_*.go`. Facts below were read from the nixpkgs
pin (`fe13242`, 2026-10-15), the Hub's LFS listing and llama.cpp's tree on
2026-11-19; the unit has **not** been run (nothing was built on x86 or on Node B).

- **llama.cpp.** The nixpkgs `llama-cpp` is 0.6.0 (2026-10-05) and has no
  `src/models/gemma-embedding2.cpp` (404 at that tag; present at b12259). The module overrides
  the package to b12259 (`351025ec5`): `src` by rev with hash
  `sha256-tB+aeXiqzy9D5tBHLNy+AJRjfilggVaX/ppA4YlGM70=`, `npmDepsHash` of `tools/ui` at that
  commit, no patches, no tests. Both fixed-output hashes were built on the Mac
  (`fetchFromGitHub`, `fetchNpmDeps`) and matched. The CPU build compiles the C++ of one
  llama.cpp (no CUDA, no Vulkan) and the web UI's npm build; the time on Node B is not measured.
- **The model file.** `ggml-org/embeddinggemma-2-GGUF`, `embeddinggemma-2-Q8_0.gguf`, 309,855,456
  bytes, SHA-256 `2188ac1deca4b77dffefd603c2776a9d76d9d74ec01841392982ebb840b09135`, fetched from
  the Hub at commit `6ea3e3a3...` by the unit's `ExecStartPre` into
  `/var/lib/go-choir-embedder` (not the repo, not the Nix store). A file whose hash differs is
  deleted and the unit fails and retries every 60 s with no start limit.
- **The unit** `go-choir-embedder`: `llama-server --embeddings --pooling mean --host 127.0.0.1
  --port 8090 --threads 4 --parallel 2 --ctx-size 4096 --batch-size 2048 --ubatch-size 2048`.
  `CPUWeight=10`, `Nice=10`, `IOWeight=10`, `MemoryMax=1536M`, `OOMScoreAdjust=500`, dynamic user,
  secrets paths inaccessible, `Restart=on-failure`. No request limit, no counting. Loopback only;
  the firewall is unchanged (22, 80, 443). Only corpusd is meant to call it; a loopback port cannot
  say which local process connects (residual).
- **Health.** `GET http://127.0.0.1:8090/health`: 200 when the model is loaded, 503 while loading.
- **Calls.** `POST /v1/embeddings` with `{"input": [text, ...]}` (one call for a batch). The server
  returns each mean-pooled vector L2-normalised at 768 dimensions (verify on first run: section
  0.2 says the raw pooled vector is not unit length, so a caller must not rely on it).
- **What corpusd stores.** The first 256 values, normalised again (Matryoshka), as `halfvec(256)`
  with the model's hash (the file's SHA-256 above). The input strings are the card's:
  documents `title: <title> | text: <passage>` (a story passage carries the story's title; a
  capture's single vector is its title and lead passage), queries `task: search result | query: <q>`.
  The server adds `<bos>` and `<eos>`.
- **pgvector.** `postgresql_17.pkgs.pgvector` 0.8.7 in the pin, with `halfvec` and HNSW on
  `halfvec_cosine_ops`. It is not a trusted extension, so `CREATE EXTENSION vector` is run as
  `postgres` by `go-choir-postgresql-roles` in the `graph` database; corpusd's code only checks
  it is there. Adding it restarts PostgreSQL once on the deploy.

### 0.11.1 The vector index in corpusd (station V2; built, not deployed)

Code: `internal/platform/vector_store.go`, `vector_worker.go`, `vector_search.go`, `vector_import.go`;
client and prompt strings in `internal/embedder`. Tests: `vector_test.go` (run against a real pgvector).

- **Tables** (corpusd's graph database, beside `proj_capture_*`; created by the worker, not by
  `MigrateRetrieval`, so a missing extension cannot hold the capture projection's retries):
  `vec_story_passages (story_id, version_id, ordinal, text_sha, day, model, embedding halfvec(256) NULL, refused, embedded_on)`,
  primary key `(story_id, version_id, ordinal)`; `vec_captures (capture_id PK, owner_id, computer_id, text_sha, day, model,
  embedding halfvec(256) NULL, refused, embedded_on)`. A row with no vector and a `refused` reason is a passage or lead the embedder
  refused (see 0.11.3). `text_sha` is the SHA-256 of the passage text, `model` the GGUF's SHA-256, `day` the
  version's creation day or the capture's published, else fetched, day. A story version is written whole in one
  transaction, so one row means all its passages. HNSW on `halfvec_cosine_ops` on each table, built `CONCURRENTLY` under a
  session advisory lock, an invalid leftover dropped and rebuilt by the next pass.
- **One writer.** corpusd alone writes vector rows. The worker's vectors and the Mac's first-pass file (input to the import
  below) pass the same checks (the model's hash, `text_sha` against the passage corpusd cuts itself, public text only) and are
  written by the same two functions; the import adds a sampled re-embedding here.
- **Worker.** (`vectorBatch = 4` is a throughput choice, not a quota or a limit on calls: the largest input the unit takes is 2,048
  tokens, so a batch is at most about 8,000 tokens and about 20 s of CPU at the measured 350 tokens a second, a query waiting behind it at
  most that long; four keeps that wait short while a call is still a handful of passages. Nothing is counted against it.)
  Each pass derives its list: versions of published stories and opinion pieces without a vector of this model
  (newest publication first, newest version first), then projected captures without one (newest first, keyset cursor).
  Owner-private, tombstoned and superseded captures never get one (the full-text index's exclusion); the sweep removes the
  vectors of a withdrawn story and of a capture that has since become private. **No pacing**: it sends one batch of 4 inputs at
  a time and waits for the answer, so a search's query waits behind at most one batch (4 passages of about 300 tokens, one to
  three seconds at the 350 tokens a second measured on this CPU; batches did not change tokens a second). The batch is a size,
  not a limit; it does not rest for searches, and the unit's low CPU weight is what lets guests win. It backs off only after the
  embedder's real failure: one line (`embedder down, backing off`), a doubling wait to 5 minutes, one line when it returns. No
  cap, no queue, nothing dropped; a restart re-derives the gap.
- **Journal.** `corpusd retrieval op=embed_gap shape=stories|captures ms= rows= have= want= remaining= model=<12>` at the start of
  a pass and at each batch of 4 (stories count versions); `op=vector_embed`, `vector_write`, and `op=vector_search
  shape=vector_stories|vector_captures`. `scripts/status-vectors` reads them over ssh (have, want, remaining of each kind, and the
  last hour's searches and mean ms); the acceptance is that line, not SQL on Node B. `remaining` is exact: `want` counts only captures that satisfy the one SQL
  predicate and story versions that pass it (a capture that can never be eligible is in neither `have` nor `want`; an empty or
  unembeddable capture is recorded as refused and held). Exceptions, both visible as `skipped` in the pass line: a capture whose text blob cannot be read at the
  moment, and a version found not public when the pass reaches it. The story walk reads the publications in windows and their
  version edges in one statement per 500 publications (not one query per publication); an idle pass skips even that (see 0.11.3).
- **Search.** `Store.SearchVectors`: the nearest story passages (publication id, version id, ordinal, cosine score, day) and, for a
  named owner and computer, the nearest captures; `since` inclusive and `until` exclusive by day. The query is embedded by the
  Node B unit with the query prompt. No fusion. When the embedder is down, absent or the tables are not ready, the answer is an
  empty result whose `Unavailable` says why and no error, so a caller (station V4) answers from BM25 alone and says so; a
  caller that holds a vector can search without the embedder.
- **How the index is filled: by the server's own worker, newest first** (stories newest publication and newest version first,
  then captures newest first), one pass until its gap is closed, **and by the import below** of vectors the Mac made in a first pass, which
  spares the worker most of that time; the index does not depend on the import. The worker's rate: a pass works through the whole gap in batches of 4 with no wait between them (5 minutes when
  nothing is left). Measured: the unit's 4 threads embed about 350 tokens a second on the build VM (§0.9; 127 to 349 under load there);
  the Mac's passes averaged 285 tokens a story passage and 232 a capture lead (2,563 tokens/s over 9.0 rows/s; 2,669 over 11.5). Reasoned:
  45,673 story passages is 13.0 million tokens, about 10 hours; 215,926 captures is 50.1 million tokens, about 40 hours; about **50 hours (two days)** in all at
  350 tokens a second, 35 hours at 500, over five days at the loaded 127. Not measured on Node B (the unit runs at `CPUWeight` 10 beside the guests).
- **The ranked answer** (station V4, `platform/capture_rank.go`; landed 2026-11-19 with the index): `SearchCaptures` with a
  query calls `SearchVectors` once (4 times the answer size of each kind), keeps only the latest version of each story (a story is
  walked only through the version whose passage passed V2's eligibility predicate), replaces a story by the captures its latest version
  cites (at most 8, `cites_capture` edges), and guarantees BM25's top three pages a place at their BM25 rank. A window narrows the
  vector scan by day and each page again by its exact time, also a page reached through a story. **One deadline**: the vector half
  has 4 s (`captureVectorDeadline`), and each SQL statement's timeout is nine tenths of what is left of it (V2's own 5 s applies only
  to a caller with no deadline), so the two do not race and an exact read that passes it can still come back `partial`. The answer
  says how the vector lists were answered: `vector_state=exact` (the true nearest of the filtered rows), `approximate` (the HNSW
  index's answer on a list over 100,000 rows) or `partial` (an exact read hit the deadline); `vector=` stays the rows returned.
  When the vectors are unavailable the answer is BM25's and `vector=unavailable vector_state=unavailable`.
  Journal: `op=capture_search shape=ranked ... vector=<rows>|unavailable vector_state=exact|approximate|partial|unavailable bm25_kept=`.
- **Neighbours of named captures** (station V5, `platform/vector_neighbours.go`; built on `lane/event-shortlist`, not landed):
  `POST /internal/platform/captures/neighbours` `{owner_id, computer_id, capture_ids, since, limit}` (a bound guest for its own
  scope, or an internal caller; at most 5,000 ids and 50 neighbours each). It reads the named captures' stored vectors (no
  embedder, so a down embedder does not stop it) and for each returns the nearest other captures of the same owner and computer
  under the same eligibility predicate, `since` inclusive by day. **One SQL statement answers up to 500 named captures** (a
  lateral join of their vectors against the scope's eligible rows), so a flush of up to 500 is one statement and one request;
  a larger one is a statement a group of 500 in the same request (at most 5,000 ids; the event layer sends the rest of a larger
  flush in a second request), and a group the deadline or the statement timeout passes is cut, not the whole answer. The list is
  exact when the eligible rows in the window number at most 100,000 and rows times named captures at most 40 million distance
  computations (the count uses the query's predicate), else the HNSW index's, and a list it returns short of the limit is read
  again exactly. Reasoned cost, not measured on the paper: an exact list is one pass over the window's rows for each named
  capture, about 0.2 to 0.5 microseconds a halfvec distance, so a 100,000-row window costs 20 to 50 ms a capture and the 40-million
  bound is a few seconds. Measured on the build VM with the fake embedder: 520 named captures against 520 rows, one request, 1.6 s with the
  eligible rows found once (a first version that repeated the predicate for every pair took 29 s, hence the pool); a flush on the paper is tens of captures (about 17 research reads a minute plus a poll's 20 to 100
  new items a source), so it is well inside the 30 s deadline. Answer: `neighbours` by capture; counts of the captures with no
  list, by reason: `unembedded` (eligible, no vector yet), `excluded` (private, tombstoned, superseded, another scope, unknown)
  and `cut` (not reached before the deadline: that is `state: partial`, a fact about the answer, not about the captures);
  `state` is `exact`, `approximate` or `partial`; `unavailable` ("the vector tables are not ready"; the database's words go to
  the journal) says the vectors cannot answer. The event layer sends every waiting capture of a flush in one request, before it
  takes the event index (a slow request does not hold the Stories read), maps the neighbours to events by their memberships
  and offers each capture its three nearest events beyond the 30 recent. Journal: `op=vector_neighbours shape=captures rows=
  ids= unembedded= excluded= cut= state=`; the event layer's flush line carries `vector_state=ok|approximate|partial|unavailable`,
  `missing_unembedded=`, `missing_excluded=` and `vector_unmapped=` (neighbours whose capture maps to no event in the index).
- **Import** (the Mac's first pass): `POST /internal/platform/vectors/import` on corpusd's loopback port 8086 with the host
  credential, body one JSON row a line (`kind` `story_passage` or `capture`, `id` a publication version's or a capture's, `ordinal`,
  `text_sha`, `model`, `dim` 256, `v` base64 of 256 float16); answers 202 and a job, `GET ?job=` reads it. Refused whole for another
  model or dimension, a malformed row, or a sample of 32 rows re-embedded here that does not reach cosine 0.999. Past that a row is
  written only when this host cuts the same passage (`text_sha` equal) and the text is public; the rest are counted by reason.
  A route and not a subcommand because it needs the running process's embedder, text store and database credentials.

### 0.11.2 Primary sources for the server and pgvector (read 2026-11-19 from the pinned sources)

llama.cpp b12259 (`351025ec5`), source tree at `/nix/store/i552x14p...-source` (the nix `src` of `nix/embedder.nix`; paths below are in the
repository), and pgvector 0.8.7 (`github.com/pgvector/pgvector` tag v0.8.7, `README.md`, `src/hnswscan.c`).

- **Request and response** (`/v1/embeddings`): route `tools/server/server.cpp:196` (`post_embeddings_oai`); handler
  `tools/server/server-context.cpp:6785` (`handle_embeddings_impl`): `"input"` is a string or an array of strings, an empty array or an
  empty tokenisation is a 400 (:6839-6849), `--embeddings` is required (:6786-6789) and pooling `none` is refused for this route
  (:6792). The answer, `tools/server/server-common.cpp:1586-1616`, is `{"data": [{"embedding": [...floats], "index": i, "object":
  "embedding"}, ...], ...}`; the embedding is a plain float array unless the request says `encoding_format: "base64"`. The client reads
  `data[].index` and `data[].embedding` and places by index (`internal/embedder/embedder.go`).
- **Normalisation:** `embd_normalize` defaults to 2 (euclidean), `common/common.h:639`, applied by the server (:6852-6857). corpusd cuts
  to 256 and normalises again, so it does not depend on this.
- **BOS and EOS:** the server tokenises with `add_special = true` and `parse_special = true` (`server-context.cpp:6823-6827`,
  `tokenize_input_subprompt(..., true, true, ...)`), so the vocabulary's own BOS and EOS flags decide (the GGUF sets both, section 0.2).
  **Consequence to watch:** `parse_special = true` means a passage that contains the literal text of a special token (for example
  `<bos>`, `<eos>`, `<start_of_turn>`) is tokenised as that token, not as characters. It is rare in prose and the Mac's library path
  may tokenise differently; the 0.999 sample check does not exercise it. Not changed (the first vectors exist); recorded here.
- **Context per slot and an input that is too long: an error, not truncation.** Slots: `src/llama-context.cpp:429-432`
  (`n_ctx_seq = n_ctx / n_seq_max`, padded to 256) unless the KV cache is unified (`common/common.h:594`, default false), so
  `--ctx-size 4096 --parallel 2` is 2048 tokens a slot. An embedding task cannot be split (`can_split()` is false without a memory module,
  `server-context.cpp:502-520`), so `server-context.cpp:4076-4098` applies: more tokens than `--ubatch-size` (2048) is
  "input (N tokens) is too large to process. increase the physical batch size", and more than the slot's context is "input (N
  tokens) is larger than the max context size ... skipping"; both are errors returned to the client (HTTP 500 / 400), the slot is
  released, nothing is embedded. A passage over 2048 tokens (about 2,400 CJK runes can reach it) therefore fails loudly: the worker's
  one-by-one retry logs `embed refused` and skips it, the import's sample cannot embed it. It is never silently truncated.
- **Flags at b12259** (`common/arg.cpp`): `-t/--threads` :1487, `-c/--ctx-size` :1609, `-b/--batch-size` :1639, `-ub/--ubatch-size`
  :1646, `--pooling` {none,mean,cls,last,rank} :2284, `-np/--parallel` :2515, `-m/--model` :3027, `--host` :3323, `--port` :3339,
  `--embedding/--embeddings` :3506. `/health`: `tools/server/server.cpp:171`. All used flags exist. Not verified by a run: that the
  `gemma-embedding2` model loads under these flags on x86 and the 503 while loading (first start on Node B).
- **pgvector 0.8.7:** `halfvec`, `halfvec_cosine_ops` and HNSW (`sql/vector--0.8.7.sql`; built in the test overlay). Filtering is
  applied after the index scan; iterative scans (`hnsw.iterative_scan` `strict_order`/`relaxed_order`) arrived in 0.8.0 and stop at
  `hnsw.max_scan_tuples` (default 20,000) or `work_mem` x `hnsw.scan_mem_multiplier` (README "Filtering", "Iterative Index Scans",
  `src/hnswscan.c:256-264`). pgvector gives **no signal** when a scan is cut. `strict_order` keeps the rows it returns in distance
  order; it does not make them the nearest, because HNSW is approximate even unfiltered (recall depends on `hnsw.ef_search`; corpusd
  uses 200). What corpusd's search guarantees, per list, and how it says so:
  (1) **Exact** (`exact_scan` in the result): the filtered rows number at most 100,000 (`vectorExactMaxRows`, counted first with the
  btree on scope and day), or the index came back short of the limit. The statement runs with index scans off, so the planner reads
  the filtered rows and sorts by true distance: the true nearest of the rows the filter keeps. (2) **Approximate** (`approximate`):
  more than 100,000 filtered rows (in practice an unwindowed search of the paper's captures or stories): the index's answer, ordered
  by distance, approximate in which rows it found, and not rechecked when it fills the limit. (3) **`partial`**: set when the count, the exact read or the index read
  passes the 5 s statement deadline (each under its own savepoint, so no statement of a search is a hard failure; an index read that passes the deadline
  returns an empty list and says so); an exact read that fails leaves the index's list, which may lack nearer rows. A caller that needs to know has all three.
  The code states its minimum, 0.8.0 (`vectorMinPgvector`), and the CI action requires it numerically.
- **The planner and the ORDER BY.** The statements order by the bare distance (`ORDER BY v.embedding <=> $q LIMIT n`; ties are broken in
  Go), which an HNSW index can serve; `TestVectorSearchStatementUsesTheHNSWIndex` shows the plan uses `idx_vc_hnsw` and `idx_vsp_hnsw`
  once the alternatives to an ordered index scan are priced out. On a small or unanalysed table the planner may rightly prefer the scope's
  btree and a sort (seen in the test before ANALYZE); the production plan is to be read with `EXPLAIN (ANALYZE)` on staging.
- **Memory of the unit** (measured 2026-11-19 on the Mac, one short run): llama.cpp b12259 (`351025ec5`, the trial's source tree)
  built CPU-only, `llama-embedding -m embeddinggemma-2-Q8_0.gguf -c 4096 -b 2048 -ub 2048 -np 2 --pooling mean -t 4`. A 2,007-token
  input (the largest batch the unit accepts is 2,048): peak memory footprint 506 MB, maximum resident set 765 MB (the 310 MB model file is
  mapped, file-backed, reclaimable); a 13-token input: 206 MB footprint, 555 MB resident. The embedding context has no KV cache, and the
  compute buffer is sized once for the ubatch, so two slots do not double it. `MemoryMax = 1536M` is about twice the largest measured
  footprint plus the mapped file. Not measured: x86 (the CPU backend's buffers are similar, not identical) and the server process's own
  overhead (HTTP, tokeniser, a few tens of MB).
- **The Web UI in the nix build.** `scripts/ui-assets.cmake` takes pre-built assets in `tools/ui/dist` first, then an npm build if
  `LLAMA_BUILD_UI=ON`, then a download from a Hugging Face bucket; with none of them it fails (seen: a Mac build without network). The
  nixpkgs derivation runs `npm run build` in `tools/ui` before cmake (`preConfigure`), which is why `npmDepsHash` is overridden; that this
  produces `tools/ui/dist` at b12259 is read from the cmake script, not built (no x86 builder here).
- **The import's spool** (`artifacts/vector-import/`) is not under any namespace the artifact collector sweeps
  (`artifact_gc.go` `artifactGCNamespaces`: projection-base, parts, platform-update, capture-bodies, file-cas-roots, file-cas-chunks, og);
  corpusd removes leftover spool files when its worker starts.

### 0.11.3 Operating the vector index (what an idle pass costs, refusals, the policy clause, rollback, backups)

- **Idle cost.** A pass with nothing to embed reads: one publication row and three counts (the story mark: newest publication, live
  publications, live versions, the newest live version's creation time, vector rows held), the refused counts, the captures' gap count (an anti-join of the 215,000 projected
  captures against `vec_captures`), and the sweep (two `DELETE ... WHERE NOT <predicate>` statements; each row costs a few primary-key
  lookups, and a story row one JSON parse). It does **not** walk the publications: the walk (one edge read per publication, 10,000 of
  them) runs when the mark differs from the last complete walk's, and once after a restart. `TestVectorIdlePassReadsOneRowAndTwoCountsNotEveryPublication`
  proves no edge read happens in an idle pass. The sweep and the gap count are not measured at production size; read
  `corpusd retrieval op=vector_sweep shape=both ms=` on the first staging pass, every 5 minutes. If the sweep is too dear the search's own
  predicate still keeps removed text from being returned, so the sweep can then run less often.
- **What may become a durable state: one rule, in one place** (`embedInputs`, with `embedder.classifyStatus`). A refusal is recorded only
  when the server **identifies the input** as the problem: its two too-large answers, "input (N tokens) is too large to process" and "input (N
  tokens) is larger than the max context size" (0.11.2; `embedder.InputRefusedError`). Then the version's row (or the capture's) holds no
  vector and the reason, so the version is whole in the table and out of the gap, the model is not asked again until the model changes (the row's
  `model`), `refused=` counts it, and a search returns the version's other passages and never that one (BM25 still finds it). Every other failure
  is not recorded and is retried: a status error of any code (a 500 without those words, a 503 while loading, a 429, a 404 from a mismatched
  route) is the server's state, so the pass stops and backs off like a down embedder; a transport or decode failure of one input while the server is
  healthy leaves that input out of this pass (one log line with its id; passes that do no other work are 5 minutes apart, so the volume is
  at most one line per input per 5 minutes) and the older captures go on. The audit of every place a failure could become durable:
  the refused row (only the identified refusal above); the "empty: no title and no text" row (a capture whose text was read and is empty: a
  property of the data, not a failure); an unreadable text blob or unreadable metadata (skipped, retried every pass); a version whose manifest policy is not public, or with no
  passages (skipped; walked again when the mark changes); the story idle mark (stored only after a walk with no error). **One function decides
  what an embedder outcome may do** (`vector_outcome.go`, applied by `embedInputs` for every caller): the server names the input (its two too-large
  answers) = `refused`, the only effect that may write anything permanent and only about that input; a status error that names no input, or health failing = `down`, nothing
  durable, the caller stops and the same work is found next pass; one call failing in transport = `skip`, nothing written. The worker, the import's sample and the inbox all get those
  three effects, none inspects an error type. **A file** is refused (terminal, `refused/`) only for a fault of the file itself (`importFate`): format, model, dimension, bounds,
  a file that cannot be opened, a `text_sha` this host does not cut the same, a sampled vector that disagrees with this host's embedding of a row it could embed, a symlink or
  directory. Everything else (the server cannot answer, too few rows could be checked now, a database error) leaves the file in `inbox/` for the next pass (state `retry`
  in the journal): unstarted work, not resumed work; a file found in `running/` at boot still goes to `interrupted/`. A sampled row the host's embedder refuses as too large is set aside and
  another is drawn; pass 3 drops the set-aside rows (`oversized`) so the worker records its own refusal; an unsampled oversized row keeps the Mac's vector (see 0.11.4).
  One consequence: a file whose sample keeps meeting rows that are too large or failing in transport waits (it is logged each pass), it is not refused.
  Import row drops are never a stored state; the worker fills what the file did not write. The third too-long wording of the server ("request (N tokens) exceeds
  the available context size", `server-context.cpp:4098-4102`, a splittable configuration) is not classified: the unit's configuration cannot produce it, and an
  unrecognised wording degrades to retry, never to a durable refusal. The import's 32-row sample remains a pipeline check and not a per-row check (a design limit).
- **No DDL on a request path.** Tables and plain indexes are created only when `to_regclass` finds them missing, by the worker and the
  import; a search that finds no tables answers unavailable. The HNSW builds are `CONCURRENTLY` under advisory locks, with
  `maintenance_work_mem` reset before the connection returns to the pool.
- **The story access policy.** The Mac export and the write-time check read the manifest's `access_policy`; the read routes read the
  `has_policy` object's `access_policy_json` (`service_publication_read.go:465`). The one SQL predicate now also refuses a version whose
  policy object says its access is not public (visibility `public` and route empty or `public`); a missing, empty or malformed policy object adds
  no constraint (read with `pg_input_is_valid` and a CASE, so malformed data cannot fail a search or a sweep). The manifest's policy
  stays a Go check at write time, because it is not always in a database column.
- **The extension is created when the roles unit runs after PostgreSQL restarts on the new package.** From the evaluated unit
  (`nix eval` of `go-choir-b`): `postgresql.service` is `Type=notify` (ready when it accepts connections), `go-choir-postgresql-roles` is
  `After` and `Requires` `postgresql.service`, `RemainAfterExit`, `restartIfChanged = true`, and its script text changes with this
  work, so the switch restarts it, after PostgreSQL (`After=`), once the server accepts connections. That is why it runs in this deploy; a later restart of
  PostgreSQL alone restarts it too only by systemd.unit(5) `Requires=` ("stopped (or restarted) if one of the other units is explicitly stopped (or restarted)"), which was
  not tried here. A
  `CREATE EXTENSION` that fails is logged and the unit goes on (the switch does not fail); corpusd reports it every five minutes
  and `scripts/status-vectors` prints `pgvector MISSING or too old` when that is the newest line. `systemctl restart go-choir-postgresql-roles` repeats it.
- **Pgvector below 0.8.0.** The worker's ensure step reads `extversion`; below the minimum it logs `corpusd vectors disabled: pgvector X is older
  than the 0.8.0 ...`, the pass fails with that reason, and the search answers `unavailable` with it (a server that errors on
  `hnsw.iterative_scan` is also taken as unavailable).
- **The guest content digest.** `scripts/guest-content-digest` hashes the guest's inputs and the deploy refreshes every active
  computer when it moves. `nix/embedder.nix` is now in its host-only exclusion, beside `postgresql.nix`. Two other things in this work move it
  regardless: `flake.nix` (the dev shell's PostgreSQL with pgvector; `flake.nix` is a fixed input of the digest) and the Go in
  `internal/platform` and `internal/embedder` (the guest binaries import `internal/platform`, so any non-test change there moves it). So this
  deploy refreshes every active computer through the planned, marker-backed path, as any Go change in those packages does; the
  embedder unit's later flag changes will not.
- **Rollback.** Reverting V2 (the Go) alone is safe: the worker stops, the tables stay (unused), the search is gone. Reverting V1 (the
  nix) after the tables exist is **not** safe in one deploy: the old server cannot load `vector.so`; opening an HNSW index (a `DROP`
  of the table, or any scan) is expected to call the access method from it (not run), and the daily `pg_dump` of `graph` calls `halfvec_out` for every
  vector and fails (`nix/node-b.nix:287`: a failed dump ends the backup run partial). Order: **deploy 1** carries the revert of
  V2 plus a roles-unit step `DROP TABLE IF EXISTS vec_story_passages, vec_captures; DROP EXTENSION IF EXISTS vector` while the package
  still has pgvector; **deploy 2** removes `extensions`, the embedder unit and the step. Between them no dump breaks (the tables are gone
  after deploy 1). If both go out together PostgreSQL restarts without the plugin before the roles unit runs, the DROP fails (it is
  non-fatal now), and the dump fails until pgvector is put back.
- **A table of an earlier schema** (no `refused` column, a NOT NULL embedding) is migrated once by the ensure step after it reads the
  catalog (`ADD COLUMN IF NOT EXISTS`, `DROP NOT NULL`); a second ensure alters nothing.
- **Two conditions on the landing plan** (host first, V1; the Go after). (1) The deploy classifier's output must show the guest content digest
  `same` before the switch is trusted to refresh no guest: if CI cannot compute the digest the state is `unknown` and every active computer is
  refreshed (`deploy-impact-classify`, `mark_guest_boot_contract`); read it, do not assume it. (2) Landing 1 ships no corpusd that reports a
  missing extension, so after the host deploy check it by hand: `psql -d graph -c '\dx'` must list `vector`. The vmctl and corpusd restart
  windows (a guest's capture write is re-offered, below) happen in landing 1 as in any host deploy.
- **Backups and restores.** From the first deploy the `graph` dump contains the extension and the vector tables; a restore needs a
  PostgreSQL with pgvector 0.8 or later.
- **A guest's capture write during the restart.** corpusd's guest-write gate asks vmctl who owns the caller (`authority.go:50-90`); while
  vmctl or PostgreSQL is down the lookup fails and the refusal is the forbidden-style one. The guest's sink (`researchtools/capture_sink.go`)
  does not retry: `poll_source.go:261-270` stops the batch on the error and leaves the batch's items out of the handled set, so the next
  poll offers them again (feed items are not lost); `read.go:161-164` returns the error to the agent, which sees the failed Read. Nothing in the
  guest drops a capture silently. (No change made.)

### 0.11.4 The spool inbox (the Mac's file without HTTP or a credential)

`internal/platform/vector_inbox.go`. Whoever can write corpusd's disk on the host already holds more than the host credential, so a
directory adds no authority; the file passes every check the route applies (`ImportVectorFile`: another model or dimension, a bad row,
a failed 32-row sample, the 4 KiB line and group bounds, `text_sha` against the host's own cut, public text only) and is one job among
the route's: one at a time, so a file waits while a route job runs.

```
/var/lib/go-choir/platform-artifacts/vector-import/
  inbox/        *.jsonl files wait here; *.part is ignored; a file younger than 60 s is left alone
  running/      the file being imported (the rename into it is the claim)
  done/         imported; <name>.job.json beside it
  refused/      refused or failed (wrong model, bad row, failed sample, over 2 GiB, a symlink, unreadable); <name>.job.json says why
  interrupted/  found in running/ at start; corpusd never runs it again
```

- **Rules.** A pass takes the oldest eligible file before the gap work, one at a time. A crash restart resumes nothing: at start
  anything in `running/` is moved to `interrupted/` and logged (`moved to interrupted/ and not run again`). A file is retried only
  by a person moving it back to `inbox/`; a file that could not be imported *now* (the server cannot answer, too few sample rows could be checked) stays in `inbox/` untouched and is
  tried again next pass, and a pass takes one file, the oldest. A file put back because it cannot be imported *now* is not counted as work (the pass keeps its ordinary interval and the idle mark), leaves no job entry (only the journal line; finished jobs are kept to 32), and is tried after any newer file that is not waiting; a file whose sample rows are rejected for the file's own reasons (not found, not public, a text this host cuts differently) is refused even if some other rows were set aside. A symlink, a directory named `*.jsonl` or other non-regular file is moved to `refused/` as it is, never opened or followed. A name already used in `done/` or `refused/` is never overwritten (the new file is `name.2`, its record `name.2.job.json`). Nothing
  deletes `done/`, `refused/` or `interrupted/`: the artifact collector sweeps named namespaces only (`artifact_gc.go`
  `artifactGCNamespaces`: none of these), and corpusd removes nothing there. The files are large (the stories about 30 MB, the captures about
  200 MB); the lead deletes them after reading the job record.
- **The Mac's embedder does not refuse an over-long input: it cuts it.** `LlamaEmbedder.swift` takes `toks.prefix(n_ctx)` (2,048 tokens), so a
  row the unit would refuse as too large is embedded on the Mac from its first 2,048 tokens. Measured 2026-11-19 on the captures file (215,926 rows, the lead
  passage; `llama-tokenize` of b12259 with the GGUF over the prompt `title: ... | text: ...` for the 1,656 rows that could possibly reach it: more than 2,400
  runes, or mostly CJK scripts above 400 runes): **1 row is over 2,048 tokens (the longest, 2,110), 2 more are between 1,800 and 2,048**; every other row
  is shorter than the candidates' bound by construction (a Latin or Cyrillic passage of 2,400 runes is about 1,200 tokens). So one capture's vector is of a truncated passage; the host's
  worker would have recorded its own refusal for it. If the sample meets it, it is set aside; if not, the Mac's vector stays (a vector of the first 2,048 tokens). Not changed.
- **The journal line**, one per file (and per route job), the same format: `corpusd retrieval op=vector_import shape=inbox|route ms= rows=
  written= refused= sample_min_cosine= err= reasons=<reason:n,...> job= state= file=`. `refused=` is the rows dropped, `reasons=` says by
  which check. `scripts/status-vectors` shows the last one.
- **To import a file, on Node B as a user who can write that directory (root):**

```
D=/var/lib/go-choir/platform-artifacts/vector-import/inbox
cp captures.jsonl "$D/captures.jsonl.part" && mv "$D/captures.jsonl.part" "$D/captures.jsonl"   # copy whole, then rename
# the worker takes it on its next pass after the file is 60 s old (at most about 5 minutes)
journalctl -u go-choir-corpusd --since '-30 min' --no-pager -o cat | grep 'op=vector_import'
cat /var/lib/go-choir/platform-artifacts/vector-import/done/captures.jsonl.job.json      # or refused/captures.jsonl.job.json
scripts/status-vectors                                                                    # from a checkout: the last import and have/want/remaining
```

## 1. What the model is

- Upstream: `google/embeddinggemma-2` (Gemma 4 family; Matryoshka; text, image
  and audio in one space). **verified** S2.
- The package: a port by Everpaper to Apple's **Core AI** runtime, repo
  `EverPaper/embeddinggemma-2-coreai`, version "v1.2" in the card, last
  modified 2026-10-12, not gated, library tag `coreai`, licence tag
  `apache-2.0`. **verified** S1, S3. Weights are upstream's (bf16 rounded to
  fp16, a few constant scales folded in, unused position rows cut; listed in
  NOTICE). **verified** S1.
- **Core AI** is Apple's framework for running models on Apple silicon (CPU,
  GPU, Neural Engine), format `.aimodel`, Swift API (`AIModel`,
  `AIModelAsset`, `AIModelCache`). Availability on the framework page: iOS,
  iPadOS, macCatalyst, macOS, tvOS, visionOS, watchOS all "27.0.0 -".
  **verified** S4. Apple says Core ML stays for non-neural models; whether
  Core AI replaces Core ML is outside the page (unknown).
- **The port itself targets macOS 27 on Apple silicon only.** Card: "converted
  to Apple's Core AI runtime for macOS 27 on Apple silicon"; requirements:
  "macOS 27 or later on Apple silicon (validated on an M5 with 24 GB)", Xcode
  27 / Swift 6; its `Package.swift` declares `platforms: [.macOS("27.0")]`
  and links CoreAI, ImageIO, CoreGraphics, AVFoundation, Accelerate.
  **verified** S1, S5. Nothing in the card or package says iPhone works or
  does not. iOS is **unknown for this package** (the framework is iOS 27).

## 2. Files and sizes (HF API, 2026-11-17)

**verified** S3 (tree API).

| File | Bytes |
|---|---|
| `model/embeddinggemma-2-text-mm.aimodel/main.mlirb` (text encoder, functions `mm_32`..`mm_2048`) | 632,206,233 |
| `model/embeddinggemma-2-vision.aimodel/main.mlirb` (function `vision_2520`) | 307,025,240 |
| `model/embeddinggemma-2-audio.aimodel/main.mlirb` (`audio_192`..`audio_3000`) | 589,332,135 |
| `model/tokenizer/tokenizer.json` | 32,170,510 |
| `model/tokenizer/tokenizer_config.json` | 1,599 |
| `model/config.json` | 4,643 |
| `swift/Sources/EmbeddingGemma2CoreAI/Embedder.swift` (the runner) | 28,274 |

Text only (our need): text asset + tokenizer + config, about 0.66 GB
(**est** from the rows above); the card says text + image is 0.97 GB with the
tokenizer and the whole folder 1.56 GB, and that image and audio are
separate assets that can be excluded from the download (`--exclude`). S1.
The card shows the download as `hf download EverPaper/embeddinggemma-2-coreai
--local-dir ...` and lists a `CHECKSUMS.sha256` in the repo. S1, S3.

## 3. Technical facts (card, all **verified** S1 unless noted)

- Precision fp16 (upstream says "Do not use float16" because activations
  overflow; the port divides RMSNorm rows and the final L2 normalisation by
  their max-abs to avoid it; upstream card S2). Output is fp16: convert to
  float32 before accumulating dot products.
- Quantisation: none beyond fp16 (no int8/int4 variant in the package).
- Dimension 768, **already L2-normalised, mean-pooled by the graph**. The
  caller does not pool or normalise at 768.
- Matryoshka: 512, 256, 128 (config lists 768, 512, 256, 128): take the first
  N values and re-normalise (`embed(..., dim:)` does it). Queries and
  documents must use the same dimension (upstream, S2). Upstream: "close to
  lossless down to 256", 128 degrades multimodal quality (S2). Multilingual
  MTEB v2 mean upstream: 61.36 (768), 61.17 (512), 60.41 (256), 57.89 (128).
- Max input 2048 tokens including `<bos>`, `<eos>` and the prefix; longer is
  truncated by the runner (upstream accepts 8192). Card advice: chunk
  documents at 512 tokens or fewer for retrieval. Sequence buckets 32, 64,
  128, 288, 512, 1024, 2048; padding does not change the result. Batch is 1.
- Tokenizer: the unchanged Gemma `tokenizer.json`, shipped in the repo and
  implemented in Swift by the package (`GemmaTokenizer`, "exact"
  re-implementation, no third-party dependency).
- Languages: tagged `multilingual`; upstream "100+ languages", unequal
  quality (S2). Audio tested in 18 languages (S1).
- Prompts, verbatim (text only; images and audio take none). The runner adds
  `<bos>` and `<eos>` itself. Strings from `model/config.json`:

```
document             "title: none | text: "      (titled: "title: {title} | text: {content}")
search_query         "task: search result | query: "
question_answering   "task: question answering | query: "
fact_checking        "task: fact checking | query: "
code_retrieval       "task: code retrieval | query: "
classification       "task: classification | query: "
clustering           "task: clustering | query: "
sentence_similarity  "task: sentence similarity | query: "
```

  Our use: passages as `document` (with the story or book title when known),
  the spoken or typed question as `search_query`.
- Swift (card sample): `let model = try await EmbeddingGemma2(bundle:
  modelURL, compute: .neuralEngine)`; `try await model.prewarm()`;
  `let q = try await model.embed("...", kind: "search_query")` gives
  `[Float]` (768); `embed(..., kind: "document", dim: 256)`;
  `cosineSimilarity(a, b)` is provided.

## 4. Measured by the porter (one Mac only)

**verified** S1; all on an M5 (24 GB), macOS 27, on AC power. **Not an
iPhone.**

- Short text: about 5 ms per call on the Neural Engine (floor about 4.8 ms).
  Sequence functions on the Neural Engine: `mm_128` 7 ms, `mm_288` 15 ms,
  `mm_1024` 70 ms. `mm_512` is not given.
- Accuracy against the fp32 reference: cosine typically above 0.9998
  (Matryoshka 512/256/128 about 0.99998 / 0.99998 / 0.99999 on their set).
- Memory text only: about 225 MB process plus about 770 MB on the Neural
  Engine (the figure the summary gave; the card's all-assets figure is about
  2.1 GB). Disk cache: per-app specialisation cache about 1.4 GB for the text
  asset on the Mac (S1).
- **First load by a new app: minutes** (about 140 s for the text asset alone
  in 1.0; 3-4 minutes text + image; 5 minutes all three), once per app and
  OS build; warm load 0.5 to 1.8 s. The card says load and prewarm in the
  background with progress, never lazily before the first query.
- Energy: the only figure is images, 3.1 J per image above idle on the Neural
  Engine (whole-machine, MacBook on AC). No figure for text energy.

## 5. How it would fit the app (phone lane read-only, `origin/lane/autoradio-integ` a565232b0)

- Models are declared in `AutoradioKit/.../KnownModels.swift`: `ModelSpec`
  (id, url, sha256, bytes, fileName) for files the kit fetches and pins, and
  `ManagedModelSpec` (id, label, source, approxBytes, licence, cachePath,
  cacheFolders) for libraries that fetch their own (FluidAudio Core ML
  bundles, under its own cache folder). `ModelStore` fetches on demand to
  Application Support, resumable (HTTP Range on a `.part`), SHA-256 pinned,
  unloads what the configuration no longer needs. llama.cpp b12259's
  xcframework is linked (Mac command; the Exact app links a static build).
  Source: `Package.swift`, `ModelStore.swift`, `KnownModels.swift` in that
  lane (read via `git show`). The registry count "22 models, 34 stages" was
  **not found by grep** in the lane at that commit (unknown where it is
  printed).
- `mobile/scripts/check-no-weights.sh` fails a built `.app` that holds
  `*.gguf`, `*.mlmodelc`, `*.mlpackage`, `*.mlmodel`, `*.onnx`,
  `*.safetensors` or `*.bin` over 100 KB. `.aimodel` and `.aimodelc` are
  **not** in its list: add them in the same change (yellow). Rule stays:
  nothing ships in the bundle; download at first use.
- Core AI is not referenced anywhere under `mobile` in that lane (grep for
  `coreai`, `CoreAI`, `Core AI`: no hit). Our existing page already notes
  "Core AI exports need iOS 27; whether this phone runs it is to check"
  (ON-DEVICE-MODELS.md section 1). The lane's README mentions iOS 27.0.1.
- Fit: a new `ManagedModelSpec`-like entry whose fetch is the HF files (not a
  GGUF), checked by `CHECKSUMS.sha256`; loaded through the package's
  `EmbeddingGemma2`, or through Core AI directly (`AIModel`) after porting the
  tokenizer and bucket logic, because the package's `Package.swift` only
  declares macOS.

### Core AI specialisation on a phone (Apple docs, **verified** S6, S7)

- Every `.aimodel` is "specialized to the specific device" on first load;
  the result is cached per app. The system deletes the cache on OS update,
  when the source file changes, and under storage pressure (unless
  `.persistent` policy). `AIModelCache.default.model(for:options:)` tells
  whether a cache exists; `AIModel.specialize(contentsOf:options:)` does the
  work at a time of our choosing (after download, with progress).
- Ahead-of-time compile (`xcrun coreai-build compile ... --platform iOS`) only
  covers "iPhone or iPad with the A17 Pro chipset or later, a Mac with the M1
  chipset or later". The owner's iPhone 14 Pro Max has an A16 (ON-DEVICE-MODELS
  section 1): **no ahead-of-time asset for it**, so the full on-device
  specialisation cost lands on that phone. Whether Core AI runs at all on an
  A16 is **unknown** (the docs say specialisation is per device; they do not
  list a floor for plain `.aimodel` loading). The porter's `.aimodel` is
  portable; no AOT `.aimodelc` is shipped.
- Memory: the card's figures are Mac. The phone's budget is
  `os_proc_available_memory()` (our setup measures it). Unknown on a 6 GB
  phone.

## 6. The alternative runtime: llama.cpp with an official GGUF

- `ggml-org/embeddinggemma-2-GGUF` exists (not gated, Apache-2.0, last
  modified 2026-11-01, converted automatically by ggml-org): text
  `embeddinggemma-2-Q8_0.gguf` 309,855,456 bytes, `embeddinggemma-2-BF16.gguf`
  557,950,176 bytes, plus `mmproj-*` files (554,821,056 Q8_0; 982,074,816 BF16)
  for image/audio. **verified** S8. Others: `unsloth/embeddinggemma-2-GGUF`,
  `AtomicChat/embeddinggemma-2-GGUF`, ONNX, litert-lm, an MLX int8 text
  port. S9.
- We already link llama.cpp b12259 and have a pinned-GGUF fetch path (Qwen3
  GGUFs). **Tested 2026-11-17 on the Mac (section 0):** b12259 runs this
  architecture, the file mean-pools but does not normalise (we do), the card's
  prompt strings work, and its vectors are within cosine 0.9991 of the Core AI
  vectors (mean 0.99987). Phone: unknown until the device trial.
- Q8_0 is 310 MB against 632 MB fp16: half the disk. Quality loss from
  Q8_0 is unmeasured by us (the upstream card gives no quantised figures, S2).
- Note a vector from the GGUF and a vector from Core AI are not guaranteed to
  be interchangeable; an index must record which engine and dimension made
  it, and rebuild on change.

## 7. Licence and shipping (not legal advice)

- The package: Apache-2.0, "Copyright 2026 Everpaper", with `LICENSE` and
  `NOTICE` (modification list). The base model is stated as Apache-2.0
  and "Deployments must adhere to the Gemma Prohibited Use Policy". **verified**
  S1, S3 (licence tag), S2.
- Oddity to resolve: the upstream card's "Apache 2.0" link points at a page
  named `gemma_4_license` and the card does not explain the relation (S2).
  Read the licence text itself before first shipping. **Unknown** whether
  terms beyond Apache-2.0 and the Prohibited Use Policy apply.
- Redistribution inside an app: Apache-2.0 permits it with the licence,
  notices and a change statement. We would not redistribute: the app
  downloads from Hugging Face at first run. No gating on the repo
  (`gated: false`, S3). Include attribution in Settings (Everpaper port,
  Google DeepMind model, trademark notice that neither is affiliated).
- Hosting: downloading from the Hub depends on Hub availability; mirroring
  the files on our host is allowed by Apache-2.0 (with the notice), the
  decision is open.

## 8. Vector store (on device)

Corpus size guess: under 50,000 passages (500 stories at about 8 passages
= 4,000; 5 books at about 100,000 words in 500-token chunks = about 1,300
(**est**); the rest grows with the library).

### Arithmetic (**est**, ours)

Bytes per vector = dim x bytes per element. float32 768d = 3,072 B; float32
256d = 1,024 B; float16 halves it.

| Vectors | f32 768d | f16 768d | f32 256d |
|---|---|---|---|
| 10,000 | 30.7 MB | 15.4 MB | 10.2 MB |
| 100,000 | 307 MB | 154 MB | 102 MB |
| 500,000 | 1.54 GB | 768 MB | 512 MB |

Brute-force scan cost is N x d multiply-adds per query: 10k x 768 = 7.7 M;
100k = 76.8 M; 500k = 384 M (at 256d one third of that). A modern phone
core does well over 10 G multiply-adds a second with Accelerate
(`cblas_sgemv` / `vDSP_dotpr`; that rate is an **unmeasured assumption**),
which gives about 1 ms, 8 ms and 40 ms for 10k, 100k, 500k at 768d, memory
bandwidth allowing: the 100k case reads 307 MB per query, so it is
bandwidth-bound and may be several times slower. Measure it (section 11).
At 50k passages (150 MB f32, or 77 MB f16) a scan is well inside what a
voice turn tolerates.

### Options

- **Plain in-memory brute force (Accelerate).** No dependency, exact results,
  trivially correct, no index to corrupt; persistence is a flat file of
  vectors plus ids, mapped with `mmap`. Matches the existing in-memory
  `PassageIndex` BM25 and the lane's "no dependency" style.
- **sqlite-vec** (S10 to S13): Apache-2.0 and MIT dual; **pre-v1** ("expect
  breaking changes"); latest stable release v0.1.9 (2026-03-31), newer ones are
  alphas (v0.1.10-alpha.4, 2026-05-18). Brute-force KNN through `vec0`
  virtual tables, float, int8 and binary vectors, metadata (16), partition
  (4) and auxiliary (16) columns. The repository also contains IVF and
  DiskANN source and the alpha notes mention "ivf/diskann features", but the
  docs fetched did not describe them (treat as experimental/unknown). For
  Apple platforms: it ships as C (`sqlite-vec.c`, amalgamation) with a
  `SQLITE_VEC_STATIC` switch "meant for statically linking", which the docs
  say is unstable across patch versions. A static link registers the init
  function on a connection you own; iOS's system SQLite and extension loading
  are **unknown** from the docs read (the docs fetched do not cover iOS), so
  the safe route is our own SQLite build with sqlite-vec compiled in, not
  the system library. Swift wrappers exist (`jkrukowski/SQLiteVec`, MIT; and
  others); how they link SQLite is **unknown** (page did not say).
- **USearch** (S14): Apache-2.0, v2.26.6 (2026-10-25), Swift listed among
  bindings and iOS among platforms, HNSW-family graph, f16/i8 storage,
  save/load. An approximate index: worth it above hundreds of thousands of
  vectors, with build cost and a file to rebuild.
- **Apple ships**: no vector database in the frameworks I checked; the Core
  AI page and the Natural Language framework were not read for one. Unknown.

### Recommendation

For a corpus under 50,000 passages: **brute force with Accelerate over a
float16-stored (or f32) flat file, 768 dimensions at first**, in memory,
with BM25 beside it. Reasons: exact, no dependency, no release-quality risk
(sqlite-vec is pre-v1 and its static-link flag is not stable), fits the
existing `PassageIndex` design, and the arithmetic above is small. Keep the
vectors in a table (id, source, dim, engine, model hash, vector blob) in the
same store as the passages so the filters are a plain query.

Revisit when: a measured scan on the A16 floor phone exceeds about 50 ms at
p95 (probably past 150,000 to 200,000 vectors at 768d, **est**), or memory
for the vectors exceeds what `os_proc_available_memory()` leaves, or we
need filtered search inside the index. Then, in order: Matryoshka 256d for a
three-times smaller scan, a cheap prefilter by source, then sqlite-vec
(stable release by then) or USearch.

## 9. Hybrid ranking with BM25

- **Reciprocal rank fusion** (Cormack, Clarke, Buettcher, SIGIR 2009, S15):
  RRFscore(d) = sum over rankings r of 1 / (k + r(d)), with **k = 60**,
  "fixed during a pilot investigation"; the paper found k = 60 "near-optimal,
  but ... the choice was not critical". Weaviate and Azure AI Search both
  cite 60 (S16, S17). It uses ranks only, so BM25's unbounded scores and
  cosine's 0..1 scores need no calibration.
- **Weighted score sum**: Weaviate's `relativeScoreFusion` (default since
  v1.24) min-max normalises each list's scores (top 1, bottom 0) and adds
  them weighted by `alpha` (0 keyword only, 1 vector only, default 0.75). S16
  (the page does not give the exact formula for alpha).
- Recipe for us: take the top 50 from BM25 and the top 50 by cosine; fuse
  with RRF k = 60; return the top 5 to 10. Start unweighted. Weight a list
  only with a measured reason (Azure documents per-list multipliers, S17).
  Re-rank by a threshold on the cosine only after measuring (an
  out-of-corpus question must still return nothing: keep `PassageIndex`'s
  coverage rule as a gate on the BM25 side, and a cosine floor on the vector
  side, both set from our own measurements).
- Upstream on retrieval (S2): use the query prefix for queries and the
  document prefix for corpus items; truncate and re-normalise Matryoshka
  vectors; the card gives no BM25 or hybrid guidance.

## 10. Costs: embedding 500 stories and 5 books

No published iPhone figure exists (not for this port, and the card's
numbers are one M5 Mac). Our assumptions (**est**):

- 500 stories x about 8 passages x about 250 tokens = about 4,000 passages,
  about 1.0 M tokens.
- 5 books x about 130,000 tokens (100,000 words) = 650,000 tokens, as
  500-token passages about 1,300.
- Total about 5,300 passages, 1.65 M tokens.

Mac M5, from the card's bucket latencies: 4,000 passages at `mm_288`
(15 ms) = 60 s; 1,300 at `mm_512` (not published; guess 30 ms) = 40 s;
about 100 s in all (**est**). A16 Neural Engine against M5: unknown, and the
A16 will not have ahead-of-time compilation, so the first load adds a
specialisation of minutes (card: minutes on a Mac; phone unknown). Energy:
no figure for text on any device (image 3.1 J on a Mac only).

Measurement to run on the iPhone 14 Pro Max, once, with the app's setup
measurement harness: (1) first-load time with `AIModel.specialize` and the
cache check; (2) peak `os_proc_available_memory()` drop and resident
footprint after load and after 100 embeds; (3) latency per bucket 32, 128,
288, 512, 1024, 2048, 50 trials each, p50 and p95, with `ProcessInfo`
thermal state before and after; (4) the full 5,300-passage run with
`MXMetricManager` or Instruments' energy and the battery level before and
after on battery, in Low Power Mode and not; (5) cosine of the phone's
vectors against the Mac's for the same 20 strings (expect above 0.999);
(6) brute-force scan of 50k random vectors at 768d and 256d with Accelerate,
p50 and p95. Embed in the background; make the index resumable by passage
id and content hash so a kill never loses work.

## 11. What could block us

1. **The package declares macOS 27 only** and the card says macOS. No iPhone
   claim. We need our own iOS 27 build of the package (the source is
   Apache-2.0 Swift with no third-party dependency) and a proof on the
   phone.
2. **A16 has no ahead-of-time compile**, and first load on a Mac took
   minutes; a phone may take longer or be refused. Show progress, specialise
   once after download, keep the cache `.persistent`, and plan the fallback
   (section 6, llama.cpp GGUF) if Core AI on the A16 fails.
3. Core AI is new in iOS 27 and the card warns runtime behaviour may change
   between OS releases; the cache dies on every OS update (S6), so each OS
   update means a re-specialisation.
4. Licence: a Gemma Prohibited Use Policy duty and the unexplained
   `gemma_4_license` link; read before shipping.
5. Single-maintainer port validated on one machine; pin the repo commit
   (`305fc2a9...`, 2026-10-12) and verify `CHECKSUMS.sha256`; mirror the files.
6. The GGUF route's equivalence to upstream is unverified.

## 12. Unknown, to settle

iPhone support of the package; A16 behaviour of Core AI; specialisation time
and memory on the phone; the "22 models, 34 stages" registry count; text
energy; whether llama.cpp b12259 runs the GGUF (**settled**: it does, section 0.2); sqlite-vec on iOS system
SQLite; IVF/DiskANN status; the licence relation above.

## Sources (all fetched 2026-11-17 unless noted)

- S1 Model card (README): https://huggingface.co/EverPaper/embeddinggemma-2-coreai and raw https://huggingface.co/EverPaper/embeddinggemma-2-coreai/raw/main/README.md
- S2 Upstream card: https://huggingface.co/google/embeddinggemma-2
- S3 Hub API: https://huggingface.co/api/models/EverPaper/embeddinggemma-2-coreai and https://huggingface.co/api/models/EverPaper/embeddinggemma-2-coreai/tree/main?recursive=1
- S4 Core AI framework: https://developer.apple.com/documentation/coreai
- S5 Package and config: https://huggingface.co/EverPaper/embeddinggemma-2-coreai/raw/main/swift/Package.swift and .../raw/main/model/config.json
- S6 Ahead-of-time compile: https://developer.apple.com/documentation/coreai/compiling-core-ai-models-ahead-of-time
- S7 Specialisation and caching: https://developer.apple.com/documentation/coreai/managing-model-specialization-and-caching
- S8 GGUF: https://huggingface.co/api/models/ggml-org/embeddinggemma-2-GGUF and https://huggingface.co/ggml-org/embeddinggemma-2-GGUF
- S9 Variants search: https://huggingface.co/api/models?search=embeddinggemma-2
- S10 sqlite-vec repo: https://github.com/asg017/sqlite-vec (README: https://raw.githubusercontent.com/asg017/sqlite-vec/main/README.md)
- S11 Releases: https://api.github.com/repos/asg017/sqlite-vec/releases
- S12 Compiling: https://alexgarcia.xyz/sqlite-vec/compiling.html ; vec0 features: https://alexgarcia.xyz/sqlite-vec/features/vec0.html
- S13 Swift wrapper: https://github.com/jkrukowski/SQLiteVec
- S14 USearch: https://github.com/unum-cloud/usearch
- S15 RRF paper: http://cormack.uwaterloo.ca/cormacksigir09-rrf.pdf
- S16 Weaviate hybrid search: https://weaviate.io/blog/hybrid-search-explained
- S17 Azure hybrid ranking: https://learn.microsoft.com/en-us/azure/search/hybrid-search-ranking
- Local: `origin/lane/autoradio-integ` (a565232b0): `mobile/AutoradioKit/Package.swift`, `Sources/AutoradioKit/{ModelStore,KnownModels,PassageIndex}.swift`, `mobile/scripts/check-no-weights.sh`; [ON-DEVICE-MODELS.md](ON-DEVICE-MODELS.md).
