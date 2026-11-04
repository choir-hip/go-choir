# A capsule's Chromium cannot load podcast artwork or audio (2026-11-04)

Mutation class: green (this doc). Fix: orange (scripts/choir-shot.mjs and fixtures; no runtime or allowlist change).

## Evidence

live8 trace, deepseek test arm computer-e80d229b0d78c35d47656b8c66426a43, op
selfdev-694b6c10232d93834ba8200093090325, Podcasts r1. Verify failed on "shot 01
shows broken-image glyphs where subscription artwork should be"; the owner-side
gate agreed. The arm's capsule-egress log refused the artwork and audio hosts,
each with "host is not on the L2 ecosystem allowlist": ssl-static.libsyn.com (92),
static.megaphone.fm (91), traffic.libsyn.com (31), archive.org, itunes.apple.com.
The page said "Fixture feed: this computer cannot reach the podcast directory or
the feed hosts, so this show is read from a captured copy of its real RSS feed."
That text is the engineer's own page; the platform has no podcast fixture.

## Cause

The capsule's egress is closed on purpose (the L2 ecosystem allowlist is a
security surface). A feed can be captured into the app, but the artwork and audio
URLs inside it are remote, and the capsule's Chromium cannot fetch them. The
catalog fixture solved the same shape for catalog media (scripts/fixtures/
capsule-catalog-media); podcasts had none. Whatever the engineer does, a Podcasts
round shows broken-image glyphs in every shot, and the judge fails it.

## Fix (fixture, allowlist not widened)

- scripts/fixtures/capsule-podcast/: 5 synthetic shows (fictional titles and
  authors, RSS written by the generator with itunes tags and 3 episodes each,
  generated 200x200 covers, one generated 20 s tone), written by
  scripts/generate-podcast-fixture.mjs. No third-party cover art or feed text is
  stored in this open-source repo. URLs keep real-looking shapes on hosts the
  fixture owns under the reserved .invalid TLD (feeds./art./media.podcast-fixture.choir.invalid).
- choir-shot answers a capsule Chromium request for any manifest artwork URL or
  episode enclosure URL (and the feed URL) from those files (audio with Range),
  shows the "Podcast fixture, not live" badge, and logs once per host any other
  external image or audio URL it cannot serve.

## Failure modes

1. A shot's own routes() must still win over the fixture (registered first).
2. Same-origin and data/blob URLs must never be intercepted.
3. An audio element seeks with Range requests; a 200-only answer breaks -15/+30.
4. An unmapped host must not be silently aborted: logged once per host.
5. Artwork URLs differ by query string between a feed and its rewrites: match exact, then without the query.
6. No third-party content and no real podcast host in the fixture (a test asserts every fixture host ends in .invalid).
