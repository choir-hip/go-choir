# Podcast clips: audio that sources the paper's stories (2026-11-15)

Owner, 2026-11-10: "we should look for audio content that sources the
autopaper articles." Owner, 2026-11-15: "i want podcast clips ... autoradio
with ai dj ... mixing between news, podcast clips." Status: tonight's slice,
a rapid prototype on the real path. What was decided and measured on
2026-11-10 is in
[the problem record](../problems/podcast-clipping-work-was-never-written-down-2026-11-15.md);
the programme it plugs into is
[AUTORADIO-PROGRAMME.md](../reference/AUTORADIO-PROGRAMME.md) and
[the DJ programme design](autoradio-dj-programme-2026-11-11.md) section 5.

## 1. Recorded facts (all from the problem record, not re-measured here)

- An episode is a source, its transcript is the capture, and a time-bounded
  passage of it is the clip.
- None of eight news feeds carried a transcript tag. All six enclosure URLs
  tested answered a byte-range request, so a clip plays from the publisher's
  own file. Ad insertion shifts timings, so times are valid for the file we
  transcribed, and the phone streams that same enclosure URL.
- Exa finds episodes and quotes (about $0.78 for 31 passages from 25 shows)
  but returns no clip times; it is not the clipper.
- Hit rate, six episodes from six shows: 8 of 25 stories had a strong matching
  passage.
- Transcription: Parakeet 110M through parakeet.cpp, 8-bit, about 25 times
  real time on four cores, 0.7 GB, word timestamps, 5.3% word error against
  the largest Parakeet on one news episode. Output (`parakeet-cli transcribe
  --timestamps --json`): `{"text", "frame_sec", "words":[{"w","start","end","conf"}]}`,
  punctuation attached to the words.
- Rights (owner, 2026-11-10): playing and clipping is allowed; audio is
  streamed from the publisher's URL and never re-hosted; transcripts are
  stored for search; the display shows a short attributed quote.

## 2. Where things live (custody)

| Data kind | Store and kind | Writer and reader |
|---|---|---|
| episode | Autopaper object graph (Store B, corpusd), `choir.podcast_episode`, keyed by feed URL and guid | the platform podcast service in corpusd, sole writer and reader |
| transcript | same store, `choir.podcast_transcript`, one per episode: timed segments (sentence or pause bounded: `start`, `end`, `text`), not words | same |
| clip | same store, `choir.podcast_clip`, naming the story by its bare route and the episode by id (no edge: the row is the join) | same; the public route reads it |

**Shortcut, named residual R1.** The doctrine says one custodian desk per
data kind. No desk owns public audio today and a desk needs the in-cell
module surface and precommitment records, which is more than tonight allows.
Tonight the platform podcast service (the same custody as the Autopaper
object graph's publications and story entities) is the custodian, and its
write routes are operator-only. The follow-up is to make the Autoradio desk
(or research, read-only world evidence) the custodian and have clips return
by precommitment record.

Chosen over a guest-side store because the paper, its front list and its
public routes already live in Store B; a guest copy would need a second
export. Chosen over new SQL tables because the object graph needs no schema
change (the same kinds-by-name pattern as `choir.entity`).

## 3. The pipeline

```
feed list (data) -> worker: poll feeds ----------------> POST episodes
worker: GET episodes needing a transcript -> download enclosure -> ffmpeg 16 kHz mono
        -> parakeet-cli (110M q8, VAD, word timestamps) -> sentence segments -> PUT transcript
worker: POST match ->  corpusd: stories x transcripts -> BM25 -> ONE Jev call -> clips   (the red commit)
operator: PUT clips  (a clip an operator, or the proof run, matched by hand: the same store)
phone / any client: GET /api/platform/podcast-clips   (public, bulk, one request)
```

1. **Intake and feed list.** The feed list is data, not code: a JSON file of
   public feed URLs and show names (`config/podcast-feeds.json`, facts not
   content), read by the worker with `--feeds`. The paper's source schedules
   are guest records started through an operator route (`/api/autopaper/sources/schedules`);
   that is a guest mechanism for Autopaper sources and not for a host-side
   store, so tonight the worker polls the feeds and the server only stores
   what it is sent. The worker fetches feed XML itself, so the server never
   fetches an operator-supplied URL.
2. **Transcription runs in a worker program** (`cmd/podcast-worker`), which
   needs ffmpeg, `parakeet-cli`, the 110M GGUF and the Silero VAD GGUF, and
   runs anywhere that has them. Tonight the lead runs it by hand on the build
   VM as an operator job against staging, with `--api-key-file` (never
   printed). Why not a managed service: that needs a host unit and model
   files on Node B (a `nix/` change and the Node A `nixos-rebuild test` rule,
   an owner act), and Go builds never run on Node B. **Next step, not tonight:**
   a unit on a worker host that runs this same binary on a timer, with the
   model files in the image, and a managed operator credential.
3. **Matching** runs in corpusd (`POST .../operator/podcasts/match`), because
   the stories and the transcripts are both in Store B and it has the host
   door to Jev (the gateway's `/provider/v1/host/judgments`, caller
   `podcast-clips`, one allowlist entry, a red surface).
   - Stories: the front list, those updated within the last 72 hours (title,
     dek and an opening of the body).
   - Candidate transcripts: episodes published from 48 hours before the
     story's last update to 6 hours after it.
   - Retrieval: overlapping 120 s windows with a 60 s stride over the
     transcript segments; BM25 (k1 1.2, b 0.75) over lowercased,
     accent-folded, stop-worded, crudely stemmed tokens, plus a tolerant
     match for names (a token of six or more letters counts as matching a
     query token within edit distance 1, at half weight), because every
     transcriber garbles names. The top 4 windows per story go on.
   - Support check: **one Jev call per run** carrying every story and every
     candidate passage in one state, one yes/no `choice` question per (story,
     passage) pair, per `docs/reference/SYSTEM1.md` rules 1, 2 and 7. The call
     splits only if the vendor's own limit (`internal/jevbudget`: 32k tokens
     for state plus one question, 64k a request) is crossed, and the split is
     logged with the limit's name. A pair counts as supported at P(yes) of
     0.6 or more (a guess, like the other thresholds; it is stored with the
     clip so it can be scored later).
   - Boundaries: inside the supported window, the best contiguous run of
     segments, 20 to 75 s, on sentence or pause boundaries (the worker cut the
     segments there). At most 3 clips per story, best first.
   - If Jev is unavailable, no clip is published (a correctness refusal, not
     a switch): the route simply has fewer clips.
4. **Serving** is one public read route and the programme (section 4).
5. **Not built tonight:** the managed transcription service; episode intake
   inside corpusd; Exa discovery (optional, second priority; if built it is
   an operator-run step that only adds feed and episode candidates to
   intake); a desk custodian and precommitment records (R1); a forecast claim
   per clip (a `podcast_clip` segment carries none tonight, R2); transcripts
   for episodes of subscribed shows of a listener; deleting stale episodes
   and transcripts (R3: they stay until a retention rule exists); scoring of
   the support threshold; speaker labels; aligning clips across ad insertion (R4, measured 2026-11-15: NPR, NYT, Vox and Marketplace serve a different ad-stitched file on each download, 5 to 21% longer than the feed says; BBC and Pod Save America were identical). The worker stores the length it measured on the file it transcribed as `duration_seconds`; the phone skips a clip whose loaded item differs from it by more than about 3 seconds. Re-aligning by text, or keeping the exact file, is the real fix.

### 3.1 The worker keeps running (owner, 2026-11-15)

Owner: "why not get transcription worker running going forward too? we have
concurrency issues currently (its on node a shared with mike and choir builder)
but otherwise should be a good service to have running; we will scale hardware
next week probably". So `podcast-worker serve` is the long-running mode.

- **What it does.** Every 30 minutes it polls the feed list and posts the new
  items; between polls it asks the platform for episodes without a transcript
  (newest first, published within 72 hours), transcribes ONE at a time, posts
  the transcript, and loops at once. When nothing is pending it sleeps 30 s,
  doubling to 10 minutes, and starts over from 30 s when work appears.
- **Restart and kill.** The platform's episode list is the queue: an episode
  leaves it only when its transcript is stored, so a kill (SIGTERM stops the
  child processes and removes the scratch directory) leaves an unfinished
  episode pending and the next start redoes it. No server-side claim exists
  tonight because one worker runs; the transcript put replaces, so a second
  worker would at worst repeat an episode. When more than one worker runs, a
  claim with a deadline is added to the listing route (named residual R7). An
  episode that fails (a dead enclosure, a bad file) is skipped for 15 minutes,
  doubling to 6 hours, in memory, and does not hold the queue.
- **Manners on a shared host** (not limits on model calls): `nice 19` and the
  idle I/O class on the worker, inherited by ffmpeg and the model; at most 4
  threads (the benchmark showed 8 give nothing over 4; the command line cannot
  raise it); one episode at a time; the unit adds `CPUWeight=1` and
  `MemoryMax=2G`. Cost to neighbours: about 0.7 GB for the model plus ffmpeg's
  buffer, and per episode a temporary download (tens of MB, at most 600 MB)
  and a 16 kHz WAV (about 115 MB an hour), both deleted when the episode ends.
  The build VM has 3.9 GB, so the 2 GB ceiling matters.
- **Observed.** One line per episode (episode, show, segments, seconds of audio,
  seconds of work), one volume line per hour (stored, failed, audio and work
  seconds), a line per failure with its skip, and no line per poll, so the log
  volume does not follow the idle rate. `journalctl --user -u choir-podcast-worker`.
  The platform side is observed by what it holds: the episode listing with
  `need=transcript` is the backlog.
- **Where it runs, and how it is kept running.** The build VM `choir-agents`
  is a NixOS guest declared in Mike's infra repo (`infra/node-a/choir-agents.nix`),
  not in this repo, so a declared service (option a) is not ours to write or
  apply from here. Option (b) is `scripts/podcast-worker-user-unit`, which writes
  a `systemd --user` unit (restart on failure, nice 19, idle I/O, CPUWeight 1,
  2 GB ceiling); it survives logout only with linger on, which is off on that
  guest today (`loginctl show-user agent` says `Linger=no`), and turning it on is
  an act on the host that the lead decides. Option (c), tonight's stopgap,
  is `systemd-run --user` or `nohup` with the same flags, which lasts while the
  guest's user manager lives.
- **When hardware scales.** Nothing in the worker changes: it is one process
  with a fixed thread cap. The service moves to its own host (or a guest of its
  own) as the same binary under a unit declared in that host's config, the model
  files in the image, the operator key as a credential file. More throughput
  means more such workers, which needs the claim of R7.

## 4. What the phone receives (the exact shapes; do not change without the lead)

### 4.1 `GET /api/platform/podcast-clips`

Public, no sign-in, no computer (like `/api/platform/publications/front`).
Query: `route` (repeatable, at most 60; a story's bare route, `route.bare`
of the front list) to ask for named stories; with no `route`, the clips of
every story updated in the last 72 hours that has any (the front page in bulk, one
request). `limit` (default 120, at most 200). `Cache-Control: public,
max-age=60`. 400 on a bad query; 405 not GET.

```json
{
  "generated_at": "2026-11-16T06:12:00Z",
  "clips": [
    {
      "clip_id": "clip-7f3a9c0e41b2d855a6e0c1f2",
      "story": {"route": "/<bare route>", "version": 3, "title": "the story's title"},
      "show": "Up First",
      "episode_title": "the episode's title",
      "episode_guid": "the feed item's guid",
      "feed_url": "https://example.org/feed.xml",
      "published": "2026-11-16T09:00:00Z",
      "audio_url": "https://example.org/episode.mp3",
      "duration_seconds": 812.0,
      "start": 412.4,
      "end": 468.9,
      "quote": "a short attributed quote, at most 280 characters",
      "support": 0.91
    }
  ]
}
```

`story.route` is the bare route exactly as the front list spells it, so the
cite ref of the story is `pub:<route>@v<version>` (a clip is for a story at
the version it was matched to; the latest version is usually the one the
phone has). `start` and `end` are seconds into `audio_url`; `duration_seconds`
is the length of the file the transcript was made from (0 when the feed gave
none). Clips are ordered by story recency, then best first, at most 3 per
story. `audio_url` is the publisher's enclosure URL, streamed by the phone and
never re-hosted. The quote is a short excerpt for display, never the transcript.

### 4.2 The programme: `podcast_clip` segments

`POST /api/autoradio/programme` stays one request, one model call. Request
change: a candidate may carry `clips` (at most 3), copied by the phone from
4.1 (only these fields), so the DJ can place a clip after the story it
sources:

```json
{"route": "...", "version": 3, "title": "...", "dek": "...",
 "clips": [{"clip_id": "clip-7f3a...", "show": "Up First",
            "episode_title": "...", "published": "2026-11-16T09:00:00Z",
            "seconds": 56.5, "quote": "at most 280 characters"}]}
```

Bounds (400 over): at most 3 clips per candidate; `clip_id` matches
`clip-[0-9a-f]{12,64}` and is unique in the request; `show` and
`episode_title` one line, at most 200 bytes; `quote` at most 400 bytes;
`seconds` between 5 and 300. The model writes one JSON line for a clip:

```json
{"kind": "podcast_clip", "title": "Up First on the harbour", "clip": "clip-7f3a...",
 "text": "Here is how Up First put it this morning."}
```

The segment event for it (the existing fields, plus `clip`):

```json
{"stretch_id": "...", "segment_id": "<stretch>.s4", "kind": "podcast_clip",
 "title": "Up First on the harbour", "text": "Here is how Up First put it this morning.",
 "cites": [{"ref": "pub:<route>@v3", "role": "quoted", "label": "the story's title"}],
 "block_count": 1, "est_seconds": 4, "cut_at": "segment",
 "clip": {"clip_id": "clip-7f3a...", "story_ref": "pub:<route>@v3", "seconds": 56.5}}
```

`text` is the DJ's lead-in, spoken by the phone's voice; the phone then plays
the clip: it looks `clip.clip_id` up in the set it fetched from 4.1 for
`audio_url`, `start` and `end`, streams `[start, end]`, and shows
`show`, `episode_title` and `quote` with the story it cites. `cites` is
filled by the server from the clip's story, never by the model. A clip not in
the request, or a `podcast_clip` line with no `clip`, is dropped like any
invalid line. A `podcast_clip` carries no forecast fields (R2).

### 4.3 Operator routes (the worker's, and nobody else's)

Behind `authorizePlatformOperator`: a signed-in passkey session or an API key
that is not bound to a computer, whose account is on the platform operator
roster (`CHOIR_PLATFORM_OPERATOR_USERS`); the proxy forwards to corpusd's
internal routes, which accept only the proxy's host credential.

| Route | Body and answer |
|---|---|
| `POST /api/platform/operator/podcasts/episodes` | `{"episodes":[{"feed_url","guid","show","title","published","duration_seconds","enclosure_url","notes"}]}`, at most 200, `notes` cut to 2000 bytes; answers `{"recorded":n,"new":m}` |
| `GET /api/platform/operator/podcasts/episodes?need=transcript&since=<RFC3339>&limit=<n>` | `{"episodes":[{"episode_id","feed_url","guid","show","title","published","duration_seconds","enclosure_url"}]}`, newest first, episodes with no transcript |
| `PUT /api/platform/operator/podcasts/transcripts` | `{"episode_id","engine","audio_seconds","file_seconds","segments":[{"start","end","text"}]}`, at most 6 MB; `file_seconds` (optional) is the length of the whole file as downloaded and replaces the feed's duration as the episode's `duration_seconds`; answers `{"segments":n}` |
| `PUT /api/platform/operator/podcasts/clips` | `{"clips":[{"story_route","episode_id","start","end","quote","support"}]}`, at most 200; the story must be on the front list, the episode known, 5 to 120 s, inside the episode, quote cut to 280 characters; answers `{"clips":n}`. The clip id is made from story, episode and the start rounded to the second, so a repeat replaces |
| `POST /api/platform/operator/podcasts/match` | (the red commit) `{"since_hours":72}`; answers `{"stories","episodes","windows","pairs","jev_calls","clips","published","dropped"}` |

Enclosure and feed URLs must be `http` or `https`, with no credentials in the
URL and at most 2048 bytes; the server stores them, never fetches them.

## 5. Mutation classes and rollback

The slice lands in two parts so the first can deploy while the second is reviewed:
part 1 (orange) is the stored kinds, the public read, the operator write routes
(episodes, transcripts, clips), the programme's `podcast_clip` and the worker;
part 2 (red) is the match job and the gateway entry. Clips from part 1 come from
the operator's own matching posted through the clips route.

- Platform store, public route and operator routes (new kinds, additive): orange,
  rollback revert; the objects are inert without the route.
- The gateway's host-caller entry `podcast-clips`: red (gateway and provider
  calls); rollback revert (the match run then refuses and publishes nothing).
- The programme's `podcast_clip` kind: orange (the route's behaviour; guest
  runtime, so a planned refresh on deploy); rollback revert.
- The worker and the feed seed: yellow.

## 6. Residuals

R1 no desk custodian (section 2). R2 no forecast claim per clip. R3 no
retention for episodes and transcripts. R4 ad insertion offsets. R7 no server-side claim for more than one worker. R5 the
support threshold (0.6) and the window size are guesses to be scored against
listening outcomes. R6 matching is operator-triggered tonight, so clips appear
when the lead runs the worker, not on a schedule.
