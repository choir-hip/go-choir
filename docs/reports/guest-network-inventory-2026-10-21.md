# Guest network inventory (SDC NET-01), 2026-10-21

Status: inventory, read from the code. Mutation class of this document:
green (docs only). It is the table that [SDC NET-01](../security-density-architecture-checklist-2026-10-21.md)
("host fetch service; no general guest internet") and NET-04 ("remove the
guest NIC") need before either can close an egress path without breaking a
capability. It also answers SDC NET-01's complaint that the desk-cells
inventory "misses YouTube, iTunes, podcast feeds and Obscura"
([desk-cells-run-as-guest-root-and-egress-is-open-2026-10-14.md](../problems/desk-boundary-residuals-2026-10-21.md),
table at line 87).

**Basis and method.** The tree of branch `lane/b8-network` after the Obscura
deletion (commit e57b7e3cb). Every row comes from reading code: no packet was
captured and no network was called. A research agent swept the tree for
every outbound call and every environment variable that carries a URL; I
re-checked the claims that change a decision (the list under "Checked
by hand", below) and corrected the line numbers that moved when
`internal/browsercontrol` was deleted. A line number is a pointer to read
the code, not a promise: where a row says "not verified", it is not. The
inventory is of what the code can dial, not of what happened on a given day;
the paper's real traffic should be measured against it (see "What would
confirm this").

**Capabilities** are the seven in the SDC section "Networking must enable,
not starve": C1 research reads an arbitrary public URL; C2 web search;
C3 source ingestion (RSS, Telegram, GDELT, keyed APIs, podcasts, YouTube,
content import); C4 Web Lens live inspection; C5 engineering (the L2
ecosystem proxy, the Go module proxy, the delegate's model API); C6 Mail
in and out; C7 app agents' declared ingress and egress.

## Facts that change how to read the checklist

1. **There is no Telegram MTProto code.** A search of `internal`, `cmd` and
   `go.mod` finds none. Telegram is read as HTTP: `https://t.me/s/<channel>`
   through the guarded client (`internal/sources/telegram.go:28-102`). The
   owner then ruled "no MTProto, no cookie routes, no Telegram account" and
   kept the 73 Telegram rows unscheduled
   ([owner notes](owner-notes-2026-10-14.md) lines 758 and 797-803).
   SDC NET-01's "Telegram's MTProto connection given its own host route" and
   capability 3's "(MTProto, not HTTP)" describe a plan that is not in the
   code. If MTProto is reinstated it is a new row here.
2. **The guest does not call maild.** `RUNTIME_MAILD_URL` is read into the
   config (`internal/provideriface/config.go:111`) and passed on
   (`internal/autoputer/run.go:827`), but nothing in the guest dials it. Mail
   reaches the guest as a push from the host (`internal/autoputer/maildir.go:153-212`).
   Host port 8087 is open to the guest for no guest caller, as SDC SEC-03
   item 5 suspected. C6 needs no guest egress.
3. **`objectgraph.HTTPStore` is never instantiated outside tests.**
   (`internal/objectgraph/http_store.go:31`.) The guest reaches the platform
   store (corpusd) directly at `CHOIR_PLATFORM_URL`, which is
   `http://<tap-host>:8086` (`internal/vmmanager/manager.go:1809`;
   `internal/autoputer/run.go:278, 847-848`), not through the proxy,
   whatever the comment at `internal/proxy/handlers.go:1862-1864` says.
4. **Every research and source fetch runs in the daemon, as guest root, not
   in a desk worker.** A desk cell sends a frame; `deskEgress`
   (`internal/agentcore/tools_desk.go:1069-1089`) calls
   `researchtools.HostEgress` (`internal/researchtools/host_egress.go:22-81`).
   The cell interpreter's allowlist bans `net` and `net/http`
   (`internal/yaegikernel/allowlist.go:39-40`). So moving a fetch to a host
   verb changes the daemon's caller, not the cells'.

## A. Caller-chosen public URLs (the surface NET-01 exists for)

All of these are guarded by `sourcefetch.Client` unless the row says
otherwise: resolve once, refuse any non-public answer, dial the checked
address, at most five redirects each re-validated, `Proxy=nil`
(`internal/sourcefetch/policy.go:21-80`). The guard has no port limit and
no body limit of its own; each caller sets its limit.

| # | Destination | Caller (file:line) | Why | Limits | Cap. | Closing route |
|---|---|---|---|---|---|---|
| A1 | any http(s) URL, any port | `internal/researchtools/read.go:105-121` (client at `internal/agentcore/tool_profiles.go:373,380`) | research `choir.Read` and `fetch_url` | 30 s, 16 MiB (`read.go:23,128`); a PDF then goes to local `pdftotext` (`internal/content/extract.go:108,237`), unsandboxed | C1 | host fetch verb |
| A2 | any http(s) URL | `internal/content/content.go:279-396` (validate 284, client 298, fetch 483-568); routes `internal/apihandler/routes.go:37-38`; also `internal/textureowner/texture_media_sources.go:78`, which fetches URLs found in Texture document text, so model-authored text can trigger a fetch | content import, the Source Viewer snapshot (the Web Lens launcher's second button), Texture media | 30 s; 2 MiB (`content.go:44`), 25 MiB for documents (`:45,505-516`); `pandoc` and `pdftotext` run locally (`extract.go:106-162,215-237`) | C3 | host fetch verb |
| A3 | a podcast feed URL | `internal/agentcore/podcast.go:191-243` (subscribe), `:148-189` (refresh, due after 30 min, `:245-252`), then `ImportURL` at `:218`; routes `routes.go:29-30` | podcast library | as A2. Audio files are not fetched server-side | C3 | host fetch verb |
| A4 | article links named by feed items | `internal/sources/article_import.go:28-73`; from `rss.go:138-140` or lazily `internal/researchtools/poll_source.go:233-235` | reader snapshots of polled items | 30 s, 2 MiB | C3 | host fetch verb |
| A5 | zip URLs read out of GDELT's `lastupdate.txt` | `internal/sources/gdelt.go:380-407` | GDELT bodies | 60 s; **no size cap** at `gdelt.go:92` and `:395` (a bare `io.ReadAll`) | C3 | host fetch verb, and add a cap |
| A6 | YouTube caption `baseUrl` from the player JSON | `content.go:821-832, 928-939` | transcripts | 2 MiB (`:838,945`) | C3 | host fetch verb |
| A7 | SearXNG result URLs | `content.go:306-323, 1395-1402, 1469-1529` | discovery fallback | gated by `SEARXNG_URL`, not set in the guest image (`nix/autoputer-vm.nix`); set only for a host service (`nix/node-b.nix:1933`) | C3 | none needed |
| A8 | **Obscura**: any URL, its redirects and subresources | deleted in e57b7e3cb (was `internal/browsercontrol/handler.go`, scheme-only check, exec of the binary, plain `http.Client` follow fetches) | Web Lens "Snapshot" | none | C4 | deleted; the image part is L2-06 step 5 |
| A9 | anything: the Management Console's `zot` terminal and its `!` shell lines | `internal/autoputer/terminal.go:231-258, 289-310`; `/api/super-console/ws` at `:438`; `internal/zot/session.go:97`; binary path `nix/autoputer-vm.nix:921` | operator console | **none and unconfined**: the environment is stripped of `CHOIR_`, token, secret and key names (`terminal.go:249-250, 266-281`), but `PATH` holds curl, git, nodejs, uv, python3, libreoffice and pandoc (`nix/autoputer-vm.nix:830-857`) | none of the seven | **not covered by NET-01 or NET-04: needs an owner decision** (a capsule, or gateway-only) |

## B. Fixed third-party hosts

| # | Destination | Caller (file:line) | Why | Guard and credential | Cap. | Closing route |
|---|---|---|---|---|---|---|
| B1 | `itunes.apple.com:443` | `internal/agentcore/podcast.go:337-340` (URL), `:355-373` (GET); the client is a plain `http.Client{Timeout: 12s}` at `:91`; route `routes.go:31` | podcast search | **no address guard**, follows redirects unchecked; 1 MiB cap (`:371`). The override `CHOIR_PODCAST_SEARCH_URL` (`:331,338`) is unset in the image | C3 | host fetch verb (or a host search proxy) |
| B2 | `www.youtube.com:443` | InnerTube POST `content.go:857-862, 754-855`; watch-page GET `:872-962`; only after `IsYouTubeURL` (`:1789-1796`) | transcripts | `sourcefetch.Client`; the destination follows the video ID (`:575`) | C3 | host fetch verb |
| B3 | `gettranscript.io`, `transcriptapi.com`, `www.youtube-transcript.io` | `content.go:986-1014, 1016-1060, 1111-1125` | optional transcript APIs | gated by `CHOIR_YOUTUBE_TRANSCRIPT_*` (`:988-991`), unset in the image; would put an API key in guest environment | C3 | host fetch verb, or the gateway |
| B4 | `data.gdeltproject.org:80`, plain HTTP | `internal/sources/registry.json:8`; `gdelt.go:72-101` | GDELT | `sourcefetch.Client` | C3 | host fetch verb |
| B5 | `t.me:443` | 73 rows in `registry.json`; `internal/sources/telegram.go:28-102` (2 MiB cap `:55`) | Telegram previews (unscheduled by the owner's ruling) | `sourcefetch.Client` | C3 | host fetch verb |
| B6 | 136 distinct feed hosts across the 255 registry rows (181 rss, 73 telegram, 1 gdelt), including plain-HTTP `rss.slashdot.org` (`registry.json:3135`) and `en.kremlin.ru` (`:6132`) | embedded registry `internal/sources/registry.go:13-21`; `rss.go:43-147`; schedule `internal/agentcore/source_schedule.go:193-196`, default 15 min (`:34`) | polling | `sourcefetch.Client`; 2 MiB default body (`internal/sources/types.go:83`). The `raw.githubusercontent.com` row (`registry.json:5640`) runs only as a fixture (`poll_source.go:94`) | C3 | host fetch verb |
| B7 | `api.anthropic.com:443`, claude-mode capsules only | allowlist `internal/capsule/claude_delegate.go:116-128` (512 MiB up, 1 GiB down); policy pick `internal/capsule/executor.go:150-158`; env `internal/claudedelegate/spec.go:231-232` | the Claude delegate | CONNECT to 443 only (`internal/capsule/egress.go:342-349`). The credential is the owner's `CLAUDE_CODE_OAUTH_TOKEN` (`spec.go:36`) read from the credential disk (`claude_delegate.go:60-110`). One test computer | C5 | SDC SEC-08 Option K (key held by the gateway) |
| B8 | the L2 allowlist, 20 suffixes: `pypi.org`, `pythonhosted.org`, `download.pytorch.org`, `registry.npmjs.org`, `registry.yarnpkg.com`, `repo.yarnpkg.com`, `crates.io`, `proxy.golang.org`, `sum.golang.org`, `conda.anaconda.org`, `repo.anaconda.com`, `github.com`, `githubusercontent.com`, `gitlab.com`, `codeberg.org`, `huggingface.co`, `hf.co`, `arxiv.org`, `zenodo.org`, `doi.org` | `internal/capsule/egress.go:104-119`; the match admits subdomains (SDC SEC-06); ports `:342-349`; the DNS-answer check `:383-391`; dials `addrs[0]` `:392-394`; budgets `:125-127`; ledger `executor.go:592-605` | engineering builds | default for implementation capsules (`internal/agentcore/engineering_assignment_runtime.go:364-370`); the capsule's network namespace holds only `lo` (`executor.go:664`) | C5 | NET-02 mirrors |

The image has no `cargo` or `rustc` (`nix/autoputer-vm.nix:830-857, 956-987`),
so `crates.io` is allowed and unused.

## C. Host services reached over the tap

The host peer is `10.(200..215).X.1`. Ports 8082, 8083, 8084, 8086, 8087 and
8088 are DNATed to `127.0.0.1:<port>` on the host
(`internal/vmmanager/manager.go:481-490, 3542-3555`;
`internal/vmmanager/hostfirewall.go:150-163`). The guest learns its address
from kernel parameters (`manager.go:1806-1813, 1840-1844`;
`nix/autoputer-vm.nix:420-434`). These stay: they are host services, and the
later move is a vsock control plane (SDC NET-03), not removal.

| Port | Service | What the guest calls | Credential |
|---|---|---|---|
| 8084 | gateway | inference (`internal/gatewayruntime/provider.go:340-351`); judgments (`gatewayruntime/jev.go:44,59-65`); web search (`internal/search/search.go:153-192`, C2); keyed source APIs (FRED, Congress, CourtListener, YouTube Data, NewsAPI.ai: `internal/search/source_api.go:38-65`, `internal/researchtools/source_api.go:19,30`, C3); zot's model base URL `/provider/openai/v1` (`terminal.go:289-296`) | Bearer from `/run/choir-bootstrap/gateway-token` (`nix/autoputer-vm.nix:890`; `internal/provideriface/config.go:263-271`); zot gets no token (`terminal.go:249-250`) |
| 8086 | corpusd (platform store) | credentials (`internal/selfdev/credentials.go:108,215,287`); events (`internal/computerevent/http_client.go:65-236`); releases (`release_client.go:23,32,44`); self-development mode, consensus, checkpoints, route certificates (`credentials.go:373,403,441,498,545,585`); host builds (`internal/hostbuild/client.go:17`); files (`file_sync.go:199,304,379,608,634`); key escrow and delivery (`key_escrow.go:102,113,130`, `key_delivery.go:48-54`); projection base (`projection_base.go:216`, `internal/projectionbase/source_http.go:66,211,230`); **captures** (`internal/researchtools/capture_sink.go:108,147,165`, `capture_search.go:76,96`) and **story verdicts** (`internal/agentcore/story_publish.go:446`), both sent with no Authorization; published search (`published_search.go:49`) | Bearer computer capability, except captures and verdicts |
| 8082 | proxy | story publish (`internal/agentcore/story_publish.go:436` to `internal/types/story.go:154`), a plain POST (`story_publish.go:406-411`) | none |
| 8083 | vmctl | route resolve and apply (`internal/vmctl/route_authority.go:425-437`, `route_client.go:40-50`); effective-release report (`reported_release.go:44,122`); memory lease `/internal/vmctl/memory/guest` (`memory_hotplug_routes.go:192`, `internal/autoputer/job_memory.go:85-105`) | a marker header only, sent to loopback hosts only (`internal/hostauth/hostauth.go:304-316`); the memory lease sends no marker (`memory_lease_client.go:72`). Authority is the tap source address |
| 8087 | maild | **nothing in the guest** (Fact 2) | n/a |
| 8088 | goproxy | the name `goproxy.choir.internal` is a route, not DNS (`internal/capsule/egress.go:52-86, 350-361, 371-375`); address from `capsule_executor_linux.go:70-77`; env `egress.go:44` | none; the capsule header is attribution only (`egress.go:57,421-423`) |

Port 8085 is the guest's own listener (`SERVER_HOST=0.0.0.0`,
`nix/autoputer-vm.nix:860,862`), used host to guest.

Also reachable from the guest, dialed by no guest code: the host's sshd (:22),
80 and 443 (`nix/node-b.nix:524-530`; SDC SEC-03 item 2), and other loopback
services on the host. These are SEC-03's floor to drop, not rows to carry
forward.

## D. Non-HTTP and ambient paths

| # | Path | Evidence | Route |
|---|---|---|---|
| D1 | **DNS** (UDP/TCP 53). The module sets none: no `DNS=` (`nix/autoputer-vm.nix:357-366`) and no resolver field in the `ip=` parameter (`manager.go:1813,1835`). Every Go resolver and every tool uses it (`policy.go:23`, `egress.go:196-198`) | the resolver identity is the NixOS default; the desk-cells inventory says "public resolvers, systemd-resolved" (line 88) and I could not verify it | SDC SEC-10: a host resolver, then none |
| D2 | **NTP** | no time-sync setting in `nix/` or `flake.nix`; the desk-cells inventory claims `*.nixos.pool.ntp.org` over UDP 123 (line 89). Not verified | **gap:** NET-01 names no time route; closing the NIC needs one |
| D3 | Telegram MTProto | no code (Fact 1) | none unless reinstated |
| D4 | git | the daemon's git use is local (`internal/capsule/source_snapshot.go:216-275`, `source_patch.go:108`, `carried_patch.go:277`); the only `clone` is host-side (`vmmanager/manager.go:2970`). In capsules git works over L2 HTTPS CONNECT; SSH cannot leave | NET-02 |
| D5 | Unit hardening: the runtime unit has no `User=`, no `RestrictAddressFamilies` and no IP deny (`nix/autoputer-vm.nix:928-941`); the updater unit allows `AF_INET` (`:800`); the signer, migration, diagnostics and probe units are `AF_UNIX` only (`:574,595,635,672,699`) | keep the closed ones | SDC SEC-15 |
| D6 | IPv6 is off in the guest (`nix/autoputer-vm.nix:365-366`) | | n/a |

## E. Looks like egress and is not (do not re-add)

- **Updater loopback:** `127.0.0.1:8085` restart-prepare and health URLs
  (`nix/autoputer-vm.nix:781`; loopback enforced at `internal/updater/runtime.go:45-60`).
- **Qdrant and Ollama defaults:** `127.0.0.1:6333` and `localhost:11434`
  (`internal/provideriface/config.go:50,60,132-133`). Nothing in the guest
  listens on either; `QDRANT_URL` and `OLLAMA_URL` are unset.
- **Unix sockets and the fd pair:** the signer
  (`internal/receiptsigner/receiptsigner.go:211-212`), the updater
  (`internal/updater/client.go:191-195`), the capsule broker
  (`internal/capsule/broker_client.go:76-91`), the egress forwarder
  (`cmd/capsule-broker/egress.go:48,67`), the delegate
  (`internal/claudedelegate/server.go:573`) and the desk-worker socketpair
  (`internal/yaegikernel/desk_session_worker.go:127-146`).
- **The in-capsule proxy:** `127.0.0.1:3128` (`internal/capsule/egress.go:28`).
- **Visual capture:** a loopback server (`internal/visualcapture/capture.go:115-126`) and a
  Chromium started with `--no-proxy-server` and
  `--host-resolver-rules=MAP * ~NOTFOUND , EXCLUDE 127.0.0.1` (`:207-208`).
- **The diagnostic oracle:** `/internal/diag/tcp-dial`
  (`internal/autoputer/boot_timeline.go:585-633`) is host-initiated, dials an
  IP literal, and is gated to host peers (`:590`).
- **Host to guest:** the proxy calls the guest's `/api/content/import-url`
  (`internal/proxy/platform_publish.go:721`), and inbound mail is pushed to
  the guest.
- **Not network:** `sourceRepoRemote` is only a module argument
  (`nix/autoputer-vm.nix:24`); Dolt is embedded (`internal/store/store.go:654`)
  with no `dolt_push` or `dolt_fetch`.

## What this table says NET-01 has to build

1. **One fetch verb on the host for rows A1 to A6 and B1 to B6**, because
   they share one policy: the address guard, size and time limits, logging.
   The caller-chosen rows (A1, A2) must stay unrestricted by name: SDC
   "no allowlist for research reads". The fixed-host rows (B2 to B6) are
   candidates for the same verb with the guard unchanged.
2. **Two gaps in the guard itself, before it moves**: no port limit and no
   body limit in `sourcefetch.Client` (callers cap, GDELT does not: row A5),
   and `Proxy=nil` (so a host-side route needs a code change, not an
   environment change). `readPage` checks only the scheme and relies on the
   dial-time guard (`read.go:106-109`).
3. **One path without the guard**: B1 (iTunes). It is a fixed host today and
   a plain client; it should move to the verb or take the guarded client in
   the meantime.
4. **A decision for A9** (the Management Console's `zot` and shell): it has
   unrestricted egress and unconfined tools, and neither NET-01 nor NET-04
   covers it.
5. **A time route and a resolver route** (D1, D2) before the NIC can go.
6. **Not needed:** a maild route (Fact 2), an MTProto route (Fact 1), a Web
   Lens route (A8, deleted).

## Checked by hand

The claims above that change a decision, and what I ran to check them:

- No MTProto: `grep -rli mtproto internal cmd go.mod` finds nothing; the
  owner notes carry the ruling (lines 758, 797-803).
- Registry shape: the embedded `registry.json` parses to 255 rows (181 rss,
  73 telegram, 1 gdelt) and 136 distinct hosts.
- Research Read uses `sourcefetch.Client` (`read.go:105-125`), with the
  scheme check only before it.
- iTunes search is a plain client (`podcast.go:91`, URL `:340`).
- GDELT reads bodies with a bare `io.ReadAll` (`gdelt.go:92, 395`).
- The guest's config carries `MaildURL` and nothing reads it.

Not rechecked by hand and taken from the sweep: the rest of the file:line
pointers in section C, the NixOS network facts (D1, D2), and the claim that
no third-party binary dials home (`zot`, the delegate's Claude Code, the
image tools). Treat those as leads.

## What would confirm this

A measurement, not more reading: the host already sees every guest's
outbound connections at the tap. A day of conntrack or flow records for the
paper's computer, grouped by destination, would show which rows the real
traffic touches and which destinations exist that this table does not name
(a third-party binary, a library that dials home, NTP). That is host-side
and read-only, so it is FP0's (or the host lane's) to run, not this lane's.

## Open questions for the owner

1. A9: should the Management Console's terminal keep unrestricted egress,
   run in a capsule, or reach only the gateway?
2. D2: which time source does a guest with no NIC use (a host service, or
   a guest clock set from the host at boot)?
3. B3: are the optional transcript APIs wanted at all? If not, delete the
   code rather than carry three more hosts.
4. B5/D3: are the 73 Telegram rows and the HTTP scraper kept as dead
   registry weight, or removed with the MTProto plan?
