# Exa search always errors (2026-10-15)

Owner ruling 2026-10-15: "fix exa". Mutation class of the fix: **orange**
(provider adapter in `internal/gateway/search.go`). Rollback: revert the fix
commit (Exa goes back to `exa=error`; the other providers are unaffected).
Problem first; the fix commit follows and references this doc.

## Evidence

- Staging gateway journal: `exa=error` on every search since at least
  2026-10-14 14:56Z (2,335+ lines in the skipped-provider summary). Exa has a
  daily cap of 150, so a healthy Exa would be answering some searches.
- The recorded failure reason in the gateway search-health DB:
  `parse response: unexpected end of JSON input`.
- Journal check at 2026-10-15 ~10:00Z (`journalctl -u go-choir-gateway
  --since -2h | grep -i exa`) showed only request and routing lines, no
  per-failure line with provider/status/step for exa, so the health DB reason
  is the evidence of the failing step.

## Root cause (confirmed from code)

`ExaProvider.Search` in `internal/gateway/search.go`:

1. The request asks for `numResults` = the caller's `max_results` (the
   Autopaper computer sends 40) and `"contents": {"text": true}`, which is
   **full page text for every result**, unbounded per result.
2. The response is read through `io.LimitReader(resp.Body, 256*1024)`. Forty
   full-text pages are far larger than 256 KiB, so the body is cut mid-JSON.
3. `parseExaResults` then fails with `unexpected end of JSON input`, reported
   as the generic `parse response:` error. Truncation is indistinguishable
   from a malformed provider answer, and every call fails the same way.

The same silent-truncation shape exists in the other adapters' 256 KiB reads;
they request small snippet payloads, so only Exa is affected today (hypothesis,
not verified for each).

## Fix

- Request bounded text: `contents.text.maxCharacters` of 4,000 per result (a
  few thousand characters of useful content per result, per the owner's
  "do not economize on search content" ruling), keep 40 results.
- Read up to 4 MiB (comfortably above 40 x 4,000 chars plus JSON escaping and
  metadata) and detect truncation by reading one byte past the limit; report
  it as `response truncated: ...`, a distinct failure stage, never as a parse
  error.
