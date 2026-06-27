# Production Readiness Checklist

**Status:** planning  
**Date:** 2026-06-27  
**Scope:** Everything required beyond features to ship Choir to production

## Observability

- [ ] **Structured logging** — replace `log.Printf` with JSON structured logs (slog or zerolog). Include request IDs, cycle IDs, run IDs for correlation across services
- [ ] **Metrics exporter** — Prometheus exporter for: cycle duration, item count per source type, dedup drop rate, processor dispatch latency, LLM call latency/cost, Qdrant search latency, Dolt merge latency
- [ ] **Distributed tracing** — OpenTelemetry traces across sourcecycled → runtime → LLM provider → Qdrant → Dolt
- [ ] **Alerting rules** — alert on: cycle failures, LLM provider errors, Qdrant unavailability, Dolt merge conflicts, stale sources (no items in N cycles)
- [ ] **Dashboard** — Grafana or similar: ingestion throughput, dedup effectiveness, article publication rate, error rates, cost
- [ ] **Log retention policy** — how long are logs kept, where stored, who can access

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

| Priority | Item | Why |
|----------|------|-----|
| P0 | Structured logging + metrics | Can't operate what you can't see |
| P0 | Privacy policy + ToS | Legal requirement before any real users |
| P0 | LLM cost tracking | Primary variable cost, can spiral quickly |
| P1 | Health checks + circuit breakers | Reliability baseline |
| P1 | PR-based workflow | Quality gate before production |
| P1 | Rate limiting | Protect against abuse |
| P1 | Data retention policy | GDPR compliance + storage cost control |
| P2 | Distributed tracing | Debugging E2E latency issues |
| P2 | Secrets management | Operational security improvement |
| P2 | Runbooks | Operational readiness |
| P2 | Load testing | Validate capacity before production |
| P3 | Blue/green deploys | Reduce deploy risk |
| P3 | Status page | User trust |
| P3 | Article quality feedback | Product quality loop |
