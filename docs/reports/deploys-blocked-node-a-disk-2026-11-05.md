# Deploys are blocked on Node A's disk

I wrote this on the afternoon of November fifth. It covers the modular architecture work since the morning letter, about six hours.

Every deploy is blocked because Node A's disk is full. Node A builds the deploy artifacts, and its drive has a few megabytes free out of nine hundred and fifty gigabytes. The space is held by the workstation virtual machines and about sixty snapshots of them taken since October thirtieth. Those are yours and other people's, so I have not pruned any of them. Freeing space needs your call: either prune old workstation snapshots, or delete the Nix system generations older than fourteen days now rather than at Node A's weekly cleanup on November ninth.

The work itself kept moving. Three slices are deployed, one is reviewed and waiting on the disk, and one more is built and in review. Model quota is still exhausted across the fleet, so every model run on the test computer fails, and the paper is probably not getting its model calls either.

## What landed today

In the morning I deployed the reader gate. It counts every read of an object kind by a desk that is not that kind's declared reader. On the test computer it found the six wrong-reader settings reads I predicted, and nothing else surprising.

Next came leases. A desk that is not a kind's custodian can now borrow read access to it only through a lease that the custodian issues and that ends with the run. Devin took three rounds to accept it, and each round found something real, including a list of probe actions and a ceiling on what an app's manifest may ask for.

Then came the disclosure gate. It records every time a desk sends something outside itself, and it gives you a place to approve or deny those sends. Devin took two rounds on that one too. The leftover System 1 caps and their settings are deleted.

## What is waiting

The management and Texture boundary slice is reviewed and on main. Management now asks Texture to open a story, and both writes commit together in one step, so a wake can never find a story that does not exist yet. Its continuous integration run failed only because Node A ran out of disk, plus one module proxy error. It will deploy once space exists.

The counter-zero slice is built. It declares the right reader on every host read I found and gates the Texture folded casts and the resolve, disagreement and cancel messages. On a simulated representative day the built-in counters read zero, except four crossings from research's corpus search. Those wait for the Texture search lease. Devin is reviewing it now.

## Expedites and open questions

I expedited three small fixes: the app package route declaring its owner reader, the capsule lease naming its desk from the assigned agent, and a test count that the disclosure event moved.

Open questions for you. The app iframe sandbox. What to do with the nineteen old version one releases. Whether the judge should settle casts. Whether to fund the DeepSeek review seat, which is returning payment errors. The model quota. Node A's disk. And a vmctl defect: a computer marked behind after a deploy waits silently for two idle minutes, and my own probe traffic kept it from ever refreshing.

The one thing to remember is that the disk, not the code, is what holds the next deploy. If I were holding the phone, I would free Node A's space first, then let me re-run the integration run so the boundary slice ships.

## Names and receipts

- Goal: docs/definitions/choir-modular-architecture-2026-11-04.md
- Reader gate MA3.3: 89db770e9, fix 24a3ad5c2 (expedite)
- Leases MA3.4: cd6ea493a, fix c9f63914d (expedite)
- Disclosure MA3.6: 947fd2078, 26f2ed6df, fix 1a5b8ef61 (expedite); staging at 1a5b8ef61
- MA3.7a: 0d8014abc, 5e836949f; CI run 42055425083 failed (no space on node-a-builder)
- MA3.7b: 2169e26c4 on branch ma3-7b-zero-counters, in Devin review
- Ledger entry for the disk blocker: ae1182512
- Node A: /dev/md126 953G, 948G used, 100%
