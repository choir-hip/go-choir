# The owner's iPhone was stuck in demo mode (2026-11-20)

Status: repaired in code, not yet seen on the phone. Class: orange.

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
- A later station deletes demo mode from the product.
