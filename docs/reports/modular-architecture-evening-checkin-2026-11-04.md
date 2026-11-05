# Modular Architecture Evening Check-in

I wrote this on the evening of November fourth, at about ten o'clock Eastern. It covers the five hours since you signed off on the modular architecture mission and left for the art event.

Most of the first two phases are now on staging, with one gap: a stranded-research bug that turned up during testing, which is older than tonight's work and is being fixed now.

Here is where things stand. Ten code slices of the plan have landed, each through CI, the staging deploy and a check on d@e, the test computer. Desks now talk only through records, and every address goes through one resolver. Every request carries a probability of success, a weight and a way to settle it, and the reducer stamps where it came from. Every research sub-call and delegated task is now a scored Cast, judged later by a blind model call that never holds up the work. A new detector flags predictions that could never have been wrong, and nothing is refused for it. Two things need your eye: two open questions about packages, and a decision on whether to keep the new uninformative-prediction count out of the desks' sight until we have a baseline. I held it back.

## What landed this evening

The first hour and a half took out old machinery. The emit and message channels are deleted, about sixteen hundred lines of Go. The old "adopt" verb is gone, and installs are the one way a package arrives. Both kernels, the Go one and the front-end one, are on main with structural gates that refuse imports a plugin should not have.

The next stretch changed how desks address each other. The fixed allowlists that said which desk may message which are deleted. One data-driven resolver decides instead, and it refuses any address it cannot resolve. The desk lists scattered across the code became lookups into one registry. The first review of that registry found a real hole: an app desk could have claimed management's powers at registration. That is fixed and re-reviewed. The package manifest moved to version two. A side effect is that the nineteen old version-one releases are no longer served, so the Library is empty.

Then came the records themselves. A request now must say how likely it is to succeed, how much it matters, and who settles it. Its materiality is frozen and hashed when it is written, so nothing learned later can reweight it. I ran a real request on d@e and read the record back through the product API: a Cast from Texture to research at a probability of point seven eight and a weight of point six two, with the reducer's own stamp on it.

The last two slices are the scoring. Sub-calls became Casts with a blind, asynchronous judge, and the inertness detector began counting uninformative predictions. Two reviewers sent the judge back once, for three real defects. The most serious was that delegated tasks reached the judge with an empty objective, so its verdicts would have been noise. All three are fixed and re-reviewed.

The gVisor experiment on the Mac also finished. gVisor runs the real desk worker inside the arm64 guest on Apple's virtualization layer. Real computation costs five to eleven percent more, and an idle sandbox costs about seventy-eight megabytes more on the M1. Mac support holds.

## What went wrong, and what it taught

Four things broke CI tonight, and none reached a user. A date-based check on an old switch expired at midnight UTC and failed every commit, so I retired the switch as its own registry entry said to. Two Linux-only tests were broken by slices whose builders tested only on the Mac. One new test depended on a timer and failed under the race detector. The lesson I acted on is to vet for Linux before every push, and to keep wall-clock timers out of test synchronization.

The more important finding came from testing on d@e. A research run sat waiting forever. The investigation found that a second Cast into an open conversation joins the same round, and that the first research run's final Report then closed that round with the second Cast still unread. Nobody was left to wake Texture. This is older than tonight's work, from mid October. Tonight's faster exchange between desks only makes it likelier. I documented it first, and a fix is being built: a final Report is refused while a Cast in its round is unread, and a safety net wakes any caster whose Cast landed in an already closed round.

## How the predictions are scoring

Five records from this mission's ledger have settled tonight. I gave the plan review a forty percent chance of passing first time, and it passed. I gave the goal file seventy percent, and it passed. I gave the request record sixty percent to read back on d@e with no code fix, and it did. My estimate for the emit-channel deletion was too low: it removed sixteen hundred and thirty-two lines against an interval topping out at sixteen hundred. And the plan's claim that the old routes listed every rule held only in part: every route was listed, but some rules were not. The judge-stall record is still open, because staging has no way yet to read the judge's records. A small read route for them is being built now.

## What I need from you

There are three things.

First, should packages that run their own front-end code install from other publishers before the iframe sandbox exists? Today they do, and I kept that.

Second, should the nineteen hidden version-one releases be republished, or do you accept an empty Library for now?

Third, I held back the line that shows each desk its own count of uninformative predictions, so the first measurements are a clean baseline. Say if you would rather it ship now.

## Names and receipts

Goal file: docs/definitions/choir-modular-architecture-2026-11-04.md. Ledger: the matching ledger and precommit files beside it.

Landed and deployed tonight: MA1.1 e13c082a0; MA2.1 1a5ca5437 and 39501dcba; MA2.2 2cd55c973 and d59b73673; MA2.4 24e3437e2; MA2.3 de4f93b10 through e434f8bf1; MA1.2 de5960fa1 and 2bae9cbdd; MA2.5 cf2e7ed0f, 0359fff5e and 432dfdfef; MA1.3 7826565c8 and 658a0c324; MA1.5 90a26080e, d3d6f9ee2 and d792d33e3; MA1.6 23c381ac8, cc34288a5 and 69ff34c30. Staging serves 69ff34c30. Security epoch 41 to 49.

Fixes: nix filter 0987b876a; CI switch retired 97018316f; Linux test e385fd349; timer test 897662321. Expedited releases: 2cd55c973, 1a5ca5437, e13c082a0, 0987b876a, d59b73673, 39501dcba, 24e3437e2, 97018316f, e385fd349, 897662321; slices touching the self-development loop paths also auto-expedite.

Held: 222d946f8, the inert count shown to desks.

Problem doc: docs/problems/final-report-strands-unread-cast-2026-11-05.md. Stranded trajectory on d@e: 5c725f71-360f-5d20-a779-df8e879c6a07. Evidence: docs/evidence/ma1-3-de-request-record-2026-11-05.json, docs/evidence/gvisor-arm64-mac-guest-2026-11-04.md.

Still to come in Phase 1: the round fix, the sub-call read route, then MA1.4, the protocol lock.
