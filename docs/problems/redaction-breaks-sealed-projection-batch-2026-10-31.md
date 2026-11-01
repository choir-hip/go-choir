# Redaction breaks a sealed projection batch and bricks the paper

Date: 2026-10-31. Status: fix landed with this record (cutover tempo,
rule 6). Mutation class of the fix: red (tape payload write path, guest
boot replay).

## What happened

At 22:30:13 UTC the paper's computer (computer-1355b83...) appended
sequence 5647041, a sealed projection batch. The live finalize failed, so
the guest's local projection stayed at 5647040 while the host's tape held
5647041. Every later append refused with "computer event projection repair
required" (thousands of run updates and telemetry seals failed, 22:30 to
22:35). The deploy at 22:35 restarted the guest; boot replay of the one tail
event died with:

    replay finalize sequence 5647041: computer event projection batch:
    invalid: decode: invalid character ')' after object key:value pair

vmctl recorded a recovery refusal (guest_startup_failed) and marked the VM
failed. The deploy of f80a7ba65 and ff0fda1b8 failed on that refresh. The
paper was down from 22:30 until the fix deployed.

## Cause

`PrivateArtifactCipher.Encrypt` redacts secrets before sealing
(`redactPrivatePayload`). It ran the text patterns over the raw JSON bytes
and spliced a `secret-handle:v1:<kind>:<hex>` in place of each match. The
`credential_assignment` pattern
(`(token|secret|password|api_key...)"?[ \t]*[:=][ \t]*"?[^\s,;"']{8,}`) matches
across a JSON key and its value: `"password":"hunter2abc"` became
`"secret-handle:..."` (a member with no value), and an unquoted value after a
token-named key took the closing brace and following characters with it. The
store's dry run decodes the batch before sealing, so it saw the unredacted,
valid bytes; nothing checked the bytes that were sealed. Reproduced by
`TestRedactionKeepsJSONPayloadsJSON` before the fix (three of six shapes
produced invalid JSON).

The envelope on the tape is digest-checked and authenticated, so it holds
exactly what the guest sealed; the unredacted batch no longer exists
anywhere. The event's content is not read for this record: the decode error
and sizes are the evidence.

## Fix

1. Redaction keeps a JSON payload JSON: it redacts inside each unescaped
   string, re-encodes the string, redacts a value under a credential-named
   key whole (what the pair pattern meant), and refuses to seal if the result
   does not parse.
2. A sealed (private) batch whose authenticated payload fails JSON syntax
   finalizes as a no-change stand-in: the head advances, no ops apply, a log
   line names the sequence. The live append that sealed it never applied it,
   so no change is exactly the state replay must reach. Replay and recovery
   continue; a live writer receives `ErrProjectionBatchUnreadable` instead of
   bricking the computer. A public payload, or a batch that parses and fails
   validation, still fails closed.

## Residuals

- The writes in 5647041 are lost (a telemetry seal or a run update; their
  callers already saw the failure as outcome unknown).
- Other computers' tapes may hold earlier such events only if they bricked
  at the time; none is known.
- The append path still trusts the dry run of the pre-seal bytes; with (1)
  the sealed bytes now parse whenever the input did.
