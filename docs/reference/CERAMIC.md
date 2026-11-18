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
`CHOIR_PROVIDER_ONLY_KEYS=CERAMIC_API_KEY nix/deploy-provider-creds.sh`).
The provider, its place in the routing order and its measured behaviour are
written here by the change that adds it.

## Not known

- What a credit is: one per request is assumed, not stated.
- Whether a failed or empty request is charged.
- The `code` string of a 402 and of a 429, and whether `retry_after_seconds`
  is a body field or a header (the OpenAPI file has neither).
- What happens to a query over 50 words or with an unsupported character:
  an error, or a silent trim.
- How fresh the index is. Autopaper searches for news of the day; this
  matters more than anything else in this list and must be measured first.
