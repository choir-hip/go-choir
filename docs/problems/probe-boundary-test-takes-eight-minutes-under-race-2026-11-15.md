# The probe boundary test takes eight minutes under the race detector

Date: 2026-11-15

## Problem

`TestProbeAtANonzeroTrimBoundary` (internal/agentcore/canonical_witness_test.go,
landed with 304a81277) appends 20,010 signed events through the real appender
to reach a non-zero trim boundary. Under `-race` on a CI runner it takes 501
seconds. The runtime shard that draws it runs past go test's ten-minute
default and fails with "panic: test timed out after 10m0s", naming whichever
test happened to be running.

## Evidence

CI run 43678513266 on c7eabe010, job "Go Test (race, agentcore/textureowner
shard 7)": failed twice (first attempt and one rerun), 150 tests summing 618
seconds, of which this test is 501.26. The run on 304a81277 (43672725876)
passed, so the shard was already near its limit and the deal moved.

## Belief

The test is correct and too expensive for a shared race shard. It is moved to
`scripts/go-test-scale-tests`, which runs it alone, without the race
detector, with one retry.

## Remaining error

The test no longer runs under the race detector in CI. Its time without the
detector in the scale job is not yet measured. The landing script still runs
it inside the whole agentcore race suite, about eight minutes of each landing.
The durable repair is a cheaper fixture: a way to reach a non-zero trim
boundary without 20,000 signed appends.
