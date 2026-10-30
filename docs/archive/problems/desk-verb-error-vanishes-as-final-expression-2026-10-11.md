# A desk verb's error vanishes when the call ends a cell

Found 2026-10-11 in S1e (staging 78c9173e, computer demo-arxiv-2026-10-10c,
document b2352fec). Evidence class: run event traces (scratchpad
demo13/t2) plus a local kernel probe.

## What happened

Research round 3ccf4126 (the document's first round) ran three research
children. They returned about 11.7 KB of findings, including funding and
compute leads with URLs: Catalyst, Firmus, Crusoe, Axe Compute,
PaleBlueDot, Alpha Compute, SoftBank, and Manus. The parent then ended a
cell with a bare call:

    choir.ReportPacket("texture", p, "research:b2352fec-...")

The cell result was `stdout "=> "`, with `intents 0`, `error ""`, and
`is_error false`. The packet was refused, but nothing in the result said
so. The desk went on to a one-line `choir.Report` summary that carried
two URLs. Texture's first grounded revision therefore had only the two
confirmed items. None of the round's leads reached the document in any
later revision. They were never delivered.

A local probe (yaegikernel, `serveCell`) reproduces this. Any cell ending
in a call to a verb that returns `(string, error)` shows only the first
value. A refused `choir.ReportPacket` shows `=> ` and nothing else. The
interpreter's Eval returns only the first result of a multi-value call,
and `describeCellValue` renders that first result.

The probe also reproduced a second, already-visible friction. A cell that
declares a function among statements fails to compile with "expected '(',
found f". Texture turn ea5e62fa lost a cell to this with `func min2`.

## Size

Across all 721 desk cells on this computer, 2 cells ended with an empty
`=> ` result, and both were refused ReportPackets. That is rare, but each
one silently loses a whole round of evidence. Composite-literal syntax
errors are the most common visible cell failure, at about 27 cells. When
a long packet literal fails to compile, the desk retries with a shorter
packet: 5,447 characters became 2,833 in round 7629c97f, and 5,993 became
3,802 in round 348535e2. Content is lost at the report step there too.

## Cause (belief)

The kernel shows a cell's final value, but a verb's error is the second
value, and the kernel never records it anywhere. The desk can only see a
refusal if it writes `_, err := ...; err` itself.

## Test

A cell ending in a refused verb call shows the refusal in its result. A
cell that handles the error itself is unchanged. No verb refusal goes
unreported in a cell result.
