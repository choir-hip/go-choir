# The owner's iPhone was stuck in demo mode (2026-11-20)

Status: demo mode deleted from the product (station D2); the leftover cleanup is the one part kept; not yet seen on the phone. Class: orange.

## Problem

The owner's iPhone showed sample stories and offered no way to sign in. Demo
mode had been switched on by one tap on Settings > Developer > "Demo mode",
which wrote the `AutoradioDemo` flag into UserDefaults. The flag survives
app updates, and while it is set
the app reads a placeholder key instead of the Keychain, so the real
credential was never consulted.

## Evidence

- `Demo.isOn` read the stored flag with no check for where it ran.
- The Developer row and its `demoSet` send were drawn on every build.
- Demo launches also seeded a sample book (`ReaderSample.id`) and wrote
  onboarding, edition topics, voice setup, open pages and signals records.

## Belief

The flag is the whole cause. Demo mode is a proof aid for the simulator and
the Mac only: `Demo.allowedHere` is false on every other platform (compiled
guard `os(macOS) || targetEnvironment(simulator)`, fails closed), a stored
flag there is removed, the row and the `demoSet` send are gone, and a phone
that held the flag clears the sample book, demo addresses in open pages and
signals once, and (only when no real key is stored) onboarding, edition
topics and voice setup. The Keychain is never touched.

## Remaining error

- The device branch has no automated proof: the tests run on the Mac, where
  demo is allowed. They pin the decision with `allowed:` passed in and the
  cleanup on temporary folders. The owner's eyes on the phone are the only
  instrument.
- A phone with a real key keeps onboarding, edition topics and voice setup
  from its demo days: those files carry no mark of their origin.
  `recent-files.json`, demo themes and notebook drafts are not cleaned for the
  same reason.
- Demo addresses are recognised by the `demo-` prefix after the scheme; a
  demo address of another shape would stay.
- (done, below) A later station deletes demo mode from the product.

## What was removed (station D2, 2026-11-20)

The owner: "what is demo mode? i think we can get rid of that. i never asked for that."

- Native: `Demo.swift` (the flag, `install`, the key store, the run sheet affordance, `DemoURLProtocol`), `DemoFixtures.generated.swift`,
  `DemoStretch.swift`, `TakesDemo.swift`, `TextureDemo`, `DemoComputerClient`, `DemoActivityClient`, `DemoThemeSource`, `DemoEditionWriter`,
  `DemoTakeRecorder` and `DemoTakeTranscriber`, the learned seed, `StoryTopics.demoMap`, `Places.demoExtra`, `ReaderSample` (now a test fixture),
  `SimulatedSetupBackend` (now a test double), every `Demo.isOn` branch (module, books, cards, onboarding, settings, takes, voice, spine,
  `CredentialReader`, `ContentSync`, `ModelsEntry`), the `demoInfo` and `onboardingDemoSay` interface names, `AboutInfo.demo` and `Onboarding.demo`.
  Kept: `DemoLeftovers.swift` (`Demo.clearLeftovers`, once per device) with its tests.
- Shared code: the demo backends of files, mail, paper, settings and activity, the `demo` argument of every source, `dm` in the contracts, the Developer
  row and the controls it showed (paper editions and Composition line, mail demo states, the reader's demo menu, the demo theme button, the onboarding
  demo line, Settings > About "Mode"), `AliveWait`'s demo script.
- Proofs: `proof-sim`, `proof-lane1` to `9`, `proof-mac`, `proof-default-themes`, `proof-app` and the demo tours (`fixtures/tours`, `fixtures/radio-page`)
  are deleted; `proof-real.sh` gained the spine, About, Activity and Mail folder checks. The table of what moved and what was lost is in the landing report.
- Fixtures: `mobile/fixtures/demo` is gone; the data tests still read moved to `fixtures/paper/day-*.json`, `fixtures/activity/changelog.json`,
  `fixtures/files/sample-tree.json`, `fixtures/texture/story-*.json` and `fixtures/texture/theme-ember.json` (ids renamed from `demo-` to `sample-`).
- Boundary ceiling: 101 files, 22,036 lines, 104 names, now 98, 19,386, 102.

Remaining error: reader experiments (side notes, Markdown atom pills) stay in the contract with no control that reaches them; the cleanup still cannot tell
real onboarding files from demo ones when a key is stored.
