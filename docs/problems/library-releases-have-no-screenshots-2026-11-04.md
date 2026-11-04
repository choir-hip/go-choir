# Library releases have no screenshots (2026-11-04)

Status: problem recorded and design chosen; the build follows in the next
commits.

## Problem

Owner, 2026-11-04: "Key thing. Library app needs screenshots. Basically it's
an App Store. Ideal would be playwright video tbh. Stretch goal."

The Library app is the computer's app store: every published package release
is public to every signed-in owner (8e2814b6f), and installing is the owner's
choice. A store page with no pictures cannot show what an install gives. Today
a release cannot carry any.

- The record (`internal/pkgrelease/record.go`) holds the package subtree's
  three patches, the manifest, the publisher, the operation's receipts and the
  precommitments. Its package doc says "The record carries no executable bytes
  and no evidence assets". It decodes strictly, so a field it lacks cannot be
  added on the side.
- The catalog (`GET /api/catalog/packages`, `.../packages/<name>/releases`,
  `.../releases/<id>`, `internal/proxy/packages.go` relaying
  `internal/platform/packages.go`) serves only that record and its listing.
  Nothing serves image bytes.
- The screens exist. A round's engineer takes them with
  `scripts/choir-shot.mjs` into `/var/lib/artifact/screens/`. Freeze stages
  them in the bundle bound by sha256 (`internal/capsule/screens.go`,
  `CapsuleEffectBundle.Screens`). The runtime saves them into the computer's
  files and asset store (`internal/agentcore/selfdev_screens.go`), and the
  judge re-hashes them before apply. They stay on the publisher's computer.
  Publish (`packageReleaseDraft`, `HandleComputerPackagePublish`) never reads
  them.
- There is no video. choir-shot writes only PNGs, and Freeze refuses any file
  in the screens directory that is not a plain `.png` name. Playwright's
  `recordVideo` encodes with an ffmpeg binary at
  `$PLAYWRIGHT_BROWSERS_PATH/ffmpeg-<rev>/ffmpeg-linux`. No guest image
  carries one: the browser layer is only the store Chromium
  (`nix/guest-tools.nix`).
- The capsule fixture (`scripts/fixtures/capsule-catalog.json`, generated from
  corpusd's handler) has no media. A Library round cannot render a gallery
  against realistic data.

## Design

Extend the mechanisms that exist; add no parallel store.

1. **The record names its media; the release id binds them.** `Record` gains
   `media` (omitempty, so every record published before it canonicalizes and
   hashes as before). An item has these fields:
   - `kind`: `screenshot` or `video`;
   - `name`;
   - `media_type`: `image/png` or `video/webm`;
   - `sha256`;
   - `bytes`;
   - `width` and `height` (screenshots only).

   `Validate` bounds the set: at most 8 screenshots of at most 2 MiB each, and
   at most 1 video of at most 8 MiB.
2. **Only the round's accepted shots.** The guest draft takes media only from
   the applied operation's bound frozen bundle, the same bundle whose source
   patch it publishes and the judge accepted. It never takes them from a path
   or from a desk.
   - It skips a blank, oversize or over-tall screen.
   - It takes the first 8 by name.
   - It reads the bytes and re-hashes them before naming them.

   A bundle with none drafts a release with no `media`, which publishes as
   before.
3. **Bytes travel like the patches.** The proxy fetches each named item from
   the publisher computer: `GET /api/computers/<id>/packages/operations/<op>/media/<sha256>`
   answers only a digest that operation's bound bundle names. The proxy then
   does three things:
   - verifies the bytes against the record: the hash, the length, a PNG
     decode with the stated size, or the WebM magic;
   - PUTs them to corpusd `/internal/platform/packages/media/<sha256>`, a
     content namespace of its own, `package-media`, with its own cap, which
     leaves the 2 MiB patch cap untouched;
   - publishes the record.

   corpusd re-reads and re-verifies every item before it lists the release.
4. **Public like the release.** Two routes serve the bytes:
   - corpusd `GET /internal/platform/packages/releases/<id>/media/<sha256>?owner_id=`
     answers under the release's own visibility rule. A withdrawn release's
     media is its publisher's alone, and an unseen release answers as missing.
     It answers only a digest the record names.
   - The proxy serves it as `GET /api/catalog/releases/<id>/media/<sha256>`
     with `Content-Type` from the record, `X-Content-Type-Options: nosniff`,
     and `Cache-Control: private, max-age=31536000, immutable`.

   Each release view gains `media`, always an array (empty means "no
   screenshots"), with each item's `url`. Each catalog entry gains
   `head_media`, the head release's items, so a gallery's grid needs no
   second read.
5. **Video, the stretch goal.** choir-shot `--video` records each shot with
   Playwright's `recordVideo`:
   - the length is capped at 20 s, and the file at 8 MiB;
   - the file is WebM, saved next to the screens as `<name>.webm`;
   - only one video is kept per run.

   Freeze stages at most one `.webm` from the screens directory into the
   bundle's `video` field. It checks the file in four ways: the name, the
   8 MiB bound, the EBML/WebM header, and the secret scan.

   The browser layer gains `ffmpeg-headless`, which carries VP8. Capsule jobs
   get its path as `CHOIR_FFMPEG`, and choir-shot points Playwright's registry
   at it through a private `PLAYWRIGHT_BROWSERS_PATH` shim.

   Memory: the heavy measured choir-shot run peaked at 1,684 MiB under the
   2,048 MiB default lease
   (`docs/reports/playwright-in-capsules-memory-2026-10-15.md`). Playwright
   runs ffmpeg with `-threads 1 -deadline realtime` on JPEG frames, which costs
   tens of MiB, so one video run fits the lease. A video run on a capsule
   measures its own peak; the measurement is an open item below until it does.
6. **The fixture shows a gallery.** The catalog fixture test publishes
   releases with real generated PNGs through the real handler. The fixture
   then carries `media` and `head_media`, with the bytes beside it as files.
   choir-shot answers `/api/catalog/releases/<id>/media/<sha>` from those
   files, and one release has no media, so the "no screenshots" state renders
   too.

## What stays unchanged

- A release with no media publishes exactly as before (same id rule, same
  routes).
- The judge sees only the screens. A video is staged, bound and published, and
  no judge looks at it before apply. It is the publisher's walkthrough, and
  only the secret scan checks it.

## Rollback

Revert the commits. Records published with `media` stop decoding on the
reverted host, and their listings answer as unreadable. Those releases need to
be withdrawn and re-published without media. No record without media is
touched.

## Open items

- Measure a `--video` run's peak inside a capsule. Until then, the claim that
  it fits the 2,048 MiB lease is an estimate.
- The judge does not view the video.
- No gallery UI ships on main. The Library rounds build it against the API
  above.

## Video on the owner's iPhone: byte ranges and codec (2026-11-04)

Two separate things stand between a Library video and the owner's phone.

1. Byte ranges (fixed in the same commit as this note). Safari's `<video>`
   will not play a file from a server that ignores `Range`. The media relay
   in `internal/proxy/packages.go` now serves through `http.ServeContent`
   over the bytes it already holds (corpusd's answer is capped at
   `MaxMediaBytes`), so it answers 206 with `Content-Range`, sends
   `Accept-Ranges: bytes`, and honours `If-Range` and `If-None-Match` (the
   digest ETag, 304). The visibility check, the two media types, nosniff and
   `private, no-cache, must-revalidate` are unchanged. corpusd keeps
   returning whole items to the proxy; ranges are the proxy's job.
2. Codec (not fixed, not verified on a device). Playwright records WebM
   with VP8 (no audio). From documentation and memory, not from a test on a
   phone: Safari on macOS plays VP8/VP9 WebM from 14.1, and iOS Safari added
   WebM container support in 17.4; older iOS plays no WebM. So a Playwright
   VP8 WebM is expected to play on an iPhone on iOS 17.4 or later, but this is
   unconfirmed here. Playwright's recorder also writes a live-streamed WebM
   (often no duration or seek cues), which Safari can play badly or refuse
   even when the codec is supported. The honest status is "unknown until the
   owner's phone plays one". If it does not, the fix is a transcode at
   publish time to H.264/AAC MP4 (needs ffmpeg in the capsule) and a new
   media type, not more header work.
