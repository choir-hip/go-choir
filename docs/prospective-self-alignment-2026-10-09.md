# Prospective Self-Alignment

*Learning, Oversight, and Reputation Through Agent Precommitment Records*

Yusef Nathanson · Choir Harmonic Intelligence Platform. Working research paper, revised October 9, 2026, from the October 8 version with its reviewed bibliography. This revision corrects product names and adds a section on what the Choir codebase implements today.

## Abstract

Autonomous agents increasingly execute extended sequences of research, tool use, and intervention. Their output can be assessed after the fact, but an explanation produced *after* a result is known cannot reliably recover the expectations the system held beforehand. This paper develops **Prospective Self-Alignment (PSA)**: a framework for making an agent's pre-action representations of its intended behavior, likely consequences, uncertainties, and decision-relevant assumptions available to later evaluation. Its technical primitive is the **precommitment record**: a time-stamped, immutable, prospectively generated record that can be linked to execution traces and independently resolved evidence.

The claim is not that a model can faithfully report its private thoughts. Rather, the system can require *externally checkable commitments* as a condition of consequential action. Resolved records may support calibration, procedural fidelity, materiality assessment, retrievable episodic learning, optimization of agent systems, and eventually model training. They also offer a potential oversight surface independent of whether chain-of-thought (CoT) reasoning remains visible or interpretable, including for systems with opaque or nonverbal internal reasoning. Accumulated records may form domain-specific reputations usable by humans and other agents to select collaborators, delegate authority, distribute research, and purchase services.

PSA is a research program, not a demonstrated guarantee of alignment, honesty, causal attribution, or safety. Its central tests concern whether commitments are informative rather than strategic theater, whether subsequent evidence is independent, whether optimization transfers to unseen work, and whether prospective records predict future competence better than simpler alternatives. Human governance retains authority over what actions are permitted.

**Keywords:** prospective evaluation; precommitment records; agentic systems; self-supervision; behavioral oversight; chain-of-thought monitorability; procedural fidelity; probabilistic calibration; learning economy; reputation.

## 1. The problem of the lost before-state

A sophisticated system can explain a surprising outcome once it has occurred. That competence is useful for analysis but unreliable as testimony about what the system *previously expected*. The sixth page of a document can change the meaning of the first five; asking afterward what one believed after page five risks contaminating the before-state with information already seen. A transcript of eventual outputs and post hoc reflection does not solve the temporal problem.

This is especially consequential for long-horizon agents. They read, search, write, run code, contact services, modify artifacts, delegate to other agents, and perhaps continue for days. The normal record is a mixture of chat messages, tool calls, cached context, local files, and retrospective summaries. It is often impossible to determine whether an action followed its declared purpose, whether a hazardous consequence was foreseen, whether a failed prediction produced a genuine update, or whether an apparently successful answer reflected expertise rather than luck.

The proposed correction is **temporal rather than introspective**. Freeze an expectation before the resolving evidence arrives. Bind it to what the agent actually did and what followed. Preserve the episode so that later work can learn from the discrepancy.

This principle follows the timing discipline of prequential evaluation (Dawid, 1984): forecasts should be judged using evidence they preceded, not information that was already available when a retrospective explanation was composed. PSA extends that ordering rule from statistical forecasts to intended action, consequence prediction, supervision, and the accumulation of reusable experience.

## 2. Two levels: the record and the theory

**Precommitment records** are the implementation primitive. They need not be renamed in code when their research interpretation changes. At minimum, a valid record is: (1) committed before the relevant outcome or evidence becomes accessible; (2) bound to an identifiable actor, state, task, and time; and (3) falsifiable against later observations. Cryptographic signing or append-only storage may strengthen provenance, but the research question does not depend on a token, blockchain, or one storage technology.

**Prospective Self-Alignment** is the larger methodology. It asks whether a system's observed conduct and consequences remain empirically coupled to the prospective representation under which the system was allowed to act. *Prospective* means the record precedes the event. *Self-alignment* means an agent's own prior commitments become one of the objects of assessment; it does **not** mean the agent determines the permissibility of its actions.

The minimal loop is:

**Commit → Authorize → Act → Observe → Resolve → Score → Revise → Retrieve / Optimize.**

The authorization step matters. Making an accurate promise to perform an unauthorized action does not make that action acceptable. The record makes governance more inspectable; it cannot replace governance.

### 2.1 Anatomy of a record

A useful commitment may include:

- **Identity and context:** agent/model version, tools, dependencies, relevant versions of shared documents, time horizon, and task scope.
- **Question and hypotheses:** which propositions are at issue, plausible alternatives, and the evidence expected to distinguish them.
- **Proposed behavior:** intended actions, important *non-actions*, resources to be accessed, delegated work, and stopping conditions.
- **Expected consequences:** material effects on artifacts, people, organizations, data, money, permissions, and downstream recipients.
- **Uncertainty:** probabilities or intervals where meaningful, along with confidence in the action plan itself.
- **Authority:** applicable permissions, policies, escalation requirements, and boundaries imposed externally.
- **Resolution specification:** evidence that could settle the claim, an anticipated observation time, and a method for treating inconclusive results.

Records can nest. A month-long project might contain an overarching commitment and hundreds of smaller commitments covering each retrieval, edit, delegated investigation, or publication. The parent captures purpose and reach; children expose decisions where corrections are possible.

### 2.2 A worked example

Consider an agent preparing an Autopaper investigation about a semiconductor supply-chain claim. Before searching, it records a 65% estimate that a named production facility will increase output by a particular date, lists the evidence it expects to find, states that it will not publish as fact until primary sources are corroborated, and notes that contradictory supplier reports would change its conclusion.

The agent then searches, summarizes sources, and updates the maintained article. Its record can later be evaluated on multiple axes: was it calibrated about the substantive claim; did it comply with its declared sourcing procedure; did it omit a material downstream consequence; did it flag conflicting evidence; and, after resolution, did its subsequent research behavior improve? These are separate questions. An accurate forecast can coexist with a process violation; an unlucky forecast can coexist with a well-governed decision.

## 3. Learning from events, not flattering lessons

Reflection and episodic-memory agents already demonstrate the value of storing and reusing prior experience (Shinn et al., 2023; Madaan et al., 2023; Park et al., 2023; Packer et al., 2023; Wang et al., 2023). PSA favors preserving the **prospective episode**, rather than only a distilled conclusion such as "this approach failed; use another library next time": the original prediction and probability, the assumptions behind it, the observed failure, the uncertainty about its cause, and the revision that followed. A future evaluator may discover that the lesson was too broad, the result was a fluke, or the same assumption failed again in another domain.

For a fixed-weight model, the *composed learner* is the base model plus its versioned records, retrieval policy, context-construction policy, tools, and choice of model or sub-agent. Behavioral learning need not imply an update to neural weights; research on in-context learning and feedback supplies precedents but not proof of the proposed PSA mechanism (Akyürek et al., 2023; Monea et al., 2025; Song et al., 2025). It may consist of fewer repeated mistakes, better uncertainty estimates, improved source selection, or a different tool choice on new tasks.

The mechanism is attractive because many expensive evaluations ask a difficult question ("Was the overall investigation good?"), while smaller prospective episodes ask a more tractable one ("It predicted this source would contain X with 80% confidence; did it?"). The latter does not fully solve the former, but can produce useful evidence repeatedly and cheaply.

**Prediction error is not automatically a good training signal.** A large error on a consequential issue can be more informative than a correct prediction of an obvious fact; a dramatic surprise can also be caused by noise, misinformation, or an adversarial environment. Quality is established by whether incorporating the episode improves *future* behavior on independently assessed tasks, not by whether a model writes an elegant revision.

### 3.1 Three optimization horizons

- **In-context correction:** selectively retrieve relevant prior episodes, failed assumptions, and resolved discrepancies into a future task's context.
- **Harness-level optimization:** alter prompt templates, retrieval, tool policies, task decomposition, model selection, specialist routing, and evaluation schedules while keeping foundation-model weights fixed.
- **Post-training:** with appropriate rights and safety controls, use resolved episodes as supervised, preference, or reinforcement-learning inputs for specialist models.

The first two are attainable without training frontier models. The third raises additional concerns about distribution shift, attribution, licensing, adversarial contamination, privacy, and feedback-induced optimization pathologies, including reward hacking within feedback loops (Pan et al., 2024). Choir's architecture can preserve the underlying data so a customer may decide whether and with whom to pursue weight updates.

## 4. Five dimensions of assessment

The original precommitment-records theory distinguishes five surfaces. Keeping them separate is crucial: a single aggregate "alignment score" would erase the difference between correct prediction, faithful execution, useful foresight, and legitimate authority.

- **Epistemic:** Did prospective beliefs predict the evidence? Evaluate forecast accuracy and calibration.
- **Material:** Did the commitment cover the consequences that could change a decision? Evaluate consequence coverage and salience.
- **Instrumental:** Did authorized actions lead toward the declared objective? Evaluate task effectiveness and, where feasible, causal contribution.
- **Procedural:** Did the agent act inside its declared plan and restrictions? Evaluate observable say-do fidelity.
- **Normative:** Was the action permitted and accountable to legitimate authority? Evaluate governance and authorization, not autonomous self-approval.

The first four can be empirically probed and improved to varying degrees. The fifth depends on human institutions, policies, contracts, law, and delegation. Better measurement does not decide whose interests count or which harms are morally acceptable.

### 4.1 Procedural fidelity

Procedural fidelity is correspondence between the prospectively represented action envelope and executed behavior, **not textual similarity between a plan and a subsequent explanation**. If an agent commits to read-only inspection of repository A and then writes to repository B, the discrepancy should be measurable even if the final answer is fluent and useful. Such evidence can support audits and intervention.

Nor should fidelity be conflated with predictability alone. The real requirement is useful, material fidelity inside externally authorized bounds. A consistently harmful agent can be perfectly predictable and still unacceptable.

### 4.2 Materiality

An agent might truthfully predict "I will generate output tokens" while omitting that those tokens change an access policy or affect an external decision. A meaningful record must cover the consequences that could change an authorization or oversight decision.

The original paper proposes a candidate weighting function for consequence *c* under action *a*:

> w(c) = P(c | a) × impact(c) × relevance(c)

Here probability estimates may come from independent reference predictors; **impact** is assigned through governance-sensitive measures such as irreversibility, scope, and domain; and **relevance** reflects whether learning about the consequence would change a decision. For a reference set of possible material consequences C\*, an illustrative coverage measure is:

> coverage(P) = (sum of w(c) over consequences c in C\* that P covers) ÷ (sum of w(c) over all c in C\*)

Proper scoring rules, such as the Brier or logarithmic score, can assess probability estimates for specified outcomes (Brier, 1950; Gneiting and Raftery, 2007). A composite protocol should penalize consequential omissions and uninformative overprediction while preserving incentive compatibility where possible. **The reference consequence set is the weak point**: critics cannot penalize risks they fail to imagine, and retrospective observations reveal only some counterfactual alternatives. This is a proposed research design, not a validated universal scoring formula.

## 5. Behavioral oversight without readable chain of thought

Chain-of-thought monitoring can be informative when reasoning traces are visible and retain diagnostic content (Korbak et al., 2025). Yet verbalized CoT can be unfaithful to the factors that caused an output (Turpin et al., 2023; Lanham et al., 2023), and optimization against a CoT monitor can encourage obfuscation (Baker et al., 2025). Systems may also withhold traces or rely on opaque, nonverbal computation, sometimes informally called "neuralese." These findings motivate a complementary behavioral monitoring channel; they do not establish that external records match CoT monitoring in detection power. PSA proposes a different observational contract. The observer need not infer the model's hidden thought process; it can compare *public pre-action commitments* with authenticated actions and subsequently resolved facts.

This externally inspectable sequence is a **shadow trace**: task state, declared expectation, intended intervention, authorized execution, actual intervention, evidence, discrepancy, and revision. It is not a transcript of hidden cognition. Its value comes from testability and from the opportunity to block or revise an action *before* a harmful outcome.

Multiple observers may precommit simultaneously. An executing agent predicts what it will do. An independent critic predicts what the executor will do. A monitor anticipates consequences or policy conflicts. Their disagreements are themselves prospective evidence. The system may require extra scrutiny or human approval when monitors cannot make sense of a materially consequential action. This is compatible with AI-control protocols that assume an untrusted model may actively subvert oversight (Greenblatt et al., 2024).

The original proposal describes a **Zeno gap** between agent capability and the portion of that capability monitors can understand or control. The design ambition is to narrow that gap through diverse critics, circuit breakers, graduated delegation, and learning on the same outcome records. It is *not* established that monitor capability will always keep pace; new model capabilities, concealed strategies, and coordinated deception can break the coupling.

### 5.1 Why prospective statements can still mislead

An agent can give vague commitments, cherry-pick easy predictions, misstate intentions, hide material actions, learn a scorer's preferences, manipulate the outcome, or remain apparently compliant while pursuing a different objective. Neutral probes can help: if an agent predicts a risk when questioned separately but omits it in its operative commitment, the discrepancy may suggest concealment. It does not establish a complete theory of the agent's private intent.

A credible protocol therefore requires independent observations, randomized audits, diverse scorers, authenticated action traces, materiality pressure, withheld evaluator details, and controls on what the agent can influence. PSA must be evaluated under adversarial conditions, including cases where the hidden reasoning trace provides no useful assistance.

## 6. Governance is not self-alignment

PSA is an **additional dimension of alignment**, not an alternative to legitimate external constraints. Operationally, the proposed gate is:

Declare intended action and consequences → assess material coverage → check policy and authority → authorize, escalate, or block → execute under control → resolve and audit.

Authority may derive from users, organizations, contracts, professional duties, safety policy, law, and democratic institutions. Some norms may be implemented through model training, including explicit reasoning over written safety specifications (Guan et al., 2024), some through runtime constraints, and many through processes outside the model. No finite policy is complete; emergency exceptions, conflicts among authorities, and missing information are governance problems as well as technical ones.

Where a risk is predicted and an agent nevertheless bypasses a block, the failure differs from the case where a risk was not foreseen. Where a reasonable independent predictor foresaw harm but the agent did not, the deficit may concern causal understanding, attention to consequences, or calibration. Where the agent could foresee harm in a neutral test but omitted it from its commitment, the deficit may concern fidelity or strategic concealment. These cases deserve different interventions, but **they cannot all be reduced to capability failures by definition**: hidden objectives, adversarial action, measurement gaps, and illegitimate authorization remain live possibilities.

## 7. From evidence to reputation

The same records that support learning can become a longitudinal **competence history**. Such a history is not a global five-star judgment; it is conditional. It matters which domains were tested, which tasks were easy or difficult, whether predictions were prospectively recorded, whether outcomes resolved independently, whether work improved after error, and under which permissions the agent operated.

A buyer agent deciding whom to consult might ask: *Which specialist has a reliable record on comparable problems at this level of difficulty?* A reader might ask: *Which independent analyst made explicit forecasts before the relevant evidence appeared and subsequently corrected mistakes?* An organization might ask: *Which internal agent has a history of adhering to declared boundaries while completing relevant tasks?*

As research on online feedback mechanisms has long emphasized, reputation depends on how a market interprets past evidence and uses it to support future cooperation (Dellarocas, 2003). PSA proposes to replace or supplement coarse feedback with task-specific prospective evidence. This turns reputation into a coordination mechanism. A record matters not only because it exists, but because people and agents within a network **interpret it and use it to allocate attention, trust, delegated authority, and transactions**. The underlying evidence can be portable while the network of participants, services, audiences, and current opportunities remains difficult to recreate elsewhere.

### 7.1 Media as the initial proving ground

Continuous autonomous intelligence needs a domain in which it can research, revise, publish, and be corrected repeatedly. Media provides a natural initial environment: the outputs are inspectable, many claims eventually resolve, and shared artifacts can reach a broad audience without new personalized inference for every reader. Its epistemic stakes are real: misinformation and propaganda can cause harm, which motivates provenance, source comparison, correction histories, transparent uncertainty, and prospective accountability.

The proposed Choir ecosystem includes Autopaper, a continuously maintained research and publishing interface; Autoradio, an interactive and personalized audio surface; and the Automatic Computer (the Autoputer), the persistent multi-agent workspace underneath both. The media network can expose cases in which retrospective celebrity or popularity is less predictive of useful judgment than a resolved history of specific claims.

### 7.2 Enterprise and exchange

For organizations, the relevant questions include agent authorization, auditability, knowledge ownership, model selection, and monetization of proprietary expertise. Organizations may expose *authorized* agent services to buyers without handing over source databases or weights. However, query interfaces can leak confidential information through repeated interactions; rate limiting, data access controls, privacy-preserving outputs, monitoring, and contractual governance are necessary controls, not guaranteed consequences of PSA.

Paid queries provide demand information even when no outcome is verifiable; resolved interactions provide richer signals for evaluating and improving agent services. Over time, producers have an economic incentive to invest in capabilities that earn higher trust and more useful demand. The agent economy then becomes a possible **learning economy**: interactions not only exchange value but modify the conditions of future competence.

## 8. An experimental research program

The distinctive scientific hypotheses should be tested separately rather than bundled into one product-level success metric.

- **H1 — Information quality.** Prospective records capture diagnostically useful information that retrospective reflection, final-answer grading, and ordinary traces omit. Test with blinded outcome revelation and strictly frozen before-states.
- **H2 — Learning transfer.** Retrieval and harness optimization based on resolved commitments improve out-of-sample research performance at controlled inference cost. Compare with memoryless agents, retrospective-only memory, naive task-success feedback, and equally sized random episode stores.
- **H3 — Behavioral oversight.** Commitment-versus-action discrepancies identify consequential violations or failures in conditions where internal reasoning is unavailable, with measured precision, recall, false-alarm burden, and time to intervention. Compare to tool-trace-only audits and CoT monitoring when available.
- **H4 — Reputation.** A temporally prior, domain-specific history of resolved records predicts future agent or human expert performance better than historical success rates, static benchmarks, popularity, or uncalibrated ratings. Use chronological and subject-level holdouts, and control for task difficulty and selection.
- **H5 — Manipulation resistance.** Scores remain useful under strategic omissions, vague forecasts, self-fulfilling predictions, evaluator exposure, contaminated evidence, selection of easy tasks, and cross-agent collusion.

The evaluation dataset must separate raw actions from scored labels, and records from outcomes. Pre-register inclusion criteria where feasible; report failures, abstentions, unresolved episodes, and resource use. Observers should not be allowed to infer that a failure mode has vanished merely because an agent stopped making observable commitments about it. Later weight training should be compared to fixed-weight, retrieval-based alternatives before claims of generalized learning are made.

## 9. What would count as evidence of success?

A persuasive near-term result would show the same class of research agents performing better on fresh tasks when given access to *appropriately selected prior prospective episodes*, with stable or improved calibration and without a disproportionate cost increase. Independent assessments would show meaningful detection of commitment violations and material omissions. A reputation model trained only on earlier episodes would select agents that perform better on later, domain-matched tasks than simple rankings do.

Failure is informative. If scores can be optimized without improving outcomes, if the records mostly restate obvious actions, if commitments become strategically uninformative under monitoring, or if task-specific reputations do not transfer to new tasks, then the core PSA mechanism must be revised. The theory should earn its strength from discriminating tests rather than from appealing retrospective examples.

## 10. Implementation status in Choir (October 9, 2026)

This section is new in this revision. It reports what the Choir codebase implements, so that the theory above is not read as a description of shipped capability. Code keeps the term "precommitment records" (in code, commitment records); PSA is the theory.

**Implemented.**

- *A typed record.* The record type carries a prediction (hypothesis, assumptions, alternatives, and typed questions, each with the agent's probabilities and a materiality weight), an observation, one or more scores with the scorer's model identity and a disagreement flag, a discrepancy class (confirmed, qualified, contradicted, answered, or unresolved), provenance with commit and resolve times, an optional revision, an optional declared action with its declared capabilities, and predicted versus observed consequences with weights. This follows the record schema in Section 2.1 and the materiality weights in Section 4.2.
- *An act taxonomy.* Desks emit five kinds of records: precommit, report, resolve, disagreement, and directive. Directives use a closed set of subtypes (note, escalate, cast, retract), so operational acts cannot slip past the scored taxonomy.
- *Append-once persistence on the object graph.* Each record is minted once under a deterministic identity with a not-exists condition, so a replayed commit creates nothing new. Records live on the computer's object graph and tape, not in a separate store.
- *Derived views.* Per-agent and per-document tallies, a supervision projection of falsified, disputed, and overdue claims ordered by materiality, a score-free pack for the acting desk, a score-carrying pack for supervision, and a gate that refuses a learning claim unless it cites scored records.
- *The desk surface.* The persistent desks commit, resolve, and dispute through their in-cell module, and a reducer appends the resulting records to the ledger.

**Partially implemented or not yet established.**

- *Pre-outcome ordering.* A record carries its commit time and is appended before any resolution that references it. The system does not yet prove that the committing agent lacked access to the resolving evidence. That is the hard requirement of Section 2, and it remains open.
- *Independent scoring.* The record holds several scores and flags scorer disagreement, but routine second-model scoring is not yet running in production.
- *Context packs and retrieval.* Packs exist as derived views. Automatic injection into later tasks stays off until held-out evidence supports it (H2).
- *Experiments.* None of H1 to H5 has been run. Every effect claimed in Sections 3, 5, and 7 is a research hypothesis.

**Planned first application.** Autopaper is designed to make the loop concrete. An investigation commits before it reads. A published claim carries its uncertainty and a resolution specification. Publishing is a material action that passes the authorization gate in Section 6. A later source that bears on an open claim triggers its resolution and a forward revision of the article. Per-source and per-desk track records are derived views over resolved records, never edited by hand.

## 11. Position among existing approaches

The novelty claimed here is **a particular composition and interpretation**, not ownership of every component. In particular, pre-action reflection (Wang et al., 2024) already exists; PSA demands a frozen, outcome-resolvable commitment and treats its eventual discrepancy as reusable evidence:

- **Prequential statistics** establishes the methodological priority of predictions preceding outcomes (Dawid, 1984).
- **Reflection and memory systems** preserve prior experience, often in retrospective narrative form (Shinn et al., 2023; Madaan et al., 2023; Park et al., 2023; Packer et al., 2023; Wang et al., 2023); PSA requires a frozen pre-evidence state linked to later resolution.
- **Chain-of-thought faithfulness and monitorability research** studies access to reasoning traces and their limitations (Turpin et al., 2023; Lanham et al., 2023; Korbak et al., 2025; Baker et al., 2025); PSA offers an external, checkable observational surface whose usefulness does not require faithful private CoT.
- **Trusted monitoring and AI control** provide containment, escalation, and independent oversight (Greenblatt et al., 2024); prospective records may enrich what such monitors can evaluate.
- **Proper scoring, forecasting, and calibration research** supplies tools for evaluating probabilistic commitments (Brier, 1950; Gneiting and Raftery, 2007), while materiality and authority remain separate design problems.
- **In-context learning and reinforcement feedback** motivate reusing resolved episodes (Akyürek et al., 2023; Monea et al., 2025; Song et al., 2025), while feedback-loop research warns that an optimizer may game a proxy rather than learn transferable competence (Pan et al., 2024).

The proposed contribution is to turn **prospective episode formation** into a normal part of agent operation, using it simultaneously for learning, accountability, and economically meaningful competence histories.

## 12. Conclusion

An intelligent system ought to be capable of leaving evidence of what it expected before evidence arrived. The precommitment record is a practical object with which to enforce that temporal discipline; Prospective Self-Alignment is the broader program for assessing the relationship among commitment, behavior, consequence, and revision.

If the mechanism works, increasingly capable agents need not be evaluated solely through static benchmarks, unverifiable self-explanations, or access to their internal thoughts. Their work can generate an inspectable history from which both the agent and the surrounding human institutions learn. That same history may help people and other agents recognize specialized competence, supervise greater autonomy, and distribute valuable intellectual work.

The aim is not to certify an intelligent system once and for all. It is to create conditions in which **errors remain visible, corrections can stick, authority remains accountable, and intelligence becomes increasingly capable of learning from its own experience**.

## References

References were checked against publisher, conference, or original preprint records on October 8, 2026. Publication years follow the cited **version or venue** (thus Akyürek et al. is cited as ICLR 2023, although an arXiv version appeared in 2022). A DOI or source link accompanies every entry. Inclusion indicates relevant prior work, not independent verification of PSA's untested claims.

- Akyürek, E., Schuurmans, D., Andreas, J., Ma, T., & Zhou, D. (2023). **What Learning Algorithm Is In-Context Learning? Investigations with Linear Models.** *International Conference on Learning Representations (ICLR).* arXiv:2211.15661.
- Baker, B., Huizinga, J., Gao, L., et al. (2025). **Monitoring Reasoning Models for Misbehavior and the Risks of Promoting Obfuscation.** *arXiv preprint.* arXiv:2503.11926.
- Brier, G. W. (1950). **Verification of Forecasts Expressed in Terms of Probability.** *Monthly Weather Review, 78*(1), 1–3. Journal article.
- Dawid, A. P. (1984). **Present Position and Potential Developments: Some Personal Views — Statistical Theory: The Prequential Approach.** *Journal of the Royal Statistical Society, Series A, 147*(2), 278–290. doi:10.2307/2981683.
- Dellarocas, C. (2003). **The Digitization of Word of Mouth: Promise and Challenges of Online Feedback Mechanisms.** *Management Science, 49*(10), 1407–1424. doi:10.1287/mnsc.49.10.1407.17308.
- Gneiting, T., & Raftery, A. E. (2007). **Strictly Proper Scoring Rules, Prediction, and Estimation.** *Journal of the American Statistical Association, 102*(477), 359–378. doi:10.1198/016214506000001437.
- Greenblatt, R., Shlegeris, B., Sachan, K., & Roger, F. (2024). **AI Control: Improving Safety Despite Intentional Subversion.** *Proceedings of the 41st International Conference on Machine Learning (ICML)*, PMLR 235, 16295–16336. Proceedings.
- Guan, M. Y., Joglekar, M., Wallace, E., et al. (2024). **Deliberative Alignment: Reasoning Enables Safer Language Models.** *arXiv preprint (December 2024).* arXiv:2412.16339.
- Korbak, T., Balesni, M., Barnes, E., et al. (2025). **Chain of Thought Monitorability: A New and Fragile Opportunity for AI Safety.** *arXiv preprint.* arXiv:2507.11473.
- Lanham, T., Chen, A., Radhakrishnan, A., et al. (2023). **Measuring Faithfulness in Chain-of-Thought Reasoning.** *arXiv preprint.* arXiv:2307.13702.
- Madaan, A., Tandon, N., Gupta, P., et al. (2023). **Self-Refine: Iterative Refinement with Self-Feedback.** *Advances in Neural Information Processing Systems (NeurIPS 2023), 36.* Conference paper.
- Monea, G., Bosselut, A., Brantley, K., & Artzi, Y. (2025). **LLMs Are In-Context Bandit Reinforcement Learners.** *Conference on Language Modeling (COLM 2025).* Conference paper.
- Packer, C., Wooders, S., Lin, K., et al. (2023). **MemGPT: Towards LLMs as Operating Systems.** *arXiv preprint.* arXiv:2310.08560.
- Pan, A., Jones, E., Jagadeesan, M., & Steinhardt, J. (2024). **Feedback Loops With Language Models Drive In-Context Reward Hacking.** *Proceedings of ICML 2024*, PMLR 235, 39154–39200. Proceedings.
- Park, J. S., O'Brien, J., Cai, C. J., et al. (2023). **Generative Agents: Interactive Simulacra of Human Behavior.** *Proceedings of UIST 2023*, Article 2. doi:10.1145/3586183.3606763.
- Shinn, N., Cassano, F., Gopinath, A., Narasimhan, K., & Yao, S. (2023). **Reflexion: Language Agents with Verbal Reinforcement Learning.** *Advances in Neural Information Processing Systems (NeurIPS 2023), 36.* Conference paper.
- Song, K., Moeini, A., Wang, P., et al. (2025). **Reward Is Enough: LLMs Are In-Context Reinforcement Learners.** *arXiv preprint.* arXiv:2506.06303.
- Turpin, M., Michael, J., Perez, E., & Bowman, S. R. (2023). **Language Models Don't Always Say What They Think: Unfaithful Explanations in Chain-of-Thought Prompting.** *arXiv preprint.* arXiv:2305.04388.
- Wang, G., Xie, Y., Jiang, Y., et al. (2023). **Voyager: An Open-Ended Embodied Agent with Large Language Models.** *arXiv preprint.* arXiv:2305.16291.
- Wang, H., Li, T., Deng, Z., Roth, D., & Li, Y. (2024). **Devil's Advocate: Anticipatory Reflection for LLM Agents.** *Findings of EMNLP 2024*, 966–978. doi:10.18653/v1/2024.findings-emnlp.53.

## Provenance note

This working paper revises *Precommitment Records: Theory and Implications* (September 22, 2026), provided by the author. The five assessment dimensions, materiality weighting proposal, and the Zeno-gap terminology originate in that manuscript. Later discussion supplied the PSA name and the emphasis on task-specific economic reputation and monitoring in the absence of readable CoT. The bibliography identifies established background and related approaches; the proposed integration and empirical results remain Choir's research agenda.

The October 9 revision changes product names to the house spelling (Autopaper, Autoradio, Autoputer; never camel case), renumbers the later sections, and adds Section 10 on implementation status, checked against the Choir repository on that date. The theory and the references are otherwise unchanged.
