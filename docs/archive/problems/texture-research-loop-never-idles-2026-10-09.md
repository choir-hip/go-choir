# Texture and research loop without a stopping rule; the document never idles (2026-10-09)

Found by the Texture acceptance suite, sixth run on staging (build
19b7ef48, the research packet fix; receipt
`evidence/texture-acceptance-2026-10-09T20-23-25-707Z.json`), fresh
disposable account on VM `vm-b26e20f6…`, document `e87d2bd3…`. Problem
first; no fix in this commit. Belongs to the cluster in
`clustering-texture-obligation-closure-2026-10-09.md` (item 5 there).

## What the owner sees

T2a passes (first draft in 46 s). T2b, "rewrite as two short paragraphs
about persistent computers": Texture's revision lands about a minute
after the request (20:26:07), and then the document keeps changing and
stays "Revising…". The suite's 5-minute idle check fails. At 20:36Z the
document is still pending and holds more than the two paragraphs the
owner asked for.

## Trace (trajectory events, times UTC)

| Time | Event |
|---|---|
| 20:24:25 | first draft; `work_opened` (research) |
| 20:25:43 | research report queued |
| 20:26:07 | owner's rewrite applied; `work_opened` (research again) |
| 20:26:56 | research report queued |
| 20:28:06 | Texture revision "sharpens the owner's two-paragraph rewrite with the AWS EC2 evidence"; `work_opened` |
| 20:29:44 | research report queued |
| 20:33:24 | revision "adds a session-layer paragraph … and a closing paragraph"; `work_opened` |
| 20:35:00, 20:35:58 | Texture judges three research returns non-citable, no revision; `work_opened` again at 20:35:00 |

State at 20:36Z: six open work items (oldest 20:23:45Z), none settled;
one run running, four passivated; document pending.

## What this shows

1. **The research packet fix works.** Packets now arrive with sources and
   Texture incorporates them (20:28:06, 20:33:24).
2. **Nothing stops the loop.** Texture opens a research assignment on
   almost every turn; each research report wakes Texture; that turn
   opens another assignment. No rule bounds the rounds per owner request,
   and no assignment settles (the O24 gap), so open work accumulates.
3. **Evidence overrides the owner's directive.** The owner asked for two
   short paragraphs; later evidence turns added paragraphs. Nothing ties
   evidence-driven turns to the scope of the owner's last request.
4. **"Revising…" means "a Texture run is not terminal"**
   (`pendingAgentMutationByDoc`); with research cycling, the run never
   ends, so the editor shows Revising indefinitely.

Before 19b7ef48 every research packet was rejected, so reports carried no
sources and Texture stopped revising; the loop was there but quieter.

## Fix directions (decision needed; see the clustering doc)

- **Round budget per owner request.** An owner revise opens a bounded
  number of research rounds; when the budget is spent, Texture settles
  its work and open research assignments close. Conservative for Gate 1.
- **Research settles when it reports (A2 of the clustering doc).** Stops
  open work accumulating; does not by itself stop Texture opening more.
- **Owner scope binds evidence turns.** An evidence-driven turn may cite
  and correct but not expand beyond the owner's last directive. Prompt
  framing; a model choice, so it does not discharge O24 alone.
- Living documents woken by evidence are the Autopaper direction (Gate
  3); the bound is what makes that safe, not a reason to skip it.
