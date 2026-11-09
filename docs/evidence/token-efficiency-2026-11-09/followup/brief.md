# Owner rulings, 2026-11-09 afternoon (binding on every follow-up investigation)

Context: docs/evidence/token-efficiency-audit-2026-11-09.md and its slice reports in
docs/evidence/token-efficiency-2026-11-09/ (read brief.md there first: it has the read-only
access helpers, the computers, and the safety rules). Repo checkout to read: /Users/wiz/Dev/go-choir/.claude/worktrees/followup-token (main = 5dc98bcf5); all doc paths above are relative to it.

Rulings that overrule parts of that audit:

1. NEVER batch messages between Texture and other desks. Research's interim results reach Texture
   as they happen; that is the product (a live supervision surface). Each wake must be cheap because
   it APPENDS to Texture's existing prompt context and hits the warm cache.
2. Texture is NOT made stateless and does not forget its history. (Audit lever 5 is withdrawn.)
   The job is: when another desk wakes Texture, the request is the old prefix byte-for-byte plus
   an appended message, so the cache is read.
3. NEVER trim tool results or history. Trimming breaks caching. Assume caching is on for every
   provider (crouton is free and does not matter). Anything that rewrites, reorders, truncates or
   re-renders earlier bytes of a transcript is a defect to find.
4. Research: search, read, and sending the interim result to Texture should all happen in ONE cell.
5. System prompts: do not explain every choir.* verb up front. Explain how the REPL works and let
   the model discover verbs from the REPL as needed (progressive disclosure). It must be
   cache-aware (discovery output is appended, never changes the prefix).
6. All system prompts get reviewed for simplification with behavior held constant; behavioral bugs
   found on the way are welcome. Management's ~12k-token prompt is the first suspect.
7. Claude delegate: delegate MORE, not less. It explores the repo itself BEFORE/instead of being
   handed a pasted brief. Open question to investigate: give Claude Code an MCP server through
   which it sends and receives desk messages, so it can be treated as a desk.
8. Jev parallelism is tabled (separate document in progress). Do not work on it.
9. Budget: paper on Haiku 5.5 (Bedrock, explicit cache breakpoints) or River DeepSeek, <= $20/day.

Rules for you: read-only. No commits, no pushes, no posts to any computer, no paid model calls.
Never print secrets or email addresses. Do not read /var/lib/go-choir/secrets on Node B.
Use the helper scripts named in brief.md for live reads; do not inspect the helpers' contents.
Label every claim measured (with the command) or inferred. Cite file:line.
Write your report to the path your brief names, then reply with a summary under 300 words.

Helpers: SCR=/private/tmp/claude-501/-Users-wiz-Dev-go-choir/f298736e-cabb-51ff-9cea-34588a4989cb/scratchpad  ($SCR/paper.sh <path>, $SCR/de.sh <path>, $SCR/bc.sh <path>; authenticated GETs).
Reports go in /private/tmp/claude-501/-Users-wiz-Dev-go-choir/f298736e-cabb-51ff-9cea-34588a4989cb/scratchpad/followup/.
