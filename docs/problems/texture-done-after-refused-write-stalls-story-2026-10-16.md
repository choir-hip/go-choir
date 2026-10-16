# Texture's Done after a refused Write closes the turn at the seed, and the story stalls for good

Found 2026-10-16 about 02:35 UTC. It came from tracing why stories on the
V3a test computer `v3-events-0146` stayed at their seed. It is not an
event-layer path: the paper shows it too.

## Evidence (run traces)

Test computer, story bc130a54 (the Channel Tunnel collision), run
`3dfa06a8…` at 01:49:42. Texture was woken by management's reply to its
ask, and its one write cell did this:

```
fmt.Println(choir.Write(edits, "<why>", 0.98, 0.98, 0.97, 0.98))
fmt.Println(choir.Done("The current story integrates the two reports, …"))
```

- `Write` refused the call ("choir: Write takes one pSupported, not 4")
  and staged nothing. The model gave one self-score per citation.
- `Done` then staged a turn, `rlm:texture_turn:0`. Done alone "completes
  it at the current revision", which here is the seed.
- The loop ended with `terminal_tool_success`. No revision committed.
  Nothing wakes the story again, so it stays at the seed for good: 40
  minutes and counting at the time of writing.

The paper, over the last 6h: 30 stories opened, 26 published, and 4 held
only their seed for more than 30 minutes. Three of the four have a
refused Write in their runs:
- Argentina university protest: "Write takes one pSupported" and "Write
  refused";
- Pepsi/KFC: "Write refused";
- odour monitoring: "Write takes one pSupported".

Neither string occurs in prompt text; both are runtime refusals.

## Classification

This is substrate: the Texture desk surface's turn contract. A cell's
`Done` does not fate-share with the cell's `Write`. A refused Write is
reported as a printed error, but the cell's later `Done` still closes
the turn as complete. The per-citation pSupported misuse is the most
common trigger, and its refusal text does not say one number is
expected.

## Fix direction

- In a cell whose `Write` was refused, `Done` is refused too, naming the
  Write's refusal. Fixing the script and writing again is the only way
  out, so the turn never closes on a failed revision.
- The pSupported refusal says it is one probability for the whole Write
  (that every new citation's passage supports its sentence), not one per
  citation.

Ways the fix could fail (tests): Done after a refused Write commits a
turn; Done after a successful Write is refused; Wait (a legitimate turn
without a revision) is refused after a refused Write, which would be the
same stall the other way.
