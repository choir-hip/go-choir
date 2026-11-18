# Podcast clip fingerprint, version 1

What it is: a small audio signature (8 log-energy bands, 10 frames a second,
8 bits each) cut from the first and the last 12 seconds of a podcast clip, so a
phone can find the clip again in the file it is served. Why: a host inserts
other ads in each download, so the same episode is a different file on the
phone than on the worker. Measured 2026-11-17 on 27 episodes with clips: all
27 differed in length; the clip's offset in a fresh download ran -37 s to
+130 s (median +17 s; 32 of 33 within 120 s, all within 240 s); all 27 files
were constant bitrate and every host answered byte ranges. Code:
`internal/podcastfp` (the definition, `Locate`, the tests), the worker in
`internal/podcastworker` (`tracks.go`, `fprepair.go`), the platform in
`internal/platform/podcast.go`. Conformance fixture:
[mobile/fixtures/podcast-clip-fingerprint-v1.json](../../mobile/fixtures/podcast-clip-fingerprint-v1.json).
Status: built in lane `clip-fingerprint`, not deployed, no Swift implementation
yet; nothing here has run on a phone.

## 1. The definition (version 1)

Input is 16 kHz mono signed 16-bit PCM (the worker's ffmpeg: `-ar 16000 -ac 1
-c:a pcm_s16le`; a phone decodes the same way and resamples to 16 kHz).
Everything below is IEEE double arithmetic.

| Name | Value |
| --- | --- |
| Frame length | 1024 samples |
| Hop | 1600 samples (0.1 s, 10 frames per second) |
| Frame k | samples `[1600k, 1600k + 1024)`; emitted only when complete, so a trailing partial frame is dropped; the 576 samples after each frame are not used |
| Window | periodic Hann: `w[n] = 0.5 - 0.5*cos(2*pi*n/1024)`, n = 0..1023 |
| Sample scale | `x[n] = s[n] / 32768 * w[n]` |
| Transform | complex DFT of length 1024 (any exact algorithm; Go uses radix-2), power `P[k] = re^2 + im^2`, bin k is k * 15.625 Hz |
| Bands | 8, summing bins `[6,10) [10,16) [16,25) [25,39) [39,62) [62,98) [98,154) [154,243)` (round(f/15.625) of 100 Hz * 38^(b/8); 93.75 Hz to 3796.875 Hz) |
| Band value | `e = (sum of P over the band) / (bins in the band) / 1048576`; `dB = 10 * log10(e + 1e-10)` (log base 10; digital silence is -100 dB) |
| Quantise | `q = clamp(floor((dB + 100) * 2.55 + 0.5), 0, 255)` |
| Frame | 8 bytes, band 0 first |

A **track** is the frames of a stretch of audio, frame-major, so byte
`8*k + b` is band b of frame k. A frame k of a track whose audio starts at
file time `t0` lies at `t0 + k/10` s.

**Packed fingerprint** (what is stored): byte 0 = version (1), byte 1 = band
count (8), bytes 2-3 = frame count n (uint16, big endian), then n frames of 8
bytes. A clip fingerprint has n = 120 frames, always: 4 + 960 = 964 bytes, about
1.3 KB in base64.

**Cutting** (`podcastfp.Cut`): for a clip `[start, end]` in a file with a track
whose frame 0 is at `t0` (a whole second, so frames fall on the 0.1 s grid of
the file): `f0 = round((start - t0) * 10)`, `f1 = round((end - t0) * 10)`,
`n = 120` (a clip with `f1 - f0 < 240`, under 24 s, is refused). **Head** = frames `[f0, f0 + n)`, **tail** = frames
`[f1 - n, f1)`. `head_at = t0 + f0/10`, `tail_at = t0 + (f1 - n)/10`.

**Conformance.** The fixture's signal is integer only: LCG `x = x*1664525 +
1013904223 mod 2^32`, seed 1, advanced before each sample; noise = `(x >> 16) -
32768`; envelope `[1,2,4,8,16,8,4,2][(i/1600) mod 8]`; `v = noise * envelope /
64` (integer division truncating toward zero) plus 3000 when `(i/20)` is even
and -3000 when odd; 48000 samples, 30 frames. A Swift implementation must
produce `frames_hex` byte for byte, then `pack_hex` for frames 6 to 17, and
locating that pack in the whole track must give position 6 with a peak of at
least 0.99. The Go test asserts that no value of the fixture lies within 0.001
of a quantiser rounding boundary, so any IEEE-double implementation (Accelerate
or not, any FFT) rounds the same; if Swift disagrees by one on a value, its FFT
or its log10 is wrong. Real audio has no such guarantee: the phone's decoder
(AVFoundation, not ffmpeg) gives slightly different samples, which is what the
correlation threshold is for.

## 2. Locating (the phone's algorithm, `podcastfp.Locate`)

Input: a template (the frames of a packed head or tail) and a window track
(frames from PCM of the downloaded window, whose frame 0 is at the window's
own time zero; its start need not be on any grid). For each position p from 0
to `m - n`: normalised cross-correlation over all `8n` values,

    score(p) = sum_i (t_i - mean_t) * w_{p,i} / sqrt( sum_i (t_i - mean_t)^2 * (sum_i w_{p,i}^2 - (sum_i w_{p,i})^2 / (8n)) )

(score 0 when either variance is zero). `peak` = the best score at `pos`;
`second` = the best score at positions more than 15 frames from `pos`;
`margin` = peak - second. **Found** when `peak >= 0.80` and `margin >= 0.19`
(correct peaks measured 0.80 or more with margin 0.19 or more; wrong places
0.51 or less). Gain does not matter (the values are in dB); noise and an MP3
re-encode cost a little peak (a 64 kbps re-encode with a 9.4 s ad in front
measured peak 0.999). Time of the match in the window = `pos / 10`.

**Using it.** Every time in the `fingerprint` record (`start`, `end`, `head_at`,
`tail_at`) is seconds in the file the fingerprint was cut from. The clip's
top-level `start` and `end` are its identity and, for a repaired clip, seconds
in an older file: the phone never plays from them. Steps:

1. Fetch a window of the served file centred on the record's `start`:
   `window_t0 = record.start - 240` (at least 0) to `record.start + 240`
   (4 to 7 MB at 128 kbps), decode to 16 kHz mono, build the track.
2. Locate the head and the tail (both must be Found).
3. `off_head = window_t0 + pos_head/10 - head_at` and
   `off_tail = window_t0 + pos_tail/10 - tail_at`.
4. If `|off_head - off_tail| <= 0.3` s: play from `head_at + off_head` to
   `tail_at + 12 + off_tail` (12 s is the tail's length, always 120 frames).
   One not found, or the offsets differ (an ad inside the clip): refuse the clip.

Worked example, fresh clip: stored `start` 400, `end` 440, the record has
`start` 400, `end` 440, `head_at` 400, `tail_at` 428. Window [160, 640]. The
served file has a 17 s ad in front: the head is found at `pos` 2570 (time
417.0 in the file, 257.0 s into the window) and the tail at `pos` 2850 (445.0).
`off_head = 160 + 257.0 - 400 = 17.0`, `off_tail = 160 + 285.0 - 428 = 17.0`:
play 417.0 to `428 + 12 + 17.0 = 457.0`.

Worked example, repaired clip: stored top-level `start` 395, `end` 435 (the
file downloaded long ago); the worker found its quote 17 s later in a fresh
download and cut the record there: `start` 412, `end` 452, `head_at` 412,
`tail_at` 440. The phone centres on 412, not 395: window [172, 652]. The
file it is served has 1.5 s more ahead of the clip than the worker's
re-download: head found at 430.5, tail at 458.5. `off_head = 430.5 - 412 =
18.5` and `off_tail = 458.5 - 440 = 18.5`: play 430.5 to `440 + 12 + 18.5 =
470.5`.

A clip shorter than 24 s gets no fingerprint (its 12 s head and tail would
overlap, so an ad inside it could not be seen). The automatic matcher's minimum
is 24 s, so every clip it writes is relocatable; a shorter clip written before
that stays valid and unfingerprinted, and the repair pass skips it.

## 3. The record on the platform

`GET /api/platform/podcast-clips` gives each clip an additive `fingerprint`
object (absent for a clip made without one; such a clip is valid and is not
relocatable):

| Field | Meaning |
| --- | --- |
| `version` | 1 |
| `head`, `tail` | packed fingerprints, base64 (standard alphabet, padded) |
| `head_at`, `tail_at` | seconds in the fingerprinted file where head and tail begin (0.1 s grid) |
| `start`, `end` | the clip's bounds in the fingerprinted file |
| `file_bytes`, `file_seconds` | size and duration of that file as downloaded |
| `first_frame_offset` | bytes before the first MPEG frame (the ID3v2 tag size) |
| `bitrate_kbps` | the constant bitrate |
| `seekable_by_bitrate` | true only for an MP3 whose frames, from the first to within 4 KiB of the end, are all of one bitrate and with no tag in the middle; then `byte = first_frame_offset + seconds * bitrate_kbps * 125`. False (and `bitrate_kbps` 0) for anything else: the phone needs a probe |

Platform checks (`podcastfp.ClipRecord.Validate` and the write route): version
1; head and tail unpack and have exactly 120 frames (964 bytes); every time finite and in a
day; `end - start` at least 24 s and `tail_at` at least 12 s after `head_at`; the fingerprint's `end - start` within 1 s of the clip's;
bitrate at most 640; seekable needs a bitrate and a length.

`PUT /api/platform/operator/podcasts/clips` takes the same object as
`fingerprint` on a clip; the same clip (story, episode, start second) written
again replaces the row. There is no delete route: nothing in this design
deletes a clip. The write route has a 6 MB body bound and 200 clips a request. The clip row is an
object-graph row of kind `choir.podcast_clip`, not the tape; the route is
operator only. Old servers refuse unknown fields (the decoder disallows them):
a new worker against an old platform would have its whole clip refused. The
matcher therefore retries such a refused clip once without the fingerprint,
logs the downgrade once, and marks the clip written only when a request
succeeds; the old platform stores it unfingerprinted and the repair pass
upgrades it later (against an old platform that pass's PUT is
refused and skipped for 6 hours). Deploy the platform first all the same.

## 4. Where fingerprints come from

- **New clips.** The worker computes the frame track of the audio when it
  transcribes (from the WAV it already makes: one pass, about 0.5 MB for 90
  minutes) and keeps it in `fp-tracks/` beside the match state with the file
  facts (`podcastworker.TrackStore`; tracks older than 10 days are pruned). The
  matcher, which writes clips later when the audio is gone, cuts the head and
  tail from the track of the very file the transcript was made from. An
  episode with no track gets its clip written without a fingerprint.
- **Held clips with none** (`podcast-worker repair`, and in `serve` whenever
  the transcription queue is drained, at most every 30 minutes). For each such
  clip: download the episode again once per episode; find where the clip's
  quote begins in the stored transcript (the segment holding its first word, found over the clip's segments joined, so a quote may run across two); cut
  `[start - 300 s, end + 300 s]` (nearby clips share a span), transcribe it
  with the worker's own recogniser, and find the quote (`FindQuote`: longest
  common subsequence of words, at least 75% of the quote's words in order;
  of near-equal matches the one nearest the stored place). The quote's shift
  moves the clip's start and end; the fingerprint is cut from that audio at the
  shifted place; the clip's stored start and end are not changed (they are
  its identity). The audio is deleted after use; `--min-free-gb` and the work
  directory bound apply. If the quote is not found, or the shifted clip runs
  past the audio, the episode is downloaded once more and the whole file is
  transcribed, but only when the served file is no longer than the worker's
  `--max-minutes` recogniser bound (90 minutes by default; a longer file is
  logged and not searched whole). A clip whose quote is still not found stays
  valid and unfingerprinted, recorded as not found and not looked at again for
  24 hours; a failed download or search, or a clip that cannot be cut where its
  quote now is, skips it for 6 hours; so does a clip whose episode left the
  14-day listing. **The repair pass deletes nothing**: a clip without a
  fingerprint is dropped from the phone's feed by the phone, and clips leave the
  list with their stories (one of 34 held clips was such a case on 2026-11-17:
  its episode had been re-edited). A run has a 20 minute wall-time budget: the
  episode in hand when it ends writes nothing. No model service is called.

## 5. What is not known

- The Swift implementation, and whether AVFoundation's decode of an MP3 or an
  AAC file correlates as well as ffmpeg's. Re-encodes at 64 kbps measured
  peak 0.999 in Go; no phone audio was measured.
- Whether the quote's shift is the clip's shift when an ad sits between the
  quote and the clip's start; the phone's head/tail agreement check catches an
  ad inside the clip, not one between start and quote (the fingerprint is cut
  from the clip's own start, so its head is the clip's, wherever the quote is
  heard).
- The repair pass has run only against fakes and a real-codec check of the
  fingerprint; it has not run on the 34 held clips.
