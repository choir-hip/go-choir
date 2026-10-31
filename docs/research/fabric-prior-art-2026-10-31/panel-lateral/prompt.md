# Choir resource fabric: prior-art research brief (2026-10-31)

Choir is a persistent personal computer for each person: an agent system that runs in a VM, whose whole state is an append-only, hash-chained event log called the tape. Today all computers run on one server (Node B). The end state, sketched in docs/design/resource-fabric-2026-10-31.md (read §0, §10 and skim the rest; also docs/design/branching-tape-2026-10-31.md), is a "resource fabric": every machine an owner or organization brings (servers, Mac/Windows/Linux workstations, sandbox machines) is a site; phones and browsers are clients. Work (a computer's single writer, leaf VMs for burst, native capabilities like computer use, microphone, on-device models, child computers controlled by another computer) is placed on sites.

The owner does NOT want committal decisions yet. They want the architecture derived from first principles: the simplest, most general mechanism (a small kernel with proven properties), with organization-specific behavior as configuration/policy layered on top, so that every reasonable configuration is safe and the computer itself can later tune policy within proven bounds (self-development). Criteria: optimal, general, secure, simple, scalable, stable, facilitates self-development. Many organizations with different needs; e.g. an org generally will not allow employee A's VM on employee B's workstation.

Open questions the research must inform:
Q1 Writer authority across sites: one writer per tape branch; leases, epochs, fencing; failover when the writer's machine sleeps, dies or partitions; offline work; whether leases expire; branching instead of merging.
Q2 Reachability: a phone reaching a computer wherever it runs; a "home" per computer; NAT traversal and relays; home failover; multiple homes.
Q3 Access control between persons, organizations, sites and computers: delegation, attenuation, revocation, audit.
Q4 Placement and admission of work on heterogeneous, intermittently available, differently trusted machines.
Q5 Irreversible external effects (send mail, pay) under partition and offline: at-most-once/exactly-once, outbox, precommitment.
Q6 One computer controlling other computers without writing their state.
Q7 Mediating native OS capabilities (screen control, mic, files) safely.
Q8 Organization lifecycle: onboarding, offboarding, who owns what.
Q9 Mechanism vs policy: designing a kernel so that a whole configuration space is safe; verifying all configurations (model checking, simulation).
Q10 Self-modifying/self-tuning systems that stay inside proven bounds.

Task: Do not decide. Name the hidden assumptions in this framing (sites, homes, single writer per branch, leases, management desk placement, rings) and reframe the problem; import analogies from outside distributed systems (biology, law, economics, institutions, physics); object to your own reframes. Read-only; do not modify any files.
