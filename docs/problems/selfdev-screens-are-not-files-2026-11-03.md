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

## Screens are required, and before/after for what exists (owner, 2026-11-04)

Gap found: the shot review returned nil when a bundle carried no screens
(FM8), so a candidate with no evidence of what it did to the UI went through
unchallenged. Owner rulings, 2026-11-04, about 17:45Z: "screenshots are a
requirement for self-development, period. For anything that is changing the
UI, the look and feel, it needs a screenshot." Widened about 18:00Z: "even
back-end changes which touch on the front-end should have a screenshot just
to prove that there's still continuity. No regressions... For anything which
is modifying anything that already exists, like making an updated version of
an app, you should take a screenshot before and after... in order to publish,
you should need to show before and after."

The rule, in `internal/capsule/screens_required.go`, enforced in Freeze
beside the screen staging (`Executor.StageGrantedRelease`):

- `screens_required`: every candidate carries at least one screen, except a
  candidate whose changes are all exempt (`ScreenExemptPath`): docs, `.md`,
  tests (test directories, `_test.go`, spec and test files, the Playwright
  config), build output and tool state (`dist`, `node_modules`, `.claude`,
  `test-results`, `coverage`). Backend changes need a screen too. No switch,
  no bypass.
- `before_after_required`: a candidate that modifies or deletes a non-exempt
  file that exists on the base carries a before/after pair of one view,
  named `NN-before-<view>.png` and `NN-after-<view>.png` (they pair on
  `<view>`; a name with neither word is an after screen). A candidate that
  only adds files (a brand-new app) needs after screens only.
- The before shot is the base build: the engineer shoots the views it will
  touch before its first edit (the capsule has no second copy of the base;
  `choir-shot.mjs` builds the working tree). Freeze refuses a before shot
  modified after the first source edit (`refuseLateBeforeShots`), and does
  not call a before shot stale. This checks file time, not content: a before
  shot of an edited tree whose file time was preserved would pass.
- Why Freeze: it is the one producer of a bound bundle, and the source stack,
  package release drafts and S12 runtime install all start from one, so
  nothing reaches Verify, apply or a release without having passed; the
  engineer still holds its capsule, so it reads the reason in-loop and shoots
  and Freezes again in the same round. The engineer is told up front in the
  capsule browser recipe (`run_context.go`) and the Claude delegate brief.
- The shot review (`selfdev_shot_review.go`) keeps a pair in one call and
  asks the judge, for the after image, whether anything outside the intended
  change regressed against the before image (it answers question 2).
- Publish: a package release draft whose parent is not the empty tree (an
  update) is refused `before_after_required` unless its media holds a pair
  (`packageUpdateNeedsBeforeAfter`). The `appstore` branch's rule that a
  listed release carries a screenshot is separate and complements it.

Dated future intents (not built):

- Video (owner, 2026-11-04): a requirement later, "once we get it working
  and we have enough resources". It is not required now.
- Overlay builds (owner, 2026-11-04, design intent): build a change as an
  overlay so the old and the new both exist and the new supplants the old at
  freeze or publication, which would give the before shot from the old
  build directly instead of from the engineer's discipline.

Review fixes (tier-2, 2026-11-04): the shot review pairs whichever side it
meets first (screens are path-sorted, so the after shot comes first) and
never splits a pair across calls; a before shot is excluded from question 2
(it is the base build and cannot show the change) and is judged only as a
real, readable screen; a backend change's after shot is a continuity screen
judged against its before shot, not the request text; the package-update
pair is proved on the bundle's screens, the media keep a pair through the
cap, and a pair of identical pixels names its digest once; Freeze asks for
the pair when files are added to an app package that already exists on the
base, so Freeze and publish agree; the first-edit time ignores exempt
paths, so writing tests first does not make a before shot late.

Named residual: before-shot integrity is documentary, not enforcement. Freeze
compares file times only (`touch -d`, or copying an after shot, defeats it, and
a deleted file has no time to set the first edit). Until the overlay build
lands, a before shot is the engineer's claim with its bytes pinned. A
host-attested receipt is not built now.
