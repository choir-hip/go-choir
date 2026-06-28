# Production Readiness Checklist

**Status:** planning  
**Date:** 2026-06-27  
**Scope:** Everything required beyond features to ship Choir to production

## Observability

Choir's observability stack is self-owned and AI-native. The consumer of
observability data is not only the human operator but also the supervision
hierarchy (trajectory supervisor, meta-conductor) and the self-learning
layer. Trace events are the canonical observation surface; everything else
is derived from them. No SaaS log export. The data is the system's own
experience memory and the prerequisite for self-improvement.

- [ ] **Trace as primary observability** — trace events are the canonical observation surface. Persist to Dolt (self-owned, versioned, queryable). Trace already exists as the causal ledger for tool calls, LLM I/O, and agent-to-agent messages; promote it from debugging tool to primary observability store. No SaaS export. The supervision hierarchy reads trace events as structured observations, not prose. **Est: 5 pts**
- [ ] **PII retraction pipeline** — SLM actor (7B or smaller, local) that redacts PII from trace events *before* persistence. Redact at ingestion, never store raw PII. SLM not regex because: regex misses novel PII patterns in multilingual content, over-redacts destroying learning context, and can't preserve semantic structure. SLM runs locally as another actor in the runtime — receives trace events as Updates, processes, sends retracted events as Updates. Fine-tunable on Choir's specific patterns over time. This is privacy-by-design as a pipeline stage, not a deletion-after-the-fact policy. **Est: 5 pts**
- [ ] **Materialized metric views** — derive cycle duration, item count per source type, dedup drop rate, processor dispatch latency, LLM call latency/cost, Qdrant search latency, Dolt merge latency, actor count, backlog depth from trace events. Store as Dolt materialized views, not Prometheus time series. Derived from actual agent behavior, not separately instrumented. Queryable by supervision hierarchy and self-learning layer. **Est: 3 pts**
- [ ] **Texture observability documents** — human-readable projections of trace state for supervision. Replace Grafana dashboards. Show: ingestion throughput, dedup effectiveness, article publication rate, error rates, cost, actor health, trajectory status. Versioned, editable, same surface as mission supervision. The human reviews observability through the same Texture documents they use to supervise missions — not a separate dashboard tool. **Est: 3 pts**
- [ ] **Supervisor-based alerting** — trajectory supervisor findings replace Prometheus alert rules. Findings are addressed actor messages that reach the system that can act, not pager notifications to a human. Alert on: cycle failures, LLM provider errors, Qdrant unavailability, Dolt merge conflicts, stale sources (no items in N cycles), actor death, backlog overflow, protocol violations. Findings are fingerprinted (trajectory_id + invariant + actor + subject + evidence_hash) for idempotency — no alert spam. **Est: 3 pts**
- [ ] **Retention and retraction policy** — Dolt time-travel retention window for trace events (e.g., 90 days raw, 1 year aggregated). Explicit retraction for deleted users across trace, Dolt, Qdrant. PII never stored raw; retracted at ingestion by the SLM pipeline. Document: what is retained, for how long, who can access, how user deletion propagates. **Est: 2 pts**
- [ ] **Structured logging compatibility** — emit slog/zerolog JSON for components that still use `log.Printf`, for transitional correlation. This is a compatibility layer, not the primary observability path. Include request IDs, cycle IDs, run IDs. The target is to route these into the trace pipeline, not to a separate log store. **Est: 1 pt**
- [ ] **OpenTelemetry export (optional, compatibility)** — OTel export for components that need to interoperate with external systems (e.g., LLM provider telemetry). This is an export sink, not the primary path. The primary path is Trace → Dolt → supervision hierarchy. **Est: 2 pts**

## Deployment & CI/CD

- [ ] **PR-based workflow** — move from trunk-based to PRs with required reviews for platform changes. Choir-in-choir per-user development is the long-term path, but platform changes need a review gate
- [ ] **Staging → production promotion** — separate production environment with explicit promotion. Currently push-to-main deploys to staging (choir.news). What is production?
- [ ] **Database migrations** — Dolt schema changes need migration strategy. Document coordinated schema change procedure for branch-per-VM
- [ ] **Secrets management** — API keys (LLM providers, Telegram, future MTProto) need proper secrets management, not env vars in Nix configs
- [ ] **Backup & recovery** — Dolt has time travel, but document: recovery procedures, Qdrant rebuild from Dolt process, backup schedule
- [ ] **Blue/green or canary deploys** — reduce deploy risk for platform changes
- [ ] **Rollback procedure** — documented and tested. NixOS generational rollback + Dolt time travel + Qdrant rebuild

## Security & Compliance

- [ ] **GDPR — right to access** — user data inventory (what PII is stored where), procedure for responding to access requests
- [ ] **GDPR — right to erasure** — deletion procedures for user data across Dolt, Qdrant, logs, LLM provider retention
- [ ] **GDPR — data portability** — export user data in machine-readable format
- [ ] **GDPR — consent management** — track consent for data processing, especially LLM processing
- [ ] **Privacy policy** — what data is collected, how it's used, third-party processors (LLM providers), retention policy
- [ ] **Terms of service** — acceptable use, liability, API terms if applicable
- [ ] **Cookie consent** — if any cookies/tracking on choir.news
- [ ] **Data retention policy** — how long are source captures, web captures, articles, LLM logs retained?
- [ ] **LLM data handling disclosure** — are user prompts/content sent to OpenAI/etc? Disclose and configure provider data retention
- [ ] **Rate limiting** — protect public endpoints from abuse
- [ ] **Authentication security audit** — passkey auth production-grade? Session management, token rotation, brute force protection
- [ ] **TLS everywhere** — verify all internal service communication is encrypted (sourcecycled → runtime → Qdrant → Dolt)
- [ ] **Dependency audit** — scan Go modules and Nix inputs for known vulnerabilities

## Reliability

- [ ] **Health checks** — health endpoints for sourcecycled, runtime, Qdrant, Dolt, Ollama. CI deploys should verify health before routing traffic
- [ ] **Graceful shutdown** — ensure in-flight cycles complete or safely abort on SIGTERM. Audit sourcecycled and runtime
- [ ] **Circuit breakers** — LLM provider failures circuit-break (not retry endlessly). Qdrant failures degrade gracefully (skip dedup, don't block ingestion)
- [ ] **Dead letter queue** — failed processor/reconciler runs need retry/DLQ mechanism, not just logging
- [ ] **Idempotency** — ensure duplicate cycle runs or dispatch retries don't produce duplicate articles
- [ ] **Qdrant failure mode** — if Qdrant is down, ingestion should continue without semantic dedup (content-hash dedup still works)
- [ ] **Ollama failure mode** — if Ollama is down, either skip semantic dedup or queue items for later embedding
- [ ] **Dolt failure mode** — if Dolt is down, what happens? Document degradation behavior
- [ ] **LLM provider failure mode** — if OpenAI/etc is down, processor should retry with backoff, not hang indefinitely

## Operational

- [ ] **Runbooks** — documented procedures for:
  - [ ] Qdrant rebuild from Dolt
  - [ ] Dolt merge conflict resolution
  - [ ] Source addition/removal
  - [ ] LLM provider switch
  - [ ] Rollback (NixOS + Dolt + Qdrant)
  - [ ] Staging deploy verification
  - [ ] Ollama model update
- [ ] **On-call procedure** — who responds when staging/production breaks, alert routing, escalation
- [ ] **Cost monitoring** — per-cycle, per-article LLM API cost tracking. Alert on cost anomalies
- [ ] **Capacity planning** — how many sources, articles/day, Dolt storage, Qdrant vectors before scaling needed
- [ ] **Source health monitoring** — detect dead sources (RSS feeds that 404, Telegram channels that disappear)

## Code Quality

- [ ] **Error handling audit** — replace `log.Printf` + continue patterns with proper error propagation where appropriate
- [ ] **E2E integration tests** — tests that exercise full pipeline: sourcecycled → dedup → processor → Texture → publish
- [ ] **Load testing** — k6 scripts exist (`load/k6/`), but need production-representative load tests
- [ ] **Dependency pinning audit** — go.mod is pinned, Nix flake inputs audited for staleness
- [ ] **Dead code removal** — audit for unused code paths after Mission 3 completes

## User Experience

- [ ] **Error pages** — user-facing error pages for 500s, 404s, maintenance mode
- [ ] **Article quality feedback** — way for users to report bad articles (wrong sources, hallucinations, duplicates)
- [ ] **Source transparency** — users can see which sources an article was synthesized from
- [ ] **Status page** — public status page for choir.news uptime and incidents

## Prioritization

| Priority | Item | Est | Why |
|----------|------|-----|-----|
| P0 | Trace as primary observability | 5 pts | Can't operate or supervise what you can't see; foundation for all supervision layers |
| P0 | Privacy policy + ToS | — | Legal requirement before any real users |
| P0 | LLM cost tracking | — | Primary variable cost, can spiral quickly |
| P0 | Snapshot save race window | 3 pts | Correctness bug — stale memory overwrite under concurrent activation |
| P0 | Old runtime removal | 8 pts | Any caller on the old path is using the borked concurrency model |
| P0 | Race detector in CI | 2 pts | Primary defense against the bug class that made the port borked |
| P1 | PII retraction pipeline | 5 pts | Privacy-by-design; PII in trace events is a GDPR liability and product risk |
| P1 | Health checks + circuit breakers | — | Reliability baseline |
| P1 | PR-based workflow | — | Quality gate before production |
| P1 | Rate limiting | — | Protect against abuse |
| P1 | Data retention policy | — | GDPR compliance + storage cost control |
| P1 | Bounded inbox with backpressure | 5 pts | Unbounded memory growth under burst; production OOM risk |
| P1 | Backpressure on Send | 3 pts | Unbounded durable log growth under agent loops |
| P1 | Actor failure observability | 3 pts | Silent actor deaths are undebuggable in production |
| P1 | Graceful shutdown drain | 3 pts | In-flight handler cancellation = partial side effects |
| P2 | Materialized metric views | 3 pts | Derived metrics from trace; queryable by supervision hierarchy |
| P2 | Texture observability documents | 3 pts | Replace Grafana with versioned, editable supervision surface |
| P2 | Supervisor-based alerting | 3 pts | Actor messages that reach the system that can act, not pager spam |
| P2 | Retention and retraction policy | 2 pts | Dolt time-travel retention + user deletion propagation |
| P2 | Secrets management | — | Operational security improvement |
| P2 | Runbooks | — | Operational readiness |
| P2 | Load testing | — | Validate capacity before production |
| P2 | TLA+ spec sync | 3 pts | Protocol verification drift risk |
| P2 | Actor count metrics | 2 pts | Actor runtime vital signs |
| P2 | Handler side-effect ordering doc | 1 pt | Document non-obvious invariant |
| P2 | Structured logging compatibility | 1 pt | Transitional; route into trace pipeline, not separate store |
| P3 | OpenTelemetry export (optional) | 2 pts | Compatibility for external system interop |
| P3 | Blue/green deploys | — | Reduce deploy risk |
| P3 | Status page | — | User trust |
| P3 | Article quality feedback | — | Product quality loop |
