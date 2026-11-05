# Modular Architecture Morning Check-in

I wrote this early on November fifth, at about a quarter past six Eastern. It covers the eight hours since last night's check-in, and it replaces that letter as the one to read.

The first two phases are closed and three Phase 3 slices are on staging, but since about half past two this morning the shared model accounts have been out of quota, and the paper has been failing its model calls for almost four hours.

Here is where things stand. Phases one and two of the modular architecture mission closed overnight, faster than I predicted. Phase three, which gives every kind of data one custodian desk, has three slices on main and on staging. Every kind now has a named custodian. Every write to the object graph now names its writer. Every act a desk takes now carries labels saying what outside content it read. The trouble is outside the code. The ChatGPT and Synthetic accounts that every computer shares hit their usage limits at about two twenty this morning, Eastern. The paper is the largest caller, and its calls are being refused. So is every test run on d@e, which means I cannot finish staging acceptance for the last two slices. The second review seat, DeepSeek, also ran out of funds. The fix for both is yours: money on those accounts, or another provider in the chain.

## What landed overnight

The first stretch closed Phase one. The fix for the stranded research run landed and deployed. A final Report is now refused while a Cast in its round is still unread, and a Cast that arrives in a closed round wakes its sender. A test run on d@e with three Casts per round, over three rounds, settled in about two and a half minutes with nothing stranded. The exact race it targets did not occur in that run, so that part is proved by tests only. The protocol lock landed after three Devin rounds. It writes down every desk's verbs, kinds and lifecycle in a file that CI compares on every change. The first round caught a seven megabyte binary I had committed by mistake.

Then I landed the line that shows each desk its own count of uninformative predictions, after committing the baseline first. The baseline is stark. Every typed prediction on d@e was inert, all for the same reason: no typed question to settle it against.

Phase three began at about half past twelve Eastern. The first slice gave every kind of data a custodian. Its first run found twenty-six kinds that no desk claimed, and five claims on kinds that no longer exist. Once deployed, it immediately counted a real violation. Management, while opening a story, writes Texture's story document and revision in the same batch. That is exactly the cross-desk write this mission exists to remove, and a later slice fixes it. The second slice made every write name its writer. Package kinds are enforced now, and built-in kinds are only counted until that management write is fixed. The third slice carries labels from what a desk read to everything it writes afterwards, and stores them on the run record so they survive a planned restart.

## What went wrong, and what it taught

Review became the bottleneck. At about half past three Eastern, the DeepSeek seat started answering "insufficient funds". Since then every red slice has had one seat, Devin. Devin also ends a review the moment it tries to write a scratch file, and two reviews ended that way before I learned to ask it for a static review only. Static reviews work. So I am building red slices one at a time, not in parallel.

The quota failure needed a look before I could tell you what it is. A read-only investigation of the gateway logs on Node B found real limits upstream, not a stuck circuit. ChatGPT reports that its usage limit was reached, and Synthetic reports subscription rate limits. The gateway then refuses calls locally and guesses when to try again, and each failed retry pushes the guess later. In the last three hours it refused about fourteen hundred calls, and about eleven hundred of those came from the paper. I have not touched the paper or the accounts. Both are yours.

One small lesson: after the writer slice deployed, the counters on d@e read empty, which looked like success. But almost nothing ran, because the models were out. An empty counter on a quiet computer proves very little, so I recorded it as partial.

## How the predictions are scoring

Phases one and two took about a third of a day, against my interval of half a day to three days, so that prediction failed in the good direction. The prediction that the custodian map would find unclaimed kinds on its first run held. The prediction that labels would show up on a real research run with no fix is still open, waiting on quota.

## What I need from you

There are four things, most urgent first.

First, the model accounts. ChatGPT and Synthetic need more quota, or the chain needs another provider. The paper is not getting its model calls until one of those happens.

Second, the DeepSeek review seat on the opencode account needs funds. Until then, red slices get one reviewer.

Third, two package questions from last night are still open. Should other publishers' front-end packages install before the iframe sandbox exists? And should the nineteen hidden version-one releases be republished?

Fourth, a new question. Most traffic between desks is Texture asking research, and those Casts now carry a probability and a weight, but nobody settles them. Should the blind judge settle them too?

Next I review and land the reader gate, which is built and testing now, then retry the staging acceptance as soon as the models answer.

## Names and receipts

Goal file: docs/definitions/choir-modular-architecture-2026-11-04.md, with its ledger and precommit files beside it. Previous letter: docs/reports/modular-architecture-evening-checkin-2026-11-04.md.

Landed since that letter: round fix 3dd75f208 and afeca9f2c (epoch 50 to 51); subcalls route bede836a9; MA1.4 protocol lock 3ff42209e and 4b8cf26ad; inert digest line f23c16f84; MA3.1 custodians 14ee014bd and bafd6eb32; MA3.2 writer gate 31d8bdc79; MA3.5 labels 5a47ba515. Staging serves 5a47ba515. Security epoch stays 51; MA3.1, MA3.2 and MA3.5 carry reviewed Security-Effect: none trailers.

Expedited since the last letter: afeca9f2c (the round fix). Releases touching the self-development loop paths also auto-expedite through the deploy script.

Quota evidence: Node B gateway journal 07:00 to 11:00 UTC; first ChatGPT usage_limit_reached at 07:23:33Z; about 1,401 circuit-open refusals, 1,080 from vm-1669020a (the paper); breaker code internal/gateway/circuit_breaker.go. Ledger entries 471e318ce and e7eda7902.

Cross-desk write found by MA3.1: X19, management's inbox batch writing Texture's story document, revision, story library head and support verdict; target test internal/store/x19_writer_test.go; fixed in MA3.7.

Blocked acceptance: d@e trajectory e34a16f6-09ac-5330-a07d-d5c13d487847. Stranded trajectory still needing an owner revise: 5c725f71-360f-5d20-a779-df8e879c6a07.

Predictions: R0b-02 contradicted, R0b-08 confirmed, R1-02 confirmed, R1-06 open.
