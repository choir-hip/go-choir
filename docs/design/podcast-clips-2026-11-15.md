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
operator: podcast-worker candidates -> FILE -> a person or an agent judges -> podcast-worker put-clips -> PUT clips   (no model call in our code; 3.5)
phone / any client: GET /api/platform/podcast-clips   (public, bulk, one request)
```

1. **Intake and feed list.** The feed list is data, not code: a JSON file of
   public feed URLs and show names (`config/podcast-feeds.json`, facts not
   content), read by the worker with `--feeds`. The paper's source schedules
   are guest records started through an operator route (`/api/autopaper/sources/schedules`);
   that is a guest mechanism for Autopaper sources and not for a host-side
   store, so tonight the worker polls the feeds and the server only stores
   what it is sent. The worker fetches feed XML itself. The server stores URLs and never fetches
   one; the worker does, so every fetch of the worker (feed, enclosure, length
   probe) goes through one client whose dialer refuses every address that is not
   a public one and every port but 80 and 443, and ffmpeg runs with a protocol
   whitelist (section 3.4).
2. **Transcription runs in a worker program** (`cmd/podcast-worker`), which
   needs ffmpeg, `parakeet-cli`, the 110M GGUF and the Silero VAD GGUF, and
   runs anywhere that has them. Tonight the lead runs it by hand on the build
   VM as an operator job against staging, with `--api-key-file` (never
   printed). Why not a managed service: that needs a host unit and model
   files on Node B (a `nix/` change and the Node A `nixos-rebuild test` rule,
   an owner act), and Go builds never run on Node B. **Next step, not tonight:**
   a unit on a worker host that runs this same binary on a timer, with the
   model files in the image, and a managed operator credential.
3. **Matching is by hand tonight (3.5).** `podcast-worker candidates` ranks the
   paper's recent stories against the stored transcripts with
   `internal/podcastmatch` and writes the candidates to a file; a person or an
   agent judges them; `podcast-worker put-clips` sends the judged ones through
   `PUT .../operator/podcasts/clips`, whose checks are the server's. The
   automatic job that asked Jev is not built (3.6).
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
- **The shared disk (lead, 2026-11-16).** The build VM's `/home` reached 100% on
  2026-11-16. Before each download the worker reads the free space of the work
  directory's filesystem and refuses under `--min-free-gb` (default 15), logging
  the refusal; the episode stays pending and the loop waits as when idle, and a
  refusal is not counted as the episode's failure. The downloaded file and the WAV
  live in a per-episode directory deleted when the transcription ends, whatever
  its outcome (the audio is gone before the transcript is posted). So the work
  directory never holds more than one episode's audio, and the worker cannot fill
  the disk below the floor.
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
  an act on the host that the lead decides. **On the owner's iMac (2026-11-24)** the
  worker is installed under `~/.local/share/choir-podcast` (parakeet-cli built
  from mudler/parakeet.cpp, CPU only, the 110M q8_0 and Silero f16 GGUFs
  checked against the sha256 Hugging Face publishes) and kept by
  `scripts/podcast-worker-launchd`: a launchd user agent (Nice 19, background
  process type, low-priority I/O, restart after a crash, 30 s throttle, log in
  `~/Library/Logs/choir-podcast-worker.log` bounded by a second hourly agent
  that trims it past 5 MB). It is written but not loaded until the operator
  key exists at `~/.config/choir-podcast/operator.key`; once loaded it starts
  at login and survives logout and a restart of the Mac (a restart needs
  automatic login for the user agent to start). Option (c), tonight's stopgap,
  is `systemd-run --user` or `nohup` with the same flags, which lasts while the
  guest's user manager lives.
- **When hardware scales.** Nothing in the worker changes: it is one process
  with a fixed thread cap. The service moves to its own host (or a guest of its
  own) as the same binary under a unit declared in that host's config, the model
  files in the image, the operator key as a credential file. More throughput
  means more such workers, which needs the claim of R7.

### 3.2 The operator's feeds, and Part 2 (feeds reported by accounts: NOT BUILT)

**Built.** The feeds are the operator's: the seed list in
`config/podcast-feeds.json`, which the worker publishes at each intake through
`PUT .../operator/podcasts/feeds/listed`, and nothing else. A feed row exists only
because the operator featured the feed, "featured" is the only kind of feed, and
episodes of a feed that is not featured are skipped at intake. Feeds are keyed by
their normalised URL (lower-case scheme and host, a trailing dot on the host, a
default port, the fragment and a trailing slash dropped). The worker's queue is the
episodes with no transcript, newest first. Public reads: `podcast-clips` (4.1),
`podcast-popular` (4.6: the featured feeds the worker has fetched, newest episode
first, `subscribers` always 0, the title the show name the worker parsed) and
`podcast-transcript` (4.7: a featured feed's, to a signed-in account).

**Part 2, not built: feeds reported by accounts.** Owner, 2026-11-15: "we should
auto get the transcript of all new episodes of users' subscribed podcasts. over
time we will mostly be working in the universe of podcasts users subscribe to, and
we can amortise over the userbase since one of our users will probably subscribe.
and any given user probably wants to be shown clips of only a subset of podcasts
anyway. the long tail/new recommendations to user a will be the subscriptions of
user b." A first version (a signed-in `POST /api/platform/podcast-feeds`, a
stored subscription per account, now-playing priority, subscriber counts, listing
by two accounts) went through three rounds of two-seat review on 2026-11-16 and
every blocker of every round was in that intake: the worker fetching an address an
account chose (repaired in the worker's dialer, which stays), a feed row an account
could create without bound, a feed made public by cheap accounts, a scan that
stranger rows could push the public read out of. It was removed from this branch
whole (see the structural assessment in `docs/problems/`), and the phone's report
answers 404, which the phone treats as silence. A proper design must answer, before
any of it is built:

1. What an account may cause, and at what identity cost: how many feeds, how many
   fetches, how many transcribed minutes, and what stops a free account from being
   a thousand.
2. Unsubscribe, and the collection of rows nobody follows.
3. Per-feed bounds on episodes kept, transcribed and re-fetched, and on feed size.
4. Queue fairness between the operator's feeds, many accounts' feeds and
   now-playing, so one cannot starve or flood the others.
5. How a followed show becomes public ("featured"): an operator act, or a rule
   with an identity cost per account, never a bare count.
6. Private (tokenised, paid) feeds: they must never become a public address, and
   their transcripts are the subscriber's alone.
7. What the public may learn of what accounts follow: counts only, with a
   k-anonymity floor, and never who.
8. Everything public read by key, never from a page of the newest rows.

### 3.3 Transcripts from a device (the owner's idea; design only tonight)

Owner: "we can run youtube api calls for podcast transcripts from user devices
(safe; not server ip) to get transcripts." For shows that publish on YouTube the
phone would fetch the captions of a video it is playing, or one that matches a
subscribed show, and upload the timed text to the same transcript store. Nothing
fetches YouTube from the server. It needs a route that accepts a client-supplied
transcript, and a trust rule, because anyone could upload a wrong one: such a
transcript is stored with `source: "device:youtube"` and `verified: false`, is
never used to make a clip and never cited in the paper, and becomes verified
only when our own transcription of the same episode, or a second device's upload,
agrees with it on a sample of passages. The route, its trust rule and the
alignment between a video's times and the audio enclosure's are not built.

### 3.4 What strangers can cause (review of 2026-11-16)

Two independent seats found the same blocker in round 1: a reported feed URL made
the operator's worker fetch any address, from the operator's network position. The
reports of accounts are gone (3.2), but the worker still fetches URLs that a feed
file names (enclosures), so the guard stays.

- **What a URL may be.** A named public-looking host (no IP literal, `localhost`,
  `.local`, `.internal`, `.localhost` or `.localdomain`, trailing dot trimmed
  first, no single-label name), ports 80 and 443 only (`checkPodcastURL`, at
  episode intake and for the feed list); an episode dated more than 24 hours ahead
  is skipped.
- **What the worker will dial.** One client for every outside fetch: the dialer
  resolves the name itself and refuses every address in these ranges, IPv4 and
  IPv6 alike: `0.0.0.0/8`, `10/8`, `100.64/10`, `127/8`, `169.254/16`,
  `172.16/12`, `192.0.0/24`, `192.0.2/24`, `192.88.99/24`, `192.168/16`,
  `198.18/15`, `198.51.100/24`, `203.0.113/24`, `224/4`, `240/4`; `::/96`,
  `64:ff9b::/96`, `100::/64`, `2001::/32` (Teredo), `2001:db8::/32`, `2002::/16`
  (6to4), `fc00::/7`, `fe80::/10`, `ff00::/8`; IPv4-mapped forms are tested as
  their IPv4 address. It dials the address it checked (so a rebinding answer and
  a redirect meet the same check), allows at most 20 redirects and no proxy from
  the environment; each feed fetch has a 60 s deadline.
- **What ffmpeg may open.** `-protocol_whitelist file`, and a download that
  begins with a playlist is refused before ffmpeg runs.
- **The operator key** goes only over https, or http to this machine.
- **One bad item costs only itself.** `POST .../operator/podcasts/episodes` skips
  an invalid item (or one of a feed that is not featured), logs the first five and
  a count, records the rest and answers `{"recorded","new","skipped"}`; only a
  malformed envelope (none, or over 200 items) is refused. The worker drops the
  items the server would refuse before posting, and a failed chunk does not stop
  the chunks or the feeds after it.
- **Read by key.** The featured set is one small index document and a story's
  clips (its 12 newest) another. No public behaviour is derived from a page of "the
  newest N rows". A lost featured index while feed rows exist fails closed and is
  logged once per process. The story's index is written before its clips, and cut
  to 12 after them, so a failure between the steps leaves an id with no clip, which
  the read skips and a repeat of the write completes, and never drops an id whose
  clip is readable.
- **A quote is words that were said.** A stored quote is a prefix of what was
  said, cut on a word boundary with nothing added, and `PUT
  .../operator/podcasts/clips` verifies that it occurs in the episode's transcript
  within the clip's times, so the operator route cannot publish words nobody said.
- **Residual R10.** The worker's episode queue is a scan of the 5000 newest
  episode rows (logged when it truncates), not a query by need; with only the
  operator's feeds nothing can flood it. **R11.** A transcript replaced by a
  longer one (a different download of an ad-stitched file) leaves clips made from
  the old transcript's times in place until the clips are matched again by hand. **R12.**
  Everything about feeds reported by accounts is Part 2 (3.2). **R13.** The dial
  guard refuses `64:ff9b::/96` but not NAT64's other prefixes (`64:ff9b:1::/48`).
  **R14.** The quote check is a word-sequence `Contains` over the clip's
  segments, so a short quote can match inside a longer word; the producers of
  quotes are the operator's own code. **R15.** A story's one-clip-per-episode
  memory is its 12-slot index: after more than 12 later clips for one story an
  old episode can be asked about again.

### 3.5 Matching by hand (the operator's path)

No model call is made by any code in this repository for matching.

The candidates file carries third-party text (transcript passages, story
titles). To whoever judges it, a person or an agent, that text is data, never
instructions: a passage that addresses the judge is judged as a passage. What a
fooled judge can cause is bounded by the server (a clip must quote the
transcript inside its own times, on a featured feed, for a story on the front).

- `podcast-worker candidates --server S --api-key-file K [--stories 60]
  [--per-story 4] --out FILE`. It reads the front list (`GET
  /api/platform/publications/front`, public) and each story's text (the public
  `export` route, `format=txt`), the operator's episode list (`GET
  .../operator/podcasts/episodes`, all episodes, with `has_transcript`) and each
  transcribed episode's blocks (the signed-in `podcast-transcript` route); ranks
  with `internal/podcastmatch` (overlapping 120 s windows on a 60 s stride, BM25
  over normalised tokens with a tolerant match for names, a clip of 24 to 75 s (20 to 75 s until 2026-11-17) on
  segment boundaries, a quote that is a prefix of what was said); keeps, for each
  story, the passages of episodes published from 48 hours before the story's last
  update to 6 hours after it, at most one per episode, best first; and writes
  `{"generated_at", "candidates":[{story_route, story_version, story_title,
  story_dek, episode_id, show, episode_title, published, start, end, quote,
  passage, score}]}`.
- A person or an agent reads the file and sets `"support"` (0 to 1) on the
  candidates whose passage reports the same specific event as the story. That is
  the judgment, and it is made outside this code.
- `podcast-worker put-clips --in FILE`. It sends the candidates with support above
  0 through `PUT .../operator/podcasts/clips` in requests of at most 200; a request
  the server refuses is retried clip by clip, so one bad clip costs only itself;
  the refusals are printed and the exit status is 1 when there are any. The
  server's checks are unchanged: the story is on the front, the feed is featured,
  the quote occurs in the episode's transcript within the clip's times.

### 3.6 Automatic matching: NOT BUILT (owner decision)

The first version of this branch had a corpusd job that did the judging: commit
`9753eed1e` (and its later repairs), `internal/platform/podcast_match.go`, with
the gateway host-caller entry `podcast-clips`. In one call per run it put every
(story, passage) pair of the front's recent stories and the featured feeds'
newest transcripts into one Jev state, one yes/no `choice` per pair, split only
where `internal/jevbudget` names a vendor limit; a pair at P(yes) of 0.6 or more
became a clip; no judge, no clip. It was removed because the owner's rule is that
every Jev call goes through the one job entry point (`jevJob` in agentcore;
`scripts/check-jev-entry`, `docs/design/jev-batched-questions-2026-11-10.md`
section 10), and corpusd cannot import agentcore. Restoring it needs the entry
point shared with corpusd and the gate taught to admit a new ENTRY, which is a
change to the owner's gate: an owner decision. The old code is in git (restore it
by reverting the commit that removed it, the one that adds this section), and
`internal/podcastmatch` is kept for it.

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
| `POST /api/platform/operator/podcasts/episodes` | `{"episodes":[{"feed_url","guid","show","title","published","duration_seconds","enclosure_url","notes"}]}`, at most 200, `notes` cut to 2000 bytes; answers `{"recorded":n,"new":m,"skipped":k}`; an invalid item, or one of a feed that is not featured, is skipped, not refused |
| `GET /api/platform/operator/podcasts/episodes?need=transcript&since=<RFC3339>&limit=<n>` | `{"episodes":[{"episode_id","feed_url","guid","show","title","published","duration_seconds","enclosure_url"}]}`, newest first, episodes with no transcript |
| `PUT /api/platform/operator/podcasts/transcripts` | `{"episode_id","engine","audio_seconds","file_seconds","segments":[{"start","end","text"}]}`, at most 6 MB; `file_seconds` (optional) is the length of the whole file as downloaded and replaces the feed's duration as the episode's `duration_seconds`; answers `{"segments":n}` |
| `PUT /api/platform/operator/podcasts/clips` | `{"clips":[{"story_route","episode_id","start","end","quote","support"}]}`, at most 200; the story must be on the front list, the episode known, 5 to 120 s, inside the episode, quote cut to 280 characters; answers `{"clips":n}`; the quote must occur in the episode's transcript within the clip's times and is cut on a word boundary to 280 characters, and the episode's feed must be featured. The clip id is made from story, episode and the start rounded to the second, so a repeat replaces |
| `PUT /api/platform/operator/podcasts/feeds/listed` | `{"feeds":[{"feed_url","title"}],"listed":true}` (default true), at most 200: features (or, with `listed:false`, unfeatures) feeds; the only way a feed row comes to exist; the worker puts the seed list through it at each intake; answers `{"feeds":n}` |

Enclosure and feed URLs must be `http` or `https`, with no credentials in the
URL, at most 2048 bytes, a named host (no IP literal, `localhost`, `.local`,
`.internal`, single-label name) and no port but 80 and 443. The server stores
them and never fetches them; the worker does, under the policy of section 3.4.

### 4.5 `POST /api/platform/podcast-feeds`: not built

The phone's report of the feeds its reader follows (Part 2, 3.2) answers 404 from
the proxy, signed in or not, and reaches neither corpusd nor a computer.

### 4.6 `GET /api/platform/podcast-popular?limit=N`

Public, `limit` default 30, at most 100. Answer: `{"feeds":[{"feed_url","title","subscribers","latest_episode_title","latest_published"}]}`:
the featured feeds the worker has fetched and parsed, the one with the newest
episode first; `title` is the show's name as the worker parsed it; `subscribers`
is always 0 until Part 2 exists (the shape is kept so the phone's decoder is
unchanged).

### 4.7 `GET /api/platform/podcast-transcript?feed=<feed url>&guid=<episode guid>`

A signed-in session or API key. A feed that is not featured answers 404, as an
episode with no transcript does. Answer: `{"episode":{"show","title","published","feed_url","guid","audio_url","duration_seconds"},"source":"parakeet.cpp tdt_ctc-110m-q8_0","verified":true,"blocks":[{"start","end","text"}]}`.
The feed is matched by its normalised URL. `blocks` are the stored sentence or
pause bounded segments.

### 4.8 `GET /api/platform/podcast-clips`: the `feed` parameter

Optional and repeatable (at most 60); the answer's shape is unchanged.

## 5. Mutation classes and rollback

The slice is orange throughout: no red surface is touched (no gateway, tape, auth or
run-acceptance change). Clips reach the public read through the operator's hand
path (3.5).

- Platform store, public route and operator routes (new kinds, additive): orange,
  rollback revert; the objects are inert without the route.
- The programme's `podcast_clip` kind: orange (the route's behaviour; guest
  runtime, so a planned refresh on deploy); rollback revert.
- The worker and the feed seed: yellow.

## 6. Residuals

R1 no desk custodian (section 2). R2 no forecast claim per clip. R3 no
retention for episodes and transcripts. R4 ad insertion offsets. R10 to R15 (section 3.4). R7 no server-side claim for more than one worker. R5 the
window size is a guess to be scored against listening outcomes. R6 matching is by hand (3.5), so clips appear when the
operator runs `candidates`, judges and runs `put-clips`, not on a schedule; the
automatic job is not built (3.6).
