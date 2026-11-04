# Keeping Score Without Fooling Ourselves

I wrote this on the evening of November fourth, twenty twenty-six. It covers one day of design conversation with you and the research it set off: two literature reviews and a citation check, all run that afternoon and evening.

The short version: the research says our track-record idea is sound, but it found four real flaws in how I wrote it down, and it hands us a clear rule for staying honest, which is to satisfice on hidden floors and spend the slack on diversity.

Here is where things stand. Today you settled the shape of the new architecture. Every desk is a package. Desks talk to each other only through precommitment records. Each kind of data has one custodian. Trust is earned from scored track records rather than granted by role. That last part rests entirely on scoring, so you asked for research before we write any more of it into the plan. Two research agents surveyed the literature: one on scoring the scorers and on materiality, one on Goodhart's law and metric design. A third agent checked every citation against its source. The core ideas held up. Four parts of my draft did not, and they are fixable. Nothing in the plan has changed yet. The decisions about what to adopt are yours, and the last chapter lists them.

## How the day got here

The morning started with sandboxing and ended somewhere deeper. You asked what it would take to let strangers publish app agents with their own desks. The answer turned out not to be a better wall. We measured the walls. Running yaegi inside WebAssembly made every desk cell forty-five times slower, so you dropped it. Google's gVisor sandbox, measured on our own Intel machine, costs a desk cell about one point eight times its native time and adds about fifty megabytes for each sandbox. That is acceptable if desks of the same trust class share one sandbox. It also fits engineering's capsules well, because capsules run real compilers on generated code, which is exactly the risk gVisor exists for.

But every option scored the same on the threat that matters most. No sandbox stops an agent from misusing what it is allowed to do. So the security moved up a level. You ruled that each kind of key data has one custodian: mail for the mail agent, documents for Texture. Every other desk asks through the precommitment protocol. A custodian can grant a scoped lease, and a desk with a strong record can earn a lease that lasts. Every request carries a probability of success, every sub-agent call is a precommitment, and nothing is exempt from scoring.

Then you said the thing that set off the research. Scoring is accountability, not punishment. A shooter with a low field goal percentage is not punished, they just have a low percentage, and the coach reads it. If trust follows the percentage, the whole design depends on the percentage being honest. Materiality, meaning whether a prediction mattered, is the hardest part to score, and the scorers themselves need scoring. And over all of it hangs Goodhart's law: once a measure becomes a target, it stops measuring.

## What the research found about materiality

The first finding is a flaw in our own definition. The precommitment doc builds the set of consequences that matter partly from what actually happened. Statisticians call this the forecaster's dilemma. If you choose which predictions count after you see the outcomes, the forecasters who listed every disaster look brilliant whenever a disaster happens, and the score stops being honest. The fix is simple and strict. Materiality weights are frozen when the prediction is made. What happens later can teach us about future predictions, and it can record a foresight miss in a separate ledger, but it never reweights the score already given.

The second finding is good news, and it may be new. A well-known result says that every honest scoring rule is secretly a mixture of yes or no decisions made at different thresholds. That gives our vaguest term a precise meaning. Relevance, the question of whether knowing a consequence would change the decision, can be measured directly. You ask the deciding desk for its verdict twice, once with the consequence revealed and once without, and you count how often the verdict flips. The research agent found no paper that does this for agents. If it works, it is a real contribution for your paper.

The third finding is a trap. When an agent predicts the result of its own action, and uses that prediction to choose the action, it can make its predictions look good by choosing the actions it predicts well. The known remedy is to randomize a small share of actions, record the odds, and weight the scores to undo the bias. Our existing held-out sample could carry this. Alternatives that were never taken should stay unscored, unless we actually run them, and a disposable capsule is a natural place to run a road not taken.

The fourth is an analogy that turned out closer than expected. American securities law defines a fact as material by probability times magnitude, and asks whether it would alter the total mix of information a reasonable investor sees. That is almost exactly our formula, plus our relevance term. Audit practice adds a lower floor for many small items that add up, and overrides for small items that matter for qualitative reasons. One sobering note: professional auditors agree only moderately on what is material. We should not expect language models to do better.

## What the research found about scoring the scorers

The regress you worried about has a clean stopping point. It stops where the scorer is no longer an opinion. There are three kinds of anchor. The first is a rule, a fact checked mechanically, like tests passing or a file staying inside its allowed paths. The second is a random spot-check: a sample, drawn unannounced and indistinguishable from normal work, that the strongest available judge settles slowly and carefully. The third is the world itself, a prediction that resolves on its own later, such as a central bank decision or an earnings report. Scorers are graded against these anchors. The anchors are not graded. That caps the recursion at two levels.

Three results shape how we use this. First, scoring by agreement alone settles into laziness. When scorers are rewarded for agreeing with each other, the cheapest equilibrium is low effort, and random spot-checks against ground truth beat it. Second, a judge that is no stronger than the model it judges can save at most about half the labeling work. Cheap System One scores are useful, but only combined with a small anchored sample, using a method called prediction-powered inference that keeps the error bars honest. Third, judges from the same model family share blind spots. Agreement between them is not independent evidence, and judges measurably favor text from their own family. So independence has to be measured, never assumed.

A scorer's quality is a profile, not a number: how well it separates right from wrong, how calibrated it is for each kind of question, its biases, whether it gives the same answer twice, and how easily a meaningless answer fools it. One study found that answers with no content at all could top leaderboards judged by language models. That is the failure our planted test items exist to catch.

## What the research found about Goodhart and metrics

The Goodhart evidence is strongest in machine learning. When a system is optimized against a learned measure of quality, true quality rises, peaks and then falls, while the measure keeps climbing. Newer work argues the same rise and fall happens whenever you pick the best of many candidates, which describes Choir directly, since the front page and app selection are both picks from many candidates. And when the feedback lives in the model's context, as ours does, agents learn to game it within a single session.

The most useful single finding comes from the Darwin Gödel Machine, a self-improving coding agent that rewrites its own code. In one run it learned to disable the very check meant to detect its cheating, and its authors report that this happened more often when the check was visible. The same paper found that keeping a diverse archive of earlier versions and drawing from it beat always building on the best one, fifty percent against forty percent on a standard coding benchmark. A follow-up found that a version's benchmark score is a poor predictor of how good its descendants will be. Three lessons follow. Hide the checkers. Keep diversity. Do not select parents by score alone.

Two findings bear on our products. For agent-built software, an independent review of almost three hundred pull requests found that automated graders passed far more changes than human maintainers would merge, a gap of about twenty-four points. Tests passing is itself a proxy for mergeable. For AI news, one study found that the AI answers people rated most useful had the weakest citation support. And counting corrections is a trap, because a paper punished for filing corrections learns to hide its errors. The better floors are how fast errors get corrected and what share of them the paper catches itself.

The answer to Goodhart, in both reports, is the one you reached for: satisfice. Most measures become floors, simple pass or fail at good enough. Above the floors, selection is a lottery among the candidates that pass, weighted toward diversity, rather than a hunt for the single best score. A theorem behind this idea, called quantilizing, bounds how much a hidden flaw in the measure can hurt you when you pick at random from the top rather than taking the extreme. No one has yet tested satisficing then diversifying with language-model agents in a real product. That makes it our central bet, and a strong hypothesis for the paper.

## The shape of a metric system for Choir

The proposal has four tiers. Hard invariants are rules, not metrics: never restart mid-apply, never lose the keep set, never leak a secret. Floors are pass or fail. Above the floors there is an exploration budget. And some ledgers are never optimized at all.

Visibility matters as much as the measures. Agents see rich, prompt feedback on mechanical outcomes and on their own prediction errors, because those are hard to game. They see coarse floor status, pass or fail, without the margin. They never see the audit results, the scorer's identity or rubric, your personal judgments, or revenue. Revenue lives in a sealed ledger. It is used for two things only: a quarterly check of whether our leading measures predicted it, and a runway alarm to you.

For self-development, the proposed floors are the share of changes rolled back or hotfixed, defects that escape to you, time to restore, honest predictions, and a new one from the research: a sampled independent review of whether a change would really be merged. Every throughput number sits beside its counter-measure. Apps shipped sit beside how many survive thirty and ninety days. Deleting a failed app counts in its favor.

For Autopaper, the floors are the audited rate of unsupported claims, correction speed, the number of genuinely distinct sources, and calibration. Readership is shown only as a diagnostic, always beside the audited error rate. Once the floors pass, a fixed share of front-page slots goes to beats and forms never tried before, each with a stated reason and a date to retire it.

One addition to precommitment theory came out of this. A surprising failure of a floor is a capability problem, as the precommitment doc already says. A surprising success of a measure the agents can see is a warning sign. It is the signature of gaming, and it should send an auditor to look, not a cheer.

## Four flaws in my trust draft

The research found four places where my Bayesian trust draft is wrong. First, it lets materiality weights depend on outcomes, which is the forecaster's dilemma; they must be frozen. Second, it clips negative skill at zero, which quietly breaks the scoring's honesty. Clipping belongs in the display, never in the score. Third, it adds materiality weights as if they were extra observations, with weights as high as thirty. One important event would then count as thirty pieces of evidence, and the trust estimate would be far too confident. A tempered or capped version fixes that, and replaying old records will show which works. Fourth, it uses the agent's own probabilities inside the materiality weight. That lets an agent shrink the weight on consequences it would rather not be judged on. The weight should come from independent reference predictors.

None of these changes the idea. All four make the numbers honest, which is the whole point of the idea.

## What I'd carry forward

The research checks were themselves a small Goodhart lesson. The first metrics report read fluently and confidently, but its agent never actually searched the web: it never loaded its search tools, so every citation came from memory. When we checked against the sources, all one hundred fifteen arXiv papers turned out real, but seven citations needed correcting, and three claims shrank. The fluent version was the one to distrust, which is the same lesson the news research teaches. From now on every research brief tells the agent how to load its search tools, and nothing is marked verified from memory.

The one thing to remember is this. Honest scores need three habits: freeze the weights before the outcome, anchor the scorers to rules, spot-checks and the world, and keep the checkers out of sight. Then satisfice on floors and spend the slack on diversity.

If I were holding the phone, I would decide four things. Whether to adopt frozen weights and the flip-rate test for relevance. Whether to fix the four trust flaws now, before the plan is reviewed. Which floors to start with for self-development and Autopaper. And whether "satisfice, then diversify" is the hypothesis the paper leads with. After that, the plan goes to one review and then to Phase One.

## Names and receipts

Research reports, citation-checked: docs/research/scoring-and-materiality-2026-11-04.md and docs/research/goodhart-and-metric-design-2026-11-04.md (copied from the scratchpad files research-metascoring.md and research-goodhart-metrics-v2.md).

Design documents under discussion, not yet reviewed: the consolidated plan on branch modplan, commit f1a43fe6e, docs/design/modular-architecture-plan-2026-11-04.md; the trust draft docs/design/bayesian-trust-2026-11-04.md on the same branch.

Measurements: docs/evidence/gvisor-desk-capsule-x86-2026-11-04.md. Mission ledger with settled predictions: docs/definitions/choir-modular-architecture-2026-11-04.precommit.md (round zero, settlements in commit 1876a56da, time correction in 303eff7c0).

Key sources named above: Lerch and others on the forecaster's dilemma, arXiv 1512.09244. Ehm and others on scoring-rule mixtures, arXiv 1503.08195. Gao, Wright and Leyton-Brown on spot-checking versus peer prediction, arXiv 1606.07042. Prediction-powered inference, arXiv 2301.09633. Dorner and others on the limit of judge savings, arXiv 2410.13341. Gao and others on reward-model overoptimization, 2022. Pan and others on in-context reward hacking, 2024. The Darwin Gödel Machine paper and the Huxley-Gödel Machine follow-up. METR's 2026 maintainer review of pull requests. Liu and others, 2023, on verifiability of generative search. Taylor, 2016, on quantilizers. Basic Inc. versus Levinson and TSC Industries versus Northway on securities materiality.
