# Verifier key re-pin: review rounds (2026-11-02 to 11-03)

Branch verifier-key-repin (e15814704 problem doc, 9a465a7f2, 6de4d3133, aded39c6e), landed squashed. Red: verifier signing key pins, checkpoint authority, owner acts. Problem: docs/problems/verifier-key-regenerated-locks-out-checkpoints-2026-11-02.md.

- Round 1, tier 2. Devin: approve. DeepSeek v4.1 flash: send_back. The re-pin refused a previously revoked key, so a disk restored from an older image had no exit. The reviewer trailer was missing; that is added at landing.
- Round 2, Devin, sandboxed: stopped before a verdict, chasing whether re-activation restored the revoked key's old window. The builder's test confirmed it did, so signatures made while the key was revoked would resolve again. Fixed in aded39c6e: re-activation opens a new window, and the closed window moves to control_key_activations.
- Round 3, Devin, no sandbox: the code was verified sound. Devin ran the in-gap test and adversarial variants and could not break the window design. Its one send_back item was the missing reviewer trailer, which is added here. Security-Effect none stands; no epoch raise.
- Non-blocking residuals:
  - the closed window's own authorizing receipt is dropped host-side;
  - the proxy re-pin route checks owner:acts but no product scope;
  - /health degrades only on mismatch, not on invalid or unpinned_no_genesis;
  - the platform-update tail does not classify the key refusal;
  - a nil rt.store in /repinned;
  - already_pinned returns a receipt the guest rejects, so it logs a cosmetic tape_record_error;
  - a pre-existing macOS GNU stat test failure.
