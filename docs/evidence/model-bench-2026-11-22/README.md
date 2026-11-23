# Model benchmark on the Motorola, 2026-11-22 (first slice)

Measured with `scripts/model-bench/` (harness commit in the TSV, `+dirty` means
uncommitted when run). Everything here is CPU only, on one phone, plugged in.

## On what

moto g stylus 5G (2023), Android 14 (API 34), SM6450, 4 x Cortex-A55 + 4 x
Cortex-A78, 3.59 GB RAM, charger connected throughout (`plugged` = ac).
Runtime: sherpa-onnx v2.0.3 `android-aarch64-termux-shared` prebuilt binaries
(official release), provider `cpu`, plus one shim (below). Audio: 100
LibriSpeech utterances (50 test-clean, 50 test-other, 1.5 to 20 s each, 597.765 s
in all, 1618 reference words), seed 20261122; the smoke set is the first 10
(67.885 s). `manifest.tsv` lists id, reference, duration and sha256 of each wav.
WER normaliser: lowercase, every character that is not a letter, digit or
apostrophe becomes a space, stray apostrophes dropped, numbers not expanded;
corpus WER = total errors / total reference words. RTF = decode seconds / audio
seconds. Peak RSS = last `VmHWM` of the recogniser process (polled every 50 ms),
which counts the mapped model file.

## Results

Smoke set (10 utterances): every thread count 1, 2, 4, 6, two runs each.
Full set (100): 4 threads, two runs each. Each cell shows run 1 / run 2.
Thermal is Android Thermal Status before > after; it stayed 0 in every run;
battery temperature rose from 23 to 29 C over the whole session
(`battery_temp_*` in the TSVs).

| model | set | threads | runs | WER | RTF | load s | peak RSS MiB | thermal (before/after) | status |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| parakeet-tdt-0.6b-v2-int8 | smoke | 1 | 2 | 0.0219 / 0.0219 | 0.4957 / 0.4862 | 3.766 / 4.302 | 1857.3 / 2066.9 | 0>0 / 0>0 | ok / ok |
| parakeet-tdt-0.6b-v2-int8 | smoke | 2 | 2 | 0.0219 / 0.0219 | 0.2954 / 0.3011 | 4.363 / 4.409 | 2113.8 / 2002.4 | 0>0 / 0>0 | ok / ok |
| parakeet-tdt-0.6b-v2-int8 | smoke | 4 | 2 | 0.0219 / 0.0219 | 0.2287 / 0.2169 | 4.395 / 4.393 | 2064.4 / 1994.5 | 0>0 / 0>0 | ok / ok |
| parakeet-tdt-0.6b-v2-int8 | smoke | 6 | 2 | 0.0219 / 0.0219 | 0.2555 / 0.2717 | 4.485 / 4.451 | 2050.1 / 2045.7 | 0>0 / 0>0 | ok / ok |
| whisper-tiny.en-int8 | smoke | 1 | 2 | 0.0765 / 0.0765 | 0.1772 / 0.1729 | 0.772 / 0.743 | 337.6 / 338.3 | 0>0 / 0>0 | ok / ok |
| whisper-tiny.en-int8 | smoke | 2 | 2 | 0.0765 / 0.0765 | 0.1317 / 0.1255 | 0.76 / 0.97 | 323.5 / 324.7 | 0>0 / 0>0 | ok / ok |
| whisper-tiny.en-int8 | smoke | 4 | 2 | 0.0765 / 0.0765 | 0.1088 / 0.1095 | 0.762 / 0.763 | 325.0 / 324.6 | 0>0 / 0>0 | ok / ok |
| whisper-tiny.en-int8 | smoke | 6 | 2 | 0.0765 / 0.0765 | 0.1511 / 0.1387 | 0.78 / 0.765 | 324.7 / 325.1 | 0>0 / 0>0 | ok / ok |
| moonshine-tiny-en-int8 | smoke | 1 | 2 | 0.0710 / 0.0710 | 0.0601 / 0.0603 | 1.357 / 1.358 | 256.6 / 256.7 | 0>0 / 0>0 | ok / ok |
| moonshine-tiny-en-int8 | smoke | 2 | 2 | 0.0820 / 0.0820 | 0.0465 / 0.0459 | 1.375 / 1.366 | 257.1 / 257.3 | 0>0 / 0>0 | ok / ok |
| moonshine-tiny-en-int8 | smoke | 4 | 2 | 0.0710 / 0.0710 | 0.0418 / 0.0417 | 1.378 / 1.372 | 257.1 / 257.3 | 0>0 / 0>0 | ok / ok |
| moonshine-tiny-en-int8 | smoke | 6 | 2 | 0.0710 / 0.0710 | 0.0588 / 0.0697 | 1.365 / 1.417 | 258.2 / 258.2 | 0>0 / 0>0 | ok / ok |
| streaming-zipformer-en-2023-06-26-int8 | smoke | 1 | 2 | 0.0710 / 0.0710 | 0.1061 / 0.1063 | 4.64007 / 4.63077 | 247.8 / 247.9 | 0>0 / 0>0 | ok / ok |
| streaming-zipformer-en-2023-06-26-int8 | smoke | 2 | 2 | 0.0710 / 0.0710 | 0.0833 / 0.0839 | 4.64569 / 4.64354 | 248.2 / 248.3 | 0>0 / 0>0 | ok / ok |
| streaming-zipformer-en-2023-06-26-int8 | smoke | 4 | 2 | 0.0710 / 0.0710 | 0.0808 / 0.0763 | 4.74101 / 4.69125 | 249.0 / 249.0 | 0>0 / 0>0 | ok / ok |
| streaming-zipformer-en-2023-06-26-int8 | smoke | 6 | 2 | 0.0874 / 0.0874 | 0.1108 / 0.1157 | 4.66695 / 4.6432 | 249.6 / 249.5 | 0>0 / 0>0 | ok / ok |
| whisper-tiny.en-int8 | full | 4 | 2 | 0.1125 / 0.1125 | 0.1101 / 0.1109 | 0.786 / 0.782 | 379.2 / 379.1 | 0>0 / 0>0 | ok / ok |
| moonshine-tiny-en-int8 | full | 4 | 2 | 0.0878 / 0.0878 | 0.0422 / 0.0421 | 1.405 / 1.398 | 322.2 / 321.8 | 0>0 / 0>0 | ok / ok |
| streaming-zipformer-en-2023-06-26-int8 | full | 4 | 2 | 0.0569 / 0.0569 | 0.0804 / 0.0804 | 4.679 / 4.688 | 254.9 / 255.3 | 0>0 / 0>0 | ok / ok |
| parakeet-tdt-0.6b-v2-int8 | full | 4 | 2 | 0.0216 / 0.0216 | 0.2903 / 0.2881 | 4.28 / 4.342 | 2020.5 / 2165.1 | 0>0 / 0>0 | ok / ok |


Reading it (numbers only as above):

- 4 threads was the fastest for all four models on the smoke set. 6 threads
  was slower than 4 for every model (the 4 + 4 big/little split), 1 thread
  slowest; Parakeet RTF 0.49 at 1 thread, 0.30 at 2, 0.22 at 4, 0.26 at 6.
- Repeat spread is small (RTF within 0.01 for the same configuration, except
  6 threads, where whisper-tiny gave 0.151 and 0.139 and moonshine 0.059 and
  0.070).
- Parakeet TDT 0.6B v2 int8 is the most accurate (full-set WER 2.16 %, 27
  substitutions, 2 deletions, 6 insertions in 1618 words) and the only one
  near the memory limit: peak RSS 2.0 to 2.1 GiB of 3.59 GiB; `MemAvailable`
  before a run was 1.7 to 2.0 GiB. Its full-set RTF (0.29) is higher than the
  smoke-set RTF at the same 4 threads (0.22); the full set has longer clips and
  ran at 25 to 29 C battery temperature against 23 to 24 C. Which of the two
  causes it was not separated.
- Moonshine tiny en int8 at 2 threads gave a different WER (8.20 %) from 1, 4
  and 6 threads (7.10 %) on the smoke set, identically in both runs: the
  decoding is not independent of the thread count. Not investigated.
- Whisper tiny.en (int8) full-set WER 11.25 %, Moonshine tiny en 8.78 %,
  streaming zipformer 5.69 %, Parakeet 2.16 %. Moonshine is fastest (RTF 0.042),
  streaming zipformer RTF 0.080, whisper-tiny 0.110, Parakeet 0.29.
- "Load seconds" is the time the binary prints for creating the recogniser,
  with the files just pushed (so most likely in the page cache): 0.8 s whisper
  tiny, 1.4 s moonshine tiny, 4.3 s Parakeet, 4.7 s zipformer.

## What went wrong, and what fixed it

1. **The binary did not start.** Exact error:
   `CANNOT LINK EXECUTABLE ... cannot locate symbol "_ZNSt6__ndk113__hash_memoryEPKvm"`.
   The release binaries need `std::__ndk1::__hash_memory`, a libc++ symbol from NDK r28 on.
   The tarball has no `libc++_shared.so`; the NDK installed here (27.3) has one without
   the symbol; the phone's own `/system/lib64/libc++.so` also lacks it. Fix:
   `scripts/model-bench/shim/hash_memory_shim.c` defines the symbol (FNV-1a over the bytes),
   compiled with the NDK clang on the Mac and loaded with `LD_PRELOAD` from the bench
   directory only. This is not the upstream function. A string hash only has to be consistent
   inside one process; mixing it with code that inlines the old hash would show as
   wrong lookups, and no such failure was seen: the Parakeet WER (2.16 % on a clean/other
   mix, 2.19 % on the smoke set) was not compared with the published figure (not re-confirmed against the model card), and
   all four models decode sensibly. That is weak evidence, not a proof. NDK 27.3's `libc++_shared.so`
   is still pushed (the binary lists it as NEEDED).
2. **The phone's kernel panicked.** The first attempt at the full set ran
   `sherpa-onnx-offline` on all 100 wavs in one process (the binary decodes all given files
   as one batch). A few minutes in the phone rebooted: `uptime` showed 7 minutes
   afterwards, `sys.boot.reason` was `kernel_panic` (also in `persist.sys.boot.reason.history`),
   `/data/local/tmp/mbench-*` survived, the output file was left as NUL bytes. We did not
   cause a reboot on purpose and did not run anything that reboots. Cause not proven: memory
   exhaustion from the batch is the leading explanation (the 10-utterance runs peak at 2.0 to
   2.1 GiB), but no log was available (`dmesg` is denied to the shell user, no pstore read).
   After that the harness runs the set as processes of 10 utterances (`--chunk 10`); no
   further crash or kill in the 8 full-set runs after the change (all 32 smoke-set runs came before the crash). The earlier
   crashed attempt has no row.
3. **A USB drop.** Once during an attached run the host lost the device for about a minute
   (the phone stayed up, uptime 3 days); the run died with the adb session. The harness now
   starts runs detached on the phone and polls, and retries adb when the device is
   reported not found. The runs of that session were repeated from scratch; there are no
   rows from the interrupted one.

## Not measured

- First-partial latency of any streaming model (the online binary decodes a wav as
  fast as it can; the zipformer row is RTF and WER only).
- Noisy, far-field or radio-over-speech audio: LibriSpeech is read speech and
  Parakeet and the zipformer were trained on LibriSpeech-style data, so these WERs are a
  regression gate, not the product number.
- Battery draw (the phone was plugged in the whole time).
- Cold model load: files were pushed just before each run, so the page cache is probably
  warm; the load seconds are approximate in that direction. Whisper tiny, Moonshine and the
  zipformer were each pushed fresh for every invocation.
- Sustained thermal behaviour (no run longer than about 4 minutes; Thermal Status was 0
  throughout, so throttling was not seen, and not provoked either).
- Any accelerator (NNAPI, XNNPACK, GPU), any model other than these four, any thread count
  above 6, the second phone, the iPhone.
- The smoke thread sweep ran before the `MemAvailable` columns existed, so
  `rows-smoke-thread-sweep.tsv` has no memory-available columns; `rows-full-t4.tsv` has them.
- The exact sherpa-onnx source commit of the binaries and the ONNX Runtime build options
  (not published in the tarball).

## Files

- `rows-smoke-thread-sweep.tsv`, `rows-full-t4.tsv`: one row per run (column meanings in
  `scripts/model-bench/README.md`).
- `manifest.tsv`: the fixture. `transcripts/`: every run's hypotheses (id, text).
- `download-sha256.txt`: SHA-256 of every download (below).

## Downloads (sizes checked against the release API; SHA-256 of the files as received)

Sources: runtime and models from `github.com/k2-fsa/sherpa-onnx/releases` (tags `v2.0.3`
and `asr-models`), LibriSpeech from `openslr.org/resources/12`. Total 1.75 GB downloaded
(`~/.cache/model-bench` holds 3.7 GB with everything extracted).

```
d5fe6ec4334fef36255b2a4010412cad4c007e33103fec62fb5d17cad88086f2  sherpa-onnx-moonshine-tiny-en-int8.tar.bz2  107600538 bytes
6fc4426b54aa549a0019b8a75b8dea59164e71790a37ac56a045ad57f915f714  sherpa-onnx-nemo-parakeet-tdt-0.6b-v2-int8.tar.bz2  482440461 bytes
639e25b578e9e997131402199419c13a941f8e4e198e2da1ce57dbf5cf401282  sherpa-onnx-streaming-zipformer-en-2023-06-26.tar.bz2  310414022 bytes
f3175bb258db62badcaa0d7b6a3da4bd898bc80d567944511ca30c0f08aee4fb  sherpa-onnx-v2.0.3-android-aarch64-termux-shared.tar.bz2  17867876 bytes
fe4ccf7b36a0cb479724df2f1ba0b8fcf896c12118151ee7a25569565e2fbebb  sherpa-onnx-v2.0.3-android-static-link-onnxruntime.tar.bz2  36158867 bytes
2bd6cf965c8bb3e068ef9fa2191387ee63a9dfa2a4e37582a8109641c20005dd  sherpa-onnx-whisper-tiny.en.tar.bz2  118071777 bytes
39fde525e59672dc6d1551919b1478f724438a95aa55f874b576be21967e6c23  test-clean.tar.gz  346663984 bytes
d09c181bba5cf717b3dee7d4d592af11a3ee3a09e08ae025c5506f6ebe961c29  test-other.tar.gz  328757843 bytes
```

The `android-static-link-onnxruntime` tarball was fetched to look for a libc++ and a CLI
and holds neither; it was not used. The Android binaries were run only on the phone.
