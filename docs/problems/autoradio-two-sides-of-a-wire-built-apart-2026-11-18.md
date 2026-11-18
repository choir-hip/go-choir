# Autoradio: the two sides of a wire were built apart and disagreed, six times in one night (2026-11-18)

Status: open. This is the clustering assessment the contract asks for after
three bugs of one kind in a week; the six instances are each fixed or being
fixed, the cause is not. Class when fixed: yellow (a shared fixture and two
tests per wire). Owner: the Autoradio learning-loop mission
(docs/definitions/choir-autoradio-learning-loop-2026-11-18.md).

## Problem

A wire between two programs has two authors. Each was briefed from the same
design text, built its side in its own lane, tested it against fixtures it
wrote itself, and reported green. The two sides then disagreed, and nothing
failed until a person or a reviewer looked at the joined result. Each side's
tests marked its own work.

## Evidence

1. **A Swift field the interface file did not declare emptied the Autoradio
   page.** `RadioPageRow.builtOn` was added in Swift but not in
   `mobile/exact-package/interface/radio.json`. The resource failed to load
   and the page fell back to empty. The Kit suite passed, and the builder's
   own tour passed while the page was empty, because it took no screenshot
   and asserted nothing about the rows. Found by the lead reading a
   screenshot of candidate integ9. Fixed in lane/integ9-fix; a test now
   compares the Swift fields with the declared shapes.
2. **The phone's takes client and the takes server chose different routes
   and shapes.** The phone calls `/api/autoradio/takes` and uploads to the
   listener's computer first; the server (lane/takes-server 5a2b49115) serves
   `/api/platform/takes…` and takes the audio in the publish call; the track
   record and the take segment are laid out differently. The server's tests
   decode the phone's fixtures for the shapes it shares, so both suites were
   green. Found by the server builder's own report, after the phone build was
   installed. Being fixed in lane/takes-wire (the phone follows the server).
3. **The run sheet reducer has a Go copy and a Swift copy.** The settlement
   review added a takeover rule to the Go copy (a phone `aired on_air` for
   another entry closes a stale on-air entry). The Swift copy still drops the
   op as `other_on_air`. The shared fixtures (`mobile/fixtures/runsheet/`)
   are the right mechanism and did catch this on paper: fixture 34 exists and
   the Swift suite will fail on it when the two lanes meet. Not yet fixed on
   the phone.
4. **Builders' tour files used operations the driver does not have**
   (`expect`, `shot`) and test ids that do not exist (`bar-pages`). A tour
   with an unknown operation was reported as passing. Found by the lead
   running them. The lead now writes tours with the driver's real operations
   and reads every screenshot.

5. **The app target called a Kit type that no file defined.** The first-run
   microphone check's wiring (`AutoradioModule+Onboarding.swift`, app target)
   called `MicCheckMeter`; the builder's lane defined only `MicCheck`. The Kit
   suite was green because it does not compile the app target; the simulator
   build of candidate integ16 failed. Found by the lead's build; the lead
   added the type and a test. The next round's brief asked the builder to list
   every Kit member the app-target file calls, and the reviewer checked each.
6. **The server and the phone built a take's run sheet entry differently.**
   The server placed a take segment as kind `story` with the answered story as
   its source and the lead-in's length; the phone builds the same entry id as
   kind `take`, source `take:<id>`, the lead-in plus the take's length. Each
   side's tests passed. Found by review seat 2 in round 4 of the takes server,
   after a rebase onto the settlement code made the entry matter. Fixed in
   lane/takes-server (the server copies the phone's fields; T9 pins them).

Separately seen tonight, a different kind: `ClipPlayerTests.test7…` gives a
background task two scheduler turns and then asserts; it failed twice under
load (once alone). Being fixed in lane/takes-wire by waiting on the condition.

## Belief

One cause: a wire's contract lives in prose (a design section, a builder's
brief) and in each side's private fixtures, not in one checked file both
sides must satisfy. Where one checked file exists (the run sheet fixtures),
the disagreement shows up as a failing test, which is the wanted behaviour.
Where it does not (routes, the Exact interface, tour operations), it shows up
on the owner's phone.

## What should change

- **One fixture directory per wire, written by the server side, decoded by
  both.** A route table (method, path, encoding, example request, example
  response) is a fixture like any other: a Go test asserts every row is a
  registered route that accepts the example, and a Swift test asserts the
  client builds exactly that request. The phone never invents a route.
- **The server side of a wire is built first, or at least its fixtures are.**
  Tonight the phone was built first for takes because the phone had the
  deadline. The cost was a third phone build.
- **A tour that names an unknown operation or an unknown test id fails.** The
  driver should refuse the step, not skip it.
- **A builder's brief for a page names the screenshot the lead will read.**

## Remaining error

- The route-table test does not exist yet. Until it does, every new wire is
  exposed to instance 2.
- The driver still accepts a tour with an unknown operation (not checked
  tonight beyond the builders' reports; the lead's tours avoid it).
- Instance 3 is repaired in lane/reducer-twin (the Swift reducer mirrors the
  takeover; fixtures 19 and 34 pass on both sides) and is in the build on the
  owner's phone; it is not on main, because none of the phone lanes are.
- Instance 5 has no mechanical guard: a Kit suite cannot see the app target.
  Only the simulator build catches it, and only the lead runs that build.
- A take's sheet entry has no shared fixture (instance 6): the server's test
  pins the server's fields and the phone's test pins the phone's.
