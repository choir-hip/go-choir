# Inworld Voice Isolator and SAVA, read against the Autoradio app

Read 2026-11-22. Shallow clones in
`outside/vi` and `.../sava`
(read only, nothing run). Paths below like `vi/...` and `sava/...` are inside those clones.
HF file sizes come from the Hugging Face API (`https://huggingface.co/api/models/inworld/voice-isolator?blobs=true`
and `.../inworld/sava?blobs=true`). Anything not in a source is marked "not stated" or "EXTRAPOLATION".

## 1. Voice Isolator (v1)

### What it is
- Voice isolation (denoising) for 48 kHz speech: removes noise, music, competing voices and the late room tail,
  keeps the foreground voice. "They are not a speaker selector or a restoration model."
  Source: `vi/README.md` TL;DR; `vi/MODEL_CARD.md` "Two checkpoints".
- Two checkpoints sharing one architecture (`vi/README.md` TL;DR table):
  - `voice-isolator-v1`: offline, every frame sees its whole 8 s chunk, long inputs run in 8 s chunks overlapping 2 s
    (`vi/MODEL_CARD.md` "Two checkpoints"). 64.0M parameters.
  - `voice-isolator-v1-realtime`: causal stream, trained look-ahead 0, 20 or 40 ms; algorithmic latency 63 ms at 20 ms
    look-ahead (`vi/MODEL_CARD.md`). 58.2M parameters.
- I/O: input one channel at 48 kHz ("nothing is separated or synthesized beforehand", `vi/MODEL_CARD.md` Architecture).
  Output: float32 WAV 48 kHz, original levels preserved; the "background" file is `input - voice`; together they
  reconstruct the aligned mono input (`vi/README.md` Quick start). `--output-sr 0` keeps the input rate. Other input rates are
  resampled by the package (the `--output-sr` flag implies it; the exact resampler is not stated in the README).
- Streaming: `session.push()` takes 48 kHz mono in pieces of any size, 20 ms (960 samples) in the example (`vi/README.md`).

### Architecture
Band-split dual-path network on the STFT (n_fft 2048, hop 480 = 10 ms), 46 bands, token width 512, seven blocks:
four with time attention (rotary, "exclusive self attention") plus attention across bands; three with Mamba along time
then across bands (state 16). Per-band heads output a bounded complex mask and a bounded per-bin residual added to the
input spectrum. Source: `vi/MODEL_CARD.md` "Architecture".
- Masking/filtering, not a vocoder: output is an STFT-domain mask plus bounded residual on the input spectrum, not
  re-synthesis from tokens or a generative decoder. (My reading of the card's words; the card says "invents nothing" as the
  objective, `vi/MODEL_CARD.md` "Training data".) The residual term does allow adding energy, so it is not a pure mask.
  The bench itself warns that "natural-sounding re-synthesis" systems exist on the board (`vi/bench/METRICS.md:56-57`);
  it does not say Inworld's is one.

### Size and formats published
- HF repo files: `denoiser.safetensors` 256,260,128 B (offline, 64M x fp32) and `denoiser_realtime.safetensors`
  232,780,800 B (realtime, 58.2M x fp32). Source: HF API above. The repo has nothing else.
- Formats: PyTorch/safetensors only. No ONNX, Core ML, GGUF, LiteRT, MLX or ExecuTorch published (HF file list; `vi/README.md`).
  Runtime: Python >= 3.10, PyTorch >= 2.1; CUDA, Apple `mps` (PyTorch >= 2.6) and CPU; optional CUDA Mamba kernels
  (`vi/README.md`). Precision flag fp32/fp16/bf16 exists (`vi/src/inworld/voice_isolation/cli.py:140`, `isolator.py:31`);
  "auto" is bf16 on CUDA, fp32 elsewhere (`isolator.py:32-33`).
- Phone: none. README Plans: "On-device support (beyond Apple silicon)" and "Testing on smaller GPUs" are unchecked
  (`vi/README.md` Plans). "Hardware: tested on A100, H100, B200, B300 and on CPU" (`vi/README.md`). A maintainer said
  "It is not a super-tiny nnet, so feel free to experiment with quantizing / pruning" (GitHub issue 5 comment,
  https://github.com/inworld-ai/voice-isolator/issues/5).
- A community project plans an ONNX export of the streaming step "for a native Mac app" (issue 6,
  https://github.com/inworld-ai/voice-isolator/issues/6). Not delivered as of the read.

### Licence
Code and weights Apache-2.0, commercial use allowed by that licence (`vi/README.md` License; HF tags `license:apache-2.0`).
Training data includes vendor-purchased data "under private terms" and CC BY / ODC-By sets (VocalSet, IndicVoices-R, VCTK)
(`vi/NOTICE`). Attribution of those is credited in NOTICE; the Apache grant on the weights is as stated. Whether any
dataset term constrains the weights downstream is not discussed (not stated).

### Quality numbers stated
- Only the Inworld VIBE benchmark, scored by Inworld on its own non-distributed set (5,502 items, 77 conditions, 13 languages
  paired + 508 real recordings in 128 languages). Inworld "maintains both the benchmark and one of the models on it, and
  chose that model's checkpoints with these scores in view" (`vi/bench/README.md` top and "The eval set").
- Denoising board scores (0-1): Inworld offline 0.768, Inworld realtime 0.718, ClearerVoice MossFormer2_SE_48K 0.695,
  resemble-enhance 0.646, FastEnhancer-L 0.589, DeepFilterNet3 0.510 (`vi/docs/assets/board-light.svg`). Gaps under 0.03 are ties.
  The composite is built from suppression, music, cleanup, events, voices, preservation, fidelity, timbre, NISQA quality
  (`vi/bench/README.md` "What the scores mean"). The per-axis numbers are not in the repo (set and score files not distributed).
- Downstream ASR: the bench defines it (Whisper large-v3 via faster-whisper, beam 5; Whisper small; Inworld's own STT API)
  and says "a robust recognizer needs little help above 0 dB, and every system that removes sound can remove words"
  (`vi/bench/METRICS.md:285-297`). **No WER numbers are published in the repo or the model card**; the table is generated by
  a script from files that are not distributed. So the authors' ASR benefit is not checkable.
- Content check: a dropout under about 320 ms is not visible to Whisper large-v3 because it restores the word from context
  (`vi/bench/METRICS.md:315`, `vi/bench/CONTROLS.md:31`). That is a measurement caveat, not a result.
- Real-time factor, third party only: MacBook Air M4, PyTorch 2.15 on MPS, realtime model RTF 0.53 at 100 ms pushes
  (single pushes up to 2.9x), offline model RTF 0.32 per window; CPU RTF 7.5 and 4.7 (order of the two CPU numbers
  presumably realtime then offline, not labelled) (issue 6). A lower RTF means faster; CPU 7.5 means 7.5 s of compute per
  1 s of audio.
- No speed or memory figure from the authors. The demo says live use "needs a device that processes audio faster than it arrives"
  (`vi/demo/README.md`). `vi/bench/scripts/speedtest.py` exists but no results are published.
- Memory, third party: issue 5 "Offline FP32 CPU: bundled 10.77s demo exhausts 32 GiB on Windows" (PyTorch 2.8 CPU, Ryzen 5
  5600); the 3 s clip works (https://github.com/inworld-ai/voice-isolator/issues/5). The maintainer's reply: not tested beyond
  the README's machines. This is the only memory number anywhere, and it is a failure.

### Maturity
Repo created 2026-11-16, last push 2026-11-18, one commit visible in the shallow clone; HF repo created 2026-11-14, 629 likes
(HF API); GitHub 282 stars, 23 forks, 2 open issues and 3 open perf PRs from one outside contributor (gh API, issues list).
v1.0.0. Version 2 (restoration, one model offline and streaming) is only a plan (`vi/README.md` Plans).

### Known limits (stated)
- Voices: the louder voice is kept, one a few dB quieter is removed; voices at similar level are usually both kept
  (`vi/MODEL_CARD.md`). So it is not a way to pick the owner out of two similar-level talkers.
- Music: removed as background (training mix includes music beds; bench axis `music`), but singing is kept as voice
  (training includes singing, `vi/MODEL_CARD.md`; bench "voice policy" rows `singer`, `music_vocal`).
- Far-field, reverberation: v1 "removes ... the late room tail" per the card's objective, yet the bench contract says keep "its own
  room" and a system that leaves the room scores zero on the restoration axis (`vi/MODEL_CARD.md` Training data vs
  `vi/bench/README.md` Task contracts). The two statements are not fully consistent; how much reverb it removes is not stated. Real-room
  restoration is on the v2 plan. Far-field speech as such is not stated.
- Band-limited input: it "restores nothing the recording lost, such as missing bandwidth" (`vi/MODEL_CARD.md`).
- Heavily compressed narrowband audio: a third-party report found the realtime model suppressed the voice (-38 dB) on one
  compressed SSB recording until bands were cut; the offline model kept it (issue 6). One recording, anecdotal.
- Whispered speech is scored for retention only, outside both scores (`vi/bench/METRICS.md:160`).
- Artefacts that hurt recognition: the repo names the risk but measures nothing public (see above).

## 2. SAVA

### What it is
Streaming voice activity plus end-of-turn detector, one model, two probabilities per 40 ms frame: `speech` and `turn`
(`sava/README.md` TL;DR).
- Input 16 kHz mono (other rates and channel counts converted on the way in, `sava/docs/USAGE.md`).
- Output: two probabilities every 40 ms. Speech probability for a frame is emitted 120 ms later (3-frame look-ahead);
  end-of-turn probability has no look-ahead (`sava/README.md`).
- 25.5M parameters. Front end causal log-mel (128 bins, 25 ms window every 10 ms), mixer to 512, three Mamba-2 blocks plus one
  attention block (512-frame causal window), a VAD head and an end-of-turn head with four horizon columns (0, 200, 400, 800 ms;
  column 0 released) (`sava/MODEL_CARD.md` Architecture).
- It uses "acoustic and sentence context" to separate speech from coughs, laughs, music, keyboard (`sava/README.md`). It is
  audio-only: no ASR text is an input in the architecture table (`sava/MODEL_CARD.md`).
- Reference turn policy: turn opens on first speech frame and starts 800 ms earlier; closes when end-of-turn prob crosses 0.9
  after the speaker goes quiet, or after 1.6 s silence; turns under 400 ms of speech dropped (`sava/docs/USAGE.md`).

### Size, formats, licence, maturity
- `sava.safetensors` 51,051,668 B (bf16, loaded fp32 on CPU) (HF API; `sava/docs/USAGE.md` "Weights 51 MB"). PyTorch only: no ONNX,
  Core ML, GGUF, LiteRT, MLX, ExecuTorch published; no phone build or example (HF file list; `sava/README.md`).
- Apache-2.0 code and weights (`sava/README.md`). Training data NOTICE: pipecat smart-turn data v3.2 (CC BY 4.0), VoxConverse,
  VocalSound (CC BY-SA 4.0), Tatoeba, FineWeb-2, C4 as text for synthetic speech, plus vendor data (`sava/NOTICE`, first 30 lines read).
  The CC BY-SA 4.0 VocalSound licence's effect on weights is not discussed (not stated).
- Created 2026-11-14 (HF), last push 2026-11-18, 53 stars, no issues listed (gh). About 5,000 hours of training audio
  (`sava/MODEL_CARD.md`). Recipe, code and data not released.

### Speed and hardware
Apple M3 Max CPU, fp32: one stream 3.8 ms per 40 ms frame; a 12 s recording in 0.04 s; A100 hundreds of times real time
(`sava/docs/USAGE.md` tested configurations). Memory beyond the 51 MB weights: not stated. Phones: not stated.

### Stated quality (all `sava/bench/README.md`, public datasets, public baselines, bench code and result JSONs in repo)
- Voice activity frame AUC vs Silero VAD 6.2.1: TEN-VAD 0.9769 vs 0.9541; VoxConverse curated 0.9807 vs 0.9311; VoxConverse
  diarization labels 0.9555 vs 0.9520; AISHELL-4 0.9327 vs 0.9350 (tie); AliMeeting 0.9376 vs 0.9635 (SAVA worse, CI excludes 0);
  AVA-Speech 0.9188 vs 0.9041; Earnings-21 0.9722 vs 0.9656; LibriParty 0.9670 vs 0.9572; MSDWild 0.9329 vs 0.9148.
  Caveat stated by the authors: SAVA uses 120 ms of look-ahead, so the comparison is not latency-matched.
- Noise (ESC-50, no speech): clips without a false trigger SAVA 97.3% vs Silero 93.2%; frames above 0.5: 0.32% vs 0.57%.
  The README still warns "Quiet speech-like noise can trigger SAVA" (`sava/README.md`, `sava/docs/USAGE.md` limits).
- End of turn vs smart-turn v3.2: pipecat test accuracy at 0.5 0.9335 vs 0.9371 (AUC 0.9795 vs 0.9825) = parity.
  Easy Turn raw: SAVA AUC 0.5119 (chance) vs 0.8266; with +200 ms noise anchor SAVA 0.7923 vs 0.8396; SAVA calls 92% of
  "incomplete" clips ended in the anchored read (false alarm 92.3%). FastTurn: 0.6480 vs 0.6211. eot-bench (14 languages):
  AUC 0.8157 vs 0.8616 full trace. AMI real turn ends: fired by +200 ms 85.5% vs 30.8%, latency p50/p90 71/192 ms vs 210/930 ms.
  So SAVA is fast at ends but its end-of-turn is not clearly better and is weaker on incomplete-sentence cases; it is
  very sensitive to trailing context.
- Not measured anywhere: echo of a device's own speaker, a TV in the room, hands-free on a phone, and any mobile run.

### Limits (stated, `sava/docs/USAGE.md`)
It responds to any voice, no target speaker ("SAVA responds to any voice. It does not pick out a target speaker"); speech verdict lags
120 ms; turn head is conversational, no model of interruptions or backchannels; hears quiet and distant speech by design, so a far
television or a ventilation drone can trigger it.

## 3. What the app does today (code)

- Mic: `AVAudioEngine`, input tap resampled to `busSampleRate = 16_000` mono (`mobile/AutoradioKit/Sources/AutoradioKit/Audio.swift:19`,
  `LiveAudio.swift:349-356`). Reference of the radio's own output is also tapped at the mixer (`LiveAudio.swift:366-370`).
- Echo: Apple voice-processing I/O is turned on with `engine.inputNode.setVoiceProcessingEnabled(vp)` (`LiveAudio.swift:343-347`),
  and the coordinator uses it for the `.talking` state; `alwaysListening` is the lab choice with no voice processing
  (`SessionCoordinator.swift:110-117`; `docs/design/autoradio-on-device-voice-configurations-2026-11-11.md:395`). The turn policy is told
  whether echo cancellation exists: without it, it "needs three words and ignores the answer's own words" (`HandsFree.swift:20`, `:66-67`);
  the radio's own words never count as a cut-in (H3, `HandsFree.swift:15`); two new words plus 300 ms of speech cut in (`HandsFree.swift:6-7`).
- Voice activity: an energy VAD stage drives the policy in hands-free (`HandsFree.swift:3-4`); Silero is a registered stage (measured on A16,
  `docs/reference/ON-DEVICE-MODELS.md:12`). End of turn: no Smart Turn model yet; `fallbackSilenceMs = 1000` (`HandsFree.swift:56-58`).
  Parakeet EOU's `<EOU>` is a candidate input (`Voice/TurnPolicy.swift:17-19`).
- Recognition: on-device `SFSpeechRecognizer` (`AppleStages.swift:23-29`) plus Parakeet in the lab; takes are transcribed after recording by
  the same stage fed 16 kHz mono frames (`TakeSession.swift:236-245`).
- Speaker: WeSpeaker ResNet34-LM (Core ML, 7.95 MB) and CAM++ embeddings against a profile learned from the person's first turns;
  thresholds fitted on LibriSpeech, not verified on the phone microphone (`Voice/SpeakerProfile.swift:25-42`; `ON-DEVICE-MODELS.md:52`).
  A speaker check gates a cut-in (`Voice/TurnPolicy.swift:46-50`). This is verification of a stretch of speech, not extraction.
- Takes: `AVAudioRecorder` to AAC-LC `.m4a`, mono, 24 kHz, 32 kbit/s (`TakeSession.swift:211-218`, `Choir/Takes.swift:80`); 3 to 60 s
  (`Choir/Takes.swift:71`); the radio is paused for the whole take (R1, `TakeSession.swift:9-10`; although the coordinator comment at
  `SessionCoordinator.swift:114` calls the take state "capture and playback": not reconciled here). Server accepts at most 512 KB, `ftyp` present,
  header length 2.5 to 62 s (`docs/reference/AUTORADIO-TAKES.md` "Bounds"); publish is a direct multipart upload with the transcript;
  the computer-side copy is not built (same page, "Where this differs"). A take is dedup-keyed by sha256 of the audio
  (`AUTORADIO-TAKES.md` "Records").
- Not in the repo evidence: any noise suppression or enhancement model other than Apple's voice processing; no Android native audio
  processing path was read (Exact on Android is shared code, `docs/current-architecture.md:142`).

## 4. Answers

### a. Takes
**Where it could run.**
- Because a take is recorded to AAC 24 kHz 32 kbit/s mono, the file is already band-limited to 12 kHz and lossy
  (`TakeSession.swift:211-218`). The isolator wants 48 kHz and restores no bandwidth (`vi/MODEL_CARD.md`). Enhancing the AAC file
  (decode, resample to 48 kHz, enhance, re-encode) would be a second lossy generation; better is to record PCM, enhance, then encode once.
  That needs a recorder change (an orange change: the recorder is `AVAudioRecorder` writing AAC directly).
- On the phone: no published build (HF file list; README Plans). Not possible today without exporting the model. Mamba selective
  scans and rotary attention over 46 bands x 800 frames are not a trivial Core ML, LiteRT or ONNX export (EXTRAPOLATION: no export exists
  to inspect; the community ONNX plan in issue 6 is for the streaming step and unfinished).
- On the server (the listener's computer, which holds the take before publish in the design; today the phone uploads directly to
  the proxy, `AUTORADIO-TAKES.md`): feasible with PyTorch, but the VM's hardware is not stated in what I read. CPU speed is the third-party figure
  RTF 4.7 (offline, if the order I assumed is right) on an M4 Air, i.e. about 4.7 min for 60 s and 7 min for 90 s on a laptop-class CPU, before any VM penalty.
  A GPU, where the model was tested, would be fast but we have none stated.
- Time and memory on the phones (EXTRAPOLATION, all of it; no number in either source is for a phone):
  - M4 Air GPU (MPS), offline: RTF 0.32, so 30 s = 10 s, 60 s = 19 s, 90 s = 29 s (issue 6 figure times duration).
  - iPhone 14 Pro Max (A16, 6 GB): no measurement exists. The GPU has fewer cores than an M4 and the PyTorch mobile runtime is not available for
    this model, so the practical answer is "not runnable"; if exported and run on the GPU, I would expect several times the M4 figure (tens of
    seconds to minutes for 60 s). That multiplier is a guess, to be replaced by a measurement.
  - Snapdragon 6 Gen 1, 3.4 GiB free RAM: weights alone are 256 MB fp32 (128 MB fp16, about 64 MB int8, arithmetic, not published); the only memory
    datum is issue 5 (32 GiB exhausted on a 10.77 s clip, fp32 CPU). Even if that is a CPU-path quirk and an exported graph is much smaller, I would not expect
    this phone to run the offline model in useful time; CPU at RTF 4.7 on a laptop implies many times real time on a 6-series core. Treat as no.
  - Memory in general scales with the 8 s chunk (fixed), so a 90 s take costs time, not more memory, if chunked (`vi/MODEL_CARD.md`: 8 s chunks, 2 s overlap).
    Peak resident memory on any device: not stated.
- Fidelity: it is a mask-and-bounded-residual network on the input STFT, not a vocoder (see Architecture), and its training objective is
  "return the voice as it was recorded ... invent nothing" (`vi/MODEL_CARD.md`). So it cannot make up different words the way a generative codec
  can; it can delete or dull soft consonants and breaths. The repo names the risk: "every system that removes sound can remove words"
  (`vi/bench/METRICS.md:296`) and has a timbre axis and a speaker-similarity metric but publishes no numbers. Speaker-similarity per system is not in the repo. The
  take must be tested with our own speaker embedding (below). The offline model and the realtime model give "slightly different results" (`vi/demo/README.md`).
- Product fit: for a take, the radio is already paused (R1), VPIO is not used for the take, so what contaminates a take is room noise, wind and other talkers, which
  is what this model is built to remove. The "remove the radio" use case does not arise for takes if R1 holds.
- Honest-report rule: a take's `sha256` and transcript are made before publish and the transcript is not editable
  (R4); enhancing after transcribing means the transcript is of the raw audio; the choice (transcribe raw or enhanced) is a design decision, not a model fact.

### b. Before recognition
**What evidence exists.**
- Inworld: the benchmark has a downstream-ASR view but no numbers are published (`vi/bench/METRICS.md:285-297`); its text says robust
  recognisers "need little help above 0 dB" and that Whisper small gains more. Nothing about Apple's recogniser or Parakeet.
- Third party (found by web search; I read only the search summaries, not the papers, so treat as pointers):
  - arXiv 2603.04710 (2026): Meta's SAM-Audio enhancement raised WER/CER for five Whisper variants on noisy Bengali YouTube and an English noisy corpus,
    worse for larger Whisper models. https://arxiv.org/abs/2603.04710
  - arXiv 2512.17562 (Dec 2025): MetricGAN-plus denoising degraded WER across Whisper, Parakeet, Gemini Flash 2.0 and another on 500 medical
    recordings in nine noise conditions. https://arxiv.org/abs/2512.17562
  - Counter-evidence: enhancement trained jointly with, or adapted to, the recogniser helps (arXiv 2406.18928, front-end adaptation network
    for packet loss with a frozen Whisper, https://arxiv.org/pdf/2406.18928). A paper on switching between raw and enhanced input exists because extraction can hurt (arXiv 2106.00949).
  - These are generic enhancers, not this model; no paper on Inworld's was found. The pattern: perceptual gains do not carry to WER for
    recognisers trained on noisy audio; the gain, if any, is at low SNR, with interfering talkers, and for weaker recognisers.
- Our recognisers: SFSpeechRecognizer on-device and Parakeet are not stated to be noise-robust to any particular degree in our docs; "recogniser's weakness
  is the owner's stated concern" is the brief's statement, I did not find a doc quantifying it. The on-device page records no WER for any of our stages
  (`ON-DEVICE-MODELS.md:54` "unmeasured").
- Versus what the phone does now: Apple voice processing does echo cancellation plus noise suppression and AGC in the OS, on, only in `.talking`
  (`LiveAudio.swift:343-347`, `SessionCoordinator.swift:117`). Its reference signal is the radio's actual output, which a pure single-channel model cannot
  see: for the radio's own voice the AEC is the right tool, with a known reference, and the isolator would treat the radio as "another voice or noise" only if
  it is quieter or different; per `vi/MODEL_CARD.md` the louder voice is kept, so a radio louder than the owner at the mic could be the voice kept. That is
  an inference from the stated rule and must be tested. They are complementary in order: VPIO (AEC) first, isolator second, not isolator instead.
  Also stacking cost: the isolator on top of VPIO output means two nonlinear processors in series; whether that hurts is the experiment.
- Target-speaker: neither project does target-speaker extraction (both say so: `vi/README.md` "not a speaker selector"; `sava/README.md` "does not pick out a
  target speaker"). A target-speaker approach (enrolled embedding conditions the extractor, as in VoiceFilter/SpeakerBeam lines; search summary of arXiv 2308.11380 reports WER 80% to 26.4% on overlapped
  speech, 14.5% with joint tuning) addresses the TV/other-person case our `SpeakerProfile` only gates after the fact. We have the embedding and a verifier, not an extractor.
  Isolator only helps there if the interferer is quieter than the owner.

### c. SAVA in our terms
- It is a replacement candidate for the VAD stage and for the missing end-of-turn model, in one 51 MB (25.5M) network, at 16 kHz mono which is our bus rate.
  Today: energy VAD or Silero (stage), no Smart Turn, 1000 ms fixed silence fallback (`HandsFree.swift:56-58`). SAVA's AMI numbers (end-of-turn fired by +200 ms 85.5%,
  p50 71 ms) suggest it can cut the dead second; its end-of-turn is weak on incomplete sentences (Easy Turn AUC 0.51 raw, 0.79 anchored), so a mid-sentence pause risk
  remains; keep the 0.9 threshold policy and our `maxTurn` bound.
- It complements, does not replace: addressing (no target speaker; responds to any voice, so a TV or the radio leaking through still opens a turn),
  speaker verification (WeSpeaker/CAM++ stays), recognition (SAVA produces no words), echo handling (VPIO). It may partly reduce false opens from coughs, keyboard, music
  (stated), and it does not remove the need for the policy's "words not in the sentence being heard" rule (H3).
- Unknowns: behaviour on a phone mic with the radio playing; no phone runtime (PyTorch only; Mamba-2 conversion unknown); SAVA's own 120 ms speech look-ahead adds to
  barge-in latency when used for speech onset (use the zero-look-ahead `turn` head only for end).

### d. Cheap experiments
Common rules: precommit the claim and threshold in `docs/calibration/claude-ledger.md` before running (CLAUDE.md "Proof"); run on the Mac in a lane; do not
build anything on the phone first.

**Experiment 1: take cleaning (isolator).**
- Recordings: 24 takes, 30 to 90 s, 4 speakers (including the owner), 6 conditions each time: quiet room, kitchen noise, street, car, TV speech 10 dB under the voice,
  room with reverb. Record each simultaneously as lossless PCM at 48 kHz (reference of what the mic heard) and through the app's exact `AVTakeRecorder` settings
  (AAC 24 kHz 32 kbit/s). Record a close-mic or quiet-room read of the same script as the clean reference where possible.
- Arms: raw AAC; isolator offline on the PCM then encoded the same way; isolator on the decoded AAC; realtime model on PCM.
- Metrics: (1) WER and deletion rate against the script with the app's on-device recogniser, and Whisper large-v3 as a fixed reference; (2) WeSpeaker cosine (the app's
  own model) of each take against the clean reference and against the owner's profile; (3) blind A/B with 5 listeners: preference and "is this the same person"; (4) encoded size
  and the 512 KB cap; (5) Mac wall time and peak RSS (`/usr/bin/time -l`) on MPS and on CPU limited to 2 threads, per take length.
- Win: deletions do not rise (paired bootstrap on utterances, CI includes 0 or below); speaker cosine to the clean reference >= raw's in every noisy condition;
  listeners prefer enhanced in >= 70% of noisy conditions and rate it "same person" >= 95%. Lose: any condition where the recogniser's deletions rise by more than the CI or any "different person"
  rating. Phone step only if it wins on the Mac and the Mac time at 2 threads is under about 2x real time: then export attempt (ONNX/Core ML) and measure on the iPhone 14 Pro Max and
  the Snapdragon 6 Gen 1; if no export works, the answer is server-side.

**Experiment 2: enhancement before recognition (isolator).**
- Recordings: the app's kept turns (`mobile/AutoradioKit/Sources/autoradio-models`, `KeptTurns.swift`), re-recorded or mixed with a hand-labelled transcript: owner utterances with
  the radio playing at three levels, a TV, kitchen noise, a second person; each both with VPIO on (as `.talking`) and off. At least 200 utterances, labelled by hand.
- Arms: raw; VPIO only; VPIO + isolator realtime; VPIO + isolator offline; raw + isolator; and a 50/50 mix of enhanced and raw (EXTRAPOLATION: mixing back is a
  common mitigation, not tested by Inworld). Add a target-speaker proxy arm: isolator, then reject segments whose WeSpeaker cosine to the owner profile is below the app's 0.35 threshold.
- Metrics: WER and deletion rate with SFSpeech on-device and Parakeet, per condition; cut-in false positive rate when the radio plays; paired bootstrap CI.
- Win: relative WER reduction >= 10% with CI excluding 0 in the radio/TV conditions and no loss above 2% relative in the quiet condition, for at least one
  recogniser we ship. Otherwise drop the idea for recognition. If it wins, the next test is streaming latency: realtime model 63 ms algorithmic latency, measured RTF on the target phone, before any hands-free use.

**Experiment 3: SAVA.**
- Recordings: the same kept turns plus 60 minutes of hands-free session audio with the radio playing (include no-speech stretches of TV and the radio's own voice).
- Arms: current stack (energy VAD or Silero, 1000 ms fallback) vs SAVA speech and turn curves replayed through `TurnPolicy` via `autoradio-replay`
  (`mobile/AutoradioKit/Sources/autoradio-replay/main.swift`), run first in Python on the Mac.
- Metrics: false turn-open rate per hour on no-owner audio (radio leakage, TV); premature-cut rate (turn closed before the owner's last word, from labels); end-of-turn latency p50/p90
  from the owner's last word; barge-in latency to the 300 ms and 2-word rule.
- Win: premature-cut rate no higher than today's, false-open rate no higher, and p50 end-of-turn latency under 600 ms against today's roughly 1000 ms. Then measure on phones
  after a Core ML or LiteRT conversion of the 25.5M network; CPU at 3.8 ms per 40 ms frame on an M3 Max (a 10x slower phone core still leaves headroom, EXTRAPOLATION).

## 5. What I could not verify
- No stated speed, memory, WER or phone figure from Inworld for the isolator; all speed numbers are one third-party M4 Air report (issue 6), the order of its CPU numbers is
  presumed, and the memory failure is one issue (issue 5). Nothing was run.
- The isolator's ASR results are referenced by the repo but not published. VIBE composites are Inworld-scored on an undistributed set by the maker.
- The third-party ASR papers were read through search summaries only (arXiv 2603.04710, 2512.17562, 2406.18928, 2308.11380, 2106.00949); I did not open them.
- Whether the isolator's mask can leave a louder radio in place: inferred from "louder voice is kept", not tested.
- Whether a take is recorded with the radio audibly playing (R1 pause versus the `SessionCoordinator.swift:114` comment).
- What hardware the user's computer (server) has; Snapdragon 6 Gen 1 and A16 performance of either model; Core ML or LiteRT convertibility of Mamba blocks.
- SAVA: first 30 lines of NOTICE and benchmark tables read; the end of `sava/bench/README.md` (reproduction) and the complete NOTICE not read. Multilingual training coverage of SAVA is not stated except the 14-language eot-bench.
