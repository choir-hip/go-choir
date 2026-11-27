# Local patches to Exact's iOS host

`mobile/.exact-src` is a gitignored checkout of `expo/exact` at the pin in `mobile/exact.pin` (`8552d20f9`, upstream `main`, 2026-11-26 23:02 -0800, since 2026-11-27). We carry **no fork**. Anything we change in it
is a patch here, applied in file-name order by `scripts/setup-exact.sh` after it checks out the pin (a patch that is already applied is skipped;
one that does not apply stops the script, so a pin move cannot silently lose it). Each patch says what it changes and why, below and in
`docs/reference/exact/capabilities.md`; the aim is to upstream it or drop it.

## 0001 the Markdown editor hides all syntax on iOS

Owner, 2026-11-16: formatting controls appear "rather than the markdown formatting being manually written". Exact's web editor hides Markdown
syntax away from the caret; its Apple host dims it (`MarkupEditor.swift` drew `h` ranges at 0.3 ink, list markers at 0.4) and hides only link
destinations. The styler already computes every hidden range (`Styled.hidden`, revealed only near a selection). The patch:
- asks the styler with no selection (`u32::MAX`, which `exact_markup_style` takes as "reveal nothing"), so every marker is a hidden range;
- marks those ranges `.markupHidden` (the mechanism link destinations already use: zero width, source kept, offsets kept), a bullet's source
  `-` drawn as a bullet glyph, a link's `[` hidden too;
- Backspace just after a block's hidden prefix (`# `, `> `, a bullet) removes the prefix, so the block loses its formatting;
- `MarkupSyntaxHiding.swift` holds the pure rules; AutoradioKit's `MarkupSyntaxHidingTests` compiles that same file.
Not changed: Return in a list and an empty item ending it (the crate's editing already does that), fenced-code lines and `---` (hidden/drawn as
the styler says), ordered numbers (shown as the number). Verified by `git apply --check` on the pin and by the Kit tests of the pure rules; **not
compiled or run** by the lane that wrote it: the first Exact build is the check.
Drop it when upstream's `host/apple/Sources/ExactMarkdown` hides all syntax. At `f53347503` (upstream `main`, 2026-11-20, 762 commits past
`de212e671`) it still does not: `docs/contract-grammar.md` (Markdown section) says macOS and iOS show "the syntax dimmed and each link's
`](destination)` hidden", and there is no prop that asks for more. **Carried across the 2026-11-20 pin move** [src]: the two files this
patch edits, `MarkupEditor.swift` and `MarkupHidden.swift`, are byte-identical between `de212e671` and `f53347503`
(`git diff de212e671 f53347503 -- host/apple/Sources/ExactMarkdown/MarkupEditor.swift ...` is empty), and so is `host/apple/src/markup.rs`
(`exact_markup_style`, whose `u32::MAX` selection the patch relies on), so every hunk's context matches. `git apply --cached --check` of this patch against a scratch index
read from `f53347503` (`GIT_INDEX_FILE=<scratch> git read-tree f53347503`, no working files touched) **passed on 2026-11-20** [run]; `MarkupSyntaxHiding.swift` is a new file that upstream's `Sources/ExactMarkdown` directory target (path-based, no file list)
compiles. Still **not compiled or run** by us.

**2026-11-27, pin `f53347503` to `8552d20f9`:** unchanged file; `MarkupEditor.swift` and `MarkupHidden.swift` are byte-identical between the two pins (`git diff` of them is empty), `markup.rs` differs only by an atom's `ariaLabel` (not `exact_markup_style`), and `git apply --cached --check` against a scratch index read from `8552d20f9` passes. Applies by `git apply --check`; not compiled or run.

## 0003 Media3 and the playback service for an app that plays in the background

**Re-made against `8552d20f9` on 2026-11-27; applies by `git apply --check`; not compiled or run.** Upstream moved the decision that links Media3 out of `scripts/agent-android.mjs` (`assembleApk`) into `androidLibraryArgs` in `scripts/android-manifest.mjs` and still reads only `uses.includes('video')` or a development build; it does not look at `audio_session` (`host/android/app/build.gradle.kts:47` still needs `video && audio_session == "playback"` to add the service). So the patch is still needed, and its one change now sits in `androidLibraryArgs`, which `assembleApk` and `app-cli.mjs` both call. The description below says what it did at the old pin; the file it edits is now `scripts/android-manifest.mjs`.

Autoradio's deck plays spoken and remote audio under a media session. Exact linked Media3 and its `PlaybackService` only when the
plan made a `video` node, or for a development build without Maven libraries; an app whose `audio_session` is `playback` had no
service and nothing kept the sound alive with the activity gone. The patch:
- `scripts/agent-android.mjs` (`assembleApk`): `-Pvideo=1` also when `app.json`'s `audio_session` is `"playback"`, so the app links
  the `:video` project (Media3) and the `backgroundPlayback` blocks in `host/android/app/build.gradle.kts` add the playback service,
  its permissions and its manifest entry. Nothing changes for an app that neither plays in the background nor makes media nodes.
- Verified by building the Autoradio lab-one APK and driving it on the Motorola: the service starts (`Background started FGS`), the
  media session reports our metadata and position, and playback survives Home (`docs/evidence/android-a2-deck-2026-11-26/`).

## 0002 a bound package's shared libraries reach the APK

**Re-made against `8552d20f9` on 2026-11-27; applies by `git apply --check`; not compiled or run.** Upstream stages only its own libraries (`stageGpuLibraries`, `stageSoundLibrary` into the app's `rust/jniLibs`, `build.gradle.kts:119`); it still passes no bound package's `jniLibs`, so the patch is still needed. Same two files, same checks (ELF magic, LOAD segment at least 16 KiB aligned, must resolve inside the package, only `.so`), re-anchored on the new `extensionArgs` (now exported) and the new `extensionDirs` line; it reuses upstream's exported `loadAlignment`.

Autoradio's Android models are native code (`libautoradio_models.so` plus llama.cpp and ONNX Runtime), and the package's
`android` directory is where its Kotlin lives. Exact passed only a package's Kotlin to Gradle, so a package could not ship a
`.so`. The patch:
- `scripts/agent-android.mjs` (`extensionArgs`): each bound package's `jniLibs/<abi>/` directory, when it exists, is
  checked (it must resolve inside its package; only `.so` files; each file starts with the ELF magic and has a LOAD
  segment at least 16 KiB aligned, so a zero-LOAD or non-ELF file cannot slip through) and passed as
  `-PextensionJniLibs`; `mobile/scripts/check-android-model-runtime.py` independently refuses a missing, stale or
  foreign staged runtime before an Android build;
- `host/android/app/build.gradle.kts`: that property adds each directory to `sourceSets["main"].jniLibs`, so the libraries
  are packaged uncompressed where `System.loadLibrary` finds them.
- The check refuses a 4 KiB library by name before Gradle runs, so a package cannot ship an APK that fails on a 16 KiB-page
  device. Not changed: the app's own Rust libraries, GPU modules and assets, which keep their existing staging.
- Verified by `git apply --check -R` against the pin at `f53347503` with 0001 applied (2026-11-26) and by building the
  Autoradio lab-one APK, whose `lib/arm64-v8a` carries the package's five libraries, all 16 KiB aligned, and by running
  that APK's JNI runtime on the Motorola (`docs/evidence/android-a6-model-runtime-2026-11-26/`).
