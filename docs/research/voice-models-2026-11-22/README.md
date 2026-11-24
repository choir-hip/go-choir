# On-device voice models: the research of 2026-11-22

The owner gave a set of links on the night of 2026-11-22 (speech recognisers,
a voice isolator, a turn and speech detector, small voices, speaker models) and
asked for candidates for the iPhone and the Android phone. Sonnet researchers
read each model's page and wrote these reports. They sat only in a session
scratch folder until 2026-11-24, when the owner asked where the work had gone;
they are copied here unchanged so they are not lost again. Numbers in them are
the researchers' readings of model pages and papers, mostly not checked by the
lead; each report says where a number is from.

| File | What it covers |
| --- | --- |
| models-asr-survey.md | speech recognisers: the leaderboard and the linked models, shortlists per phone |
| models-voice-survey.md | the owner's four links (tiny-voca, Who and Eye, Voz, Fluid Speech Pro) and the other voice jobs |
| models-isolator-sava.md | the voice isolator and sava (speech and end-of-turn probability), with three experiments |
| models-measured-evidence.md | independent measurements found, and the benchmark protocol they imply |
| models-low-end.md | candidates for a low-end Android phone |
| models-iphone-pareto.md | the iPhone trade-offs |
| models-benchmark-plan.md | how to measure models on the phones without building them into the app |
| omni-ears-2026-11-24.md, omni-ears-tables.txt | LFM3-Omni-2B as the ears: word error against Fluid Speech on noisy speech, speech out, a tool-call probe |

What was measured: `lane/model-bench` (not landed) holds a rerunnable benchmark
and first rows on the Motorola for seven recognisers
(`docs/evidence/model-bench-2026-11-22/` on that lane). Nothing was measured on
the iPhone, and none of the three experiments (cleaning takes, the isolator
before recognition, sava replayed through the turn logic) was run. The voice
mission is `docs/definitions/choir-voice-pipeline-2026-11-23.md`.

## Corrections since

- 2026-11-24 (owner): the isolator report says no phone build exists. That was
  true of the official repository only. Community builds exist: MLX (two, one
  real-time), ExecuTorch, ONNX and GGUF; see the voice mission for the names.
  The report's "server only" conclusion for takes is withdrawn: the owner's
  rule is that every voice model runs on the phone.
