# Station TE: a frozen candidate's screenshots embedded in its Texture document (design, 2026-10-25)

Owner (2026-10-25): "pull forward the embedding screenshots in the textures
... because then the evidence is persistent."

Why now: on 2026-10-24 two Reader candidates on the test computer failed
independent judgment on screenshots that showed only a loading screen. The
cause was the capture harness (fixed on lane `fp0-capture-guide`), but only a
verifier saw the image. If the image had been in the trajectory's Texture
document, the owner and the approving coding agent would have seen it at
once.

This note is the red ceremony for one narrow slice of the Texture media
design's step T1
([texture-media-and-autoradio-design-2026-10-20.md](../reports/texture-media-and-autoradio-design-2026-10-20.md)
sections 3 and 4 and "Corrections from the visual evidence build"). It
builds the consumer side that the visual evidence slice left for T1
([visual-evidence-slice-2026-10-15.md](visual-evidence-slice-2026-10-15.md)
section 10.3). Out of scope (the rest of T1): video, audio, external video,
slides, cards, and the public pinning of published assets (T2).

Mutation class: **red**. It changes the Texture canonical schema
(`choir.texture_doc.v1` gains a block type) and adds a canonical write.
Review tier 2 (a Texture canonical write and a schema change).

## 1. The schema change

One block type, `embed`, is added to `choir.texture_doc.v1`. It is
additive: a document with no embed is byte-identical to before, and the
schema string does not change, because bumping it would make every old
reader refuse every document.

```
{ "type": "embed",
  "attrs": {
    "id":      "embed-<…>",
    "kind":    "capsule_evidence" | "image",
    "target":  { … per kind … },
    "caption": "…",          // written by Texture, never by the producer
    "alt":     "…" } }       // written by Texture
```

| Kind | `target` keys | Required |
| --- | --- | --- |
| `capsule_evidence` | `digest` (64 lowercase hex, an asset-store digest), `operation_id`, `computer_id`, `route` (starts with `/`), `width`, `height` (the capture viewport, 1..16384), `blank` (bool) | all but `blank`; `caption` and `alt` too |
| `image` | `digest`, optional `width` and `height` (both or neither) | `digest`, `alt` |

Rules the validator (`internal/texturedoc`) enforces on every write, since
the store validates every revision through it:

- **Top level only.** An embed is a block of the document, never inside a
  list item or a quote. Renderers and the Write frame address top-level
  blocks.
- **A leaf.** No content, text or marks.
- **Closed attributes.** Unknown keys in `attrs` or `target` are refused.
  `selector`, `poster` and `transcript` are reserved for the later kinds and
  refused now.
- **Privacy is derived, never written.** An `attrs.privacy` key is refused.
  `texturedoc.Embed.Privacy()` derives it: `capsule_evidence` is always
  private; `image` is private until T2 adds public pinning, so today every
  embed is private and none is publishable.
- **Strict writers, tolerant readers.** The validator refuses an unknown
  kind, so no writer can store one today. Renderers do not trust that: an
  unknown kind renders as a plain card (web), so a document written by a
  newer computer still reads on an older renderer.
- **Not in world documents** (captures) or owner text.
- Caption and alt follow the Texture authoring rules on text (no legacy
  citation syntax) and are bounded (1000 and 500 characters).

The text projection of an embed (the revision's derived `content`, the
search index, the audio rendition later) is `[evidence: <caption>]` for
`capsule_evidence` and `[image: <caption or alt>]` for `image`. The web
editor's projection writes the same text, so an owner save's `content`
matches the server's projection.

### Why a block and not a source entity

`texture_evidence_sources.go` and the `screenshot` source target kind
already exist. They are the wrong carrier. A source entity is cited inline
by a `source_ref`, enters the publication source manifest, and is subject
to the publish gate's outlet counting. Evidence is not a citation of a
sentence, and it must never reach a publication manifest. A block keeps it
out of all three.

### Compatibility

- **Old revisions:** no embed blocks, so no change.
- **Old Go readers** (an older guest image after a binary revert, an older
  corpusd): `texturedoc.Validate` refuses the unknown block type. Reading a
  revision does not validate, so documents still display through the API,
  but an old binary refuses any *write* on a document whose head holds an
  embed (a Texture turn's Write, an owner save). An old corpusd refuses to
  publish it, which is the correct outcome by accident. Recovery on a
  reverted binary is the owner's existing "restore revision" act per
  document, back to the head before the evidence revision. This is why the
  rollback is the switch (section 6), not a binary revert.
- **Old web frontend** (a cached bundle): its renderer returns nothing for
  an unknown block, and its serializer drops it on the next owner save. The
  new serializer round-trips an embed by its node id from the revision it
  loaded and never builds one from the DOM.
- **The Texture desk's Write grammar** has no embed token. The frame shows
  an embed as one read-only line, `[embed capsule_evidence <route>
  sha256:<digest>]`. A Write that removes or changes an embed block is
  refused (`@replace`/`@delete` on it, or an `@all` that does not copy its
  line unchanged); `@insert` and edits to other blocks keep it.

## 2. The write path, and why Texture stays the sole writer

When a fresh Freeze observes visual captures, and the computer setting
`texture.evidence_embeds` is on, the runtime asks the Texture owner to embed
the candidate's captures in the document bound to the operation's
trajectory:

1. `settleFreezePrecommits` (agentcore) already receives the runtime's own
   Freeze observation (V23: the model cannot supply it). On a fresh freeze
   with at least one capture, it calls
   `Runtime.requestCandidateEvidenceEmbeds(owner, computer, operation)`.
   A replayed freeze carries no observation and asks nothing.
2. That reads the switch (anything but a stored true is off) and, off the
   reduction path (a bounded, detached job the tests can wait on), calls the
   bound `TextureEvidenceEmbedder` seam. autoputer binds the Texture owner
   handler, the same way it binds `TextureCellAuthorizer`.
3. `textureowner.Handler.EmbedCandidateEvidence` (Texture's package):
   - re-resolves the candidate through
     `Runtime.CandidateVisualEvidence(computer, operation)`, which is the
     visual-evidence listing's own code (bound bundle, the source-patch
     export's binding rules). The digests come only from the operation's
     frozen bundle on this computer. This is the membership check at write:
     no caller, desk or model can name a digest;
   - finds the document through the operation's trajectory
     (`trajectory.subject_refs.doc_id`) and checks the document names that
     trajectory;
   - returns without writing if the head already holds an embed of this
     operation (idempotent; a second trigger is a no-op);
   - appends one `capsule_evidence` embed per capture, in capture order
     (a filmstrip is a list, section 10.3), with Texture's words:
     caption "Screenshot of `<route>` from candidate `<operation>`
     (WxH)", and for a blank capture "Blank screenshot of `<route>` ...
     (the page rendered nothing)"; alt "Screenshot of `<route>`";
   - pins every embedded digest (section 4), then commits the revision
     through `CommitLifecycleArtifactHeadWithSourceGraph`, the store's
     atomic, head-pinned lifecycle commit that the owner revision path also
     uses, with `author_kind: appagent`, `author_label: texture`, a
     deterministic revision id and command id, and revision metadata
     `texture_evidence_embed` (operation, digests) and
     `texture_embedded_assets` (digests);
   - retries a lost head race (`ErrConcurrentStateChange`) up to three
     times against the new head.

Why this keeps Texture the sole writer (doctrine I26: the app desk is the
sole *agent* writer of its app's object kinds): the only code that writes an
embed is Texture's own package, through Texture's own commit transaction.
Management never writes it, engineering never writes it, and no desk can
name a digest. The write is deterministic, not a model turn, for the same
reason the `visual_captured` rule settles only from the runtime's
observation: a model that could write evidence digests could present any
stored image as a candidate's screenshot. The words of the embed (caption,
alt) are Texture's (Texture design correction 5); the capture carries
none. The write adds only embed blocks, no prose, so the desk still owns
every sentence of the document.

The owner revision route, the other entry for a raw `body_doc` (the owner's
editor and a coding agent's `write:texture` key), may keep or delete an
existing embed but may not add a `capsule_evidence` embed that the parent
revision does not hold: the membership rule would otherwise have a second
door.

The candidate identity is **not stored** in the revision: `bundle_digest`
(the draft digest until Verify finalizes it), `frontend_sha256`,
`capture_receipt` and `resolvable` change or can go stale, so readers
re-resolve them from the listing. The revision stores only what the capture
was: digest, operation, computer, route, viewport and blank.

## 3. Failure modes (written before the code)

| # | Failure | Handling | Test |
| --- | --- | --- | --- |
| E1 | **Missing asset.** The store does not hold a digest. | The embed is still written (it is the candidate's evidence claim); the web renderer shows "evidence not retained or unreadable" on the asset route's 404, not a broken image; `choir texture read` names the digest and the fetch command, and `choir self-dev visual` reports `resolvable: false`. | renderer, CLI |
| E2 | **Store write failed at Freeze** (V22). | Same as E1: `resolvable` is not stored, it is re-checked live. | embedder |
| E3 | **Digest mismatch** (bytes swapped or tampered). | The asset route re-hashes on every read and answers 500; the renderer's error state applies; `choir self-dev visual --out` hashes every body. | existing route test |
| E4 | **Oversized.** | Viewport bounded 1..16384 per side; caption and alt bounded; at most `visualcapture.MaxRoutes` (6) captures per evidence write, more is refused; bytes bounded by the store (20 MiB). | schema, embedder |
| E5 | **A revision referencing a digest from another computer.** | The writer only takes digests from this computer's operation; the embed records `computer_id`; the asset route serves only this computer's store, so a copied document shows the unresolvable state, never another image (a digest names bytes, not a place). | embedder |
| E6 | **Publication leak.** | Refused, not stripped, at three points: the proxy's publish route (409 with the reason), corpusd's `PublishTexture` (the authority), and story auto-publish (held with the reason). An older corpusd refuses through its validator. | platform, story |
| E7 | **A forged embed** (a desk or a key names a digest as evidence). | The Write grammar has no embed token; the owner route refuses a new `capsule_evidence` embed; only the evidence path writes one, from the bound bundle. | texturewrite, owner route |
| E8 | **A desk Write removes or alters an embed.** | Refused with the block named; the line to copy is in the frame. | texturewrite |
| E9 | **The Texture desk commits between read and write.** | Head-pinned commit; the embedder retries up to three times; the desk's own Write is refused once ("the document changed since your frame") and re-frames. | embedder |
| E10 | **A replayed or repeated trigger.** | A replayed freeze asks nothing; a repeated ask finds the operation's embeds at head and writes nothing. | embedder, trigger |
| E11 | **A crash between the Freeze and the embed write.** | No embeds: the restart rule forbids a boot resumer without a planned-restart marker. The listing and `choir self-dev visual` still show the captures. Residual R1. | none (accepted) |
| E12 | **The trajectory is no longer live, or the document is archived.** | A settled trajectory commits unbound (the store's rule); an archived document is skipped with a log line. | embedder |
| E13 | **The switch cannot be read.** | Off. | trigger |
| E14 | **The owner editor drops an embed on save.** | The serializer emits an embed only by its node id from the loaded revision; a cached old frontend is residual R3. | renderer |
| E15 | **Script in the document origin.** | Only `<img>`; no inline handlers (the error state is a capture-phase listener in the component); caption and route escaped; the asset response carries `nosniff` and a sandbox CSP. | renderer |
| E16 | **All-blank evidence presented as support.** | Each blank capture's caption says so and the renderer badges it "blank"; the renderer adds "every capture of this candidate is blank" when they all are. | renderer, embedder |
| E17 | **A pin is not recorded and a future sweep orphans the document.** | The pin is written before the revision commits; a pin whose commit then fails is a harmless extra hold. | imageblob, embedder |

## 4. Retention: pin on cite

There is no sweep of the asset store today (`internal/imageblob` has no
delete; nothing else removes from `state.image-blobs`). The pin is recorded
anyway, so the first sweep has something to honor:

- `imageblob.Store.Pin(digest, holder)` writes
  `<store>/pins/sha256/<digest>/<sha256(holder)>` holding the holder name,
  `texture-revision:<doc>:<revision>`, atomically. `Pins(digest)` lists
  them. A sweep must keep any digest with a pin.
- The revision's metadata also lists `texture_embedded_assets`, so the pin
  set can be rebuilt from the tape if the pin files are lost.

## 5. Readers

- **Web** (`frontend/src/lib/texture-structured-editor-doc.ts`):
  `<figure contenteditable="false" data-texture-embed>` holding a box whose
  `aspect-ratio` and `width`/`height` come from the viewport (no layout
  jump), `<img src="/api/assets/sha256/<digest>">` (the owner's session
  authorizes it, `loading="lazy"`, `referrerpolicy="no-referrer"`), and a
  caption with the route in `<code>`, a "blank" badge, and the operation
  id. A failed load switches the figure to "Evidence not retained or
  unreadable" with the digest. The candidate identity line (bundle digest,
  frontend build, capture receipt) is fetched live from the listing, and a
  404 or 410 there says the candidate's bundle is no longer retained.
  Unknown kinds render as a plain card with the kind and digest.
- **CLI:** `choir texture read <doc>` keeps the document JSON and adds
  `embeds`: one row per embed of the current revision (block id, kind,
  digest, operation, computer, route, viewport, blank, `asset_url`) and the
  `choir self-dev visual --computer <c> --operation <op> --out DIR` line
  that fetches and verifies the bytes.

## 6. Rollback

- **One act: the switch.** `texture.evidence_embeds` is a computer setting,
  default off. `POST /api/computer-settings {"key":
  "texture.evidence_embeds", "value": false}` stops every new evidence
  write at once, live, no deploy. It is an owner act; management's
  ProductAPI cannot reach the settings route.
- What the switch does not undo: revisions already written stay (the
  revision chain is immutable). The owner can delete an embed in the editor
  or restore the revision before it.
- **Repo rollback:** a revert of the code commits. Read section 1
  "Compatibility" first: after a binary revert, documents whose head holds
  an embed refuse writes until restored. With the switch off from the start
  (the default), no document holds one and the revert is clean.

## 7. Red ceremony

- **Conjecture delta.** C: a frozen candidate's captures embedded in its
  trajectory's document, written only by Texture's package from the bound
  bundle and never publishable, make the visual evidence persistent and
  visible to the owner and the approving agent without giving any desk a
  way to forge it. Falsified by: a screenshot that the owner or the
  approving agent needed and could not find in the document; an embed whose
  digest is not in its operation's bundle; any embed reaching a
  publication.
- **Protected surfaces.** Texture canonical schema and canonical writes
  (red); publication (the refusal only); the computer settings registry
  (one key). Not touched: Trace/evidence events, checkpoint/route
  projection, auth, vmctl, gateway calls, run acceptance, the decision
  route (the embed carries no authority; approval stays the owner's act).
- **Admissible evidence class.** Local: unit and handler tests on the real
  store (texturedoc, texturewrite, imageblob, textureowner with a seeded
  lifecycle document, agentcore's freeze trigger, platform publish, the
  CLI, the frontend renderer). Not claimed locally: the Linux Freeze
  executor end to end, a real capture, the browser loading an image behind
  the owner session, staging. The staging proof is a `--visual` drill with
  the switch on, then reading the document in the browser and with
  `choir texture read`.
- **Rollback.** Section 6.
- **Heresy delta.**
  - discovered:
    - H1. The editor's text projection writes every citation as `[n]`, while
      the server writes an `expanded_ref` as `[source n: title]`, and the
      server refuses an owner save whose `content` differs from its
      projection. Hypothesis, from the code only, not observed on staging:
      an owner save of a document holding a stored expanded ref is refused.
      Not repaired here.
    - H2. The asset route serves any digest in the owner's store (V34,
      already recorded); the embed renderer does not rely on it for
      membership.
    - H4. `frontend/tests/texture-structured-editor-doc.spec.js` cannot load
      on the current main: the module imports
      `texture-source-renderer` -> `media-utils.js` -> `apps/registry` ->
      `apps/packages.ts`, whose `import.meta.glob` exists only under Vite,
      so Playwright's loader fails with "glob is not a function" and runs
      no test (since the app packages P1 commits, 2026-10-24). Found while
      building this; not repaired here. The embed renderer lives in
      `texture-embeds.ts`, which imports nothing, so its own spec runs.
  - introduced:
    - H3. A Texture revision path that is not a desk turn: a deterministic
      write from Texture's package. Bounded to embed blocks, behind the
      switch, with no prose. If doctrine later says every Texture revision
      must be a desk turn, the alternative is a platform record to the
      desk's inbox and an `@evidence <operation>` directive that
      materializes from the listing; that relies on the model to act and
      was not chosen for that reason.
  - repaired: none.

## 8. Residuals

- R1. No embeds after a crash between Freeze and the write (E11); no boot
  resumer by rule. The listing remains the source.
- R2. The pin has no consumer until a sweep exists; T1's sweep must read
  pins.
- R3. A cached old frontend drops embeds on an owner save (E14).
- R4. Binary-revert hazard (section 1, Compatibility).
- R5. The iOS app (not in this repository) does not know the block yet;
  until it does, the block's plain-card fallback is a requirement on it,
  not something built here.
- R6. The management review's vision reading is not attached to the embed
  (10.3: "may refine alt, cited, not copied unchecked"); later.

## 9. Built (2026-10-25, lane `fp0-evidence-embed`)

- `internal/texturedoc/embed.go`: the block, its validator, projection,
  derived privacy, `Embeds`, `PrivateEmbeds`, `EmbeddedDigests`.
- `internal/texturewrite`: the frame line (`EmbedFrameLine`) and
  `keepEmbedBlocks`, the refusal of a Write that removes or changes one.
- `internal/imageblob/pin.go`: `Pin` and `Pins`.
- `internal/agentcore`: `VisualEvidenceListing` exported with the
  operation's trajectory and `Runtime.CandidateVisualEvidence` (the route
  answers as before); `texture_evidence_embeds.go` (the seam, the switch
  check, the detached job); the trigger in `settleFreezePrecommits`; story
  auto-publish holds a document with a private embed.
- `internal/store`: `texture.evidence_embeds` in the settings registry,
  default off.
- `internal/textureowner/texture_evidence_embed.go`: the write
  (`EmbedCandidateEvidence`); the owner route's refusal of a new
  `capsule_evidence` embed. autoputer binds it.
- `internal/platform` and `internal/proxy`: the publish refusal
  (`PrivateEmbedRefusal`), 409 at the proxy.
- `cmd/choir`: `texture read` adds `embeds`.
- `frontend/src/lib/texture-embeds.ts` and `.css`: the renderer, wired into
  `texture-structured-editor-doc.ts` (render, projection, serializer by id,
  a pasted copy dropped) and `TextureEditor.svelte` (identity hydration
  and the capture-phase error listener).

One behavior to know: Texture's evidence write reads the head the way a
desk's Write does (`texturewrite.Base`), so a head that is still the owner's
plain first text (one paragraph of lines) is committed as its markdown
blocks, as the desk's first Write would.

What the tests prove (local, Darwin): the schema accepts the two kinds and
refuses every shape in section 3; a desk's Write keeps, and cannot remove
or change, an embed; the write on the real store with a seeded lifecycle
document appends one embed per capture from the listing only, stores no
candidate identity, marks blank captures, pins every digest to the
revision, is idempotent, refuses a listing for another trajectory, a bad
digest, zero or more than six captures, and leaves the head unchanged on
refusal; the desk's next Write on the old head is refused and on the new
head keeps the embeds; the owner route refuses a forged embed and accepts
keeping or deleting one; the Freeze trigger asks only on a fresh freeze
with captures and the switch on, off the reduction path; corpusd and the
proxy refuse a document with a private embed; `choir texture read` lists
the embeds with the fetch command; the web renderer's output (escaping, the
asset route only, reserved box, blank badge and all-blank warning,
unresolvable state, plain card for an unknown kind, serializer by id).

What they do not prove: the Linux Freeze executor end to end into the
trigger (the trigger is driven through `settleFreezePrecommits` with the
runtime's observation, as the visual slice's own tests do); a real browser
loading the image behind the owner's session; the identity line against a
live listing; the editor component itself (only its modules: H4 blocks the
editor-doc spec, and the integration was checked with a one-off esbuild
harness outside the repo); anything on staging.
