# The App Store build guard checks the route, not the artifact (2026-11-24)

Structural assessment after three review rounds on `lane/store-build` without
convergence (CLAUDE.md, "Three rounds without convergence").

## Problem

Each round closed one way a developer build could be uploaded as a store
build, and the next round found another:

1. Round 1: the guard was a denylist of verbs; a verb it did not name passed.
2. Round 2: inverted to an allowlist; the marker could still be set by hand.
3. Round 3: the allowlist and marker hold, but the guard checks the command
   line and not the tree the build reads. After any development build the
   staged module still holds the developer launch hooks
   (`Models_EmbedDeviceTrial.swift`, `Models_EmbedTrialFixture.swift`, the two
   `runFromLaunchArguments()` calls), `exact.sh` never restages, and the
   readback (`check-store-ipa.py`) reads only Info.plist keys and the profile,
   so the `.ipa` passes every check with the hooks compiled in.

## Belief

The defect is the shape, not the third hole. A guard on inputs has to
enumerate every route to a build; a check on the output has to recognise one
bad artifact. We have been enumerating routes.

## Changed approach

- **One route.** Only `archive-store.sh` may produce a store archive, and it
  does the store staging itself, every time, immediately before the build.
  `exact.sh` refuses `--archive` outright; the hand-set marker is deleted.
- **The artifact is what is judged.** `check-store-ipa.py` inspects the
  compiled binary: it refuses an `.ipa` whose executable contains any symbol
  or string on a list of developer-only names (the launch-hook types and
  their argument strings), and it reads the signature's entitlements with
  `codesign`, not only the embedded profile. `upload-store.sh` runs the same
  check on whatever file it is handed, so a foreign `.ipa` gets the same
  judgment.
- **The list cannot rot silently.** A test builds the developer staging and
  asserts every name on the list is present in it, so a renamed hook fails
  the test instead of slipping past the check.
- `AR_APP_JSON` is refused by the two store scripts; an empty first argument
  is refused by the guard; the build-number record is claimed atomically.

## Remaining error

No native build has run, so the binary check is unproved until the lead's
first archive. That first archive is also the proof: the lead runs the check
on a development `.ipa` (must refuse) and on the store `.ipa` (must pass).
If round 4 is sent back for the same class again, the scripts are dropped
and the archive is made by hand from Xcode for the first submission.

## Round 4 review (2026-11-24): the artifact check was itself incomplete

Both blocking findings were about the check, not the route. (1) It searched
`CFBundleExecutable` only; Exact compiles the app module into
`libexact_modules.dylib` (`host/apple/build.mjs:1099`, `compose.mjs:23`), and
on an iOS production build that is wrapped as
`Frameworks/ExactModules.framework/ExactModules`
(`moduleLibraryLinkArgs({ wrapped: forIos && production })`), so the hooks
were most likely never in the file that was searched. It now searches every
Mach-O of the unpacked bundle. (2) `codesign`, `ditto` and `security` were
found on the caller's `PATH`; they are now `/usr/bin`'s, and tests reach the
checkers' stubs only through a `--tools` argument the store scripts never
pass. Belief: the shape is now right (judge the whole artifact with the
system's tools); the remaining error is still the real binaries, which have
not been seen: the first archive carries a negative control, now a script
(`mobile/scripts/store-negative-control.sh`: a release, archive-kind,
development-staged build must be refused, naming a listed hook and its
file) and a positive control (the store archive's summary names the module
binary and a product name found in it).
