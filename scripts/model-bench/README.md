# model-bench: on-device speech-model benchmark (first slice)

Developer-side. Measures speech recognisers on the Motorola (moto g stylus 5G
2023, serial hard-coded in `run_android.py`) through `adb shell`, with no app.
Python 3 standard library only on the Mac (plus `ffmpeg` to decode FLAC).
First numbers: `docs/evidence/model-bench-2026-11-22/`.

## Run it

```sh
# once: downloads into ~/.cache/model-bench (never the repo); hashes in the evidence README
#   runtime: sherpa-onnx v2.0.3 android-aarch64-termux-shared tarball -> runtime/v2.0.3/
#   models:  sherpa-onnx model tarballs, each into models/<archive>/ (urls in models.json)
#   audio:   openslr.org/resources/12 test-clean.tar.gz, test-other.tar.gz -> librispeech/{clean,other}/
scripts/model-bench/shim/build.sh ~/.cache/model-bench/runtime/libhashshim.so   # NDK compile, see below
python3 -I scripts/model-bench/fixtures.py --clean ~/.cache/model-bench/librispeech/clean \
    --other ~/.cache/model-bench/librispeech/other --out ~/.cache/model-bench/fixtures
python3 scripts/model-bench/run_android.py --model parakeet-tdt-0.6b-v2-int8 --set smoke \
    --threads 1,2,4,6 --repeats 2 --tsv rows.tsv --transcripts transcripts/
python3 -m unittest discover -s scripts/model-bench      # no network, no phone
```

`--set smoke` is the first 10 utterances of the manifest, `--set full` all 100.
Each (threads, repeat) is one process run on the phone and one TSV row.

## What it does

1. Reads `getprop`, `dumpsys thermalservice` and `dumpsys battery`; refuses
   to start when Thermal Status is 2 or worse, battery is over 40 C or under 30 %.
2. Makes `/data/local/tmp/mbench-<model>-<time>/`, pushes runtime, model files,
   the wavs and a small wrapper (`run.sh`).
3. The wrapper starts the recogniser as a child, reads its `VmHWM` from
   `/proc/<pid>/status` every 50 ms (peak resident memory = the last value,
   since VmHWM only grows) and writes stdout, stderr and the exit status.
4. Pulls those files, parses load time and decode time that sherpa-onnx prints,
   computes WER on the Mac, appends a row, removes the phone directory (also
   on failure, Ctrl-C and timeout) and checks it is gone.

## Protocol rules

- One text normaliser for every model (see WER). Empty output for a clip is all
  deletions, so that clip counts as 100 % errors. Corpus WER is total errors
  over total reference words.
- RTF = decode seconds / audio seconds. Below 1 is faster than real time.
- `mem_available_mib_before/after` is `MemAvailable` from `/proc/meminfo` read
  before the first and after the last process of a run. Never two models at once.
  A run killed by the system is a result: `status` is `killed` (exit 137) or
  `error`; it is not retried away.
- `sherpa-onnx-offline` decodes all files given to one process as one batch.
  Running the 100-utterance set as one process panicked the phone's kernel
  (see the evidence README), so `--chunk 10` (default) runs the set as
  processes of 10 utterances; for a chunked run `load_seconds` is the mean,
  `decode_seconds` and `wall_seconds` are sums and `peak_rss_mib` is the largest.
  The smoke set is a single chunk.
- The run is started detached on the phone and polled, because a USB drop once
  killed an attached run; `adb` calls are retried only when the device is
  reported not found or offline.

## The fixture (`fixtures.py`)

LibriSpeech test-clean and test-other (CC BY 4.0), seed 20261122. Candidates
are sorted, shuffled with `random.Random(seed)`, taken in order, skipping any
shorter than 1.5 s or longer than 20 s. 50 clean (seed) and 50 other
(seed+1), interleaved; the smoke set is the first 10 rows (5 and 5).
`manifest.tsv`: id, condition, reference text, duration, sha256 of the 16 kHz
mono 16-bit wav (decoded by ffmpeg). `run_android.py` refuses to run if a wav
no longer matches its hash.

## WER

Same on reference and hypothesis: lowercase; every character that is not a
letter, digit or apostrophe becomes a space; apostrophes not inside a word are
dropped; whitespace collapsed. Numbers are not expanded and spellings are not
merged. Corpus WER = total (substitutions + deletions + insertions) / total
reference words (not the mean of per-utterance rates). Implemented in `wer.py`
with a plain edit-distance and unit-tested against hand-worked cases.

## Columns of the TSV

| column | meaning |
| --- | --- |
| phone, soc, android | `ro.product.model`, `ro.soc.model`, release + API + build id |
| runtime, provider | recogniser runtime and version; execution provider (always `cpu` here) |
| threads | `--num-threads` |
| model_id, quant, model_bytes | key in `models.json`; quantisation; bytes of the model files pushed |
| set, utterances, audio_seconds | smoke or full; count; summed duration of the wavs |
| wer, subs, dels, ins, ref_words | corpus WER and its parts (empty if the run failed) |
| load_seconds | time the binary prints for "recognizer created". Not a cold load: the files were just pushed, so they are probably in the page cache |
| decode_seconds | offline binary: its own "Elapsed seconds". Online binary: wall time minus load (it prints only 2 digits per file) |
| rtf | decode_seconds / audio_seconds (below 1 is faster than real time) |
| peak_rss_mib | last VmHWM of the process, MiB (includes mapped model pages) |
| wall_seconds | process start to exit as seen by the wrapper |
| thermal_before/after | Android Thermal Status (0 none, 1 light, 2 moderate ...) |
| battery_temp_before/after | degrees C from `dumpsys battery` |
| plugged | `ac`, `usb` or `no`, at the end of the run |
| run | repeat number within this invocation |
| date, harness_commit | local time; short sha of this directory, `+dirty` if uncommitted |
| status, notes | `ok`, `error` or `killed` (exit 137); free text |

Missing values are empty, never zero.

## The hash_memory shim

The sherpa-onnx v2.0.3 Android binaries need the libc++ symbol
`std::__ndk1::__hash_memory`, which exists from NDK r28 on. The installed NDK
(27.3) `libc++_shared.so` and the phone's own `libc++.so` lack it, so the
binary stops with `CANNOT LINK EXECUTABLE ... cannot locate symbol
_ZNSt6__ndk113__hash_memoryEPKvm`. `shim/hash_memory_shim.c` defines the
symbol (FNV-1a over the bytes) and is loaded with `LD_PRELOAD` from the bench
directory. It only hashes bytes for unordered containers, but it is not the
upstream function: the evidence README says what that does and does not
guarantee.

## What it never does

Install an apk, touch app data or settings, change the thermal or battery
state, reboot, use `su`, kill anything but the one process it started, write
outside `/data/local/tmp/mbench-*`, delete a directory it did not create
(`remove_dir` refuses other paths, including `exact`), address any phone but
the one serial in the source, or send any request to the network (downloads
are done by hand, from official release URLs, with sizes and SHA-256
recorded). It does not run anything it downloaded on the Mac.

## Native llama.cpp jobs (Android text and speech)

`native.py` extends the same phone-safety rules to pinned native binaries and
GGUF models. `native_models.json` records every URL, byte length and SHA-256;
`fetch_native.py` downloads or verifies them under `~/.cache/model-bench`.

```sh
python3 -I scripts/model-bench/fetch_native.py
python3 -I scripts/model-bench/native.py --job qwen-q4 --threads 4 --repeats 2 \
  --out-dir /path/to/qwen-q4 --tsv /path/to/rows.tsv --json-out /path/to/bench.json
python3 -I scripts/model-bench/native.py --job tiny-voca-f2 --threads 4 --repeats 1 \
  --out-dir /path/to/tiny-voca-f2 --tsv /path/to/rows.tsv
python3 -I scripts/model-bench/vad.py --input /path/to/16k-mono.wav --threads 4 \
  --out-dir /path/to/silero --tsv /path/to/rows.tsv
python3 -I scripts/model-bench/embed.py --threads 4 \
  --out-dir /path/to/embeddinggemma-q8 --tsv /path/to/rows.tsv
python3 -I -m unittest discover -s scripts/model-bench
```

The wrapper runs the command detached, samples `VmHWM` every 50 ms, saves
stdout/stderr/meta and any declared output, computes the job metrics, appends a
stable TSV row, and removes only its `/data/local/tmp/mbench-native-*` directory.
The first Motorola results are in
`docs/evidence/model-bench-android-2026-11-26/`.

## Not measured yet

First-partial latency of streaming models (the online binary decodes a wav as
fast as it can), noisy or far-field audio, speech over radio, battery draw
(the phone is plugged in), sustained 10-minute thermal behaviour, cold page
cache, any accelerator (CPU only), and the iPhone. The native slice has measured
Qwen3-0.6B Q4_K_M/Q8_0, Tiny Voca F2, exact Silero v6.2.1 and EmbeddingGemma 2
Q8_0; EOU, Fluid Speech Mini, WeSpeaker and Phonon remain open.
