# Trace review: claude arm, 2026-11-03

Computer a875023a. Engineer on every round: DeepSeek-V4.1-Flash. Evidence: trace exports of trajectories b48525f9, 1ac8f713 (Files) and 22bdd242 (Slides r1); self-dev list; main at e8adf487a. UTC.

## Rounds (assignment, minutes bind to Freeze, outcome)
- 498e2829, 47: blocked at Freeze, "mode is off"; no operation. 1 delegate run ($2.81).
- 87ef8525, 24: selfdev-0c48dc23 rejected; 2 of 12 shots missing, upload bar 0%, stand-ins. 1 delegate run.
- 74d314c1, 48: selfdev-40faf6ee applied. 1 delegate run.
- c1d6f9b9 rebase, 22: selfdev-22655878 failed, `rebase_content_mismatch: hunk_missing` on app-sdk.ts. No delegate.
- 21f67d48 rebase onto 059763bb, 53: selfdev-66e5fce0 applied. No delegate.
- de31718b, 55: selfdev-d92a93c9 rejected; 11 of 12 shots missing, contrast probe read black, "delegate handoff did not happen". No delegate.
- e8cd4366, 7: cancelled by the 17:09 cold boot.
- 08b84422 (r6), 33: selfdev-5e3fad8e rejected; shot 06 missing, contrast from composed styles. 1 delegate run (53 s, $0.06, no edits).
- 82187f26 (r7), 32: selfdev-347ca82e applied, published. 0 delegate calls.
- Slides r1: 35dbf779, 1.5, blocked at Step 0. b45deac0, 42, blocked, 4 of 12 shots, no operation.

## Findings, ranked by minutes lost

### 1. Provider latency in the 53-minute rebase (about 40 min)
- **Symptom:** 21f67d48 made 46 model calls: 50.7 of 53 min was model time, 0.3 min tools. Mean call 71 s for 345 output tokens; other rounds run 7 to 12 s. Management and Texture runs from 14:26 to 15:50 also show 45 to 79 s calls.
- **Cause:** FINDING: slow DeepSeek provider in that window. HYPOTHESIS: shared load from the other arms. The rebase notice also had it run spec tests and a screenshot attempt.
- **Proposed change:** alert when p50 call latency passes 30 s. Freeze a rebase by minute 20.
- **Status:** lean notice fixed by 52e05f16c (after this round). Latency open.

### 2. Rounds asked for the delegate in capsules that had none (about 62 min)
- **Symptom:** de31718b (55 min) never ran `choir-delegate status` and was rejected for a missing handoff. e8cd4366's status said `available: false`. c1d6f9b9 and 21f67d48 were ecosystem_proxy.
- **Cause:** problem 25 (grant compared a VM id with a computer-id list). The management overlay also routes carried-patch rounds to ecosystem_proxy, "the delegate is unavailable in it".
- **Proposed change:** the engineering overlay says "no delegate here" at the first Report in a non-claude capsule. The judge does not refuse an impossible handoff.
- **Status:** fixed by 7800706b, live after the 17:09 cold boot.

### 3. Why 82187f26 never called the delegate (focus)
- **Symptom:** 157 cells, 0 `choir-delegate` calls, no status check. Its summary: "0 choir-delegate calls ... No source was authored this round: the carried patch was applied verbatim and only the shots/corpus/mock scripts under /tmp changed."
- **Cause 1, overlay** (`rlm_engineering_runtime.yaml:72`): "Delegate first: any work that edits source (writes or changes files in the platform or package tree) is the delegate's... Work that does not change source (... running a check or a test) stays with you: do not call the delegate for it." The round was shots, a corpus and mocks in /tmp. FINDING: the text, read literally, tells the engineer not to delegate.
- **Cause 2, the instruction was lost in two hops.** The owner revision said "start the Claude delegate for the edits and shots". Texture's action list, which is what the engineer read, said "no source rewrite is implied" and "report ... the delegate's call count and run ids". Management's objective added only "delegate call/run details". The engineer was asked to count calls, not to make any.
- **Cause 3, hidden state:** a claude-mode capsule withholds `/computer/files` (problem 26), so the delegate's presence cost real files and the engineer used stand-ins.
- **Contrast:** 08b84422 called it once, as a verifier. The carried patch already closed all seven defects, so it made no edits, and the engineer did the 93-iteration shot work.
- **Proposed change:** add to the overlay: "In a delegate capsule, when the Cast names the delegate, start it for shot scripts and corpus too, even when the platform tree is unchanged." Pass `owner_revision` verbatim in the Cast body. The judge counts delegate use only if the Cast required it.
- **Status:** open.

### 4. Claude-mode capsules see no computer state (about 45 min, 3 rejections)
- **Symptom:** receipt.json said `computer_state: "withheld: delegate capsule (credential present)"`. 0c48dc23 was judged on stand-ins. Slides r1 stopped at Step 0 in 1.5 min. Management re-Cast with `"network":"forbidden"`; b45deac0 then hit `failed to listen on 127.0.0.1:8082: operation not permitted`, so choir-shot could not start and `--theme`/`--contrast` were absent. 4 of 12 shots in 42 min.
- **Cause:** problem 26. The management overlay describes forbidden only as withholding the delegate, not as denying loopback listen (HYPOTHESIS for the doc gap).
- **Proposed change:** document that forbidden denies TCP listen. Refuse a real-content shot Cast in forbidden mode.
- **Status:** b495fd61a and 7beec5d5b landed at 17:39Z and 17:07Z. Still withheld at 19:11Z. HYPOTHESIS: the guest image dated 16:44Z needs a refresh.

### 5. Upstreamed hunk refused on rebase (22 min)
- **Symptom:** every resolution of app-sdk.ts was refused as `hunk_missing`. The summary: "the only text the rebase checker accepts is a file that cannot compile".
- **Status:** fixed by 9e00d16af; 21f67d48 then passed with `rebase_upstreamed`.

### 6. Mode "off" found only at Freeze (47 min)
- **Proposed change:** show the signed self-development mode in `choir.Context()` at bind. HYPOTHESIS: an arm setup issue, now cleared. No code fix found.

### 7. Minor
- 82187f26's `choir.Ask` to management got no Reply in its outputs; it proceeded after about 4 min (HYPOTHESIS: management had completed).
- The engineer grepped docs/evidence/ in the capsule's source copy and read arm-evaluation notes. Exclude that directory.
- The delegate sandbox had no bwrap in round 1 and could not compile.

## Keep
- `choir-delegate wait --park`.
- Freeze saving the engineer's PNGs as files (12 saved in 82187f26).
- Honest summaries: stand-in arm named, "0 calls" stated, contrast method named.
- The keyboard route for shot 06, which ended the chrome-click timeout.
- Every round froze before minute 56.
- `rebase_upstreamed`.

/private/tmp/claude-501/-Users-wiz-Dev-go-choir/f298736e-cabb-51ff-9cea-34588a4989cb/scratchpad/trace-review/claude-arm.md
