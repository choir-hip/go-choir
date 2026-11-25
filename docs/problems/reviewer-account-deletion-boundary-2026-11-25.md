# A missing reviewer ID is not permission to delete

Round 8 found that auth used the same empty string for an intentionally
unconfigured reviewer and for a failed identity-file read. That string also
drove the deletion refusal. A stale or restored deletion record was outside
that refusal entirely. The cancel token had a similar gap: an earlier offer
could still change durable state after the account was turned off, and the
phone kept an offer when starting a different reviewer sign-in.

The boundary is the account identity, not the reviewer's current credential
or the client screen. The request and runner must share that decision; a
failed configured read must pause deletion without pausing unrelated account
access. A valid cancellation is still the recovery path for a pending record.
The initial evidence is the frozen round-8 review and the synthetic boundary
tests; this has not been exercised on Node B.
