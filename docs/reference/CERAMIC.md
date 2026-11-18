# Ceramic (web search)

Ceramic is a web search service with its own index, added as a gateway search
provider on 2026-11-18 at the owner's word, to take load off Keenable
([problem record](../problems/search-volume-outruns-keenable-credits-2026-11-18.md)).
Everything below was read from Ceramic's documentation on 2026-11-18
(`docs.ceramic.ai`: `api-reference/search`, `api-reference/openapi.json`,
`api-reference/error-codes`, `admin/rate-limits`, `api/search/best-practices`,
`resources/faqs`). Nothing here was measured by us yet; the section "Not
known" lists what the documents do not say.

## What it is, in one paragraph

A **lexical** search engine: it matches the words in the query and does not
infer meaning or synonyms. It is cheap (the FAQ gives $0.25 per 1,000
queries) and fast (the documented example answers in 0.1 s). It indexes
**English pages only**. A query written as a question does badly; a query of
specific terms does well. That is a different instrument from Keenable, and
the same query will not suit both.

## The call

`POST https://api.ceramic.ai/search`, `Authorization: Bearer <key>`,
`Content-Type: application/json`. The body admits no unknown fields.

| Field | Type | Default | Limits | Meaning |
|---|---|---|---|---|
| `query` | string, required | | 1 to 50 words, separated by spaces | keyword query |
| `maxResults` | integer | 10 | 1 to 20 | results returned |
| `maxDescriptionLength` | integer | 3000 | 1000 to 8000 | characters of page text per result |

The answer:

```json
{"requestId": "…",
 "result": {"results": [{"title": "…", "url": "…", "description": "…"}],
            "searchMetadata": {"executionTime": 0.097},
            "totalResults": 10}}
```

There is no published date on a result and no date filter.

## Query rules

- All words must match; `OR` (capitals) matches either, and may be grouped
  with parentheses. `AND` is not needed.
- `"quoted phrase"` matches the phrase in order, and falls back to the
  separate words when no page has the phrase.
- `site:example.org` limits to a domain.
- Word order matters.
- **Not supported:** slashes (so no URLs or paths), numbers written with
  commas (`101,342`), non-Latin characters.
- Ceramic's own advice for a question from a person: rewrite it as several
  keyword queries and merge the results. Our callers are models that write
  their own queries, so the rewrite belongs in what the caller is told, not
  in a second model call.

## Errors

The body of an error is `{title, status, detail, requestId, code}`.

| Status | Meaning | Retry |
|---|---|---|
| 400 | body malformed | no |
| 401 | key invalid | no |
| 402 | credits used up, or no key | no |
| 403 | plan or account restricted | no |
| 408 | took too long | yes |
| 413, 415 | body too large; wrong content type | no |
| 422 | a parameter is unsupported or invalid (`code: "unsupported_parameter"`) | no |
| 429 | rate limit; the error page says to retry after `retry_after_seconds` | yes |
| 500, 502, 503, 504 | server or upstream | yes |

## Limits and price

- Pay As You Go: 20 requests a second. Pro: 50.
- The owner's account, 2026-11-18: 1,000 free credits, and 80,000 bought for
  $20 (payment still processing when the key was issued).

## How we use it

Environment variable `CERAMIC_API_KEY`, in the gateway's provider file on
Node B (placed 2026-11-18 with
`CHOIR_PROVIDER_ONLY_KEYS=CERAMIC_API_KEY nix/deploy-provider-creds.sh`; a
full run of that script now carries the key too). Code:
`internal/gateway/search_ceramic.go` (the provider) and
`internal/gateway/searchplane/` (routing). Written from the change that
added it; nothing below was measured on staging yet.

**Routing order.** Ceramic has the tier `first`: every search starts it,
with SearXNG and Brave (the `free` tier), and it is policed like them (one
error does not bench it; a run of five transient errors benches it for 30 s
and one probe decides). It is not free: it is prepaid, so it has its own tier
name and is not counted in `paid_calls`. Keenable is the cheapest `paid`
provider (cost 0.5, serper 1) and joins one step at a time like the others:
when the first step returns fewer than 10 distinct URLs (30 for a deep
search), or is still out after the 8 s escalation timer. Keenable has a
daily cap of 800 calls (UTC day; owner, 2026-11-18: its remaining credits
must last to 2026-12-05; resize or remove the cap on that date, in
`DefaultDailyCaps`). A search that finds Keenable at its cap skips it as
`capped` and returns Ceramic's results. Pacing: Ceramic 15 requests a second
(its plan allows 20), Keenable 8.

**When no first-step provider runs** (Ceramic declined the query, is benched,
paced or capped, or is the only one and failed), Keenable starts at once, not
after a timer.

**Query preparation** (`prepareCeramicQuery`): curly quotes become straight;
a URL scheme is dropped; slashes become spaces (so `site:bls.gov/news`
becomes `site:bls.gov news`); commas between digits are removed; whitespace
is collapsed; the query is cut to its first 50 words (a cut inside a quoted
phrase closes the quote). Quotes, `OR`, parentheses, `-word`, `site:domain`
and word order are kept. Accented Latin letters are sent as they are.

**Declined, not sent.** A query with any letter outside the Latin script, or
empty after preparation, is not sent. The search skips Ceramic with state
`declined` and reason `unsupported_query` (beside `paced` and `capped` in the
stream's `cooling` list): not counted against Ceramic's calls, no strike, not
a failure of the search.

**Request.** `POST https://api.ceramic.ai/search`, `Authorization: Bearer`,
body exactly `query`, `maxResults` (our default 40 is clamped to 20; zero or
less is 10) and `maxDescriptionLength: 1000`. The description is the
result's snippet; empty URLs are dropped. The key is never in an error text
or a log line: an error carries the status, its text, and the body's `title`
and `code` reduced to letters.

**Error classes** (by the search plane's classifier, which reads the error
text):

| Status | Class | What follows |
|---|---|---|
| 402 | `quota_limited` | benched one hour, doubling to six on each repeat; Keenable starts at once. The next search after the bench ends calls Ceramic again, with no restart. A 402 in the first hours is expected: the payment for the 80,000 credits was still processing when the key was issued |
| 429 | `rate_limited` | benched 60 s, or `retry_after_seconds` from the body (else a `Retry-After` header) when present, at most 15 minutes |
| 401, 403 | `auth_error` | a streak like the free tier: five in a row bench it for 30 s, then one probe |
| 408, 5xx | transient | the same streak |
| other 4xx | `error` | the same streak (our request is wrong; read the log's `http_status`) |

## Not known

- What a credit is: one per request is assumed, not stated. If a request
  costs more than one, the 80,000 credits last fewer days and Ceramic's 402
  arrives early; Keenable then answers alone until the owner tops up.
- Whether a failed or empty request is charged.
- The `code` string of a 402 and of a 429, and whether `retry_after_seconds`
  is a body field or a header (the OpenAPI file has neither). The code reads
  the body field first and a `Retry-After` header second; if the wait is
  carried elsewhere, a 429 is benched the default 60 s.
- What happens to a query over 50 words or with an unsupported character:
  an error, or a silent trim. The code cuts at 50 words and declines
  non-Latin letters itself; it guesses that accented Latin letters, other
  punctuation (hyphens, a leading `-`, colons) and a bare `site:` are
  accepted. If one is refused, Ceramic answers 400 or 422, classed `error`:
  the search is not lost (Keenable joins on the short answer) and five such
  errors in a row bench Ceramic for 30 s.
- How fresh the index is. Autopaper searches for news of the day; this
  matters more than anything else in this list and must be measured first.
