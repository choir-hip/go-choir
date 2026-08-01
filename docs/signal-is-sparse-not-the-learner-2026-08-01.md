# The Signal Is Sparse, Not the Learner

**Memo — 1 August 2026**

> Foundational argument for the Choir vision
> ([`choir-vision.md`](choir-vision.md)). Read before the doctrine and
> architecture docs: this is why the environment — the tape, the harness, the
> plurality, the settlement spine — is the durable layer, and why supervision
> is the binding constraint. Status: memo, not doctrine. Where it conflicts
> with [`choir-doctrine.md`](choir-doctrine.md), the doctrine is apex.

## The misdiagnosis

The standard complaint about neural networks is that they are sample-inefficient. A
child learns a word from two exposures; a model needs the word ten thousand times. The
complaint is usually made as a claim about architecture — transformers are the wrong
inductive bias, backpropagation is the wrong credit assignment, deep learning is
brute-force where biology is elegant.

Consider the alternative reading. The learner is not inefficient. The *signal* is sparse,
and it is sparse because nothing in the training loop is directing it.

A model trained on next-token prediction over a corpus receives an enormous number of
observations and almost no information about which observations matter. Every token is
weighted the same. The gradient does not know that this sentence resolved a confusion the
model had, that this paragraph contradicted something it believed, that this passage was
the one worth ten thousand of the others. The corpus is not organized around the model's
ignorance. It is not organized at all.

The child is not sample-efficient in some architectural sense. The child is *asking*. The
child has a running model of what it does not know, encounters a word in a context where
that gap is live, and receives a correction from an adult who can see the gap. Two
exposures suffice because the two exposures were selected — by the learner's own
attention, by the environment's response, by a teacher who knows what the learner
believes. The signal per observation is enormous because the observation was chosen
against a standing question.

This reframes the whole efficiency debate. Sample efficiency is not a property of the
learner in isolation. It is a property of the coupling between a learner and a source of
correction. Break the coupling and any learner looks inefficient. Restore it and the
apparent inefficiency was never in the architecture.

## Where the signal actually comes from

Reinforcement learning from verifiable rewards worked because it briefly restored the
coupling. Math and code have oracles. The model attempts, the oracle answers, and the
answer is *about that attempt* rather than about the world in general. Signal density goes
up by orders of magnitude and capability follows.

The problem is that the domains with oracles are a small and shrinking frontier. Math has
one. Competitive programming has one. Most of what people actually need has no verifier
at all: whether an argument is sound, whether a design is good, whether a claim about a
contested event is adequately sourced, whether this refactor will make the system easier
to reason about in six months. These are not soft domains. They are domains where the
correction arrives late, arrives partially, and arrives from a human who was not
instrumented to deliver it.

So the industry is in the position of having solved signal density exactly where nature
provided a free verifier, and having no plan for everywhere else. The current answer is
scale: more compute, more parameters, more test-time search. That is a bet that density
can be substituted by volume. It is the same bet as reading ten thousand sentences instead
of asking one question.

## Why swarms of the same model do not help

The most recent version of the volume bet is multi-agent inference. Run many instances,
let them work together over long horizons, aggregate. It looks like an answer to the
sparsity problem because it resembles a committee, and committees do reduce error.

They reduce error to the extent the errors are independent. Agents drawn from one model
family share pretraining data, post-training objectives, refusal boundaries, stylistic
priors, and — critically — the same blind spots. In distribution they disagree on details
and the aggregation appears to be working. Out of distribution they fail *together*, and
the swarm converts one model's error into a confident consensus. Aggregation without
independence is not error correction. It is amplification with a quorum.

The field's own strongest result on multi-party collaboration said this before the swarms
arrived. Cicero — Noam Brown's Diplomacy work, his last from FAIR before OpenAI — did not
achieve human-level play by self-play alone. Its planning module was deliberately anchored
to human data, because unanchored optimization drifted toward strategies and language that
no human counterpart would coordinate with. Two-player zero-sum yields to self-play
because the game supplies the verifier. Multi-party collaboration did not, and the fix was
to import grounding from outside the optimization loop.

That is the result, and it is worth stating plainly: *the diversity that makes
collaboration informative has to come from outside the model.* Brown went from that result
to inference scaling and then to multi-agent RL — both of which are attempts to buy more
search inside a single distribution. The swarm is an attempt to get from inside the family
what Cicero showed had to come from outside it.

## The two things that actually densify signal

If sparsity is the problem, there are exactly two ways to fix it, and both are
architectural rather than statistical.

**Self-direction.** A system that maintains a standing account of what it believes,
what it is uncertain about, and what would change its mind can *select* its observations.
It reads the source that bears on the open question rather than the next source in the
queue. It notices when new evidence contradicts a prior claim, because the prior claim is
recorded rather than dissolved into a transcript. Every observation is evaluated against a
live question, which is what makes two exposures enough. This is not a training-time
technique. It is a property of the environment the system operates in — whether that
environment holds standing state or resets every session.

**Genuine plurality.** Independent errors require independent origins. Models from
different families, trained differently, failing differently, adjudicated against each
other with the disagreements preserved rather than collapsed to a verdict. Three models
splitting two-to-one carries more information than the majority answer, and the
information is destroyed by any aggregation step that reports only the winner. Plurality
also has to be *architecturally available*: a harness that decides which models you may
consult is a harness that caps how independent your errors can be. This is why owning the
harness is not a preference. It is the precondition for the only kind of ensembling that
purchases anything.

Both of these are outside the model. Neither arrives on the scaling curve.

## Why this makes self-development an imperative rather than an ambition

The uncomfortable consequence: a system that runs continuously will encounter conditions
its author did not anticipate, and if every adaptation requires the author, the system's
ceiling is the author's availability. This is not a capability argument. It is an
arithmetic one. Long-horizon operation plus a static harness equals a system that is stale
between deployments, and in adversarial domains staleness is the vulnerability.

So the system must be able to change itself. And the moment that is true, supervision
becomes the binding constraint rather than capability — which is, on the current public
evidence, exactly where the frontier is. The leading labs are producing capability faster
than it can be safely deployed, and the deployments that exist push humans into chat
streams and approval queues: append-only surfaces with no head, no diff, no provenance, no
way to ask what the system believed at 3am versus now. Inspectability without legibility.
Two hundred agent trajectories, each auditable, none comprehensible.

The resolution is that self-development and self-supervision are the same engineering
problem approached from two sides. A system may modify itself only insofar as the
modification is a typed transaction on an audit log, projected into an artifact a human
can read at a glance and interrogate on demand. Correction is an ordinary write. Rival
proposals fork; settlement selects under an explicit rule; new admissible evidence
supersedes a settled claim. Effects that are irreversible stop at a human node, and they
are rare because most things are reversible. Supervision of last resort: rarely exercised
by design, never made to vanish.

That architecture is what makes the learning signal dense, and it is the same architecture
that makes autonomy safe. The system knows what it believes, so it can select what to
learn from. It records disagreement, so plurality survives aggregation. It keeps history,
so a correction can be traced to what it corrected. None of this is a safety tax on
capability. It is the mechanism by which capability compounds at all.

## The claim, compressed

Sample inefficiency is a symptom of undirected learning, not a property of the
architecture. Density comes from a learner with standing questions and from correction by
genuinely independent others. A model family cannot supply its own independence, and a
system without persistent state cannot supply its own direction. Both have to be built
into the environment the intelligence runs in — which means the durable layer of this
industry is not the model. It is the environment.
