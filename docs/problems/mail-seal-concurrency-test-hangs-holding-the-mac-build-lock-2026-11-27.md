# One mail seal test hangs at no CPU and holds the Mac build lock

Standing: governing | kind: happened | settled-by: five hangs seen by the lead and two builders on 2026-11-27 in Swift package test runs on the Mac | note: the cause is not known; nothing here was seen on CI or on a device

## Problem

`MailSealTests.testK3TwoFirstUsesAtOnceShareOneKey`
(`mobile/AutoradioKit/Tests/AutoradioKitTests/MailSealTests.swift:56`) sometimes
never returns. The `xctest` process sits at 0% CPU. Swift package tests run
through the one Mac build lock, so a hung run stops every other Mac build and
test until someone kills it. A plain `swift test` has no timeout of its own.

## Evidence

- Five hangs on 2026-11-27, across three lanes and both a builder's runs and
  the lead's. Each was the same test, at 0% CPU, holding
  `~/.cache/autoradio-mac-build.lock`. The same test passed in other runs the
  same day, with no code change between.
- The login keychain reported unlocked during one of the hangs.
- The test starts eight threads with `DispatchQueue.concurrentPerform`; each
  calls `MailSeal.seal`, whose first use creates one Keychain item. It is the
  only test in the package that makes concurrent first-use Keychain calls.
- The whole suite run on CI for every landing that day passed. Whether CI runs
  this test was not checked.

## Belief

Not established. Two candidates, neither tested:

1. The Mac's Keychain service does not answer one of eight simultaneous
   add-or-read calls from an unsigned test binary, and the call has no timeout.
2. `MailSeal`'s own first-use path waits on something another of the eight
   threads holds. If so the defect is in the app and could happen on a phone
   when two mail copies are sealed at once for the first time.

The second would matter to a user; the first only to the build. A sample of the
hung process's threads (`sample <pid>`) tells them apart, and none was taken.

## What is done meanwhile

Runs use `timeout 900` inside the lock and skip this one test
(`--skip MailSealTests/testK3TwoFirstUsesAtOnceShareOneKey`), and each report
says so. So the test did not run on the commits landed on 2026-11-27 after
midday, including the sign-out order fix 4f5e66020.

## Remaining error

- The cause. The next hang gets a thread sample before the process is killed.
- Whether the mail seal can hang on a phone (candidate 2).
- The mail seal itself is waiting on an owner decision (D4 of the move to
  Exact): if it is deleted, this test goes with it.
