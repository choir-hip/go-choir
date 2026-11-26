# Android native model benchmark, 2026-11-26

Device: Motorola `ZY22H5K8QR`, moto g stylus 5G (2023), Android 14 (API 34),
SM6450, 3.59 GB RAM, charger connected. Runtime: generic llama.cpp `b12497`
(`3574df84b`) Android arm64 archive, SHA-256
`543ef84950b07750899fff83c1b5e2718fe332c55f605867cb9b2cf42099722c`.
Provider: CPU, four threads, no GPU offload (`-ngl 0`).

The harness is `scripts/model-bench/native.py`; pinned artifacts and commands
are in `native_models.json`. It verifies every model and runtime archive by
SHA-256, runs only on the named phone under `/data/local/tmp/mbench-native-*`,
samples `VmHWM` every 50 ms, checks thermal/battery preflight, and removes its
phone directory on success, failure, timeout or Ctrl-C.

## Results

| Job | Quant | Prefill tok/s | Decode tok/s | Audio s | Process s | Wall s | Peak RSS MiB | Status |
| --- | --- | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| Qwen3-0.6B | Q4_K_M | 204.755 | 36.039 | - | - | 12.317 | 872.2 | ok |
| Qwen3-0.6B | Q8_0 | 248.474 | 19.374 | - | - | 14.508 | 1331.8 | ok |
| Tiny Voca F2 | F32 + mmproj | - | - | 2.36 | 0.19 | 0.625 | 64.2 | ok |

Q4_K_M is the prudent first Android responder: it clears the existing 15
tok/s decode and 150 tok/s prefill bars with margin and leaves much more memory
than Q8_0. Q8_0 also clears the bars on this phone but has roughly half the
decode margin and about 460 MiB more peak RSS. Tiny Voca F2 produced 2.36 s of
24 kHz speech in 0.19 s of process time; its WAV SHA-256 is
`4da2362cfaf40ae89abc5329e656636c5020e09092cf43915be22609fc36097d`
(113,196 bytes).

`rows.tsv` is the machine-readable result. `qwen-q4/bench.json` and
`qwen-q8/bench.json` are llama.cpp's raw JSON. Each job directory keeps its
stdout, stderr, wrapper metadata and any generated audio.

## Failed artifact kept out of the stack

The official `llama-b12497-bin-android-arm64-snapdragon.tar.gz` archive
(SHA-256 `230daf40746d3a7b3bd7a0e34549394e08c62bd9cc05b0cd80b01e8388ce09be`)
selected the Adreno/Hexagon backends on this SM6450 phone and then exited 132
(`Illegal instruction`) on the Q4 benchmark. It is not used. The generic
Android arm64 archive above loads the armv8.2 CPU backend and completes. The
failure is recorded in `snapdragon-failure.txt`; do not silently substitute a
SoC-tuned binary for the generic one.

## Not measured yet

Silero, Parakeet EOU, Fluid Speech Mini GGUF, WeSpeaker, EmbeddingGemma, Phonon,
a whole voice turn, noisy/far-field speech, battery draw and sustained heat.
The existing CPU ASR measurements remain in
`docs/evidence/model-bench-2026-11-22/`; this directory extends that harness
rather than replacing it.
