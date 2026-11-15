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
| clip | same store, `choir.podcast_clip` plus an edge `clip_for` from the clip to the story's publication object | same; the public route reads it |

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
worker: POST match ->  corpusd: stories x transcripts -> BM25 -> ONE Jev call -> clips
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
   the support threshold; speaker labels; detecting ad insertion (R4: a clip
   from a file whose ads differ from the transcribed copy plays at a wrong
   offset, and the phone should fall back to the episode start if the audio
   duration differs by more than a few seconds from `duration_seconds`).

## 4. What the phone receives (the exact shapes; do not change without the lead)

### 4.1 `GET /api/platform/podcast-clips`

Public, no sign-in, no computer (like `/api/platform/publications/front`).
Query: `route` (repeatable, at most 60; a story's bare route, `route.bare`
of the front list) to ask for named stories; with no `route`, the clips of
every story updated in the last 72 hours (the front page in bulk, one
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
| `PUT /api/platform/operator/podcasts/transcripts` | `{"episode_id","engine","audio_seconds","segments":[{"start","end","text"}]}`, at most 4 MB; answers `{"segments":n}` |
| `POST /api/platform/operator/podcasts/match` | `{"since_hours":72}`; answers `{"stories","episodes","windows","pairs","jev_calls","clips","published","dropped"}` |

Enclosure and feed URLs must be `http` or `https`, with no credentials in the
URL and at most 2048 bytes; the server stores them, never fetches them.

## 5. Mutation classes and rollback

- Platform store, public route and operator routes (new kinds, additive): orange,
  rollback revert; the objects are inert without the route.
- The gateway's host-caller entry `podcast-clips`: red (gateway and provider
  calls); rollback revert (the match run then refuses and publishes nothing).
- The programme's `podcast_clip` kind: orange (the route's behaviour; guest
  runtime, so a planned refresh on deploy); rollback revert.
- The worker and the feed seed: yellow.

## 6. Residuals

R1 no desk custodian (section 2). R2 no forecast claim per clip. R3 no
retention for episodes and transcripts. R4 ad insertion offsets. R5 the
support threshold (0.6) and the window size are guesses to be scored against
listening outcomes. R6 matching is operator-triggered tonight, so clips appear
when the lead runs the worker, not on a schedule.
