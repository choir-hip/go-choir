# Every configured model is treated as text-only (2026-10-15)

Status: open (documented before the fix). Mutation class of the fix:
**orange** (provider/model routing, a product API route). Rollback:
`git revert` of the fix commit; the gateway goes back to refusing every
image block, and no stored state needs migrating (image blobs on a
computer are inert files).

## Owner direction

> deepseek v4.1-flash and gpt-6-luna both accept images; we only need to
> switch it on. Most models do today, so vision should be on by default for
> every model we configure in future, with an explicit opt-out for
> text-only ones. (owner, 2026-10-15)

Motivation: the Playwright-in-capsules report
([playwright-in-capsules-memory-2026-10-15.md](../../reports/playwright-in-capsules-memory-2026-10-15.md)
§5.3) plans screenshots as visual evidence for self-development review,
scored by agents. It found: "No catalog model may receive an image." The
scorer stage (S3) cannot start until a model can see.

## What happens (findings)

The provider adapters already serialize image blocks. The capability gate
refuses them before any adapter runs.

1. **The catalog declares every model text-only.**
   `internal/modelcatalog/catalog.go`: every entry that sets modalities sets
   `Modalities: []string{"text"}` (glm-5.2 at line 68, deepseek-v4.1-flash
   at line 90, gpt-6-luna at line 181, and the other opencode-go entries).
   The three Bedrock Claude entries (lines 44-61) and glm-5.1/glm-5-turbo set
   no `Modalities` at all, which the gate reads as "no image".
2. **The gate defaults to "no".** `internal/provider/provider.go:1920-1938`,
   `modelSupportsImage`: a catalogued model supports images only if its
   `Modalities` lists `"image"`; an uncatalogued one only if its id contains
   `kimi-k2p6`. `validateMediaRequest` (line 1914) then refuses with "model
   %q is text-only in Choir model catalog". Every adapter's `Call` and
   `Stream` runs this gate first (lines 434, 553, 587, 710, 739, 877, 895,
   1111).
3. **A second copy of the gate.** `internal/modelpolicy/tools_model_verify.go:80`
   and `catalogModelSupportsModality` (lines 219-232) repeat the rule for
   the `verify_model_capability` tool. That tool is defined but registered
   nowhere, so no owner or desk can reach it.
4. **`artifact_ref` sources have no resolver.** `validateMediaRequest`
   refuses `artifact_ref` with "requires gateway artifact resolver". A
   screenshot can only travel as inline base64 (which then lands in every
   transcript, run event and log that copies the message) or as a URL.
5. **The OpenAI-compatible gateway route drops images silently.**
   `internal/gateway/openai_compat.go:285-289` turns a user message into one
   text block via `textFromOpenAIContent` (line 329), which keeps only
   `type: text` parts. An `image_url` part is discarded without an error, so
   the model answers as if it saw the image.
6. **No size, count or dimension cap.** Nothing bounds an image's bytes,
   pixels or per-request count before it is decoded or forwarded.
7. **No owner- or desk-reachable path.** The gateway admits only runtime
   peers (`isAuthorizedRuntimePeer`, `internal/gateway/handlers.go:726`).
   No product route makes a model call with an image, so neither a staging
   probe nor a desk can test vision end to end.

### Message builders per provider path (already present)

| Provider (wire) | Builder | Image wire form |
|---|---|---|
| bedrock (Anthropic Messages via invoke) | `convertMessages` / `anthropicImageSource` | `{"type":"image","source":{"type":"base64",...}}` |
| zai (Anthropic-compatible) | same | same; URL source `{"type":"url"}` |
| opencode-go qwen3.7-max (`/messages`) | same | same |
| chatgpt gpt-6-luna (Responses) | `convertOpenAIInput` | `{"type":"input_image","image_url":"data:...;base64,..."}` |
| opencode-go muse-spark (`/responses`) | same | same |
| opencode-go deepseek-v4.1-flash and other chat models (`/chat/completions`) | `convertOpenAIChatMessages` | `{"type":"image_url","image_url":{"url":"data:..."}}` on user messages only |
| fireworks (chat completions) | same | same |

Images inside a `tool_result` are not serialized by any builder; only user
messages carry them. That is enough for a scorer call (one user turn with
the screenshot), and is left as is.

### Models affected and their upstream image support

| Model | Provider | Image support | Source |
|---|---|---|---|
| gpt-6-luna | chatgpt | yes | owner, 2026-10-15; the Codex CLI sends `input_image` on the same endpoint |
| deepseek-v4.1-flash | opencode-go | yes | owner, 2026-10-15 |
| Claude Haiku 4.5, Sonnet 4.6, Opus 4.6 | bedrock | yes, base64 sources only | Anthropic model docs (all Claude 4.x take image input); Bedrock invoke takes base64, not URL |
| glm-5.2, glm-5.1, glm-5-turbo | zai | **UNVERIFIED**, believed text-only | Z.AI ships vision as separate GLM-*V models |
| glm-5.3-flash, hy3, qwen3.7-max, muse-spark-1.3-contributor(-free), cat-jam-free, longcat-2.5-preview-free, step-5-preview-free | opencode-go / opencode-zen | **UNVERIFIED** | no doc or code in hand confirms image input through OpenCode |

The UNVERIFIED rows are gated (opted out) until the probe in
`scripts/vision_probe.py --model <id>` passes for them; then their opt-out
line is deleted.

## Failure modes (written before the code)

Routing and capability:

1. **An image reaches a model that rejects it.** The upstream answers 4xx
   ("image input not supported") or, worse, a proxy strips the part and the
   model answers blind. The gateway must refuse first, naming the model and
   the opt-out. Today that refusal is worse than a 4xx: its error string
   carries no `: status 4xx`, so `providerFailureCounts`
   (`internal/gateway/circuit_breaker.go:169`) counts it as an upstream
   failure and trips the model breaker for every computer, and the gateway
   answers 502, which `gatewayruntime` retries as transient. A media refusal
   must be a typed client error: 400 to the caller, never retried, never
   counted by the breaker.
2. **The opt-out list is bypassed.** A second copy of the rule (the
   modelpolicy tool) disagrees with the gateway. One function in
   `modelcatalog` must decide; both callers use it.
3. **An uncatalogued model is treated as vision-capable.** Vision-on is the
   default for *configured* (catalogued) models. An id the catalog does not
   know stays refused: Choir knows nothing about it.
4. **A pool assigns an image request to a text-only member.** The default
   pool holds cat-jam-free, which is gated. A pool-resolved image call must
   land on a vision member, or fail loudly, not 4xx at the upstream.
5. **A URL source goes where URLs are not fetched.** Bedrock takes only
   base64. URL sources also let an upstream fetch arbitrary addresses on
   Choir's behalf. URL sources go only to providers whose wire documents
   them (chatgpt Responses, fireworks chat completions); others are refused
   with "inline the image".
6. **Silent drop.** The OpenAI-compatible route must carry `image_url`
   parts through as image blocks, never discard them.

Size and cost:

7. **Oversized or hostile images.** A huge PNG, a decompression bomb
   (small file, enormous dimensions), or many images in one call exhaust
   gateway memory or exceed upstream limits (Anthropic: 5 MB per image,
   8000 px per side; long edges over 1568 px are resized upstream anyway).
   Policy: refuse more than 20 images per request, or an encoded image over
   20 MiB, or over 50 megapixels by header (checked before decoding); a
   decodable PNG/JPEG/GIF whose long edge is over 1568 px is downscaled to
   1568 px (area average) and re-encoded as PNG; after that, an image over
   5 MiB is refused. Formats the gateway cannot decode (WebP) pass only if
   under 5 MiB and are not resized.
8. **Cost surprise.** Image input is billed as input tokens by every
   provider here (Anthropic about width×height/750 tokens; OpenAI by tiles;
   the subscription chatgpt path is flat-rate). The providers report the
   image tokens inside `usage.input_tokens`, so Choir's per-token cost
   accounting (`internal/llmcost`) already counts them; no per-image price
   is needed. The gateway logs the image count per call so a cost jump can
   be attributed. Per-provider token formulas for deepseek/OpenCode are
   UNVERIFIED.
9. **Token accounting drift.** Context-pressure estimates in the runtime
   count characters of text; an image's tokens are invisible to them until
   the provider's usage comes back. Acceptable for one-shot scorer calls;
   a desk transcript that accumulates images would need an estimate
   (residual).

Caching:

10. **Prompt-cache breaks.** An image in the prompt prefix caches only if
    its bytes are identical across calls. Downscaling must be deterministic
    (same input bytes, same output bytes), and images should sit at the end
    of the user turn, after the stable prefix. Refs resolve to the same
    bytes every time, so a re-sent ref keeps the cache.

Privacy and traces:

11. **Image bytes leak into logs and traces.** Base64 in a message is
    copied into run transcripts, run-progress payloads, gateway logs,
    error strings and probe output. Rules: the gateway logs counts, byte
    sizes and a short sha256 per image, never data; errors never quote
    data; a caller stores the image once by content hash
    (`artifact:sha256:<hex>`) and passes the ref; the computer resolves the
    ref to bytes only in the HTTP body to the gateway. Transcripts, events
    and API responses carry the ref, never the bytes.
12. **A ref resolves to something that is not an image, or outside the
    store.** The resolver accepts only `artifact:sha256:<64 hex>`, reads
    only `<store>/sha256/<hex>`, re-hashes the bytes against the ref, and
    sniffs an image type.

## Desk seam (design)

The minimal path, wired with the fix:

- `POST /api/model-policy/verify` on the computer (owner-authenticated):
  `{prompt, role | provider+model, image_base64 | image_fixture |
  image_ref}`. Base64 input is stored by content hash first and the model
  call carries the `artifact_ref`; the response returns `image_ref`, the
  answer text and token counts, never bytes. This is also the staging
  probe's path (choir.news → proxy → computer → gateway).
- Management's `choir.ProductAPI` allowlist admits that one route, so the
  verifying desk can score a screenshot:
  `choir.ProductAPI("POST", "/api/model-policy/verify", {"prompt": rubric,
  "image_ref": "artifact:sha256:..."})`.
- `gatewayruntime.Provider` resolves `artifact_ref` image sources from the
  computer's image store before the gateway call, so any desk transcript
  may carry refs.

Not wired here (next slices): engineering's capture step writing the
freeze bundle's `evidence/visual/` PNGs into the image store (report S1),
and a `choir.LookAt` verb for desks without ProductAPI.

## Fix (second commit)

- `internal/modelcatalog`: `SupportedModels` fills `Modalities` from the
  vision default; `TextOnlyModels()` is the one opt-out list (id to reason);
  `SupportsImage`/`ImageRefusal` are the one rule. The provider gate and
  the modelpolicy verifier both call it. The `kimi-k2p6` name special case
  is deleted: an uncatalogued model is refused images.
- Vision on: gpt-6-luna, deepseek-v4.1-flash, Claude Haiku 4.5, Sonnet 4.6,
  Opus 4.6. Opted out: the three zai GLM models (believed text-only) and
  the eight other opencode-go/zen models (UNVERIFIED).
- `internal/provider/media.go`: `prepareMediaRequest` replaces
  `validateMediaRequest` in every adapter: the capability check, the count,
  byte and pixel caps, deterministic downscale to 1568 px, URL sources only
  to chatgpt and fireworks, and a typed `MediaError`. The gateway answers a
  `MediaError` with 400, the breaker does not count it, and the inference
  log line carries `images=N image_bytes=N image_sha256=<12 hex>`.
- The OpenAI-compatible route carries `image_url` parts as image blocks.
- `internal/imageblob`: the computer's content-addressed image store at
  `<RUNTIME_STORE_PATH>.image-blobs/sha256/<hex>` (on a guest
  `/mnt/persistent/state.image-blobs`). `gatewayruntime` resolves
  `artifact_ref` sources from it into the HTTP body to the gateway only.
- `POST /api/model-policy/verify` on the computer, and on management's
  ProductAPI allowlist (POST only).

Cost: no pricing change; image tokens arrive in `usage.input_tokens` and
`llmcost` prices them as input. Residuals: images inside `tool_result`
blocks are not serialized; runtime context-pressure estimates do not count
image tokens; the pool's failover order is not image-aware outside the
verify path (a desk transcript carrying an image fails over to cat-jam-free
only to be refused, loudly).

## Evidence after the fix

- `go test ./internal/modelcatalog ./internal/provider ./internal/gateway
  ./internal/gatewayruntime ./internal/modelpolicy ./internal/imageblob`
  and the focused agentcore route test.
- Staging: `scripts/vision_probe.py --cred <0600 json>` sends a generated
  PNG (solid colour with a word) to gpt-6-luna and deepseek-v4.1-flash and
  checks each answer names the colour and the word.
