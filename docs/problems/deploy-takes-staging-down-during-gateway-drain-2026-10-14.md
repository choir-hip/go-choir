# Every Deploy Takes Staging Down While The Gateway Drains

Date: 2026-10-14, about 00:55 UTC. Found when `https://choir.news/health`
answered 502 during a deploy that a docs commit with one script set off
(`809755a7`, CI run 38491257877). Status: repaired for the site (classifier fix 6fdcf583; restart fix 2db4bc02, proven on the ecc51d79 deploy with 417 of 417 health probes at 200). Open: model calls are refused during the gateway drain (D3).

## What happens

A deploy runs `switch-to-configuration switch` on Node B. The switch stops
every changed go-choir unit at once, and it starts none of them until all
of the stops have finished. The gateway's stop is a graceful drain of
in-flight requests:
- `TimeoutStopSec = "11min"` (`nix/node-b.nix:845`);
- `SERVER_SHUTDOWN_TIMEOUT=11m`, from `4589238c` ("preserve gateway
  requests during deploy shutdown").

Proxy, auth and corpusd stop within a second. They then stay down until the
gateway's last in-flight request finishes or the 11 minutes run out. For
that whole time, Caddy answers 502 for the site, the API and auth. The
drain protects model calls already running, but every other surface goes
down to do it.

## Evidence (Node B journal, 2026-10-13/14 UTC)

| deploy | proxy stopped | gateway stopped | proxy started | site down |
|---|---|---|---|---|
| d3f7d9b2 | 00:05:06 | 00:06:44 | 00:08:15 | 3 min 9 s |
| 6c6bc1aa | 00:19:48 | 00:22:17 | 00:23:48 | 4 min 0 s |
| 809755a7 | 00:43:52 | 00:53:34 | 00:55:05 | 11 min 13 s |

The deploy at 00:43 began while the paper computer had long model calls
in flight. The journal shows completions still landing at 00:46:36,
including one with 23,019 output tokens.

## Why it matters

- **Owner ruling 6** ("we need paper publishing 24/7, even after
  crashes"). Every deploy is a full outage of the site and of every
  computer's platform calls. Deploys are frequent: there were three in 45
  minutes tonight.
- **The cause of a deploy can be trivial.** A script file under `scripts/`
  counts as deploy-impacting, so a docs commit with one script took the
  site down for 11 minutes.
- **The drain protects only the gateway.** Requests to the proxy, auth and
  corpusd made during that window fail outright.

- **Each deploy restarts the services twice.** After the switch, the
  deploy runs `systemctl restart` on auth, maild, corpusd, gateway and
  proxy (`.github/workflows/ci.yml`, "Ensuring edge config and host
  services are live"). That is the second blip in each row's journal, two
  seconds after the first start. Its gateway drain is short only because
  the new gateway has nothing in flight yet.

## Second finding: docs pushes set off full deploys (01:00 UTC)

A third outage began at 01:03 for `8d8b0084`, a docs-only commit.
- The plan job diffs a push against the live staging commit, the
  `X-Choir-Build-Commit` header (`.github/workflows/ci.yml`, "Plan base:
  live staging commit"). That header is the commit compiled into the
  proxy binary.
- A deploy that rebuilds no Go service leaves the header where it was. So
  the base stayed at `6c6bc1aa`, and every later push carried two files
  from earlier commits: `scripts/l2a_paper_window.py` and
  `testdata/fixtures/l2a-multilingual-2026-10-14.xml`.
- `.github/scripts/deploy-impact-classify` has no rule for top-level
  `testdata/` or for Python scripts. Both fall to its last case, "unknown
  deployed path: conservative host + canonical guest image", which means
  a host OS switch, the five-service restart and an active-VM refresh.
- Neither file is deployed anywhere. The fixture is served from GitHub's
  raw URL, and the script is an operator harness.
- So every push re-deployed and took the site down, until a deploy that
  rebuilt a Go service moved the header. Tonight's docs commits set off
  deploys at 00:43 (`809755a7`) and 01:03 (`8d8b0084`). A third was
  cancelled before it started.

## Candidate directions (not decided here)

1. Mark the gateway `stopIfChanged = false`. NixOS then restarts it after
   the start phase instead of stopping it with the others. The proxy,
   auth and corpusd come back at once, and only model calls wait out the
   drain.
2. Hand the listener over (socket activation, or an old and a new process
   overlapping), so the gateway keeps accepting during its own drain.
3. Leave out of the deploy-impact set the paths that do not ship:
   top-level `testdata/` and `scripts/*.py` (operator harnesses). This
   stops the docs-push redeploys.

Direction 1 is the smallest change. It touches deployment routing, which
is red, so it needs its own ceremony and a deployed proof: the site stays
up through a deploy while the gateway drains.
