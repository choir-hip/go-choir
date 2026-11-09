# Token efficiency audit — shared brief (owner, 2026-11-09)

Owner: "we should go really deep in terms of investigating ... how we can really improve the token efficiency across the board. We need to really scrutinize everything, every desk, pre-commitment records, all the communication, everything. Looking at agent traces, retrospectively." Earlier: "is there a way that we could get the same or more paper output using 50% or 10% or 2% of the tokens? ... ways we're using a language model as a classifier when we could be using Jev ... we should be using it in a much more parallel way."

You are one of eight Sonnet investigators, each on one slice. Read first:
- /Users/wiz/Dev/go-choir/CLAUDE.md (operating contract; Desk Ontology section).
- docs/evidence/paper-token-budget-2026-11-09.md on origin/main (today's measurement: paper ~50M input tokens/h; ~51% a wedged management loop now being fixed; research 30%, texture 13%, intake 7%; output ~1% of input; crouton reported 0 cached tokens; Synthetic/opencode-go cached 85-87%).
Read code at /Users/wiz/Dev/go-choir (read-only; `git -C /Users/wiz/Dev/go-choir fetch -q origin` then read files via `git show origin/main:<path>` or the working tree — do not edit or commit anything in the repo).

Hard rules:
- READ-ONLY everywhere. Never POST to or act on any computer. The paper is computer-1355b83bced1613eb048ee28d66688a8; never act on it or on the owner's computer computer-03335285269bdba4f94377e56879f9e6. d@e is computer-59c902c5d46caecc0a180a8557164e72; b@c is computer-6c50d6e566143ae32cdb660faa60e24d.
- Authenticated GET helpers (they carry their own auth; never print or read their contents): SCR=$SCR; $SCR/paper.sh <path>, $SCR/de.sh <path>, $SCR/bc.sh <path>. Useful paths include /api/costs, /api/observability, /api/runtime/observability, /api/runtime/obligations, /api/management/tasks, /api/trajectories, and run/event routes you find in the runtime HTTP handlers. Poll d@e no more than every 3 minutes for repeated calls.
- Node B: `ssh node-b "sudo journalctl -u go-choir-gateway --since '-6h' --no-pager"` (metadata and logs only). Never read /var/lib/go-choir/secrets/* or any key/token value. Gateway lines: `provider: <p> chat completion model=... tokens_in=N tokens_out=N cached_in=N`, `gateway: inference succeeded for autoputer vm-... (provider=X tokens=IN+OUT cached=C ...)`.
- Never print secrets or the owner's email addresses.
- A claim about why a model did something needs a trace; otherwise label it a hypothesis. Mark every number measured or inferred and give the command.
- Do not spend money: no paid model calls. Free probes only if essential and tiny.
- Jev = TypeSafe's Decisions classifier through OpenRouter (/provider/v1/judgments, internal/gateway/jev.go), $0.042/M input, output free; it can be called in parallel batches.

Deliver ONE file: $SCR/eff/<your-slice>.md with:
1. What this slice spends (tokens/h or per unit, per call breakdown: system prompt, tool schemas, history, frames/packets, tool results, output), measured where possible.
2. Waste found, each with trace evidence (repeats, resends, oversized payloads, work that produces nothing, retries, duplication across desks).
3. Ranked levers: estimated saving as a fraction of the slice and of the paper's total, quality risk, implementation size, and evidence. Include structural redesigns, not only tuning: what would a 10x or 50x leaner version of this slice look like?
4. Jev/classifier opportunities in this slice (what LLM calls are really a decision/label/score, how many per hour, and a parallel-batch shape).
5. Open questions you could not settle and the probe that would.
Reply with a 10-line summary of your top levers and their savings.
