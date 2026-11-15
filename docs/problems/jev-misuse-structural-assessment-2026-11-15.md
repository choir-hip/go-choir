# Why we keep using Jev wrongly: a structural assessment

Recorded 2026-11-15. The owner has corrected the same mistake about five
times since 2026-11-10: "its meant for you to ask many questions in parallel
on the same state." This is the assessment the contract asks for after
repeated rounds without convergence.

## What Jev is

One request carries one state and many questions. The questions are answered
in parallel inside that request. The vendor states no maximum on questions
per request; a choice question takes up to 255 options; the token budgets
are the only bound. Efficiency is the job done with as few calls as possible.

## What we did instead

| Date | Change | The mistake |
| --- | --- | --- |
| 2026-11-10 | `50691a823` a bounded per-computer Jev bucket | too many calls treated as traffic to throttle |
| 2026-11-10 | `c53cd09cf` one call per candidate chunk, up to four captures | our own small batch size |
| 2026-11-10 | `8a7aac080` one serial call stream per runtime | too many calls treated as a queueing problem |
| 2026-11-12 | `lane/jev-one-state`: 12 captures a call, design record said 6 | still our own batch size; "measure before raising" |
| 2026-11-15 | `5ee390747` calls asked concurrently | "not serial" read as "concurrent" |

## Why it recurs

1. **The call follows the code, not the state.** Each site (intake labels,
   event layer, support judge, sub-call, desk cast, inert judge, story
   analysis) builds and sends its own request for its own unit: a capture, a
   story, a revision, a batch. Nothing in the code owns "the job's state and
   all its questions", so there is no place where one call can be made.
2. **We reach for the habits of a chat model.** One prompt, one answer, and
   when calls pile up: rate limit, queue, or fan out. Every fix so far was a
   transport fix. None changed how many calls the job needs.
3. **Caution became caps.** A 30,000-byte guard, then 4, 6 and 12 captures a
   call, each defended by "measure before raising a batch size", which our
   own reference page listed as a rule. The measurement was never run, so the
   cap stayed.
4. **Nothing counts calls per job.** Tests assert the shape of a request, not
   how many requests a job makes. A change that doubles the calls passes.
5. **The lead passed each ruling on as a prohibition** ("not serial", "no
   caps") and builders satisfied the wording. The model of the dependency was
   never the brief.

## The change of approach

- One entry point for Jev per job: sites add questions to the job's request;
  the request goes out once. A site may not send its own call.
- Calls per job is logged and asserted in tests (a job that fits the budget
  makes exactly one call). A split names the vendor limit that forced it.
- No constant of ours bounds questions per call. Accuracy at large counts is
  measured with the comparison tool, and a loss is fixed in the context.
- A repo gate fails a new direct Jev call outside the entry point.

## Remaining error

Whether every job's sites can share one state without the event layer's
dependency on earlier answers (a later chunk reads events an earlier chunk
minted) is not yet known; the builder on `lane/jev-one-state-r2` is to
answer it with calls-per-job numbers.
