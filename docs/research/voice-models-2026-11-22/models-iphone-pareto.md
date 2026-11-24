# On-device speech models for an iPhone 14 Pro Max (A16, 6 GB, iOS 26): what is new and what could be Pareto optimal

Researched 2026-11-22. Read-only. Every number below was read from the page named beside it, through a fetch tool that summarises pages with a small model, so a figure can be mis-copied. Check any number before you rely on it.

## Read this first: the evidence is thin in three ways

1. **There is almost no A16 data.** The only public iPhone ASR/TTS measurements I found are on iPhone 16 Pro (A18 Pro), 17 Pro and 17 Pro Max. The A16 Neural Engine is listed at about 16-17 TOPS against 35 TOPS for A18/A19 (Notebookcheck, https://www.notebookcheck.com/A16-vs-A19_14602_19964.247552.0.html; spec sheet, not a measurement). Treat every iPhone RTF here as a lower bound on A16 cost. I have no measured scaling factor.
2. **Hugging Face list pages gave unlabeled counts, and no dates for "trending".** I used them only to discover names. The Open ASR leaderboard page (https://huggingface.co/spaces/hf-audio/open_asr_leaderboard) returned no table to the fetcher, so I have no leaderboard WER table from the page itself. Leaderboard numbers below come from model cards that cite it.
3. **Most numbers are maker's claims.** Third-party iPhone measurements I found: the voice2text issue (iPhone 16 Pro), the soniqo benchmark file (iPhone 16 Pro), the TranscribePsy card (iPhone 17 Pro Max, the maker's own comparison), and one blog on Gemma on iPhone 17 Pro.

Evidence classes used: **MC** maker's claim; **3P** third-party measurement; **LB** leaderboard; **none**.

Baselines you named, for comparison only: Apple SpeechAnalyzer, Parakeet TDT v2/v3, Parakeet EOU 120M, Nemotron streaming, Fluid Speech Pro/Mini, Kokoro, Silero, Smart Turn v3.2, WeSpeaker, Qwen3-0.6B/LFM.

Two facts about the Apple baseline:
- Apple's own SpeechTranscriber scored 14.0 WER and speed factor 70 on an earnings-call subset on an M4 Mac mini. Parakeet v2 scored 11.7 and 359; whisper-small.en 12.8 and 35 (https://www.argmaxinc.com/blog/apple-and-argmax, June 2025, macOS 26 beta 1; the maker is a competitor).
- A developer reports SpeechTranscriber works on iPhone 14 Pro but not on iPhone SE 2nd gen. This is not Apple-confirmed. Apple's reply was to use a runtime `isAvailable` check (https://developer.apple.com/forums/thread/807739, https://developer.apple.com/forums/thread/802863).

## Tools and runtimes found (integration cost)

| Package | What it gives | Min OS | Maturity / licence | Source |
|---|---|---|---|---|
| FluidAudio (Swift) | Core ML ASR, VAD, diarization, TTS; v0.15.6 dated 2026-08-19 was the latest I saw | iOS 18 for newer models | Active. The TTS product pulls in GPL ESpeakNG. | https://github.com/FluidInference/FluidAudio, https://raw.githubusercontent.com/FluidInference/FluidAudio/main/README.md |
| speech-swift (soniqo) | Large Core ML/MLX catalogue: LocalVQE AEC, ReDimNet2, Smart Turn, turn-1-mini, VAP, Nemotron 3 diarization, Phonon, Supertonic-3 | macOS 15 / iOS 18 | Pre-1.0 (v0.0.29). Repo Apache-2.0; per-model licences differ. | https://github.com/soniqo/speech-swift, https://github.com/soniqo/speech-swift/releases |
| mlx-audio-swift (Blaizzy) | MLX STT/TTS/VAD/STS including Kyutai-family and Granite-era models | iOS 17+ | MIT. Installs from main; no per-model benchmarks. | https://github.com/Blaizzy/mlx-audio-swift |
| WhisperKit / argmaxinc/whisperkit-coreml | Whisper on Core ML | not fetched | 95.1M downloads on the HF list | https://huggingface.co/models?pipeline_tag=automatic-speech-recognition&library=coreml&sort=trending |
| moshi-swift (Kyutai) | MLX Swift for Kyutai STT | not stated | Kyutai says the 1B model "tested to work fine on an iPhone 16 Pro" (search snippet of https://github.com/kyutai-labs/delayed-streams-modeling; not verified on the page) | |
| ExecuTorch | Reverb-TTS ships ExecuTorch MLX programs | not stated | Weights non-commercial (see TTS) | https://huggingface.co/software-mansion/Reverb-TTS |

Not applicable to iOS 26: "Core AI" runtime builds (a community Voxtral conversion needs Core AI 1.0.0b2, macOS 27 / Xcode 27 beta; https://huggingface.co/harshav/Voxtral-Mini-4B-Realtime-CoreAI as reported by search).

The soniqo iPhone 16 Pro benchmark (iOS 26.5, release build, warm-up then median of 5; https://github.com/soniqo/speech-swift/blob/main/docs/benchmarks/ios-coreml.md) is the only multi-model iPhone table I found:

| Model | RTF | Peak memory |
|---|---|---|
| Parakeet-EOU 120M CoreML INT8 | 0.04 | 297 MB |
| Omnilingual 300M CoreML INT8 | 0.28 | 495 MB |
| Supertonic-3 99M CoreML | 0.15 | 956 MB |
| Kokoro-82M CoreML | 0.08 | 676 MB |
| FunctionGemma 270M CoreML ANE | 128 tok/s | 236 MB |

It gives no date and no A16 data.

---

## Job 1: streaming ASR, hands-free, audio playing

Nothing new clearly beats Parakeet EOU / Nemotron Unified on the evidence. Candidates worth a look:

| Candidate | Size / format | Numbers (class) | Licence | Integration | Verdict |
|---|---|---|---|---|---|
| Nemotron Unified int8, in your baseline family | Core ML (FluidAudio) | 2.83% WER, 0.10 s median wait on iPhone 16 Pro, against 3.37% / 0.10 s for Parakeet v3 in the same run (3P, https://github.com/lucharo/voice2text/issues/52) | nvidia-open-model-license vs Apache conflict on one FluidInference card | FluidAudio | Baseline. It is the best measured iPhone data point. |
| nvidia/nemotron-labs-unified-asr-0.6b | 0.6B; one checkpoint for offline and streaming, 25 languages | Offline 9.49 WER over 44 runs vs 10.55 for TDT v3; streaming 11.85 (1.12 s), 13.25 (0.56 s), 15.24 (0.32 s), 19.19 (0.16 s); English 4.68 offline, 7.87 at 0.16 s (MC, https://huggingface.co/nvidia/nemotron-labs-unified-asr-0.6b) | OpenMDW 1.1, "commercial/non-commercial" | The Core ML build is the voice2text "Nemotron Unified" | In your baseline family. Its latency/WER curve is the useful new fact. |
| Kyutai stt-1b-en_fr (MLX) | 1B; 0.5 s delay; semantic VAD | no WER on the card (MC) https://huggingface.co/kyutai/stt-1b-en_fr-mlx | CC-BY 4.0 | moshi-swift; speech-swift listed it in v0.0.29 | Worth one test. The semantic VAD is only in the Rust server per search (unverified), so the Swift path may lack it. 1B on 6 GB shares memory with everything else. |
| loom-ai-org/zipformer-small-streaming-loom | 35.5M; GGUF Q8 61 MB | 12.17% / 11.91% WER on 73 clips; upstream 8.15% on LibriSpeech test-clean; 640 ms chunks; RTF 0.12 on a 4-thread laptop CPU; 373 MB for a 120 s clip (MC; https://huggingface.co/loom-ai-org/zipformer-small-streaming-loom) | Apache-2.0 | GGUF only. For iOS you would use a sherpa-onnx zipformer instead. No iPhone data. | Smallest streaming model I found. Accuracy is clearly worse than Parakeet. Only useful if memory or heat dominates. |
| Voxtral Mini 4B Realtime | 4B | none on iPhone (search result) | | MLX Python or Core AI only | Not a phone model today. |

What to measure: WER with the speaker playing, first-partial latency, and a 10-minute heat soak on A16.

## Job 2: offline recognition, 30-90 s, word timestamps

| Candidate | Size / format | Numbers (class) | Licence | Timestamps | Verdict |
|---|---|---|---|---|---|
| Parakeet TDT v3 (baseline) | Core ML | 2.29% clean / 4.51% other WER, median wait 0.105 s, about 70x real time, 16 s first compile on iPhone 16 Pro (3P, voice2text issue) | CC-BY-4.0 | yes | Baseline |
| Phonon-2 (FermionResearch) | 164 MB; distilled from TDT v3; English; Core ML repo and MLX | 5.21% mean WER on the 7 English leaderboard sets (MC/LB, https://huggingface.co/FermionResearch/Phonon-2); speed only on M5 Air (174x) | CC-BY-4.0 model, Apache-2.0 code | not stated | In the voice2text list, "not done yet" on the phone. Smaller than TDT v3 (about 360 MB per that issue). |
| TranscribePsy-250M-EU-sub | 212 MB Core ML; 8 EU languages; iOS 18+ | 40 min of audio in 32.8 s; 10 min in 1.65 s on an iPhone 17 Pro Max; the maker's own comparison against Voz (39.9 s, 2.34 s); no WER (MC; https://huggingface.co/qvac/TranscribePsy-250M-EU-sub-coreml) | Apache-2.0 | not stated | Runs through parakeet.cpp. No WER, so it cannot be placed on the accuracy axis. |
| Canary-1B-v2 | 0.98B; 25 EU languages | 7.15 mean WER, RTFx 749 on a GPU (LB per card; https://huggingface.co/nvidia/canary-1b-v2) | CC-BY-4.0 | word and segment | FluidAudio v0.15.6 lists it as a beta engine. Heavy for A16. |
| Granite Speech 5.0 470M TurboCTC (IBM) | 470M; non-autoregressive CTC; 7 languages listed | English average WER 5.51% vs 5.78% for the English-only release; no RTFx stated (MC; https://huggingface.co/ibm-granite/granite-speech-5.0-470m-turboctc) | Apache-2.0 on the card (a blog reported an NC-SA variant; the card does not mention one) | not stated | GGUF/MLX community ports only. No Core ML found. Good accuracy-per-size on paper; unproven on iPhone. |
| Parakeet CTC 110M (FluidInference) | 436 MB; Core ML | 17.97% or 12.42% on Earnings-22 (the card contradicts itself); 358x on an M4 Pro (MC) | licence contradicts itself on the card | not mentioned | Not Pareto. |
| Omnilingual 300M | Core ML INT8 | RTF 0.28 and 495 MB on iPhone 16 Pro (soniqo benchmark) | Apache-2.0 | | Slow. Only for rare languages. |

To get word timestamps you could also add Qwen3-ForcedAligner (speech-swift lists it, 0.6B) to any text-only model. I have no numbers for it.

## Job 3: long-form news TTS

Open TTS Leaderboard, English, default view (WER on an H200; RTFx on an H200; https://www.creativeainews.com/articles/open-tts-leaderboard-license-cpu-test-2026/ — a secondary article, not the leaderboard page):

| Model | WER | Size | Licence | iPhone evidence |
|---|---|---|---|---|
| Kokoro-82M (baseline) | 1.46 | 82M | Apache-2.0 | RTF 0.08 and 676 MB on iPhone 16 Pro (3P, soniqo) |
| Supertonic 3 | 1.54 | about 99M | code MIT; weights OpenRAIL-M, which carries use restrictions | RTF 0.15 and 956 MB on iPhone 16 Pro (3P, soniqo); "2 minutes of audio in about 3 s on iPhone 17 Pro" (FluidAudio README, MC); 31 languages; the article reports 1,918.6 ms time to first audio on a GPU (a poor streamer) |
| pocket-tts (Kyutai) | 1.62 | 100M | CC-BY-4.0, credit Kyutai | CPU leaderboard time to first audio 272.7 ms, RTFx 1.19; on an author's Ryzen with 6 threads 69.3 ms and 5.02x (3P, same article). The FluidInference Core ML card has no iPhone numbers. About 200 ms to first audio and ~6x real time on an M4 Air CPU per Kyutai (MC; https://huggingface.co/kyutai/pocket-tts) |
| Phonon 31M (Gradium) | not on the table | 31M, Q8 GGUF 64.5 MB | Apache-2.0 | FluidAudio README: about 20x real time, about 171 MB download, iOS 18; HF card lists an iPhone Core ML path. About 20 ms to first audio on an M5 Pro (MC). No iPhone measurement found. |
| Reverb-TTS | 1.80 avg over 7 languages (MC) | 297M | **weights CC BY-NC 4.0** | iPhone 17 20.6x real time (ExecuTorch MLX fp16, to codec codes, decoder excluded) (MC) | Fast, but **not usable commercially** |

**Long-form and drift** is the real unknown. Every small model I read splits text at sentence level and loses prosody across sentences:
- Phonon 31M: "Very long inputs are split into sentences and spoken one after the other" (https://huggingface.co/gradium/phonon-31m-en).
- Pocket TTS Core ML: "on a whole paragraph several voices never emit EOS", so keep chunks to 50 tokens or fewer (https://huggingface.co/FluidInference/pocket-tts-coreml). Kyutai's own card says "Can handle infinitely long text inputs." The two statements conflict.
- Audio8-TTS-Realistic-Air: "split long passages" (0.6B, F32 llama.cpp bundle 3.76 GB, so not phone-sized).

Nobody I found publishes a long-passage drift measurement. That needs your own test.

Dominated or not usable: Qwen3-TTS Core ML (about 5.9 GB total, output capped at about 10 s per call; https://huggingface.co/FluidInference/qwen3-tts-coreml); KittenTTS 2 (ONNX CPU, 82M Apache-2.0, no quality or latency numbers stated; mini int8 82.7 MB, nano 17.4 MB per language); ZONOS3-Mini (Apache-2.0, about 100M active of 800M total, claims "runs in real time on phones", but no phone runtime or number; https://huggingface.co/Zyphra/ZONOS3-Mini); OmniVoice (CC-BY-NC weights); Cilla-mini (Persian only, CC BY-NC; commercial licence from $99); Inflect-Mini (Apache-2.0, English, 34M, only CPU Python numbers; https://huggingface.co/owensong/Inflect-Mini-v3).

## Job 4: VAD, end of turn, barge-in

| Candidate | Size | Numbers (class) | Licence | Runtime | Verdict |
|---|---|---|---|---|---|
| Silero VAD, Smart Turn v3.2 (baselines) | 309K, 8M | not re-measured | | Core ML in speech-swift | Baseline |
| p99lab turn-1-mini v2 | about 5.4M | about 2.6 ms CPU per 160 ms step; recall 92.2% / false alarm 3.8% on a 2-5 speaker multi-party bench; the bench the maker names (eot-bench) gives 22.6% false cut-offs at 300 ms (MC; https://huggingface.co/p99lab/turn-1-mini) | Apache-2.0; keep the NOTICE (AMI, ICSI, otoSpeech attributions) | speech-swift lists a Core ML build | English only. Maker's bench. Needs a head-to-head with Smart Turn. |
| VAP (otoSpeech, soniqo ONNX) | 5.8M | catches 83.8% of turn endings with 2.1% false alarms on 2 English conversations; Core ML 113 ms compute vs ONNX 378 ms (on an M5 Pro, not a phone) (MC; https://huggingface.co/soniqo/VAP-OTO-ONNX) | weights CC BY 4.0, code MIT | needs both the user and the agent channel | Forecasts speech activity. Useful for barge-in because it takes the agent channel as input. It "does not directly decide when an agent should answer." The card warns the numbers say nothing about phone performance. |
| ai-coustics vad-voice-focus | size not stated | "30 ms latency"; claims it beats Silero; "primary speaker" only (MC) | ai-coustics-model-license (not read) | aic-sdk (Python, Rust, C/C++, WASM) | Licence trap: unread custom licence. Interesting as a primary-speaker VAD. No iOS evidence. |
| Nemotron-3-Diarization | 100M | DER 12.73 (DIHARD III) at 30.4 s buffer, 13.55 at 0.32 s (MC) | OpenMDW 1.1, commercial OK | speech-swift Core ML/MLX INT8 | A diarizer, not a turn model. |

## Job 5: owner-only voice (speaker verification, personal VAD, target speaker)

| Candidate | Size | Numbers (class) | Licence | Verdict |
|---|---|---|---|---|
| WeSpeaker / CAM++ (baseline) | | | | Baseline |
| ReDimNet2-B6 Core ML (aufklarer) | 12.3M params, 28.6 MiB (a second card says 24.7 MiB) | paper: 0.287% EER on Vox1-O (MC; https://arxiv.org/pdf/2603.11841); Core ML: 13.6 ms per 6 s window on an M5 Pro; no EER on the Core ML card (https://huggingface.co/aufklarer/ReDimNet2-B6-CoreML) | MIT | Strong embedding candidate. The EER is the paper's, not the Core ML build's. Needs your own accuracy test. |
| nomo-pvad v1.1 (TuyaNomo) | 18.16M, FP16 36.6 MB | outputs one probability per 160 ms; accuracy and latency "in a technical report" and not on the card (none; https://huggingface.co/TuyaNomo/nomo-pvad) | Apache-2.0 | The only open personalised VAD I found. PyTorch on CPU; needs a Core ML conversion; needs at least 10 s of enrolment. |
| tse-conv-tasnet-48k (penta2himajin) | not stated | causal streaming target-speaker extraction at 48 kHz; needs a separate 192-d ECAPA embedding; trained on VCTK and DEMAND; "earlier releases had bugs" (via search only, card not read) | not read | A lead, not a candidate. Nothing else with open weights verified. |

I found no 2026 open-weights, real-time, enrolment-based target-speaker extraction model with an iOS runtime. Personal VAD (gate on or off) is realistic. Extraction is research.

## Job 6: echo cancellation and speech enhancement

| Candidate | Size | Numbers (class) | Licence | Verdict |
|---|---|---|---|---|
| LocalVQE v1.4-AEC (soniqo Core ML + host filter) | residual mask 707 KB (the bundle is 0.7 MB); base model 1.3M params | M5 Pro: Core ML mask 0.177 ms per 16 ms frame, hybrid summed estimate 0.413 ms (RTF 0.0258); AECMOS echo 3.455 passthrough to 4.047 hybrid; the card says the sum is not an integrated Swift measurement (MC; https://huggingface.co/aufklarer/LocalVQE-v1.4-AEC-200K-CoreML). FluidAudio README: near-end word recall 44.1% to 77.6%, far-end leakage 34.0% to 1.1%, on a synthetic AEC-Challenge subset it calls exploratory (MC) | Apache-2.0 | The best on-paper AEC for a phone: tiny, streaming, 16 ms hop, Apache, iOS 18+. Maker's numbers on a Mac. Unproven on real phone speaker/mic coupling. Compare against the iOS voice-processing I/O unit (not researched here). |
| DeepFilterNet3 Core ML (aufklarer / iky1e) | 2.1M | M2 Max RTF 0.13 (aufklarer); about 35x real time (iky1e, Mac); no iPhone number found (MC) | not read | Noise suppression only, not AEC. |
| Inworld voice-isolator | | | | Known; skipped |

## Job 7: small language model for short replies and routing

| Candidate | Size | Numbers (class) | Licence | Verdict |
|---|---|---|---|---|
| FunctionGemma 270M (Core ML ANE) | 270M | 128 tok/s, 236 MB on iPhone 16 Pro (3P, soniqo) | not read | Best fit for routing and tool selection on the evidence. Speed and memory are very good; quality for free-form replies is untested here. |
| Qwen3.5 0.8B (Core ML / MLX in speech-swift) | 0.8B | no phone number found | Apache-2.0 (https://huggingface.co/Qwen/Qwen3.5-0.8B) | Card warns thinking mode loops more than larger Qwen3.5 models. |
| Gemma 4 E2B 4-bit | 2.3B effective, 5.1B with embeddings | iPhone 17 Pro: Core ML/ANE 33 tok/s burst, 22 sustained over 10 min (67% retained); MLX GPU 48, then 18 (38%); LiteRT-LM GPU 56, then 27 (48%) (3P blog; https://dev.to/john-rocky/iphone-on-device-llm-the-gpu-wins-the-sprint-the-neural-engine-wins-the-marathon-42lo) | Apache-2.0 | Memory not stated; 5.1B with embeddings is risky on 6 GB. The sustained-speed result is the useful finding: the ANE path holds speed under heat. |
| LFM3-1.7B MLX 8-bit | 1.84 GB weights | no phone number (https://huggingface.co/LiquidAI/LFM3-1.7B-MLX-8bit) | LFM Open License v1.0 (not read) | "No mlx-lm release loads LFM3 yet". |
| spokenly/cleanup-1 | 0.6B fine-tune of Qwen3-0.6B-Base, 1.19 GB BF16 | no speed numbers | Apache-2.0 | A narrow transcript-cleanup model. Useful only if you need dictation cleanup. |
| Bonsai-27B 1-bit | 5.13 GB MLX pack | about 11 tok/s on an iPhone 17 Pro Max, "thermally limited" (MC) | Apache-2.0 | Needs far more memory than a 6 GB phone. Not for A16. |

Apple's Foundation Models framework shipped with iOS 26, but I could not confirm which devices support it (the search returned no device list). My memory says it requires an Apple Intelligence device, which an iPhone 14 Pro Max is not; unverified.

## Job 8: merged models (speech-to-speech, audio LMs, unified)

Nothing here is a real phone model today except the unified ASR family.

| Candidate | Reality on a 6 GB iPhone |
|---|---|
| Nemotron Unified ASR (offline + streaming + punctuation in one checkpoint) | Real. It has an iPhone 16 Pro measurement and is in your baseline family. |
| LFM3-Audio-2B (ASR, TTS, speech-to-speech in one model) | 2B BF16, 32K context, TTFT "as low as 50 ms", ONNX/GGUF/MLX 4-bit builds; no phone number on the card; speech-to-speech English only (MC; https://huggingface.co/LiquidAI/LFM3-Audio-2B). FluidAudio lists it. Demo-level for a 6 GB phone until measured. |
| PersonaPlex 7B | RTF about 0.94 on an M2 Max (soniqo); not a phone model |
| Nemotron VoiceChat 11B | Mac only |
| Hibiki Zero 3B | Mac only (MLX) |
| Moshi | Kyutai says the 1B STT works on iPhone 16 Pro; full Moshi on iPhone 15 Pro appears only on a vendor comparison page (https://inworld.ai/resources/best-speech-to-speech-model) |
| Gemma 4 E2B with audio input | Accepts audio per its card; no phone speech-quality data |

## Licence traps found

- Reverb-TTS weights: CC BY-NC 4.0 (https://huggingface.co/software-mansion/Reverb-TTS).
- OmniVoice weights: CC-BY-NC; "the pre-trained model is licensed under the CC-BY-NC due to training data (Emilia)" (https://huggingface.co/k2-fsa/OmniVoice).
- Cilla-mini: CC BY-NC 4.0, with a paid commercial tier.
- speech-swift catalogue: F5-TTS non-commercial; Higgs TTS 3 and Breeze TTS 2 research / non-commercial.
- Supertonic-3: OpenRAIL-M weights carry use restrictions that I did not read in full.
- NeuTTS Nano and some others: "commercial below a size threshold" per the secondary article.
- Pocket TTS, Parakeet family, Canary, Phonon-2, Kyutai STT: CC-BY 4.0; attribution required.
- google/medasr: gated, Health AI Developer Foundations terms.
- ai-coustics VAD: custom licence, not read.
- FluidAudio TTS product includes GPL ESpeakNG.
- nemotron-speech-streaming Core ML card contradicts itself (NVIDIA Open Model License vs Apache-2.0); so does the Parakeet CTC 110M card (CC-BY-4.0 vs Apache-2.0). Check the upstream licence, not the conversion.
- Voz: "Desert Ant Labs Source-Available License"; not open (https://huggingface.co/desert-ant-labs/voz).
- Granite 5.0 470M: one blog mentions a non-commercial variant; the card I read says Apache-2.0 only.

## Five experiments on the iPhone 14 Pro Max, in order

All on the real A16 device, with the same 10-minute thermal run for each, recording peak memory (`phys_footprint`), RTF and heat state.

1. **AEC in front of streaming ASR (LocalVQE v1.4 Core ML + host adaptive filter).** Play news TTS from the speaker and talk over it, with and without LocalVQE, against the iOS built-in voice processing. Win: at least a 3x reduction in word errors caused by the app's own speech, with under 5 ms per 16 ms frame (RTF below 0.3), no audible gating, and no more than 1 C extra heat rise. This is the experiment most likely to change the hands-free product, because it fixes the stated use case rather than swapping a model.
2. **Owner-voice gate: ReDimNet2-B6 Core ML embedding + nomo-pvad (converted).** Enrol with 10 s of speech, then test with a second voice at the same loudness, on playback and in a car or kitchen. Win: false accepts of a non-owner under 2% at a miss rate for the owner under 5%, with a total cost under 10 ms per 160 ms step on A16. If nomo-pvad fails conversion, use embedding similarity alone.
3. **Long-form TTS bake-off on A16: Phonon 31M vs Kokoro vs Supertonic-3 vs Pocket TTS (all Core ML), a 10-minute article, sentence-chunked.** Measure time to first audio, RTF, peak memory, and drift (listener or ASR round-trip WER across minute 1 vs minute 10; the seam between sentences). Win: a model with time to first audio under 300 ms, RTF under 0.2 on A16, peak memory under 400 MB (Kokoro's 676 MB and Supertonic's 956 MB are the bars to beat), and round-trip WER no worse than Kokoro's.
4. **Offline 30-90 s with timestamps: Phonon-2 (Core ML) and Granite 470M TurboCTC (GGUF/MLX; needs an aligner) against Parakeet TDT v3 on your own recordings.** Win: within 0.5 absolute WER points of v3, at least 1.5x faster or 40% smaller, and word timestamps within 100 ms of v3's. Phonon-2 is the likelier winner; Granite is likelier to fail on integration (no Core ML, no timestamps stated).
5. **Turn and barge-in plus a router: turn-1-mini and VAP against Smart Turn v3.2, and FunctionGemma 270M as the router.** Use real recordings with the agent speaking (barge-in) and trailing pauses. Win for the turn model: fewer false cut-offs at an equal latency (for example under 10% at 600 ms on your data, compared to Smart Turn). Win for the router: 100+ tok/s and under 300 MB on A16 with at least 95% correct route on your own list of 200 spoken requests.

## What I could not verify

- Any A16 or iPhone 14 number for any model; scaling from A18 is a guess.
- The Open ASR leaderboard table and the Open TTS leaderboard page itself (only secondary reports and model cards).
- Whether SpeechAnalyzer runs on iPhone 14 Pro Max (a developer report says iPhone 14 Pro works) and its current WER on iPhone.
- The ios-coreml.md benchmark's date.
- Apple Foundation Models device support.
- Licence text for Supertonic-3 (OpenRAIL-M restrictions), ai-coustics, LFM Open License, FunctionGemma.
- Kyutai's iPhone claim on the repo page itself; moshi-swift and its model format.
- Any long-form TTS drift data.
- Any open-weights target-speaker extraction with an iOS runtime.
- WER for TranscribePsy, Supertonic-3 on iPhone, Phonon 31M on iPhone, and the Voz iPhone figure (the page named an "iPhone 18 Pro", which I did not check).
