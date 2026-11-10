# No computer can read the paper's own published pages (2026-11-10)

Mutation class red (gateway host fetch; protected surface: guest egress, the
SSRF guard). Rollback: revert the fix commit. Found by the wave 3 staging
proof on d@e (guest at cf048d2ac). Wave 3 is
[the overnight definition](../definitions/choir-autopaper-overnight-2026-11-09.md);
the search it proved is
[published-search-recall-2026-11-10.md](published-search-recall-2026-11-10.md).

## Evidence

`SearchWireCorpus` listed the paper's own 11-day-old story (position about
2573 of the archive) with its `read_url`. Research then called `choir.Read` on
that URL, as the overlay says, and the gateway refused the hop:

    gateway: fetch refused computer=vm-70070cf7... method=GET host=choir.news
    reason="address: the host resolves to a forbidden address"   (403)

## Cause

`internal/hostfetch/host.go` `hostOwnAddress` refuses any fetch that resolves
to one of the host's interface addresses: a fetch to the host's public address
would reach listeners the guest floor drops (host fetch design, FP10 NET-01).
`choir.news` is the host. So no computer can read the paper's own published
pages, no capture of an own-host URL can exist, and `reliesOnEdges`
(`internal/platform/publication_reliance.go`) can never write a `relies_on`
edge: the materiality clock cannot start.

## What must not break (the failure list, before any code)

The guard exists because the host's public address reaches host listeners. The
fix is a closed exception, not a weaker guard.

- F1. An arbitrary own-host URL becomes fetchable (`/api/*` in general,
  `/auth`, `/internal`, an admin or gateway path): the SSRF guard is weakened.
- F2. A loopback or private port is reachable through the exception (a URL
  `https://choir.news:8086/...`, `http://127.0.0.1/...`, `localhost`).
- F3. Host confusion passes the host match: userinfo (`https://choir.news@evil/`,
  `https://evil@choir.news/`), a suffix or prefix host (`choir.news.evil.example`,
  `evilchoir.news`), a trailing-dot or upper-case variant that dials differently
  from how it matched, an IDN lookalike.
- F4. The exception forwards credentials: a cookie, an Authorization header or
  the gateway bearer reaches the public surface.
- F5. A redirect from an allowed path to a disallowed one is followed by the
  host (the host never follows; the guest re-sends each hop through the same
  check, and the disallowed hop is refused).
- F6. A non-GET method (POST, PUT, HEAD with a body) reaches the exception.
- F7. A path that matches by prefix only (`/pub/texture/../../auth/x`,
  `/pub/textureX`, an encoded slash or dot segment) passes the allowlist.
- F8. The export route is served with a query that changes what it reads (a
  `route` outside `/pub/texture/`, `format=json` or a missing `raw=1`, extra
  parameters, a repeated `route`).
- F9. The exception bypasses the size and time bounds of any fetch.
- F10. A non-own host that resolves to the host's own address is let through
  (the exception must be keyed on the configured public host name, never on
  the address a name resolves to).
- F11. The public host is a literal in the new code and diverges from the
  corpusd public base URL that mints `read_url` and that `ownRoute` matches.
- F12. The resulting capture's URL is not one `ownRoute` recognises, so the
  fix reads the page and still writes no edge.

## Design

Options weighed:

- (A) In the gateway's host fetch, when the URL's host equals the configured
  public host and the request matches a closed allowlist, serve it from the
  public surface on the host's loopback instead of dialling the public
  address.
- (B) Have the research read tool resolve a published piece's text through the
  corpus feed that `SearchWireCorpus` uses and produce a capture whose URL is
  the public page.

(A) is chosen. Reasons: the host fetch stays the one fetch authority (B adds a
second read path in the research tool that mints captures without a fetch,
which skips the fetch log line, the budgets and the capture machinery's
provenance, and which a guest on an old image would not have); the capture is
an ordinary capture of the public URL, so `ownRoute` sees the URL exactly as
`read_url` names it (F12) and a page cited by a story carries the real fetched
bytes; the exception is a few lines in one place that tests can pin on every
refusal in the list above. (B) also leaves `/pub/texture/` pages unreadable.

The exception lives in `hostfetch.Executor` (`OwnSite`): the public host comes
from the one public base URL configuration (`internal/publicsite`, shared with
corpusd, so there is no second literal), the loopback target is the proxy's
address (`GATEWAY_OWN_SITE_URL`, a loopback URL or the exception is off), only
GET and HEAD are served, the request sent is rebuilt from validated parts
(headers: a fixed user agent and `Accept`, nothing the guest sent), redirects
are not followed, and the response is relayed through the same bounded path as
any fetch.
