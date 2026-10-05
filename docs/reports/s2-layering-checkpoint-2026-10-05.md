# S2 checkpoint, not a close

October fifth, evening. About eight hours of overnight work on the layering station, and the honest headline is: the mechanism is proven and the station is not closed.

The guest on staging now takes builder-built releases through the updater with no virtual machine reboot. A signed release applied, the machine stayed up, and the new binary answered healthy. Six different ways of making a bad release were each refused before anything moved, and each refusal left the machine exactly as it was. A release built to fail did fail, and the machine put the old one back, down to the executable file. Three real defects surfaced and were fixed along the way, each with a test that fails without the fix. An eight-model review panel read the claim and sent it back, seven to one, with a list of gaps I agree with.

Thursday night started with a stuck machine. An offer had been accepted but never finished, and because its epoch had moved on, the machine kept trying to replay it on every boot and refusing everything new. That took hours to see clearly, partly because the updater threw away the reason for every refusal. By morning the fixes were in: a refused offer now clears its own transition, a stranded offer from a dead epoch is laid to rest on the next boot, and the updater says why it refused.

The afternoon was slower. A deploy landed halfway, with the new machine image but the old bookkeeping, and every test for two hours reasoned about a pairing that never existed. Then the fix for that confusion could not reach the machine precisely because the machine was still running the old binary. The evening proved the core: one clean apply, six clean refusals, one clean restore, all on the tape.

What remains is named, not vague. The rollback worked but the machine rebooted underneath it, so the no-reboot half of that test still needs a clean run. The machine-driven push from the build pipeline has not yet run end to end with its timing recorded. Two sharper refusal tests are still owed, one for a missing dependency after the builder is gone, one for a store schema the release cannot read. And the new screen must be shown serving by its own fingerprint, not just the new brain answering healthy.

If I were holding the phone, I would remember this: the scary failure mode, a bad release bricking the machine, now fails safe in every shape we threw at it. The next session has a short list and a working harness, and the station file says exactly where it stands.

## Names and receipts

The work is commits 6fcb05e5, 34041932, 297eedf1, 0b9d5186, 63865ede, 7d78b182 and dac021cd on the main branch, deployed to the staging site at 63865ede. The station file is docs/definitions/choir-appdev-s2-layering-runtime-from-release-2026-10-01.md, now marked checkpoint incomplete. The new problem notes are docs/problems/s2-postswap-restart-loop-kills-vm-2026-10-05.md, docs/problems/s2-updater-refusal-reason-observability-2026-10-05.md, docs/problems/s2-stranded-retired-realization-permanent-wedge-2026-10-05.md and docs/problems/s2-guest-store-head-lag-corpusd-2026-10-05.md. Run evidence is in the staging file area under deploy-failures, files starting s2-acceptance-20261005T. The review panel ran eight members with three timeouts, seven send back and one approve.
