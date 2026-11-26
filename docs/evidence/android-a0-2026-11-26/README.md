# Android A0: current-main Autoradio on the Motorola, 2026-11-26

## Artifact

- Source: `d228f19bae4a89bb6091f6bde557d300ddda17d2` on `origin/main`.
- Exact pin: `f5334750386192245ee76add13a67ab51fb4e2fd`.
- Package: `news.choir.autoradio.lab.one`, versionName `1.0`, versionCode `474084`, minSdk 31, targetSdk 36.
- Device: `ZY22H5K8QR`, moto g stylus 5G (2023), Android 14, arm64-v8a, SM6450, 3.59 GB RAM.
- APK SHA-256: `46b18be1b3344256746b53d677f7656c9e28b6f28c5b10937009d90d66941ac6`.
- Signing certificate SHA-256: `dfe9767c642a2f864b297049bde97c0836cb58d5ce516ffe7a7da11f6fec9ea7` (Android Debug).

## Reproduce

From the lead's build tree, under the Mac build lock:

```sh
/usr/bin/lockf -k -t 7200 ~/.cache/autoradio-mac-build.lock \
  env AR_VARIANT=lab-one ANDROID_SERIAL=ZY22H5K8QR \
      JAVA_HOME=/opt/homebrew/opt/openjdk@21/libexec/openjdk.jdk/Contents/Home \
      ANDROID_HOME="$HOME/Library/Android/sdk" \
      mobile/scripts/build-lab.sh android --install

env AR_VARIANT=lab-one ANDROID_SERIAL=ZY22H5K8QR \
    JAVA_HOME=/opt/homebrew/opt/openjdk@21/libexec/openjdk.jdk/Contents/Home \
    ANDROID_HOME="$HOME/Library/Android/sdk" \
    mobile/scripts/exact.sh agent android --storage a0-current \
    'clock +6000 real' 'clock data' tree 'tap bar-mark' \
    'clock +600 real' 'clock data' 'tap place-paper' \
    'clock +4000 real' 'clock data' tree \
    'screenshot <path>/paper.png' logs
```

The first native build refused to proceed until Exact's Maven dependency lock
and verification metadata existed. `bun exact.mjs android --write-locks`
generated `mobile/AutoradioLab/gradle.lockfile` and
`mobile/AutoradioLab/gradle-verification.xml`, then built and installed the APK.
Those two files are committed with this evidence.

## Observed result

- The package manager read back versionName 1.0 and the expected signing
  certificate; `apksigner verify --print-certs` passed.
- The Exact Android agent launched the app, opened the launcher, opened the
  Paper, fetched 100 front stories from staging, drew story rows and captured
  `paper.png`. The tree held 28 `paper-story-*` pressables in its mounted window.
- The shared reader opened a story through the shared page path.
- The run sheet restored and the signed-out writer placed four paper stories
  ahead (`radio: pulse ... ahead=4`), but the native deck remained absent.
- The drive logged 39 substituted reads across 17 distinct package sources:
  `barFacts`, `choirAuth`, `choirPlayer`, `conversationEntries`, `deckState`,
  `importedFollow`, `pageRequest`, `radioNote`, `radioView`, `readerAim`,
  `readerFollow`, `retrievalAsk`, `settingsFacts`, `takesRecorder`,
  `voiceCards`, `voiceProfile`, and `voiceSetupFacts`.
- Android logged 48 `No value for extension` errors from the Apple-style
  app-module op bus. This is the concrete seam the package-first Kotlin
  `ExactExtension` must replace; copying `ModelsEntry.swift` is not the route.
- No `NotSupportedError` line appeared in this drive. That is not parity: the
  package sources were substituted and the app-module calls failed.

`drive.log` is the complete agent output (1,930 lines). `paper.png` is the
physical-device screenshot.

## What this does not prove

Sign-in, account wipe, radio or podcast sound, background playback, media
controls, mail secrecy, takes, hands-free voice, model execution, or full page
parity. The older `.lab` install was left untouched. No emulator was used.

## CI and landing

The wrapper commit `d228f19ba` passed the whole shared Bun suite locally
(1,782 pass, 1 skip, 0 fail), `go build ./...`, and CI's Autoradio Shared-Code
Tests, Docs Truth Check, frontend build, heresy detector and deploy-impact
classifier. CI run `45761096260` remained red on the pre-existing
`check-switch-registry` failures for five expired E1 arms; the staging deploy
was skipped. `scripts/land` reported the same gate before any push. The tested
candidate was pushed directly, and the push hook's guest-security-epoch check
passed. No staging behavior change is claimed for this mobile-only wrapper.
