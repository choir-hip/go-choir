# The rectification spine: eleven stations through the self-development gate

Written September twenty-sixth, twenty twenty-six. This letter covers three days of work, from the morning of September twenty-fourth through the night of September twenty-sixth — about two hundred commits across eleven spine stations, ending with the platform update push landing on staging tonight.

## The one sentence

The rectification spine is done through M9a — every desk is on the carrier, the ledger has a read surface, self-dev advances without a driver, and a live computer just took a signed platform update and restored back to the head it had before.

## Where things stand

The spine was ratified two days ago as the corrective pass over the RLM carrier buildout — the plan in section eleven, the one that said every desk had to sit on the in-cell carrier and the product had to prove it could supervise its own development before the world wire shipped. Tonight it has executed ten of twelve stations in order: substrate integrity, the kernel, the ledger consumer, the desk-cell carrier for all four desk kinds, the ledger read surface, derivable selfdev continuations, and the platform update push plus pinned-head restore. Two stations remain: R5a, a vocabulary decoder freeze that must precede the gate so the proof tape folds under stable decoders, and M11, the self-development gate itself — a supervised episode where a computer promotes a candidate under consensus, falsifies the loser, restores, and you read every step in the live Texture doc.

The current deploy on choir dot news is commit 44e4169e, published tonight as platform release pr-20260926-44e4169e. The spine goal file's slice now points at R5a, the last gate before M11.

## Thursday: the substrate under everything

The first day belonged to K — the ontology kernel — and it was the kind of work that doesn't look like progress until it does. The question underneath it: can the computer's own state survive a restart without anyone remembering to re-arm it. Before K, desks ran continuations in process memory, and a crash meant the work was just gone. K replaced that with a durable outbox — every wake becomes a Dolt row that survives the process, projected back into actors idempotently.

That build took most of Thursday and Friday morning. Eleven wake edges were audited and closed. Six landed as direct fixes; two turned out to be non-gaps under the kernel model; the rest classified out. There was a real incident in the middle — a boot stall where the engine mutex spanned a network compare-and-swap — and a recursion mistake where I nearly dropped the read pool in a way that serialized the wrong thing. Both are receipts now. The kernel ended unconditional: the flag came out and the in-memory continuation path was deleted, not deprecated. That is the thing this spine is for — every deletion shrinks the space where a bug can live.

R2x landed beside it: a mid-commit crash can no longer leave half a state. The fate sweep that could never run — "what happens to a past-deadline assignment" — now runs.

## Friday into Saturday: every desk moves onto the carrier

Then the desk stations ran in order. R3a made Texture consume desk acts from the commitment ledger instead of the old worker-updates channel — the dual-read against the old surface proved the new one was right before anything deleted the old. R3b built the host-side desk-cell carrier: a non-capsule desk now runs inside a yaegi-eval session worker, killable mid-cell, with a derivable wake to re-fire the cast. R3c put management live on it and proved the management cast drives an engineering assignment to resolution on the tape. R3d did the same for Texture itself — the doc now authorizes its own revisions through cell-authored turns citing ledger records. R3r moved research onto the carrier with the network-egress and memory caps resolved — that one got interesting: the first memory cap, one gigabyte, OOM'd the autoputer baseline itself, and the cap that landed is eight.

By Saturday afternoon every desk that runs the product was running inside a killable subprocess with restricted stdlib and typed tools. The old typed-tools registry and the worker-updates consumer are gone.

## Saturday afternoon: the ledger gets a read surface and the loop closes itself

R4 landed the read surface — derived accrual views over the ledger, materiality projected onto the live doc so a falsified commitment stays visible, acting packs for desks that carry zero own-score fields, and the learning-claims gate. That is the supervision surface M11 will read from.

M7 was the station that changed the product's shape. Before it, a self-dev operation advanced only when an external harness called the next API. Now a canonical decision event commits, a post-commit observer fires a coalesced drain, and the reconciler walks the op through accepted to materialized to applied — zero API calls after the owner's decision, and a crash mid-materialize recovers on the same reconciler. The owner's decision is still the only authority gate; the loop never mints one. That is the thing the whole spine was building toward: the product drives itself between decisions.

## Tonight: the push and the restore

M9a is what I did tonight and it earned its own problem file. The goal was simple: sign an update offer under platform-control, push it to a live computer through the existing proxy, have the guest apply it, and then restore the machine to the head it had before — proving updates are forward transactions on the tape, not pointer swaps.

The deployed probe peeled five separate substrate bugs, each invisible until the one above it was fixed. The apply died with its own request when the guest restarted. The post-apply tail stranded permanently — a crash between the applied event and route promotion left the tape saying "applied" with no path back. Fresh computers couldn't mint checkpoints at all — the replay-completeness guard refused them for having no ops-published base. The checkpoint path had no evidence class for platform updates — the verifier route demands a genesis ceremony these probe machines never run, so the tail minted a platform-follow class that carries the applied-event receipt instead. The update payload never reached the artifact store where vmctl's pin verifier reads it. And once the route finally promoted, vmctl's ownership classifier couldn't parse the new evidence shape and started refusing every computer on the node.

Each fix landed with the problem documented first. The last probe, run at eleven sixteen tonight, is green end to end: the fresh computer accepted the signed push, applied it, minted the platform-follow checkpoint, promoted the route to generation one, and restored to the pinned head through tape reconstruction — with no base, six tail events, witness matched, frontend restaged. That is the restore edge M11 needs.

## What is left

Two stations. R5a is small and mechanical: freeze the vocabulary decoders and settle the V1-to-V2 profile normalization, with an explicit decision to keep the co-super-assignment shape at version three. It has to exist before M11 because the gate's proof tape has to fold under frozen decoders — a tape that can't be read deterministically can't be a proof.

M11 is the gate: a reversible self-dev episode on staging where the whole spine is exercised at once — the management cell driving a real op, the candidate promoted under qualified consensus, candidate B falsified and still visible in the doc, the restore to the pinned head, every step readable in the live Texture doc. The receipts are the scored commitment records themselves.

## The one thing worth remembering

The spine's discipline was the thing it was testing: every station landed on staging with receipts, every problem was written down before it was fixed, and tonight the product pushed an update to itself and put itself back. The remaining work is a decoder freeze and then the gate — and the gate is now the only thing between this and M11's proof that the computer can develop itself under supervision.

## Names and receipts

- Spine goal: `docs/definitions/choir-rectification-spine-2026-09-25.md`
- Plan: `docs/desk-rlm-rectification-plan-2026-09-23.md` §11
- M9a evidence: `docs/evidence/choir-platform-update-push-restore-deployed-2026-09-26.md`
- M9a problem file: `docs/problems/platform-update-stranded-tail-and-baseless-checkpoint-2026-09-26.md`
- Probe: `scripts/m9a_platform_update_probe.mjs`
- Deployed build: `44e4169e` on choir.news, platform release `pr-20260926-44e4169e`, CI run `36277989883`
- Fix chain tonight: `4153b5e4` → `1d967302` → `b920c85a` → `c61be3f1` → `44e4169e`; station commit `5fb58654`, spine handoff `35fdf243`
- Earlier terminals: R2x `ebdaef45`; R3a `4cf82057`; R3b `b9f43583`; R3c `b7ae7596`; R3d `119e0edd`; R3r `3b56c34e`; R4 `68a2e023`; M7 `722b49bf`
- Remaining before M11: R5a (vocab decoders + seed freeze)
