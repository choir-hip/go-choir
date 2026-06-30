# Dispatch Proxy Architecture

## Problem

sourcecycled dispatches processor runs to the platform computer sandbox by reading a
static env file (`platform-wire-runtime.env`). The sandbox URL changes when the VM
reboots (deploy image refresh, resume, TAP reallocation), but the env file is only
written at vmctl startup. Result: stale URL, dispatch timeouts, wire goes dark.

Compounding issues:

- **Ambient authority**: the sandbox URL sits on disk at `0644` — any process on
  Node B can read it and submit runs.
- **No service identity**: dispatch auth is `X-Internal-Caller: true` — trivially
  forgeable by any process on the host.
- **Stale cache with no coherence**: vmctl (writer) and sourcecycled (reader) share
  state through a file with no invalidation mechanism.

## Design

### Dispatch proxy over Unix Domain Socket

```
┌──────────────┐   UDS    ┌──────────┐   TCP    ┌──────────────────┐
│ sourcecycled │─────────▶│  vmctl   │─────────▶│ platform sandbox │
│              │ POST     │          │ proxy    │ (TAP guest IP)   │
│              │ /{owner} │          │ resolve  │                  │
│              │ /internal│          │ live URL │                  │
│              │ /runtime │          │          │                  │
│              │ /runs    │          │          │                  │
└──────────────┘          └──────────┘          └──────────────────┘
     /run/go-choir/vmctl.sock (0700 dir, 0600 socket)
```

sourcecycled connects to vmctl over a Unix domain socket at
`/run/go-choir/vmctl.sock`. It POSTs the run payload to:

```
POST /internal/vmctl/sandbox-proxy/{owner-id}/internal/runtime/runs
```

vmctl:

1. Validates the caller (UDS peer credentials or `isInternalCaller`)
2. Extracts `owner-id` from the path
3. Resolves the live sandbox URL from the VM manager (not from cached ownership)
4. Strips the proxy prefix, yielding `/internal/runtime/runs`
5. Reverse-proxies the request to `{sandbox_url}/internal/runtime/runs`
6. Returns the sandbox's response to sourcecycled

### Security properties

| Property | Mechanism |
|---|---|
| No ambient authority | Sandbox URL never leaves vmctl's memory — never on disk |
| Caller authentication | UDS permissions (`0600`, owned by `root:choir`) — kernel-enforced |
| Always-live URL | vmctl reads from VM manager, not from ownership cache |
| Health gating | vmctl refuses proxy if sandbox health check fails |
| Observability | All dispatch flows through vmctl's existing log/metric surface |
| No restarts | sourcecycled never needs restarting on URL or VM changes |

### Implementation

| Component | File | Change |
|---|---|---|
| Server | `internal/server/server.go` | Added `SetUnixSocket` method, `udsListener` field, dual-serve in `Start()` |
| Registry | `internal/vmctl/ownership.go` | Added `LiveSandboxURL` method — prefers VM manager HostURL over ownership cache |
| Handler | `internal/vmctl/handlers.go` | Added `HandleSandboxProxy` — resolves owner, reverse-proxies to live sandbox |
| Route | `internal/vmctl/handlers.go` | Registered `POST /internal/vmctl/sandbox-proxy/` |
| vmctl main | `cmd/vmctl/main.go` | UDS socket via `VMCTL_SANDBOX_PROXY_SOCK` env var |
| sourcecycled | `cmd/sourcecycled/main.go` | `socketPath` field, UDS HTTP transport, `runtimeRunsEndpoint()` for proxy path |
| Nix | `nix/node-b.nix` | Removed `platform-wire-runtime.env` refs; added `VMCTL_SANDBOX_PROXY_SOCK` to both services; `RuntimeDirectory=go-choir` for UDS dir |

### What went away

- `UniversalWirePlatformRuntimeEnv` type
- `WriteUniversalWirePlatformRuntimeEnv` function
- `SyncUniversalWirePlatformRuntimeEnv` function
- `livePlatformSandboxURL`, `readPlatformRuntimeEnvBaseURL`, `platformRuntimeEnvPath`, `platformSourceServiceUnit` helpers
- `platform-wire-runtime.env` file
- `VMCTL_PLATFORM_WIRE_RUNTIME_ENV_PATH`, `VMCTL_PLATFORM_WIRE_SOURCE_SERVICE_UNIT` env vars
- `startUniversalWirePlatformComputer` → env writing + `systemctl try-restart` logic
- `normalizeLocalhostURL` (removed in prior cleanup)

### What goes away

- `platform-wire-runtime.env` — entire file
- `WriteUniversalWirePlatformRuntimeEnv` function
- `UniversalWirePlatformRuntimeEnv` type
- `SyncUniversalWirePlatformRuntimeEnv` function
- `normalizeLocalhostURL` function
- `startUniversalWirePlatformComputer` → env writing + systemctl restart logic
- sourcecycled's `ingestionRuntimeDispatcherFromEnv` → env file reading

### Failure modes

| Scenario | Behavior |
|---|---|
| vmctl down | sourcecycled dispatch fails; retries on next drain cycle |
| Platform VM unhealthy | vmctl returns 502; sourcecycled retries |
| TAP IP changed mid-proxy | Connection to old IP breaks; next proxy call resolves new IP |
| UDS socket missing | sourcecycled can't connect; retry with backoff |

### vmctl load

- 32 processor runs per drain cycle (~every 60s)
- Each proxy connection held for run duration (30-60s)
- Max 32 concurrent proxy connections — trivial for Go's `httputil.ReverseProxy` on UDS
- UDS overhead is near-zero compared to TCP (no network stack, no buffer copies)

## Deployment

1. Push to `main` → CI → staging deploy
2. On Node B: remove stale `/var/lib/go-choir/platform-wire-runtime.env`
3. sourcecycled picks up `VMCTL_SANDBOX_PROXY_SOCK` and dispatches through UDS
4. Monitor processor dispatch: `journalctl -u go-choir-sourcecycled -f`
