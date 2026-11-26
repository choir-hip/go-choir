# Android A3: in-app source WebView and external hand-off, 2026-11-26

## Artifact

- Runtime source: `5394124a2` (`mobile: add the package-first Android extension`), with review/docs commit `59a5330e7` on `origin/main`.
- Exact pin: `f5334750386192245ee76add13a67ab51fb4e2fd`.
- Package: `news.choir.autoradio.lab.one`, versionName `1.0`, versionCode `474177`, minSdk 31, targetSdk 36.
- Device: `ZY22H5K8QR`, moto g stylus 5G (2023), Android 14, arm64-v8a, SM6450.
- APK SHA-256: `a2a920bb8312509c64b1e7c95ee5e599ab2381c1e273fb035132d68b0e912a3c`.
- Signing certificate SHA-256: `dfe9767c642a2f864b297049bde97c0836cb58d5ce516ffe7a7da11f6fec9ea7` (Android Debug).

The installed APK hash was read from `base.apk` on the device and matches the build artifact. `apksigner verify --print-certs` passed with `JAVA_HOME` set.

## Reproduce

Run from the lead's build tree with the physical Motorola connected:

```sh
scripts/proofs/android-a3-web-view.sh \
  /path/to/evidence \
  a2a920bb8312509c64b1e7c95ee5e599ab2381c1e273fb035132d68b0e912a3c
```

The proof uses Exact's Android agent with `--bind autoradio` and a named storage directory. It discovers the first current Paper story, opens source 5 through shared testIds, asserts that the tree contains `NativeView [reader-web-view]`, captures a screenshot, expands the source controls and invokes `reader-web-safari`, then asserts that Android resumed `com.android.chrome/org.chromium.chrome.browser.ChromeTabbedActivity`. A second drive closes the in-app source, and the proof force-stops and cold-launches the normal app to check teardown/relaunch.

No coordinate taps are used. ADB is used only for package/artifact readback, activity readback and process lifecycle.

## Observed result

- The source opened inside Autoradio as `NativeView#3236 [reader-web-view]`; the reader host read `pravda.com.ua`.
- `web.png` shows the Ukrainian Pravda page rendered in the app with the Autoradio reader controls.
- `reader-web-safari` invoked the package's `readerOpenExternal` path. Android's top resumed activity became Chrome's `ChromeTabbedActivity`.
- `reader-web-close` removed the in-app source layer. `closed.png` shows the story reader again.
- After `am force-stop`, a normal `MainActivity` launch returned `Status: ok`, `LaunchState: COLD`, and a live process id. No fatal exception appeared in the following logcat read.

The complete agent outputs are `web.log`, `external.log`, and `close.log`. The external activity readback is in `external-activity.txt`; the cold relaunch readback is in `relaunch.txt`.

## What this proves

The package-first Android `web-page` view renders a real source page, the shared reader can close it, and the external URL send reaches the platform browser through `ExtensionContext.leave`. The repaired event-ordering and teardown paths survived a physical open/close/force-stop cycle.

## What this does not prove

This is not full A3 or A7 parity. File import/open/share, audio hand-off, reader follow/aim, podcast clip location, mail, Textures, notebook, takes, themes, onboarding, model settings and account controls still need their own physical rows. The external matrix is limited to one HTTPS source (`pravda.com.ua`); malformed schemes and every external-app target are covered by code review/tests, not a physical matrix. The browser was not cleared or modified.

## Rollback

Revert `5394124a2` (or the later commit that changes the Android package) and reinstall the previous known-good lab APK. No server, iOS, web or phone data migration is involved.
