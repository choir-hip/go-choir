# Engineering's screenshots are not files; platform capture should not exist (2026-11-03)

Status: open. Mutation class of the fix: orange (capsule Freeze, run
acceptance evidence, vmctl grant); the deletions touch red surfaces.

## Owner direction

- "Why no chromium in d@e? It should be accessible same for b@c. We built
  layers for this. Then screens that Eng takes should be saved as files
  texture can read." And: "We didn't need any special code for screenshots
  last week."
- "Get rid of the screenshot code you added."
- "every computer should have visual capture. That shouldn't be anything
  special."
- "No platform capture. That shouldn't exist. That's not part of the plan."
  The engineering desk takes the screenshots, of real content, and they are
  saved as files that Texture can read.

## Problem 1: d@e's capsules had no Chromium

Evidence (Node B vmctl journal, metadata only):

- `nix/node-b.nix` sets `guestToolProfile = "...;browser=grant:visual-capture;..."`
  and d@e (computer-59c902c5…, vm-70070cf7…) is the one computer in
  `guestLayeredComputers`. A layered guest's browser layer attaches only
  when the computer holds the visual capture grant
  (`internal/vmmanager/tool_layers.go`, `toolAttached`), and d@e was not in
  `CHOIR_VISUAL_CAPTURE_COMPUTERS` (it is class `primary`, not `test`).
- Every d@e launch on 2026-11-02 logged
  `tool layers: toolchain attached; browser,delegate,office empty`
  (04:38, 19:35, 19:38, 19:57, 22:41 UTC). Its engineer looked for the store
  Chromium and spent 7 minutes trying to build a browser by hand.
- b@c (computer-6c50d6e5…) runs the single image, which carries Chromium in
  its store whatever the grant says, so its engineer found it.
- 00548f21a added d@e to the grant list; deployed in db2ee2dd6 (vmctl
  restarted 23:57:47 UTC). d@e had not cold booted since. A vmctl refresh at
  2026-11-03 00:25 UTC (caller `operator.browser-layer-d-at-e`) relaunched it:
  `tool layers: browser,toolchain attached; delegate,office empty`, and
  its jail holds `tool-browser.img` (764 MB) with
  `…-chromium-155.0.8059.79/bin/chromium`.

So the cause was a grant gate on a tool. The browser is not a privilege.

## Problem 2: the screenshots an engineer takes are lost

An engineer's own Playwright screenshots stay in its capsule, which is
revoked at the end of the round. They reach Texture only as prose. The only
screenshots that left a capsule were the platform's: `choir-capture`
(internal/visualcapture, cmd/choir-capture, 5d4c305ec and 3378d38ca on
2026-10-24 and later), a manifest bound to the frontend build, a capture job
receipt, Freeze's `visual_evidence`, the `visual_captured` precommit, the
`choir self-dev visual` listing, and Texture's evidence embeds. That path
screenshots a fixture of the release frontend, not real content, and it is
gated by the grant above.

## Remaining error field

- The platform capture path is deleted outright: the package, the tool, the
  job receipt binding, Freeze's visual fields, the listing routes, the
  `visual_captured` precommit and the grant. Nothing in verify, judge or
  acceptance may wait on or refuse for the missing evidence, which would be
  a new stuck state; a `visual_captured` precommit already on a tape settles
  `unresolved` with the reason that the rule was retired.
- In its place: the engineer writes PNGs to `/var/lib/artifact/screens/`
  in its capsule. Freeze stages them into the bundle (count, bytes, PNG
  decode and secret scan bounds), binds each by sha256 in the bundle record,
  and writes them into the computer's file tree at
  `Self-development/<operation>/screens/<name>.png`. Texture embeds those
  files (the embed names the file's path and its sha256), and the judge's
  `inspect_bundle` reads and re-hashes the same bytes.
- The browser layer goes to every computer (`browser=*`); no grant.
- Accepted trade (review, 2026-11-03): screens are no longer bound to the
  frozen frontend. The platform capture screenshotted a fixture of the exact
  release frontend, so its image was tied to the bytes that shipped. An
  engineer's screenshot shows real content as the engineer saw it, and the
  bundle binds only the image's own sha256, not the frontend build it
  depicts. A screen can therefore be stale against a later edit, or show a
  page the release does not contain. The judge and the owner read it as the
  engineer's claim with its bytes pinned, not as proof of the release.
- Review fixes landed with this entry: the capsule-owned `screens`
  directory is prepared beside `release` (host uid 65534 could not create
  it); a replayed Freeze re-saves screens a crash left unsaved, by digest;
  Texture embeds only screens the asset store holds; the Files-tree writer
  refuses symlinked directories; screens stage under a path a release file
  cannot occupy (`evidence/` is reserved) and into a directory that must be
  new; `MaxScreenPixels` is 4096x8192 and the blank check compares every
  pixel; directory fsyncs follow the renames.
