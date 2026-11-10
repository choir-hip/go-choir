# Dual-process designs: a fast decider beside a slow reasoner

Reference subpage (convention: [../README.md](../README.md)). Siblings:
[model-space.md](model-space.md), [calibration-and-cascades.md](calibration-and-cascades.md),
[evaluation.md](evaluation.md). Product context:
[../../design/autoradio-turns-2026-11-10.md](../../design/autoradio-turns-2026-11-10.md)
(sections "Early structure lives in questions" and "A model on the phone"),
[../ON-DEVICE-MODELS.md](../ON-DEVICE-MODELS.md). Fetched 2026-11-10.
Tags: `[V]` vendor; `[T]` third party; `[P]` paper (arXiv or peer-reviewed);
`[R]` relayed, unverified. Papers were read from their PDF text.

## Surprises

1. **A fast model's first five words are usually fine.** In RelayS2S, 82.5% to
   95% of five-word speech-to-speech prefixes were judged contextually
   appropriate even though the model's full answers were weak `[P]`. The design
   lever is not a better fast model, it is a cheap check on its first words.
2. **The failure is a gap in the audio, not a wrong word.** With a 3-word
   committed prefix and an API-hosted slow path (GPT-4.1), the slow path's first
   chunk arrived after the prefix audio ended on 27.5% of committed turns
   (audible stalls); 5 words cut that to 1.9% and 7 words eliminated it. With a
   local 7B slow path, a 3-word prefix stalled 0.5% `[P]`. Prefix length is set
   by the slow path's latency tail, not its median.
3. **The fast path's own uncertainty marks hard turns.** The learned verifier
   routed turns to fallback, and those were the turns where even the full
   cascade did worse: quality 4.36 vs 4.83 and a low-quality rate of 20.3% vs
   5.5% `[P]`. Uncertainty of System 1 is a free difficulty signal for System 2.
4. **Verifier gating made the committed prefix better than no gating.**
   5-word prefixes: ungated quality 4.52 with 14.3% low-quality; gated (70.7%
   committed) 4.67 with 10.9% `[P]`.
5. **Talker-Reasoner publishes its own failure case.** The Talker reads the
   latest belief state, which may be stale because the Reasoner has not
   finished; that is fine for chit-chat and wrong when the user asks for
   something needing a tool or a multi-step plan, so the Talker is told to wait
   `[P]`. The same design question, "when must System 1 not answer", is a
   decision a System 1 model can make (section 4).
6. **Users do not report latency, they report interruption.** Hamming's voice
   telemetry: median response 1.4 to 1.7 s, p90 3.3 to 3.8 s, p99 8.4 to 15.3 s
   against a human gap of about 200 ms; "users never complain about latency;
   they report agents that keep getting interrupted or do not understand when
   I'm done talking" `[T]`. Endpointing is part of System 1.
7. **System 1 can be a predictor of System 2's needs.** VoiceAgentRAG's
   background "slow thinker" predicts follow-ups and prefills a cache; the fast
   agent reads only the cache: 75% hit rate over 200 queries, retrieval
   110 ms to 0.35 ms (316x) on hits `[P]`. System 1 here decides nothing; it
   removes latency from System 2.
8. **The fast learner can be tiny.** SwiftSage's fast module is a small
   encoder-decoder LM fine-tuned on an oracle agent's trajectories; the slow
   module is GPT-4; on 30 ScienceWorld tasks it beat SayCan, ReAct and
   Reflexion `[P]`.

## 1. Patterns

| # | Pattern | Fast part | Slow part | Handoff | Source |
|---|---|---|---|---|---|
| P1 | Talker-Reasoner | Talker (conversational, "on" whenever the user speaks) | Reasoner (multi-step planning, tools, updates a belief state) | the Talker reads the latest belief state from memory; the two may be decoupled; the Talker can be told to wait for the Reasoner | `[P]` Google DeepMind, Oct 2024 |
| P2 | Fast by default, slow when stuck | SWIFT: small encoder-decoder LM, behaviour-cloned | SAGE: LLM planning and grounding | a heuristic switches to SAGE when SWIFT is unreliable | `[P]` SwiftSage |
| P3 | Speculative prefix, verified | duplex speech-to-speech model drafts a short prefix, keeps listening for barge-in | cascaded ASR to LLM writes the continuation conditioned on the committed prefix | learned verifier commits or falls back; "relay margin" = prefix audio minus slow time to first chunk | `[P]` RelayS2S |
| P4 | Predict and prefetch | background LLM predicts follow-up topics | foreground fast agent reads a semantic cache | cache hit or fall through to the vector store | `[P]` VoiceAgentRAG |
| P5 | Decision model as gate | typed questions answered in one pass (Jev, d1) | the agent that acts | the answers pick the path: ack, route, which desk | Autoradio turns note; JEV.md |
| P6 | Endpointing as System 1 | Smart Turn: 8M parameters, Whisper-Tiny encoder plus a linear head on raw waveform; 8 MB int8 or 32 MB ONNX; BSD-2 | everything downstream | "has the speaker finished" boolean | `[V]` Pipecat model card |
| P7 | Cascade with async escalation | cheap decision first | stronger model only when confidence is low | acknowledge now, revise when the strong answer lands | calibration page section 7 |

What the fast part was trusted to decide, by pattern:

| Pattern | Trusted to decide | Not trusted to decide |
|---|---|---|
| P1 | tone, small talk, restating what the Reasoner already knows | anything that needs a fresh tool result |
| P2 | routine steps it was cloned on | recovery from being stuck |
| P3 | the first words of a reply, backchannels, stopping on barge-in (control symbols `[BOC]` backchannel, `[BOS]` substantive start, `[STP]` stop, `[EOS]` end, emitted every 160 ms tick) | the substance of the answer: the slow path writes it |
| P4 | which documents will probably be needed | the answer |
| P5 | closed-set labels about the turn | open-ended content |
| P6 | end of turn | meaning |

## 2. Numbers worth keeping

RelayS2S (synthetic dialogues, latency excludes TTS and network) `[P]`:

| System | Slow path | Avg ms | P90 ms | Quality (1 to 5) | Low-quality % |
|---|---|---|---|---|---|
| S2S only | none | 59 | 72 | 3.29 | 52.7 |
| Cascaded | Qwen2.5-0.5B | 304 | 418 | 3.45 | 48.7 |
| RelayS2S | Qwen2.5-0.5B | 77 | 81 | 3.49 | 47.6 |
| Cascaded | Qwen2.5-7B | 392 | 514 | 4.21 | 27.5 |
| RelayS2S | Qwen2.5-7B | 85 | 81 | 4.21 | 26.9 |
| Cascaded | Grok-4.1-fast | 726 | 876 | 4.73 | 8.4 |
| RelayS2S | Grok-4.1-fast | 112 | 81 | 4.73 | 9.4 |
| Cascaded | GPT-4.1 | 853 | 1006 | 4.84 | 5.6 |
| RelayS2S | GPT-4.1 | 119 | 81 | 4.84 | 5.3 |

Reading: the closer the slow path is to the top, the more the speculative
design is worth (gap 227 ms with the 0.5B model, 734 ms with GPT-4.1), and the
quality ceiling is the slow path's: committed-turn quality tracks the cascade
"within about 99%". On real voice dialogues the average first-chunk latency fell
479 ms (269 vs 748 ms) at 99% of the cascade's text quality `[P]`. The verifier
adds about 10 ms. Caveat: the quality scores are LLM-judged against a 3-annotator
human panel `[P]`; the authors list limitations in their paper.

Other published numbers:

| Item | Figure | Tag |
|---|---|---|
| Human turn gap | about 200 ms; natural conversation sweet spot for production 300 to 800 ms; users adapt unconsciously at 800 to 1200 ms | `[T]` Hamming |
| Smart Turn v3 | 8M parameters; a 2026 conference talk lists 58.9% recall and 68.4% precision for v3 against a hierarchical end-of-turn model (single secondary source; test sets differ) | `[R]` |
| Smart Turn on our floor | Core ML port 3.5 ms per window on a Mac; no iPhone figure | design note, relayed |
| Stream RAG | predicts tool queries in parallel with speech, 20% lower tool-use latency | `[R]` via VoiceAgentRAG |
| Jev upstream (ours) | 110 to 260 ms mean for 1 to 12 questions; the whole path to the first acknowledgement is not yet timed on a phone | M, design note |

## 3. Anti-patterns

| Anti-pattern | Why it fails | Evidence | Instead |
|---|---|---|---|
| System 1 answers content | the fast model is weak on substance (S2S-only low-quality rate 52.7% vs 5.6% for the GPT-4.1 cascade) | RelayS2S `[P]` | S1 decides the shape of the response; S2 writes it |
| Wait for System 2 to be "ready" | the cascade's latency grows with model size (304 ms to 853 ms average) | RelayS2S `[P]` | commit a verified prefix, continue behind it |
| Short prefix with a long-tail slow path | audible stalls (27.5% at 3 words on an API LLM) | RelayS2S `[P]` | size the prefix to the slow path's p10 relay margin; 5 words fixed it |
| The slow path ignores what the fast path already said | the listener hears a contradiction | RelayS2S conditions the continuation on the committed prefix `[P]`; Talker-Reasoner's Talker may speak from stale beliefs `[P]` | make what S1 said part of S2's input (our rule: the phone's lines go into the document as blocks) |
| Two voices | a prefix and a continuation rendered by different voices or sessions | RelayS2S streams both through the same TTS session `[P]` | one voice, one session |
| Asking S1 something it cannot know | knowledge is where decision models are weakest (d1-3B knowledge 23.8; AA overconfidence) | `[V]` `[T]` | route knowledge questions to S2 |
| One call per question | pays the state N times and queues | JEV.md; Jev benchmark 12.2x cheaper one-call `[V]` | many questions, one call |
| Treating confidence as truth across versions | the served model changed under the alias | JEV.md | re-calibrate per version |
| Letting a shared state carry instructions | one injected instruction in a batch can steer every answer; a batch-prompting attack benchmark exists | arXiv 2503.15551 `[R]` | the state is data; questions carry the instructions; sanitise story text |
| Evaluating latency only at the median | the tail sets the experience | Hamming p90 3.3 to 3.8 s `[T]` | measure p90 and the stall rate |

## 4. What this means for the Autoradio phone and the desk

The turns note puts Jev at the end of the listener's turn, answering a small set
of questions at once, and a phone model that "holds" while the desk works for up
to a minute or two. Mapped to the patterns above:

```text
t=0     Smart Turn says the listener finished        (P6, on the phone)
t+0.1s  System 1 answers 4-8 questions on the turn   (P5, Jev or the phone model)
          ack_now? kind_of_turn? is_followup? needs_desk? stakes?
t+0.3s  the phone speaks an acknowledgement           (P3: short, verified prefix; same voice)
t+1s..  the desk works; the phone holds                (P1: reads the desk's latest belief)
t+Ns    the desk's revision arrives; the phone's lines are already in the document
```

Design rules read straight from the evidence:

- **The phone may decide closed-set things and say short things.** It must not
  state what the desk may contradict (turns note) which is exactly the P1
  anti-pattern ("stale belief") restated; the matching safe move in RelayS2S is
  to condition the slow path on what was already said.
- **Add a "should I wait?" question.** P1's failure case becomes one `noul`
  question: "does answering require something not yet in state.beliefs?" If
  yes, the phone holds instead of answering.
- **Gate the first words with a cheap verifier**, not a bigger fast model: a
  yes/no question ("is this acknowledgement appropriate for this turn?") can
  be that verifier and costs one extra question in the same call.
- **Size the acknowledgement to the desk's latency tail.** If the desk's
  first block can take 20 s, a 5-word prefix is not a bridge; the holding
  conversation is. Use the relay-margin idea: acknowledgement length minus the
  time to the next content, and aim for a positive p10.
- **Fast part's uncertainty is a feature.** Log System 1's confidence on each
  turn; low-confidence turns are the ones where the desk's output will also be
  poorest (RelayS2S fallback subset) and deserve a human-visible flag.
- **Barge-in belongs to the phone.** Only the fast path keeps listening while
  speaking (the main-stream and speculative-stream fork in RelayS2S); the desk
  never hears a live microphone.
- **Score the fast part on the same questions** (JEV record), and promote a
  phone model per question only when it agrees with Jev (evaluation.md).

## 5. Open design questions this page cannot answer

- Whether a phone-sized decision model's calibration is good enough to gate
  speech; unmeasured (no iPhone figure for any decision model, see
  model-space.md).
- The best form of backchannel for radio (music or voice playing); RelayS2S
  trains backchannels on dialogue data, not on playback.
- Whether the Talker-Reasoner belief state maps to our commitment ledger
  (the desk's document) or needs a smaller "what I know" object for the phone.

## Could not verify / open questions

- The Smart Turn v3 accuracy figures (single secondary talk summary); Pipecat's
  "12 ms CPU" claim is a blog title I saw but did not read.
- RelayS2S numbers are synthetic or LLM-judged speech dialogue; no Autoradio-like
  setting; the repository (github.com/mailong25/relays2s) was not opened.
- VoiceAgentRAG is one paper, one cache design, 200 queries.
- Fast-model-contradicts-slow rates in deployed products: no published numbers
  found; only mechanisms (stale belief, stall). The KAME and DDTSR systems cited
  by RelayS2S were not read.
- Latency of Qwen/LFM models as phone-side fast paths: unmeasured by us.

## Sources (fetched 2026-11-10)

1. Christakopoulou et al., Agents Thinking Fast and Slow: A Talker-Reasoner Architecture, https://arxiv.org/abs/2410.08328 `[P]`
2. Lin et al., SwiftSage, https://arxiv.org/abs/2305.17390 `[P]`
3. Mai and Liang, RelayS2S, https://arxiv.org/abs/2603.23346 `[P]`
4. VoiceAgentRAG, https://arxiv.org/abs/2603.02206 `[P]`
5. Smart Turn v3.x model card, https://huggingface.co/pipecat-ai/smart-turn-v3 `[V]`; Smart Turn search summaries (soniqo, Pipecat docs, a conference talk page) `[R]`
6. Hamming, Voice AI Latency, https://hamming.ai/resources/voice-ai-latency-whats-fast-whats-slow-how-to-fix-it `[T]`
7. Efficient but Vulnerable: Benchmarking and Defending LLM Batch Prompting Attack, https://arxiv.org/abs/2503.15551 `[R]` (search summary only)
8. Qwen3Guard Stream design, https://arxiv.org/abs/2510.14276 `[P]` (per-token guard heads; shown as a P6-like fast monitor)
9. Our design note: [autoradio-turns-2026-11-10.md](../../design/autoradio-turns-2026-11-10.md); JEV.md measurements (M)
