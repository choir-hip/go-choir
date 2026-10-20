# Keyed source APIs: five keys exist, nothing can use them without putting a key in a guest (2026-10-20)

Status: problem recorded; fix designed here, built on branch
`keyed-source-apis` (not pushed, not deployed). Working document, not
doctrine.

Mutation class: this note is green. The fix is red (a new gateway provider
route: gateway/provider calls are a protected surface).

## 1. The problem

On sources day (owner notes 2026-10-14 §30) the owner created five API keys
and put them in the local `.env`:

| Name | Service | What it is for |
|---|---|---|
| `API_DATA_GOV_KEY` | api.data.gov (Congress.gov, GovInfo and other federal APIs) | bills, actions, the Congressional Record |
| `FRED_API_KEY` | FRED, St. Louis Fed | US macro and market series for economy stories |
| `NEWSAPI_AI_KEY` | NewsAPI.ai (Event Registry), trial | multilingual article search, non-English depth |
| `YOUTUBE_API_KEY` | YouTube Data API v3 | video and channel metadata |
| `COURTLISTENER_API_KEY` | CourtListener (Free Law Project) | dockets and opinions |

Each key answered one cheap call on 2026-10-20 at about 17:20 UTC from a
developer machine (HTTP 200 each: the current Congress; the FRED
unemployment series, last observation 2026-09-01; one YouTube video; the
Supreme Court court record; NewsAPI.ai usage, 2,000 tokens available, 0
used). The keys were never printed.

Nothing in the code can use them, and the obvious way to add them is wrong:

1. **Source polling runs in the guest.** Scheduled polls run in the paper
   computer's agent runtime (`internal/agentcore/source_schedule.go` calls
   `researchtools.PollSourceOnce`), and research's `choir.PollSource` verb
   crosses the cell's session socket to the same runtime
   (`internal/researchtools/host_egress.go`). Both are inside the guest VM.
2. **Provider keys never go to a guest.** Search keys live in the host
   gateway's EnvironmentFile (`/var/lib/go-choir/gateway-provider.env`);
   the guest calls `POST /provider/v1/search` with its gateway token and
   never holds a provider key (`nix/node-b.nix`, gateway unit comment,
   VAL-GATEWAY-004). A guest that held a source key would leak it to any
   code that runs in that computer, including an engineering capsule.
3. **The registry has no keyed source type.** `internal/sources` has `rss`,
   `telegram`, `gdelt` and `polymarket`; `auth_policy` is a recorded field
   that no fetch path reads.

So today the keys are inert, and the shortcut (an env var in the guest
image, or a key in a registry URL) would break the gateway credential rule.

## 2. The fix (designed)

One gateway route, `POST /provider/v1/source-api`, the source analogue of
`/provider/v1/search`:

- **Same caller authentication as search.** `authenticateAutoputer`, and a
  per-computer rate bucket of its own (`source_api`), so source calls cannot
  starve inference or search.
- **Named endpoints, not paths.** The request is
  `{provider, endpoint, params}`. Each provider has a fixed table of
  endpoints; each endpoint has a fixed upstream path template and a fixed
  list of typed, bounded parameters. Unknown provider, endpoint or parameter
  is refused (400). The guest never names a URL, a host, a header or a path
  segment, so the route cannot be turned into an open proxy that carries a
  key to a host the guest picked.
- **The key is added on the host.** As a header where the provider accepts
  one (api.data.gov `X-Api-Key`, YouTube `X-Goog-Api-Key`, CourtListener
  `Authorization: Token`); in the query only where the provider accepts
  nothing else (FRED `api_key`, NewsAPI.ai `apiKey`). Logged URLs and
  upstream error bodies are redacted of the key.
- **Bounded response.** GET only (NewsAPI.ai is a POST with a JSON body the
  gateway builds), a response cap (2 MiB), a timeout (30 s), and the
  upstream status passed through as a status, not as a raw error body.
- **A per-provider daily budget, process-wide.** Budgets are code defaults,
  overridable by `CHOIR_SOURCE_API_DAILY` (`fred=2000,newsapi_ai=50,...`).
  A provider over budget answers 429 with the reset time. NewsAPI.ai's
  trial is 2,000 tokens in total, so its default is 50 a day.
- **Available only when its key is loaded.** A provider without a key
  answers 503 `provider not configured`, like a search provider without
  one.

On the guest side, one research verb, `choir.SourceAPI(provider, endpoint,
params)`, added to the research desk's host-egress table beside `Read` and
`PollSource`, so its output is bounded and charged the same way. Research
has read-only world-evidence authority; the verb reads, it writes nothing.
Scheduled keyed polls (a Congress bill list or a CourtListener docket as a
`source_schedule`) are a second slice: a new source type that polls through
this route, once the verb has been used.

### Wave 1 endpoints

| Provider | Endpoint | Upstream | Cost |
|---|---|---|---|
| `fred` | `series` | `/fred/series` | 1 request |
| `fred` | `series_observations` | `/fred/series/observations` | 1 request |
| `fred` | `series_search` | `/fred/series/search` | 1 request |
| `congress` | `bills` | `/v3/bill[/{congress}]` | 1 request |
| `congress` | `bill` | `/v3/bill/{congress}/{type}/{number}` | 1 request |
| `congress` | `bill_actions` | `/v3/bill/{congress}/{type}/{number}/actions` | 1 request |
| `courtlistener` | `search` | `/api/rest/v4/search/` | 1 request |
| `courtlistener` | `docket` | `/api/rest/v4/dockets/{id}/` | 1 request |
| `youtube` | `videos` | `/youtube/v3/videos` (up to 50 ids) | 1 quota unit |
| `youtube` | `playlist_items` | `/youtube/v3/playlistItems` (a channel's uploads) | 1 quota unit |
| `newsapi_ai` | `articles` | `/api/v1/article/getArticles` | 1 token |

YouTube `search.list` (100 units a call) is deliberately absent.

Observed limits: Congress.gov through api.data.gov reported
`X-Ratelimit-Limit: 20000` (2026-10-20). YouTube's default quota is 10,000
units a day. FRED, CourtListener and NewsAPI.ai published limits were not
re-read this pass; the daily budgets are set well below what I believe
them to be.

### Failure modes, to be written as tests before the code

1. A caller without a valid gateway token gets a key-bearing call: 401.
2. An unknown provider, endpoint or parameter reaches upstream: 400, no
   upstream call.
3. A parameter carries a path segment, a host, a scheme or a query
   separator into the URL (`../`, `?`, `&`, `#`, `/`): refused or escaped,
   never a different path.
4. The key appears in a response body, a log line, an error string or a
   guest-visible field: never.
5. A provider without a key loaded is called: 503, no upstream call.
6. A provider over its daily budget is called: 429, no upstream call.
7. An upstream body larger than the cap is read whole: cut at the cap,
   reported as truncated.
8. An upstream 4xx/5xx body (which may echo the request) is passed to the
   guest: only the status and a fixed message.

## 3. Rights, not yet read for publication

These are readings for research use, not cleared for republication:

- Congress.gov, GovInfo: US federal works, public domain (17 USC 105).
- FRED: free API; some series are owned by third parties and carry their
  own copyright notes in the series metadata. A story may report a figure
  (a fact) with attribution; a chart of a third-party series needs that
  owner's terms.
- CourtListener: court records are public; Free Law Project's API terms
  not re-read this pass.
- YouTube: the API terms limit how long metadata may be stored and how it
  is shown. Research reads it; nothing republishes it yet.
- NewsAPI.ai: storage terms not read; treated as a reading source only
  (the gray-source rule, owner notes §30: read, not republished).

## 4. Rollout

1. Upsert the five keys into the gateway EnvironmentFile on Node B with
   `CHOIR_PROVIDER_ONLY_KEYS="..." ./nix/deploy-provider-creds.sh node-b`
   (stdin only; it restarts the gateway, so not during a drill). Inert
   until the route ships.
2. Land the route and the verb (Landing Loop). Deployed proof: from a test
   computer, one call per provider returns 200 with data; a call with an
   unknown endpoint returns 400; the gateway journal shows no key.
3. Second slice: keyed source schedules.

Rollback: revert the route commit; the keys in the EnvironmentFile are
inert without it.
