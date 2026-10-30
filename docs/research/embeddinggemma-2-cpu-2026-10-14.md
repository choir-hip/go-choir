# EmbeddingGemma 2 on older x86 CPUs: fit, throughput, API backfill, GPU option

Date: 2026-10-14. Mutation class: green (research note; no code or service
touched). Asked by the owner (2026-10-14 ~15:00 UTC): can Choir run Google's
new embedding model on CPU, how much of Node B would it take, could an API do
the corpus backfill, and when does a GPU service make sense.

**Labels used below.** **[M]** means a number someone measured and published
(source linked). **[V]** means a vendor figure (Google, a model card, a price
page). **[E]** means my estimate, with the arithmetic shown. No benchmark was
run for this note. Nobody has published an EmbeddingGemma 2 benchmark on a
Coffee Lake Xeon, so every Node B throughput figure here is **[E]** until the
script in §6 is run.

---

## 0. Answer in brief

- **The model fits Node B easily.** The text-only path has 270M parameters.
  130M of them do compute; 140M are an embedding lookup table. In int8 the
  weights are ~310 MB (GGUF Q8_0 or ONNX q8). The process should stay
  under ~1.5 GB RSS at batch 32 × 512 tokens [E]. Memory is not the problem.
- **CPU cost is moderate but real.** Estimate: 120–320 tokens/s per physical
  E-2236 core, depending on the runtime (PyTorch fp32 at the low end, ONNX
  Runtime int8 or llama.cpp Q8_0 at the high end) [E]. Today's incremental
  load is ~22–45M tokens/day, an average of 255–520 tokens/s. That works out
  to **about 1–4 cores' worth, averaged over the day**. A quota of 3 cores
  (`CPUQuota=300%`) covers the mid case.
- **Do not backfill on Node B.** A 3M-chunk backfill is ~0.9B tokens. On
  2–4 cores of the E-2236 that takes **1–6 weeks** [E]. A rented GPU does it
  in **~5 hours for under $10** (L4 [M]). An API does it for **~$18**.
- **There is an API serving this exact model.** DeepInfra lists
  `google/embeddinggemma-2` at **$0.02 per 1M tokens** [V]. The Gemini API
  does **not** serve EmbeddingGemma. Its only embedding model is
  `gemini-embedding-2` ($0.20/M, $0.10/M batch [V]), which is a different
  model in a different vector space. Gemini vectors cannot be mixed with
  local EmbeddingGemma 2 vectors.
- **A full-time GPU is ~100× oversized for text at today's volume.** One L4
  measured ~47k tokens/s [M], which is ~4B tokens/day against a need of
  ~45M. A GPU service pays off only for multimodal (image, video, audio)
  embeddings, a 10–50× jump in volume, or a GPU shared with other work.
- **Plan:** (1) Pin the embedding contract. (2) Measure with §6 on a test
  box. (3) Run incremental embedding on Node B under `CPUWeight=idle` +
  `CPUQuota=300%` + `MemoryMax=3G`, with a back-off when memory pressure
  rises. (4) Backfill on a rented GPU running the same runtime, or on
  DeepInfra once a parity test passes. (5) If the $99 EPYC is bought (it is
  justified by Qdrant RAM and disk), move the embedder and Qdrant there.

---

## 1. The model

### 1.1 Shape [V]

Released 2026-10-06 by Google DeepMind, built on the Gemma 4 architecture
([blog](https://blog.google/innovation-and-ai/technology/developers-tools/embeddinggemma-2/),
[model card](https://ai.google.dev/gemma/docs/embeddinggemma/model_card_2),
[HF](https://huggingface.co/google/embeddinggemma-2)).

| Property | Value |
|---|---|
| Total parameters | 740M (multimodal: text + image + video + audio) |
| Text-only path | **270M = 130M transformer + 140M embedder** (262,144-token vocab × 512) |
| Optional encoders | vision 170M, audio 300M. Load them selectively; text-only skips both |
| Layers / width | 24 layers, d_model 512, FFN hidden 2048, 4 heads, 2/1 KV heads (local/global), 5:1 local:global, 1024-token sliding window |
| Pooling / output | mean pooling, then a 512→768 projection, **768-d** native |
| Matryoshka (MRL) | 512 / 256 / 128 dims. Re-normalize (L2) after truncating. Queries and documents must use the same dimension |
| Context | **8,192 tokens** (EmbeddingGemma 1: 2,048) |
| Languages | "100+" (training data: 140+) |
| Precision | **No float16** (activations overflow: NaN or silently degraded vectors). Use bf16 where hardware supports it, else fp32. **Node B has no native bf16, so use fp32 or int8** |
| Task prompts | Documents: `title: {title or "none"} \| text: {content}`. Search queries: `task: search result \| query: {q}`. Others: QA, fact checking, code retrieval, classification, clustering, similarity. Omitting the prompts lowers quality |

Selective loading in sentence-transformers ≥ 6.1:
`config_kwargs={"vision_config": None, "audio_config": None}`. All
configurations load from the same checkpoint into the same vector space.
Modalities can be added later without re-embedding
([developer guide](https://developers.googleblog.com/en/embeddinggemma-2-the-developer-guide/)).

### 1.2 Quantized and packaged variants

| Package | Text-model size | Quality vs fp32 | Source |
|---|---|---|---|
| HF safetensors (BF16, full multimodal) | ~1.5 GB | reference | [HF](https://huggingface.co/google/embeddinggemma-2) |
| ONNX fp32 (text) | 1,085 MB | 1.0 | [onnx-community](https://huggingface.co/onnx-community/embeddinggemma-2-ONNX) [V] |
| ONNX fp16 (text) | 543 MB | cos ≥ 0.9998 (worst case) | same [V]. Avoid anyway: no gain on AVX2 |
| **ONNX q8 (text)** | **314 MB** | **cos ≥ 0.9997** | same [V] |
| ONNX q4 / q4f16 (text) | 175 / 157 MB | cos ≥ 0.988 (text) | same [V] |
| **GGUF Q8_0** (llama.cpp, arch `gemma-embedding2`) | **310 MB** | not published | [ggml-org GGUF](https://huggingface.co/ggml-org/embeddinggemma-2-GGUF) |
| GGUF BF16 | 558 MB | — | same. Slow on Node B (no native bf16) |
| Ollama `embeddinggemma-2:270m` | 378 MB | quant level not stated | [Ollama](https://ollama.com/library/embeddinggemma-2) |
| PyTorch int8 Linear + int4 token table ("repack") | 203 MiB | cos 0.986–0.998. SciFact nDCG@10 0.836 vs 0.854 (97.9%) | [dev.to Cloud Run study](https://dev.to/gde/embeddinggemma-2-on-cloud-run-an-int8-cpu-service-against-an-nvidia-l4-15ac) [M] |

For the predecessor, Google published quantization-aware results on
MTEB(Multilingual, v2) Mean(Task): bf16 61.15, int8 60.93, mixed 60.69,
int4 60.62 ([EmbeddingGemma paper, Table 1](https://arxiv.org/html/2509.20354v3)) [V].
Int8 costs ~0.2 points. Expect the same order for EG2 at q8, but this is
unmeasured for EG2.

**llama.cpp caveat.** EmbeddingGemma 1 had a llama.cpp-vs-transformers
embedding mismatch ([llama.cpp #19040](https://github.com/ggml-org/llama.cpp/issues/19040)).
Whatever runtime we choose must pass the parity check in §6 against the
sentence-transformers fp32 reference before any vector is stored.

### 1.3 Quality vs alternatives [V]

MTEB(Multilingual, v2), from the EmbeddingGemma paper Table 5 and the EG2
model card. "Retrieval" is the multilingual retrieval task-type mean.

| Model | Params (non-embedding compute) | Multi v2 Mean(Task) | Retrieval | License |
|---|---|---|---|---|
| **EmbeddingGemma 2** | 270M text (130M) | **61.36** | n/p (EG1: 62.49) | Apache 2.0 (see §1.4) |
| EmbeddingGemma 1 | 308M (~100M) | 61.15 | 62.49 | Gemma Terms of Use |
| Qwen3-Embedding-0.6B | 595M (~440M) | 64.34 | 64.65 | Apache 2.0 |
| multilingual-e5-large-instruct | 560M (~300M) | 63.22 | 57.12 | MIT |
| bge-m3 | 568M (~300M) | 59.56 | 54.60 | MIT |
| GTE multilingual base | 305M | 58.24 | 56.50 | Apache 2.0 |
| jina-embeddings-v3 | 572M | 58.37 | 55.76 | CC BY-NC 4.0 (non-commercial) |
| Gemini Embedding (API) | — | 68.37 | 67.71 | API only |

EG2 vs EG1: multilingual +0.21, English v2 −1.21 (68.46 vs 69.67), code
+9.92 (78.68 vs 68.76). MRL on EG2: 512d keeps 61.17 and 256d keeps 60.41
(about lossless for text). 128d drops to 57.89.

Only Qwen3-Embedding-0.6B is clearly better on multilingual retrieval, at
~3× the per-token compute (~440M vs ~130M non-embedding parameters, so ~3×
fewer chunks per core). mE5-large-instruct scores higher overall but lower
on retrieval, at ~2.3× compute. **For CPU-bound multilingual retrieval,
EmbeddingGemma 2 gives the best quality per FLOP in this set.** These
figures come from the EmbeddingGemma paper's comparison table. Re-check the
live [MTEB leaderboard](https://huggingface.co/spaces/mteb/leaderboard) for
models released since then (e.g. nomic-embed-text-v2, newer jina and Qwen
versions), none of which I compared here.

### 1.4 License

- Weights are tagged **apache-2.0** on HF. The blog says "Apache 2.0,
  commercially permissive". The card's license link points to the Gemma 4
  license, and Gemma 4 moved to Apache 2.0 in April 2026
  ([coverage](https://www.implicator.ai/google-releases-gemma-4-under-apache-2-0-dropping-its-custom-ai-license/)).
- One wrinkle: the model card also says deployments "must adhere to the
  Gemma Prohibited Use Policy". Under plain Apache 2.0 that reads as a
  usage guideline, not a license condition. Choir's use (retrieval over a
  news/web corpus) is nowhere near the policy's prohibitions either way.
  Owner or legal can confirm if it matters.
- EmbeddingGemma 1 (300M) is under the custom Gemma Terms of Use. That is
  one more reason to start on EG2.

---

## 2. CPU throughput on AVX2-era x86 (no AVX-512, no AMX, no VNNI)

### 2.1 What has been measured by others

| Setup | Result | Notes |
|---|---|---|
| Cloud Run, **1 vCPU**, PyTorch + sentence-transformers, fp32, 1 thread | **0.9 docs/s at 141 tokens ≈ 127 tok/s**. Query latency 251 ms | [dev.to](https://dev.to/gde/embeddinggemma-2-on-cloud-run-an-int8-cpu-service-against-an-nvidia-l4-15ac) **[M]**. Host CPU unknown (likely AVX-512 Xeon); a vCPU is one hyperthread |
| Same, int8 Linear (fbgemm) + int4 token table | **1.1 docs/s ≈ 155 tok/s** (1.29×). Query 121 ms (2.1×) | same **[M]**. fbgemm int8 gains little on a single thread |
| Same, **NVIDIA L4**, bf16, concurrency 4 | **333 docs/s at 141 tokens ≈ 47k tok/s**. Query 56 ms | same **[M]**. Plain PyTorch, not TEI/vLLM: a floor, not a ceiling |
| i7-9700F (Coffee Lake, 8c/8t, AVX2), PyTorch fp32, batch 32, CosQA code functions | EG2 text-only **4.9–5.6 items/s**. EG1 7.9–8.0 items/s ("better, not faster") | Search-result snippet of `github.com/ruijindev0801/embeddinggemma2-benchmark`. **The repo now returns 404, so I could not verify it.** Low confidence |
| EG1, Samsung S25 Ultra, LiteRT XNNPACK 4 threads, int4/int8 mixed | 256 tok: 66 ms. **512 tok: 169 ms (~3,000 tok/s)**. 1024: 549 ms. 2048: 2,455 ms | [litert-community](https://huggingface.co/litert-community/embeddinggemma-300m) **[M]** (phone ARM cores with int8 matmul; shows the runtime ceiling and the cost of long sequences) |
| EG1 Q4_0, `embeddinggemma.c`, Apple M5 Max CPU | 1,050 tok/s at 32 tokens, 1,350 tok/s at 2,048 | [jethac/embeddinggemma.c](https://github.com/jethac/embeddinggemma.c) **[M]**. Its x86 build dispatches AVX2/FMA kernels, but only EG1 is supported |
| M3 Pro laptop (CPU or GPU not stated) | ~78 text embeddings/s, short texts | [HN comment](https://news.ycombinator.com/item?id=49985694) (anecdote) |

**Not found:** any published EG2 (or EG1) number for ONNX Runtime,
OpenVINO, TEI-CPU or llama.cpp on an x86 server CPU. The TEI CPU (candle)
backend had an attention-mask bug for Gemma3/Qwen3 in concurrent
equal-length batches
([HF forum, TEI #882](https://discuss.huggingface.co/t/huggingface-text-embeddings-inference-cpu-bug/177129)).
Do not use TEI-CPU for this model until that bug is confirmed fixed.

### 2.2 First-principles estimate for the E-2236 [E]

- **Compute per token.** The 130M non-embedding parameters cost
  ≈ 2 × 130M = 0.26 GFLOP. Attention at 512 tokens adds about
  4 × 512 × 1024 × 24 ≈ 0.05 GFLOP. Total ≈ **0.3 GFLOP/token**, so a
  300-token chunk is ≈ 90 GFLOP. The token-table half of the model is a
  lookup and costs ~nothing.
- **E-2236 peak.** Two 256-bit FMA ports give 32 fp32 FLOP/cycle/core. At
  ~4 GHz all-core that is ~128 GFLOP/s per physical core, ~770 GFLOP/s for
  6 cores. Hyperthreads add issue slots, not FMA units: expect +0–20%.
  There is no VNNI, so int8 uses AVX2 `vpmaddubsw`. In practice that is
  ~1.3–2× fp32 end to end (the Cloud Run study got 1.29× in docs/s with
  fbgemm).
- **Achieved efficiency.** Small-model embedding inference typically
  reaches 30–50% of peak. The Cloud Run point implies ~37 GFLOP/s per vCPU.
  The i7-9700F snippet, if real, implies ~40% of peak.

| Runtime on one physical E-2236 core | tokens/s/core [E] | chunks/s/core at 300 tok |
|---|---|---|
| PyTorch fp32 (sentence-transformers), untuned | ~120 | ~0.4 |
| ONNX Runtime fp32 / llama.cpp F32 | ~200 | ~0.7 |
| **ONNX Runtime q8 / llama.cpp Q8_0** | **~250–320** | **~0.8–1.1** |

Batching: batch 8–32 at similar lengths is what reaches these numbers.
Batch 1 runs ~1.5–3× slower per token. Sort chunks by length before
batching so padding does not waste compute. Long chunks cost more than
linearly (EG1 phone data: 2,048 tokens costs ~15× a 256-token chunk, not
8×). **Keep chunks at 256–512 tokens.**

### 2.3 Sizing the workload

**Incremental.** The search audit
([search-efficiency report §5](../archive/reports/search-efficiency-and-reuse-2026-10-14.md))
estimates ~4,500 captures/day × ~10 blocks ≈ 45k chunks, plus 50–100k
distinct search hits (one short chunk each, title + snippet), for
~100–150k chunks/day.

- Case A (mixed lengths): 45k × ~300 tok + 100k × ~80 tok ≈ **22M tokens/day**
  (255 tok/s average).
- Case B (all 150k at 300 tok): ≈ **45M tokens/day** (520 tok/s average).
- **A caution on "1.4 GB/day".** That figure is capture *bodies* on host
  disk: 13,710 files at ~105 KB each, up to 1 MiB per body
  (`internal/researchtools/capture_sink.go`), and they include raw page
  bytes. If all of it were embeddable text it would be ~350M tokens/day,
  8–16× case B. Before trusting either case, measure extracted block text
  bytes per capture. §6 includes that step.

**Backfill.** At the audit the capture store held only ~13.7k bodies,
almost all from the last 24 h (≈ 140k chunks at 10 blocks each). "A few
million chunks" therefore means the corpus after ~3–4 weeks of today's
volume (100–150k/day × 20–30 days), or an import of older material. I use
**3M chunks × 300 tok ≈ 0.9B tokens** as the planning case and 10M chunks
(≈ 3B tokens) as a high case. Everything below scales linearly.

### 2.4 Cores and time [E]

Incremental load, as cores busy averaged over 24 h:

| Per-core rate | Case A (22M tok/day) | Case B (45M tok/day) |
|---|---|---|
| 120 tok/s (PyTorch fp32) | 2.1 cores | 4.3 cores |
| 200 tok/s (ORT fp32) | 1.3 cores | 2.6 cores |
| 300 tok/s (ORT q8 / Q8_0) | 0.85 cores | 1.7 cores |

Backfill of 0.9B tokens (excluding the incremental load running alongside):

| Host | Throughput [E] | 3M chunks | 10M chunks |
|---|---|---|---|
| E-2236, 2 cores | 240–640 tok/s | **16–43 days** | 2–5 months |
| E-2236, 4 cores | 480–1,280 tok/s | **8–22 days** | 1–2.5 months |
| $44 Xeon 6c/12t (assume E-22xx class, all 6 cores) | 0.7–2k tok/s | 5–15 days | 2–7 weeks |
| $99 16-core EPYC, 14 cores, Zen 2/3 (AVX2 only) | 1.5–4k tok/s | **2.6–7 days** | 9–23 days |
| Same, if Zen 4 (AVX-512 + VNNI + BF16) | ~2× Zen 3 [E] | ~1.5–3.5 days | 5–12 days |
| 1× L4 rented, PyTorch bf16 (measured rate) | ~47k tok/s [M] | **~5.3 h** (~$2–5) | ~18 h (~$7–15) |
| DeepInfra API, EG2 | rate limits unknown | **~$18** | ~$60 |

Ask the seller which EPYC generation the $99 box has. Zen 4/5 (EPYC
4004/8004/9004) roughly doubles int8 and bf16 throughput over Zen 2/3.

### 2.5 RAM [E]

- Runtime: ~310 MB weights (q8), plus activations at batch 32 × 512 tok.
  The biggest live tensors are FFN 2048 × 16k tok × 4 B ≈ 134 MB and
  attention 4 heads × 512² × 32 × 4 B ≈ 134 MB per layer, freed layer by
  layer. Peak is ~0.5–1 GB of activations, plus ~0.3–0.5 GB for the
  runtime, Python and the tokenizer. **Total ~1–1.5 GB. Set
  `MemoryMax=3G`.** Use batch ≤ 16 at 512 tokens to keep the peak down.
- **Qdrant, which is the real RAM and disk consumer:**
  - 3M × 768 float32 = 9.2 GB (float16: 4.6 GB).
  - At the audit's 40–55M chunks/year, raw float32 vectors are
    **120–170 GB/year**. Node B has 162 GB free, and capture bodies
    already fill ~1.4 GB/day.
  - Qdrant's own EG2 guidance is 768d with 1-bit quantization in RAM:
    104 B/vector, 99.0% nDCG retention without rescoring, 99.7% with 4×
    rescoring ([Qdrant EG2 post](https://qdrant.tech/blog/embeddinggemma-2/))
    [M]. That is ~0.3 GB RAM for 3M vectors and ~5 GB for a year, plus
    the HNSW graph.
  - Keep originals on disk as float16, or keep only 512d originals.
  - Even then, the year-one disk for originals (40–80 GB) argues for the
    new server's NVMe, not Node B.

### 2.6 Node B CPU-quota recommendation [E]

Run the embedder as its own systemd service, a queue consumer reading from
the object graph. Example `choir-embedder.service` fragment:

```ini
[Service]
# Only idle CPU cycles: cgroup v2 cpu.idle (systemd >= 252). Fallback: CPUWeight=1
CPUWeight=idle
# Hard cap: 3 of 12 hardware threads' worth of time (~1.5 physical cores with HT contention)
CPUQuota=300%
Nice=19
IOSchedulingClass=idle
MemoryHigh=2G
MemoryMax=3G
MemorySwapMax=0
# Thread count must match the quota. More threads than quota gives throttle stalls.
Environment=OMP_NUM_THREADS=3
# ORT: intra_op_num_threads=3, inter_op=1; llama-server: -t 3 -tb 3
# Optional: pin to two physical cores and their HT siblings so VM vCPUs on other
# cores do not share FMA units with the embedder (check `lscpu -e` for sibling ids):
# AllowedCPUs=4,5,10,11
```

Plus an application-level back-off. The service reads
`/proc/pressure/memory` and `/proc/pressure/cpu` before each batch. It
**pauses for 60 s when memory `some avg10` > 0.5** (half of vmctl's reclaim
trigger at 1.0, `internal/vmctl/pressure_reclaim.go`) or when CPU
`some avg10` > ~25. Being a queue, it simply falls behind and catches up
overnight. The index stays a derived projection, reporting the newest
indexed object time (search report §5, "one state authority").

What this budget buys: with `CPUQuota=300%` and q8 at ~250–300 tok/s per
core-equivalent (somewhat lower when the threads are hyperthreads sharing
cores), the embedder gets ~600–900 tok/s ≈ 50–75M tokens/day of capacity.
That covers cases A and B with slack, **if the VMs leave that much idle**.
Check before turning it on: `cat /proc/pressure/cpu` and a day of
`sar -u`/`mpstat`. If CPU `some avg10` already sits above ~10 during the
day, Node B has no idle cores to give.

Single query embeddings for search time (≈ 10–30 tokens) take
~20–100 ms on one thread [E, Cloud Run M: 88–121 ms int8 on 1 vCPU], which
is fine for interactive search.

---

## 3. APIs for a one-off backfill

Vectors are interchangeable only when they come from the **same weights**,
with the **same prompt template**, **same pooling and projection**, **same
truncation and normalization**, and close-enough numerics. Anything else is
a different vector space, and mixing them silently breaks ranking.

| Option | Same model? | Price [V] | Notes |
|---|---|---|---|
| **DeepInfra `google/embeddinggemma-2`** | **Yes (EG2 weights, as listed)** | **$0.02 / 1M tokens** | 8,192 context, MRL 128/256/512, "zero retention" ([page](https://deepinfra.com/google/embeddinggemma-2)). Precision likely bf16. No batch API listed. Rate limits unknown. Older `embeddinggemma-300m` (EG1) is $0.002/M |
| Gemini API `gemini-embedding-2` | **No.** A different Google model | $0.20/M standard, **$0.10/M batch** | The only embedding model on the [Gemini price page](https://ai.google.dev/gemini-api/docs/pricing). EmbeddingGemma is not served there. Vertex/Model Garden lists EG2 as "coming soon", which means self-deploying on a rented GPU, not a per-token API |
| OpenRouter embeddings | No EG2 | — | Lists `google/gemini-embedding-2/-001`, bge-m3, mE5-large, Qwen3-0.6B ([models](https://openrouter.ai/api/v1/embeddings/models)) |
| HF Inference Providers | No | — | Model page: "not deployed by any Inference Provider" |
| **HF Inference Endpoints / any rented GPU running our own container** | **Yes, the exact runtime we choose** | L4 $0.70–0.80/h, T4 $0.50/h ([HF pricing](https://huggingface.co/docs/inference-endpoints/en/pricing)); RunPod L4 ~$0.39/h ([computeprices](https://computeprices.com/providers/runpod/gpus/l4)) | 0.9B tokens ≈ 5 h on an L4 at the measured rate ≈ $2–5. Strongest parity, because we control the code |

What this means in practice:

1. **The Gemini API is not a backfill option for an EmbeddingGemma index.**
   Backfilling with `gemini-embedding-2` would mean either (a) the corpus
   lives in Gemini space and every incremental and query embedding must
   also go to Gemini (a permanent cloud dependency, ~$0.20/M,
   ~$130–270/month at case A/B), or (b) two indexes that cannot be merged.
   Gemini Embedding is the better model (68.37 vs 61.36 multilingual), but
   choosing it is a different decision, not a backfill.
2. **DeepInfra EG2 is a valid backfill path, with conditions:**
   - Send pre-formatted text (`title: … | text: …`) and check that the
     API adds no prompt of its own.
   - Request full 768d and truncate and re-normalize client-side, exactly
     as the local path does.
   - Pass the parity test in §6: per-chunk cosine vs local ≥ 0.995 on
     1,000 multilingual samples, and top-10 overlap ≥ 90% on a
     self-retrieval set.
   - Numerics: bf16 GPU vs fp32 reference measured cos 0.9999 [M], and
     local q8 ONNX vs fp32 has worst case cos 0.9997 [V]. These are the
     same space; ANN retrieval tolerates this drift.
3. **Side note on cost.** At $0.02/M, embedding *everything* through
   DeepInfra (case A/B) would cost **~$13–27/month**, less than the $44
   server. The trade is a cloud dependency (the owner prefers local) and
   corpus text leaving the host (listed zero retention; check provider
   terms vs our source terms). This is a reasonable stopgap while local
   capacity is measured.

---

## 4. Medium term: a GPU embedding service

- **Smallest sensible GPU:** a 16–24 GB inference card, either an NVIDIA
  L4 (24 GB, 72 W) or an RTX 4000 SFF Ada (20 GB, 70 W). A 270M model
  needs < 1 GB of VRAM, so the VRAM is for batching and for any co-located
  model (reranker, Gemma 4 for labeling).
- **Monthly cost (rented, 24/7), list prices to verify:**
  - Hetzner GEX44 (RTX 4000 SFF Ada + i5-13500, 64 GB): **~€184/mo** + €79
    setup at launch
    ([LowEndBox](https://lowendbox.com/blog/need-a-cheap-gpu-server-hetzner-brings-them-back-with-some-controversy/)).
    The current page lists GEX45 (RTX PRO 4000 Blackwell 24 GB) without a
    visible price
    ([Hetzner](https://www.hetzner.com/dedicated-rootserver/matrix-gpu/)).
  - RunPod RTX 4000 Ada ~$0.18/h community (~$130/mo); RunPod L4
    ~$0.39/h (~$285/mo).
  - HF Endpoint L4 $0.70–0.80/h (~$510–580/mo).
  - Cloud Run L4 $1.05/h while up, scale to zero (dev.to study [M]: L4
    beats CPU-int8 on cost per document above ~3.4% busy time).
- **Throughput:** ~47k tok/s per L4 with plain PyTorch bf16 at 141-token
  docs [M]. TEI, vLLM or ONNX-CUDA batching should reach 2–5× that [E].
  EG1 on an RTX 3090 with a custom engine did ~100k tok/s at 2,048-token
  inputs [M]. **Today's 22–45M tokens/day is ~10–20 minutes of L4 time per
  day.**
- **Verdict:** a dedicated GPU for text embeddings is unjustified at
  today's volume. It becomes sensible if (a) Autopaper embeds images, video
  or audio (EG2 shares one space across modalities, and image/video
  embedding on CPU runs at seconds per item), (b) volume grows 10–50×, or
  (c) the GPU also serves other models (Jev-like labeling, a reranker,
  Gemma 4). For bursts such as a re-embed after a model change, rent a GPU
  for hours.

---

## 5. Recommendation

**Step 0. Pin the embedding contract before any vector is written.** Record
it in `WebCaptureMetadata.embedding_model` / `embedding_version`, which
exist and are unused:

- model `google/embeddinggemma-2` at a pinned HF revision sha;
- runtime and quant (e.g. `onnx-q8@<sha>` or `gguf-Q8_0@<sha>`);
- prompt template `title: {title|none} | text: {block}`, and queries as
  `task: search result | query: {q}`;
- dims stored = 768, with any truncation done at index time;
- L2-normalized;
- chunker version.

`internal/qdrant/ollama_embedder.go` hard-codes `Dimensions: 1024`. It
must take the dimension from the contract.

**Step 1. Measure on a test box, not Node B** (the $44 Xeon, a cheap
cloud VM with AVX2 only, or the EPYC if bought). Run §6. Decision gates:
q8 ≥ 200 tok/s per physical core, and parity cos ≥ 0.995 vs the fp32
reference.

**Step 2. Incremental on Node B now, under the §2.6 unit** (`CPUWeight=idle`,
`CPUQuota=300%`, `MemoryMax=3G`, PSI back-off). Use ONNX Runtime q8 or
llama-server Q8_0, whichever wins the parity and throughput test. Ollama
works with the existing `OllamaEmbedder`, but it adds a daemon and hides
quant and thread control. Prefer llama-server with explicit `-t`, which is
also OpenAI-compatible.

Watch four things for a week: queue lag (newest indexed object age), VM
PSI, the memory-pressure reclaim count, and the throttle counters
(`cpu.stat nr_throttled`). If the lag grows day over day, or reclaim fires
more than before, stop and go to step 4.

**Step 3. Backfill off Node B.**

- Preferred: rent one L4 or 4090 for an afternoon, run the *same*
  container and quant the local path uses (~$5).
- Alternative: DeepInfra EG2 (~$18 per 3M chunks) after the §3 parity
  conditions pass.
- Either way, write vectors through the same indexer with the contract
  fields, so the backfill and incremental vectors are provably one space.

**Step 4. Buy the $99 EPYC if any of these hold:** Qdrant needs more than
the ~5 GB/year of 1-bit RAM plus float16 originals on disk, Node B's disk
runs out, or step 2 shows no idle CPU. Move the embedder, Qdrant and the
FTS index there. Embedding at ~1–3 of 16 cores, with the rest for Qdrant.
That gives a clean separation from VMs. The $44 Xeon is the same class of
CPU as Node B. It would work for embedding alone, but its 64 GB buys less
Qdrant headroom.

**Step 5. GPU service later**, triggered by multimodal embedding or a
10× volume increase. Until then, rent GPUs by the hour for re-embeds.

**Rollback:** the embedder is a separate unit and the index is a derived
projection. `systemctl stop choir-embedder`, then drop the Qdrant
collection. No authority state changes.

---

## 6. Benchmark script outline (for the lead; not run here)

The goal is real tok/s per core and parity, on the target CPU, under the
same cgroup limits production will use.

```bash
# 0. Hardware and baseline pressure
lscpu | egrep 'Model name|Flags' | grep -o 'avx2\|avx512[a-z_]*\|avx_vnni\|amx[a-z_]*' | sort -u
cat /proc/pressure/{cpu,memory}

# 1. Sample: 2,000 real blocks from capture bodies (multilingual mix),
#    plus 500 search-hit snippets. Record extracted-text bytes per capture
#    (sizes the 1.4 GB/day question in §2.3).
#    -> sample.jsonl {id, title, text, lang}

# 2. Reference vectors (fp32, sentence-transformers >= 6.1, text-only)
python ref.py sample.jsonl ref.npy   # config_kwargs={"vision_config":None,"audio_config":None}, torch fp32

# 3. Candidates, each under the production limits:
for T in 1 2 3 4 6; do
  systemd-run --scope -p CPUQuota=$((T*100))% -p MemoryMax=3G -p CPUWeight=idle \
    env OMP_NUM_THREADS=$T python bench.py --runtime ort-q8   --threads $T sample.jsonl
  systemd-run --scope -p CPUQuota=$((T*100))% -p MemoryMax=3G \
    env OMP_NUM_THREADS=$T python bench.py --runtime ort-fp32 --threads $T sample.jsonl
  systemd-run --scope -p CPUQuota=$((T*100))% -p MemoryMax=3G \
    python bench.py --runtime st-fp32 --threads $T sample.jsonl
done
# llama.cpp: llama-server -m embeddinggemma-2-Q8_0.gguf --embeddings -t $T -tb $T -b 4096 -ub 512 \
#   --pooling mean (verify the GGUF applies the 512->768 projection; parity decides)
# then: python bench.py --runtime http --url http://127.0.0.1:8080/v1/embeddings ...
# quick raw number: llama-bench -m embeddinggemma-2-Q8_0.gguf -p 256,512 -n 0 -t $T -embd 1  (check flag name)
```

`bench.py` (≈ 120 lines):

1. Format the texts with the contract prompt. Tokenize with the HF
   tokenizer to count real tokens.
2. Sort by token length and bucket into batches of {1, 8, 16, 32} at max
   length {128, 256, 512}.
3. Warm up 3 batches. Then time ≥ 60 s per configuration. Report
   **tokens/s, chunks/s, p50/p95 batch latency, peak RSS**
   (`resource.getrusage`), and the cgroup's `cpu.stat` (`usage_usec`,
   `nr_throttled`, `throttled_usec`).
4. Parity vs `ref.npy`: per-chunk cosine (min/p1/mean). Top-10 overlap on
   a self-retrieval task (title as query → own block, 500 pairs), at 768d
   and at 512d/256d after truncation and re-normalization.
5. Optional API leg: the same 1,000 texts through DeepInfra
   (`google/embeddinggemma-2`), with the same parity metrics.
6. Write `bench-<host>-<date>.json` and a one-line CSV row per
   configuration. Commit the JSON under `docs/research/` as the receipt
   that replaces the [E] numbers above.

Pass criteria for adoption:

- q8 min cosine ≥ 0.99 and mean ≥ 0.998 vs fp32;
- top-10 overlap ≥ 0.9;
- ≥ 200 tok/s per core at batch 16 × 256–512 tokens;
- peak RSS < 2 GB.

---

## Sources

- Google blog, EmbeddingGemma 2 (2026-10-06): https://blog.google/innovation-and-ai/technology/developers-tools/embeddinggemma-2/
- Model card: https://ai.google.dev/gemma/docs/embeddinggemma/model_card_2
- Hugging Face: https://huggingface.co/google/embeddinggemma-2
- Developer guide: https://developers.googleblog.com/en/embeddinggemma-2-the-developer-guide/
- ONNX export (sizes and cosine per dtype): https://huggingface.co/onnx-community/embeddinggemma-2-ONNX
- GGUF: https://huggingface.co/ggml-org/embeddinggemma-2-GGUF
- Ollama: https://ollama.com/library/embeddinggemma-2
- Qdrant EG2 quantization study: https://qdrant.tech/blog/embeddinggemma-2/
- Cloud Run CPU-int8 vs L4 study (measured): https://dev.to/gde/embeddinggemma-2-on-cloud-run-an-int8-cpu-service-against-an-nvidia-l4-15ac
- EmbeddingGemma (v1) paper with MMTEB comparison and quantization tables: https://arxiv.org/html/2509.20354v3
- EG1 ONNX (QAT scores, no-fp16 note): https://huggingface.co/onnx-community/embeddinggemma-300m-ONNX
- EG1 LiteRT CPU latency by sequence length: https://huggingface.co/litert-community/embeddinggemma-300m
- embeddinggemma.c (EG1 engine, AVX2 dispatch): https://github.com/jethac/embeddinggemma.c
- llama.cpp EG accuracy issue: https://github.com/ggml-org/llama.cpp/issues/19040
- TEI CPU attention-mask bug report: https://discuss.huggingface.co/t/huggingface-text-embeddings-inference-cpu-bug/177129
- HN thread (M3 Pro anecdote): https://news.ycombinator.com/item?id=49985694
- Secondary summary (Google AI Edge device table): https://www.eesel.ai/blog/embeddinggemma-2
- Gemini API pricing: https://ai.google.dev/gemini-api/docs/pricing
- DeepInfra EG2: https://deepinfra.com/google/embeddinggemma-2 ; EG1: https://deepinfra.com/google/embeddinggemma-300m
- OpenRouter embedding models: https://openrouter.ai/api/v1/embeddings/models
- HF Inference Endpoints pricing: https://huggingface.co/docs/inference-endpoints/en/pricing
- RunPod L4 price aggregate: https://computeprices.com/providers/runpod/gpus/l4
- Hetzner GPU matrix: https://www.hetzner.com/dedicated-rootserver/matrix-gpu/ ; GEX44 launch price: https://lowendbox.com/blog/need-a-cheap-gpu-server-hetzner-brings-them-back-with-some-controversy/
- Gemma 4 Apache 2.0 license change: https://www.implicator.ai/google-releases-gemma-4-under-apache-2-0-dropping-its-custom-ai-license/
- Repo context: docs/archive/reports/search-efficiency-and-reuse-2026-10-14.md §5, docs/archive/problems/search-is-duplicated-and-discarded-2026-10-14.md, internal/qdrant/ollama_embedder.go, internal/vmctl/pressure_reclaim.go
