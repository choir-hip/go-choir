# G5 CLI Help Credential Leak

**Observed:** 2026-07-17 during deployed no-SSH acceptance.

**Mutation class:** **red**. Protected surfaces: owner API credential, supported Choir CLI, authenticated product APIs, and G5 acceptance evidence.

Running `go run ./cmd/choir api-key create --help` printed the current `CHOIR_API_KEY` as the default value of the `-api-key` flag. Go's standard flag help rendered the secret because the CLI passed the environment credential as the flag's default instead of keeping the default empty and resolving environment fallback after parsing.

The exposed credential carried admin scope. It was rotated immediately through the supported product API: a replacement with the same scopes was created, the local gitignored `.env` reference was updated, the exposed key `ak_257c3cae-877d-4d1a-ad21-155d2b7b56b5` was revoked, the replacement successfully completed `choir computer status`, and the temporary response containing the replacement secret was deleted. No credential value is recorded in this receipt.

This is a source-level redaction failure, not a staging or browser failure. All CLI commands sharing the common `-api-key` flag are suspect until deterministic help tests prove neither an explicit environment credential nor a command-line credential appears in usage output.

**Heresy delta:** `discovered`: secret-bearing environment fallback was treated as display-safe flag metadata. `introduced`: none after rotation; the exposed key is revoked. `repaired_when`: the CLI flag default is always empty, environment fallback occurs only after parsing, help output is secret-free for every command, and focused/full CLI tests plus deployed no-SSH acceptance pass.

**Next safe action:** fix the shared CLI flag construction at the source, add a regression test using a verifier-known fake secret, deploy, then create a temporary read-only delegated key to prove immutable inspection succeeds while owner mutation is refused; revoke that temporary key after capture.
