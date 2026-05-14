---
name: cognitive-transform-portfolio
description: Use when an agent is stuck, shallow, over-literal, audience-misaligned, cargo-culting a slogan/framework, or needs route-changing lenses before implementation, verification, writing, strategy, or MissionGradient work. Select 2-5 cognitive transforms that change the next probe, route, scope, verifier, or stopping condition.
version: 1.0.0
author: Hermes Agent
license: MIT
metadata:
  hermes:
    tags: [cognitive-transforms, reasoning, writing, strategy, mission-gradient, audience-translation]
    related_skills: [mission-gradient]
---

# Cognitive Transform Portfolio

## Overview

A cognitive transform is an operator that changes the representation of a problem so new actions, risks, invariants, or values become visible.

The goal is not to make agents sound smarter. The goal is to give agents a repertoire of ways to escape local minima, reduce shallow satisficing, activate lateral knowledge, and translate ideas across audiences without becoming decorative.

A transform is not a topic. It is a move. If it does not change the next probe, implementation, verifier, scope, evidence plan, or stopping condition, it is commentary.

```text
A transform that does not change action is commentary.
A transform that changes action is cognition.
```

## When to Use

Use this skill when:

- a mission is stuck, vague, shallow, or over-literal;
- an agent is repeating conventional wisdom or cargo-culting a framework;
- a concept is true but not landing for the current audience;
- the next move depends on reframing the problem, not just doing more work;
- a MissionGradient needs better homotopy parameters, invariants, verifiers, or stopping rules;
- a writing artifact needs translation across technical, public, investor, artistic, or philosophical audiences;
- a slogan, framework, or management cliché feels right but too flat;
- before giving up, when a high-integrity stop needs one more value-of-information pass.

Do **not** use this skill to pad outputs with intellectual decorations. Do not apply every transform. Select a few whose outputs could change action.

## Operating Rule

The portfolio begins with two primary transforms because they represent the two most general movements of thought:

1. **Audience-Level Translation** — accessibility through perspective shift.
2. **Depth Extraction / Esoteric Upgrade** — depth through recovery of the hidden load-bearing truth.

They are not mandatory for every use. They are primary because almost every serious problem can fail either by becoming inaccessible to the audience that must act, or by becoming shallow enough to lose the real mechanism.

After those, select **2-5 transforms total** whose outputs could change the route, verifier, scope, evidence plan, or stopping condition.

Then produce:

```text
Current uncertainty or obstacle:
...

Selected transforms:
1. <name> — why this lens matters here
2. <name> — why this lens matters here
3. <name> — why this lens matters here

Route-changing insights:
- ...

Changed plan:
- implementation:
- verifier/evidence:
- scope:
- stopping condition:

Next high-information action:
...
```

If the transforms do not change the plan, say so and stop. Do not launder decorative analysis into action.

## Core Discipline

1. **Name the real object.** Is this a file, process, relationship, market, ritual, theorem, memory, state machine, public symbol, learning system, or projection of a deeper object?
2. **Find the load-bearing variable.** What actually changes outcomes?
3. **Preserve invariants.** What must remain true across acceptable transformations?
4. **Seek high-information probes.** What observation would most reduce uncertainty?
5. **Prefer route-changing lenses.** A transform matters if it changes the next action.
6. **Maintain evidence contact.** Every transform should eventually touch tests, traces, artifacts, users, sources, or other evidence.

## 1. Audience-Level Translation

This is the accessibility transform: change perspective and register so the same structure becomes usable to a specific audience.

Re-explain the same idea for a specific audience without losing the core structure.

Use when:

- the concept is true but not landing;
- the audience lacks the original background;
- the current explanation uses the wrong register;
- the idea needs to travel between technical, business, artistic, political, public, or intimate contexts.

Ask:

- What does this audience already understand?
- What words will mislead them?
- What analogy preserves the real structure?
- What must be omitted for now?
- What must not be distorted?
- What would make them able to act on it?

Examples:

- ELI5;
- ELI sixth grade;
- ELI coding bootcamp grad;
- ELI Linux/CLI user;
- ELI product manager;
- ELI CTO;
- ELI investor;
- ELI grad student;
- ELI theoretical physicist;
- ELI historian;
- ELI artist;
- ELI policymaker;
- ELI polymath.

Output format:

```text
Audience:
Core idea:
Words to avoid:
Analogy:
Explanation:
What this audience can do with it:
What nuance is deferred:
```

Example:

```text
Audience:
AI engineer comfortable with CLI/Linux/Claude Code, not math-heavy.

Core idea:
Long-running agents need directional guidance, invariants, evidence, and stop rules.

Words to avoid:
continuous, discrete, topology, optimization, homotopy, category theory.

Analogy:
Compass plus sysadmin safety rules.

Explanation:
Mission gradient tells the agent how to keep making good choices when the checklist is no longer enough.

What this audience can do with it:
Write better long-run prompts, prevent slop, define proof, and avoid fake demos.

Deferred nuance:
Mathematical interpretation as an invariant-preserving optimization field.
```

Polished plain-language version:

> Mission gradient is a way to prompt agents so they optimize toward a durable direction instead of just completing a brittle checklist.

## 2. Depth Extraction / Esoteric Upgrade

This is the depth transform: recover the fundamental hidden truth beneath a familiar surface.

Recover the deep mechanism beneath a familiar concept, slogan, framework, or commonplace.

Ask:

- What is the banal interpretation?
- What was the original deeper insight?
- What variable is actually load-bearing?
- What does the shallow version optimize incorrectly?
- What would change if we optimized the deep version instead?
- What practice would make the deeper version real?

Use this transform when a common concept feels true but too flat, or when a team is cargo-culting a framework.

Output format:

```text
Concept:
Banal version:
Deep version:
Load-bearing variable:
Common failure mode:
Operational implication:
```

Examples:

**OODA loop**

- Banal version: move faster.
- Deep version: update orientation faster and more accurately than the opponent.
- Load-bearing variable: learning rate under uncertainty.
- Failure mode: rushing action while preserving a wrong model.
- Operational implication: improve sensors, interpretation, culture, and reorientation, not just tempo.

**MVP**

- Banal version: build the smallest crappy version.
- Deep version: build the smallest real artifact that tests the load-bearing uncertainty.
- Load-bearing variable: valid learning.
- Failure mode: fake prototype that avoids the hard topology.
- Operational implication: minimal real proof, not demo slop.

**Agile**

- Banal version: move fast, have standups, ship tickets.
- Deep version: shorten feedback loops between reality and artifact.
- Load-bearing variable: adaptive correction.
- Failure mode: ritualized sprint bureaucracy.
- Operational implication: every ritual must increase contact with reality or be deleted.

**Trust**

- Banal version: people like our brand.
- Deep version: people believe our future behavior will respect their interests under stress.
- Load-bearing variable: credible restraint.
- Failure mode: competence theater without care or candor.
- Operational implication: trust is built when you forgo extraction you could have taken.

**Open source**

- Banal version: code is free.
- Deep version: users can inspect, fork, learn from, and collectively improve the substrate.
- Load-bearing variable: agency over infrastructure.
- Failure mode: open repo with closed ecosystem economics.
- Operational implication: open source needs governance, provenance, and usable ownership.

**Safety**

- Banal version: prevent bad outputs.
- Deep version: maintain system viability under increasing capability and adversarial pressure.
- Load-bearing variable: controlled power.
- Failure mode: safety as access-control branding.
- Operational implication: safety claims must be separable from rent-seeking.

**Taste**

- Banal version: knowing what is cool.
- Deep version: calibrated perception of what remains alive after obvious signals are discounted.
- Load-bearing variable: durable information under repeated exposure.
- Failure mode: signal imitation, tasteslop, or snobbery.
- Operational implication: judge replay value, not first-look impressiveness.

**Decentralization**

- Banal version: no central authority.
- Deep version: no single actor can corrupt, censor, or capture the system without broad consent.
- Load-bearing variable: credible resistance to capture.
- Failure mode: decentralized ceremony with centralized dependencies.
- Operational implication: inspect governance, infrastructure, identity, funding, and upgrade paths.

**Personalization**

- Banal version: adapt to the user's preferences.
- Deep version: improve the user's future agency by learning their context without trapping them in themselves.
- Load-bearing variable: agency-increasing adaptation.
- Failure mode: addiction, filter bubbles, sycophancy.
- Operational implication: personalize toward growth, not merely comfort.

**Product-market fit**

- Banal version: users like it and revenue grows.
- Deep version: a product has entered a self-reinforcing social/economic niche where use, value, distribution, and retention compound.
- Load-bearing variable: compounding demand.
- Failure mode: paid demand or hype mistaken for fit.
- Operational implication: watch pull, retention, referrals, and urgency.

This transform is especially useful for agents because models often know the slogan and the surface explanation. The transform forces them to retrieve the underlying mechanism. It is an anti-cargo-cult operator.

For MissionGradient, this matters because a mission can be derailed by shallow interpretations of good words. “Verify,” “simplify,” “move fast,” “ship,” “safe,” “open,” “agentic,” “memory,” and “autonomy” all have fake versions and deep versions. Ask: what is the non-banal version of the instruction?

## Transform Menu

Use this as a compact search menu. Choose a few; do not run the whole list.

### Frame and ontology

- **Object transform:** What is the real object: file, process, relationship, market, ritual, theorem, memory, state machine, public symbol, learning system?
- **Name transform:** Rename the object and watch the affordances change. “Sandbox” -> “computer” changes the local gradient.
- **Substrate transform:** What substrate does this live on: code, hardware, habit, incentive, myth, law, social proof, memory, capital, energy, institution, relation?
- **Interface transform:** Is the apparent problem an interface between systems: user/tool, market/product, model/artifact, state/firm, past/future?
- **Boundary transform:** Move the boundary. Is this inside the product, user, institution, model, or harness?
- **Unit transform:** Change the unit: token, task, run, computer, user, household, firm, school, scene, nation, civilization.
- **Category error transform:** Are we treating a learner as a database, a society as an economy, a computer as a sandbox, or a relationship as a transaction?
- **Projection transform:** What higher-dimensional object is projected into this surface, and what is lost?
- **Latent variable transform:** What hidden variable would make confusing observations unsurprising?
- **Reification transform:** What story, market, institution, or metric is being mistaken for a natural fact?

### Mathematical and logical

- **Inversion:** What would make this fail? What would the opposite imply?
- **Duality:** Swap object/observer, supply/demand, proof/counterexample, user/vendor, state/transition, syntax/semantics.
- **Contrapositive:** If B is absent, what must be absent upstream?
- **Fixed point:** What happens when the system acts on itself?
- **Limit case:** Take it to zero, infinity, 10x, 100x, one user, one billion users, one second, one decade.
- **Continuity:** Is there a continuous path from current state to target, or does the plan require miracle?
- **Homotopy:** Can the low-resolution version deform into the high-resolution version while preserving topology?
- **Commutative diagram:** Do two paths to the same result agree?
- **Invariant:** What must remain true across all acceptable solutions?
- **Conservation law:** What is conserved: attention, money, trust, entropy, information, authority, energy, responsibility, risk?
- **Symmetry / broken symmetry:** What remains unchanged under swaps? Where did neutrality choose a direction?
- **Basis transform:** Represent the problem in incentives, topology, UX, energy, information, power, time, trust, or beauty.
- **Dimensional analysis:** What are the units: trust per interaction, learning per artifact, risk per promotion?
- **Counterexample search:** What single case would break the claim?
- **Constructive proof:** Can I build the object, not just argue it should exist?
- **Relaxation / hardening:** What if a hard constraint is relaxed, or a soft preference is made hard?
- **Discrete-continuous:** Am I treating a continuous gradient as a checklist, or a discrete gate as vibes?

### Optimization and learning

- **Loss function:** What is the system actually optimizing?
- **Mission gradient:** What is uphill for the whole artifact, not merely the subtask?
- **Credit assignment:** Which prior action caused the outcome?
- **Exploration/exploitation:** Should we search more or exploit what we know?
- **Value of information:** What observation would most reduce uncertainty?
- **Active learning:** What question should the system ask the world next?
- **Curriculum:** What easier task teaches the hard capability without faking it?
- **Regularization:** What prevents overfitting to this user, benchmark, demo, prompt, mood, or market moment?
- **Generalization:** What would make this travel across contexts?
- **Local minimum:** What feels good now but traps future work?
- **Satisficing:** Is the success criterion too weak?
- **Pareto frontier:** What tradeoffs are irreducible? Which options are dominated?
- **Gradient hacking:** How could an agent satisfy the metric while violating the mission?
- **Reward model critique:** What would a rater like that a real user would not?
- **Transfer learning:** What learning from another domain, user, run, or artifact applies?
- **Catastrophic forgetting:** What valuable prior capability/context would be destroyed?

### Scientific

- **Instrument:** What instrument created the observation, and what does it fail to see?
- **Measurement without theory:** Are we collecting data without knowing what counts as explanation?
- **Theory without measurement:** Are we producing elegant abstractions with no contact point?
- **Falsification:** What result would revise or abandon the hypothesis?
- **Mechanism:** What mechanism could generate the pattern?
- **Causal graph:** What causes what? What are confounders, mediators, colliders, feedback loops?
- **Intervention:** What would change if we intervened rather than observed?
- **Natural experiment:** Has reality already varied the condition?
- **Phase transition:** Is this gradual change or regime shift?
- **Order parameter:** What variable reveals the phase of the system?
- **Noise model:** Is the error random, adversarial, biased, censored, survivorship, or selection?
- **Replication:** Could another system reproduce this under different conditions?
- **Scale separation:** Which variables move fast, slow, and separately?
- **Thermodynamic:** Where is energy dissipated as heat? What work leaves durable structure?
- **Information-theoretic:** What reduces uncertainty? What is compression, signal, noise, entropy?

### Engineering and architecture

- **State machine:** List states, transitions, impossible states, and stuck states.
- **Single-writer:** Who owns mutation of this object?
- **Idempotence:** Can this operation be retried safely?
- **Rollback:** How do we undo this without losing unrelated good work?
- **Failure mode:** How does this fail under load, bad input, partial outage, stale state, concurrency, or malicious use?
- **Trust region:** What is the safe mutation radius for the next move?
- **Gluing:** Do local fixes agree on overlaps? Do modules compose?
- **API contract:** What contract must hold between caller and callee?
- **Observability:** What would we need to see to know it is working?
- **Backpressure:** What happens when downstream capacity is exceeded?
- **Queue:** What should wait, drop, retry, or escalate?
- **Latency:** What must be realtime, nearline, batch, or archival?
- **Cost surface:** Where does cost scale: users, tokens, memory, disk, bandwidth, support, trust, coordination?
- **Build versus buy:** What learning do we lose if we outsource this?
- **Deletion-first:** What can be removed, collapsed, or reused instead of added?
- **Primitive:** What primitive, if it existed, would make this easy?

### Systems and cybernetics

- **Feedback loop:** What signal changes behavior? Is it fast, slow, noisy, delayed, or gamed?
- **Second-order cybernetics:** How does the observer change the system, and the system change the observer?
- **Controller:** Who controls what, with what sensors, actuators, model, and objective?
- **Sensor/actuator split:** What does the system perceive, and what can it change?
- **Loop gain:** Is feedback too weak to matter or too strong to stabilize?
- **Delay:** Where does latency create oscillation or stale action?
- **Homeostasis:** What is the system trying to keep stable?
- **Autopoiesis:** What does the system reproduce to remain itself?
- **Viable system model:** What functions are needed for survival: operations, coordination, control, intelligence, policy?
- **Recursion:** Where does the system contain a smaller version of itself?
- **Observer hierarchy:** Who observes the observer? Who audits the auditor?
- **Self-reference hazard:** Does the system's representation of itself distort behavior?
- **Ashby variety:** Does the controller have enough variety to control the environment?

### Economic and market

- **Total versus marginal value:** What does market price hide?
- **Externality:** Who receives costs/benefits outside the transaction?
- **Public goods / commons:** What will be underproduced or overused under private optimization?
- **Rent versus value:** Is profit from creating value or controlling access?
- **Market design:** What incentives make good behavior individually rational?
- **Game theory:** Players, payoffs, moves, information sets, equilibria, threats, commitments.
- **Adverse selection / moral hazard:** Who shows up? Who takes risks because others bear downside?
- **Principal-agent:** Who decides, benefits, pays, and bears blame?
- **Liquidity:** What becomes dangerous when made liquid?
- **Capital stack:** Which layer captures surplus?
- **Consumer surplus:** Who gets more capability, time, dignity, or agency?
- **Diffusion:** What happens when this reaches teenagers, schools, SMBs, nonprofits, or the Global South?

### Organization and leadership

- **Owner:** Who wakes up responsible?
- **Decision rights:** Who decides, vetoes, and must be consulted?
- **Bottleneck:** What constrains progress: money, talent, trust, compute, taste, distribution, law, attention?
- **Cadence:** What should happen daily, weekly, quarterly, annually, never?
- **Meeting:** Is this decision, alignment, discovery, ritual, performance, or avoidance?
- **Talent density:** Would a smaller sharper team outperform a larger one?
- **Culture as optimizer:** What does the culture make easy or impossible?
- **Narrative repair:** What story makes necessary action feel natural?
- **Pre-mortem:** Assume failure. What happened?
- **Post-mortem before death:** What would we wish we had instrumented earlier?
- **Delegation:** What should leader, system, and worker each own?

### Psychology and social

- **Care:** Does the system perform competence, or demonstrate care?
- **Trust dimensions:** Competence, candor, care, restraint, continuity, fit.
- **Status:** What is being done for rank, not truth?
- **Attachment:** Is the user becoming more capable or dependent?
- **Sycophancy:** Where is agreement rewarded over truth?
- **Projection / shadow:** What part of the observer is being attributed or disowned?
- **Cognitive dissonance:** What belief must be preserved?
- **Addiction loop:** What stimulus-reward cycle narrows agency?
- **Agency restoration:** What action returns ownership to the user?
- **Boundary:** Where is generosity becoming porosity?
- **Familiarity versus quality:** Is this good, or merely acclimated?

### Government, law, and political economy

- **State capacity:** Can the state implement and enforce the policy?
- **Legibility:** What must be simplified to be governed, and what is destroyed?
- **Sovereignty:** Who has final authority over data, compute, law, security, money, territory?
- **Regulatory capture:** Who writes the rule? Who benefits from complexity?
- **Rights / due process:** Whose claims become enforceable, against whom, before what process?
- **Monopoly:** Is safety becoming access-control language?
- **Diffusion versus control:** Does policy increase broad capacity or preserve elite chokepoints?
- **Public legitimacy:** Would people accept this if they understood it?
- **Democracy:** Are people more able to participate in decisions shaping their lives?

### Military and strategy

- **Center of gravity:** What, if disrupted, collapses the system?
- **OODA loop:** Who observes, orients, decides, acts better under changing reality?
- **Deception:** What does the opponent want us to believe?
- **Escalation:** What action forces a higher-level response?
- **Deterrence:** What credible costly threat prevents action?
- **Logistics:** Can the system be supplied, repaired, powered, and maintained?
- **Terrain:** Physical, informational, legal, social, economic, cognitive.
- **Defense in depth:** What happens when the first line fails?
- **Red team:** How would a capable adversary attack this?
- **Culminating point:** When does advance overextend the system?

### Design, architecture, UX, and art

- **Affordance:** What does the object invite the user to do?
- **Friction:** Which friction is bad drag, and which preserves judgment?
- **Material:** What does the medium want: text, audio, screen, paper, body, voice, room, street?
- **Embodiment:** How does this feel in hand, eye, ear, posture, breath, attention?
- **Prosody:** Does this unfold well in time?
- **Legibility versus mystery:** What should be obvious, and what should invite discovery?
- **Taste calibration:** Is the move cheap, earned, overused, novel, durable, or fake?
- **Public-space:** Who is forced to experience this?
- **Maintenance:** How does the object age?
- **Accessibility:** Who is excluded?
- **Prototype honesty:** Does the prototype preserve topology or fake the hard part?
- **Product smell:** What feels wrong before it can be explained?
- **User dignity:** Does the product respect agency and attention?

### Storytelling, media, humanities, myth

- **Narrative arc:** What changes from beginning to end?
- **Character desire:** What does each actor want, and refuse to know?
- **Scene:** Can this be shown as a scene rather than explained?
- **Mythic role:** Hero, trickster, tyrant, child, exile, witness, prophet, monster, fool.
- **Genre:** Tragedy, comedy, quest, horror, satire, romance, procedural.
- **Voice:** What voice can carry this truth without killing it?
- **Symbol / motif:** What concrete image condenses the abstraction? What returns with variation?
- **Canon:** What older work does this join, reject, invert, or redeem?
- **Critique:** What power relation is hidden by the surface?
- **Replay value:** What remains after the first surprise?
- **Close reading:** What does the text actually say, word by word?
- **Rhetorical triangle:** Who speaks, to whom, for what purpose?
- **Tone/subtext/intertext:** What is carried beneath the literal claim?

### Philosophy, religion, ecology

- **Ontology:** What kind of thing is this?
- **Epistemology:** How do we know? What counts as knowledge?
- **Ethics:** What is good, who is harmed, who is obligated, what cannot be traded?
- **Teleology:** What is this for? What would fulfillment look like?
- **Phenomenology:** What is the lived experience?
- **Hermeneutic:** What interpretation makes parts cohere?
- **Dialectic:** What contradiction generates motion?
- **Genealogy:** Where did this concept come from, and whose interests did it serve?
- **Pragmatism:** What difference would believing this make in action?
- **Sacred/profane:** What must not be optimized, priced, or casually touched?
- **Idolatry:** What tool, metric, market, model, or person is being worshipped?
- **Grace/prevenience:** What comes before conscious choice and makes response possible?
- **Covenant:** What mutual obligation creates durable relation?
- **Stewardship:** What power is held in trust rather than owned absolutely?
- **Seven-generation:** What does this do to people not yet born?
- **Kinship:** What if the nonhuman world is relation, not resource?
- **Reciprocity:** What is taken, what is given back, what is owed?
- **Commons:** What must be governed collectively because private optimization destroys it?

### AI-specific and agentic work

- **Model versus system:** Is intelligence in the model, harness, tools, memory, artifacts, users, or selection loop?
- **Weights versus memory:** What should live in weights, context, DB, artifact graph, tool, test, or policy?
- **Capability elicitation:** Is the model incapable, or is the interface failing to elicit capability?
- **Policy attractor:** What behavior is deeply baked into post-training?
- **Model plurality:** Would another model be better as reasoner, renderer, critic, verifier, or researcher?
- **Renderer separation:** Should reasoning output become an intermediate representation, then be compiled into prose/audio/UI?
- **Agent leash:** What duration is safe: 10 minutes, 1 hour, 8 hours, 60 hours?
- **Tool-use:** Would a symbolic/programmatic tool outperform model perception?
- **Context hygiene:** What context is load-bearing, stale, distracting, or poisoning?
- **Run geometry:** Is this a task, run, leap, or fly?
- **Promotion:** Should this mutate canonical state, candidate state, or only produce evidence?
- **Human-in-gradient:** Where should the human steer trajectory rather than micromanage tokens?
- **Belief state:** What does the agent believe, why, and what is uncertain?
- **Receding-horizon control:** What is the next useful observation, not the whole fantasy plan?
- **Candidate world:** Should this be explored in a disposable branch/computer rather than canonical state?
- **Compaction:** What learning should survive the run?
- **Anti-completion:** What if “done” is premature?
- **Quality gradient:** Is this merely working, or durable and worth retaining?
- **Prompt-to-artifact:** Should this instruction become a durable artifact, skill, tool, test, or doc?

### Stop and reorientation

- **High-integrity stop:** Stopping with a precise blocker is better than continuing into slop.
- **Obstacle reframing:** Is the obstacle technical, conceptual, organizational, economic, emotional, or political?
- **Narrowing:** What smaller real problem preserves topology and can be solved now?
- **Branching:** Should two candidate paths run cheaply instead of arguing?
- **Fallback:** What lower-tech approach works reliably?
- **Escalation:** What requires human judgment, new tools, new data, or changed mission?
- **Residual risk:** What remains dangerous after success?
- **Learning extraction:** Even if the attempt fails, what did it teach?
- **Return-to-mission:** Does the next action move uphill under the original mission gradient, or has the mission changed?
- **Give-up audit:** Before giving up, apply inversion, minimal real proof, deletion, and value-of-information.

## Common Pitfalls

1. **Applying too many transforms.** Pick 2-5. More is usually decorative.
2. **Producing insight without changed action.** If the route, verifier, scope, evidence plan, or stopping condition does not change, the transform did not do work.
3. **Using esoterica as style.** Depth Extraction is not mystical garnish; it must recover the load-bearing mechanism.
4. **Over-translating for the audience.** Audience-Level Translation may omit nuance temporarily, but must not distort the core structure.
5. **Treating MissionGradient and this skill as the same.** MissionGradient defines run geometry. Cognitive transforms are optional lenses that can improve a mission's route, invariants, verifier, or stopping rule.
6. **Letting analysis replace evidence.** Transforms must eventually touch tests, traces, artifacts, sources, user behavior, or other observations.
7. **Ignoring taste.** A technically correct transform can still produce dead prose or ugly artifacts; run a taste check when the output is public-facing.

## Verification Checklist

- [ ] Selected 2-5 transforms, not the whole menu.
- [ ] Each selected transform states why it matters here.
- [ ] Output identifies route-changing insights.
- [ ] Changed plan includes implementation, verifier/evidence, scope, and stopping condition.
- [ ] Next high-information action is explicit.
- [ ] Audience-Level Translation preserves core structure when used.
- [ ] Depth Extraction identifies banal version, deep version, load-bearing variable, failure mode, and operational implication when used.
- [ ] No decorative jargon remains unless it changes action.
