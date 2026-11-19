# The gateway refuses every guest while it drains on a deploy (2026-11-19)

## Problem

A deploy that restarts the host gateway leaves guests with no gateway for
as long as the old process takes to finish its in-flight calls. The old
process closes its listener at once and then drains; the new one starts
only when the old has stopped. In between, every model call, judgment and
search from every computer gets `connection refused`.

## Evidence

- The gateway's unit on Node B on 2026-11-19 (journal, UTC), stopping to
  stopped: 10:28:02 to 10:32:54 (4 min 52 s), 12:00:50 to 12:01:15 (25 s),
  12:57:38 to 13:05:03 (7 min 25 s), 19:31:26 to 19:34:05 (2 min 39 s),
  19:52:02 to 19:52:06 (4 s), 20:24:42 to 20:33:42 (9 min). About 25
  minutes in six deploys.
- The paper's console during the last one: four event-layer flushes from
  20:25:09Z to 20:32:22Z failed with `judgment client: http call: Post
  "http://10.210.210.1:8084/provider/v1/judgments": dial tcp ...: connect:
  connection refused` and left 48 captures unresolved. The first flush
  after the gateway returned (20:35:17Z) resolved its own captures; the 48
  are not retried (`internal/agentcore/event_layer.go:374`, "not retried
  again").
- The unit is given ten and a half minutes to drain
  (`SERVER_SHUTDOWN_TIMEOUT=10m30s`, `nix/node-b.nix`).

## Belief

The drain is right (an in-flight model call should finish) and the gap is
the defect: the listener should pass to the new process while the old one
drains. Candidates: systemd socket activation with the old process draining
after the new one has the socket, or two instances behind `SO_REUSEPORT`.
Either is a red change (gateway and provider calls).

## Remaining error

What the other callers did during these gaps is not read (research runs,
the pilot's units, mail): how many calls failed, and which were retried.
Whether the drain is usually one long model call or many is not read.
Whether a planned refresh of the computers in the same deploy hides part of
the gap is not read.
