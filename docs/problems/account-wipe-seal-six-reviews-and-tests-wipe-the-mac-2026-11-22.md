# The account wipe's seal: six reviews, a hole beside it each time; and tests that wipe the Mac's real folder (2026-11-22)

A structural assessment (contract rule 7) and a problem record in one.

## Problem

1. Station 1 of the phone's account boundary (`lane/account-partition-s1`)
   has been reviewed six times without landing. Each round closed what the
   last found and the next found something beside it.
2. Swift tests run the sign-out wipe against the development Mac's real
   `~/Library/Application Support/Autoradio`, and until this round reached the
   real login Keychain.

## Evidence

- Rounds 3 to 6: the wipe was made to answer from a disk listing (round 3); a
  process-wide seal was added so late writers fail (round 4); the seal left a
  sealed process offering Sign in (round 5, blocking, both seats); round 6
  (candidate `7eac4277b`): seat 1 accept with fixes, seat 2 send back, on a
  console log that is listed but not sealed, two stores that cache a path
  outside the folder, a sign-out that runs the wipe twice and so exceeds the
  shared deadline, and a test that cannot fail.
- Both round 6 seats, independently, recommended the same replacement: the
  sign-out ends the process and the next launch wipes before any store is
  built.
- Tests: `DeckTests` 11 and 17 and `HeldRestTests` run `AccountWipe().run()`
  with default paths; `AccountWipeTests` leaves 60 s hung wipes that resume
  after the test base is reset. `MailSealTests`, `CredentialReaderTests`, the
  wipe tests and `PlayerEngineTests` reached the real Keychain;
  `EditionSetupTests` wrote into the real account folder. The whole suite ran
  on this Mac on 2026-11-22 (660 tests). The folder held an `account`
  directory made at 05:57Z by a test.

## Belief

The seal is the wrong mechanism, not a mechanism with bugs. It exists to make
a late writer fail inside a process that is already finished; every hole was
a writer or a path the seal did not cover. The rule the owner already gave
("sign-out may stop the app"; "i never asked for signout without disrupting
service") makes it unnecessary: nothing is wiped in the process that signs
out, so no writer in it can race a wipe, and at the next launch nothing else
is running.

## Decision (the lead, 2026-11-22T06:10Z)

The approach is changed, not patched again. At the tap: the mark, stop audio
and the session, remove the Keychain key, the ended card. At the next launch,
first, before any store: if the mark is present, wipe, list, and clear the
mark only when clean. The seal, the join deadline, the retry loop and the
second run are deleted. The Keychain door, the three-state key and the
ended-session rule from rounds 5 and 6 stay. Test isolation is fixed in one
base case, with a guard that the account base is a temporary folder under
tests.

## Remaining error

- Whether anything on this Mac of the owner's was lost to the tests is not
  known: what the folder held before is not recorded. The phone is not
  affected; tests do not run there.
- Whether the tests' Keychain access contributed to the Mac's repeated
  "can't save to the Keychain" faults is not known.
- Between the tap and the next launch the account's files remain on the
  phone, with the key already removed. That is a change from "wiped at the
  tap" and is stated in the design note when it lands.
- The six simulator drills have still never run.
