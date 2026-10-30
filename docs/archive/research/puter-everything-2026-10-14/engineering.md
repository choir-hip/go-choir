# Engineering group review: cad, parametric, circuits, building-design, api-client

Reviewer scope: `src/{cad,parametric,circuits,building-design,api-client}` of the
HeyPuter/everything shallow clone. Read-only static review; nothing was
executed, so every quality judgement is from reading code and tests, not from
running them. Paths below are relative to `repo/src/`.

**Shared provenance note.** All four real apps come out of one generation
pipeline: the same README voice and layout, the same `tools/check-licenses.mjs`,
identical pinned dev dependency versions (`lucide 1.55.0`, `vite 8.3.4`,
`vitest 5.0.4`, `jsdom 30.1.2`), the same "no framework, vanilla ES modules +
WebGL2 + Puter.js" architecture, and the same `*.puter.localhost` dev plugin.
They are almost certainly AI-generated. They are not the slop the owner
expected, though: the code is coherent and layered (a DOM-free core under each
UI), commented with intent, and backed by real test suites. Treat them as
competent first drafts: correct-looking but unverified.

---

## 1. parametric (3D feature-based CAD) — the headline app

### What it is, stack, size
- An open SolidWorks/Fusion/Inventor-style part modeler: constrained 2D
  sketches, then a feature history (extrude, revolve, fillet, chamfer, shell,
  hole, linear/circular pattern, mirror, plane, loft, sweep, draft, STEP
  import), with a rollback bar, global variables and `=expr` parameters, 2D
  drawing sheets (PDF/SVG), and STEP/STL export.
- TypeScript, no UI framework, WebGL2 renderer, ~24.6k lines of TS/JS across
  136 files.
- **Geometry kernel: OpenCascade (OCCT) compiled to WebAssembly, via
  `replicad` 1.1.0 (MIT) and `replicad-opencascadejs` 1.1.0 (LGPL-2.1-only).**
  It runs in a Web Worker (`src/kernel/worker.ts`, `client.ts`). It is not
  manifold and not a custom kernel. `src/kernel/build.ts` (54 KB) is the
  feature-regeneration layer on top of replicad (`import * as R from
  'replicad'`).
- Licenses: everything is MIT/ISC except the OCCT wasm (LGPL-2.1). The app
  ships it "as a separate, replaceable WebAssembly module"
  (`public/licenses/THIRD_PARTY_NOTICES.txt`), which is the right way to meet
  the LGPL. A build-time license gate exists (`tools/check-licenses.mjs`).

### Quality
- **This is a real modeler, not a thin UI over a library.** The parts that
  matter are original code:
  - `src/sketch/solver.ts`: a 2D geometric constraint solver using damped
    minimum-norm Gauss-Newton per independent cluster, with Jacobian rank
    analysis that reports degrees of freedom and redundant or conflicting
    constraints (under/full/over definition). This is the correct design.
  - `src/sketch/profiles.ts`: finds planar faces on the sketch graph; nested
    loops alternate between material and holes.
  - `src/kernel/build.ts`: rebuilds the feature history with a per-step cache
    keyed on a hash chained through every earlier feature, so editing feature k
    rebuilds only k and what follows. Topological naming: each face is named
    after the feature and sketch entity that created its surface, and names
    pass to later bodies when faces lie on the same surface. Edges are named by
    their two faces. A geometric signature (`FaceSig`/`EdgeSig` in
    `src/model/types.ts`) breaks ties when a face is split. This is a
    reasonable answer to the topological naming problem, the classic weakness
    of history-based CAD. Expect it to fail on complex splits and merges, as
    every heuristic scheme does.
  - `src/model/expr.ts`: parameter expressions and global variables.
- **Limitations seen in the code:** one body per part ("Features are applied in
  order to a single body", `build.ts`), so there are no multi-body parts and no
  assemblies (`.sldasm` is refused explicitly). Sketch splines are Catmull-Rom
  exported as Béziers, not true NURBS.
- **Tests:** about 120 cases. They include `tests/kernel.test.ts` and
  `tests/examples.test.ts`, which run **the real OCCT kernel headless in Node**
  and check that every built-in example solves and has positive volume. There
  are also solver, profile, expression and document tests, assistant tool and
  agent-loop tests against a Node host (`tests/assistant-host.ts`), and
  Playwright e2e specs (modeling, assistant, drawing, mobile, a11y).
- **It already contains an AI assistant** (`src/assistant/`), and it is the
  most Choir-relevant code in this group (see section 4).

### Worth taking
1. **The part document model** (`src/model/types.ts`, `document.ts`,
   `features.ts`, `expr.ts`). It is plain JSON: a feature history, sketches
   with entities, relations and named dimensions, `Param = number | "=expr"`,
   and face/edge references by stable name plus signature. It can be posted
   unchanged to the worker and diffed for undo.
2. **The sketch constraint solver** (`src/sketch/solver.ts`, ~29 KB),
   self-contained.
3. **The feature regeneration layer** (`src/kernel/build.ts`, `occ.ts`, plus
   replicad and OCCT wasm), which runs both in the browser and headless in
   Node.
4. **The assistant tool surface** (`src/assistant/tools.ts`, `inspect.ts`,
   `sketch.ts`, `features.ts`, `prompt.ts`, `agent.ts`). Every mutating tool
   rebuilds, checks feature status, and **undoes itself and returns an error if
   any feature fails**. All changes from one message squash into one undo step.
   Tool results return exact ids, face names and coordinates for the next call.
   The prompt treats part names and attachments as data, not instructions. This
   is the right agent contract, and it maps almost one-to-one onto desk verbs.
5. **Do not take** `src/formats/sldprt/`, the SolidWorks importer. It
   reverse-engineers SolidWorks containers and decodes embedded Parasolid
   transmit data, and it ships `sch_13006.s_t`, a Parasolid schema file whose
   header shows a Siemens/Parasolid origin ("PARASOLID … SITE=sdl-cambridge-u.k.
   … DATE=24-mar-2001"). The notices relicense it as MIT through the
   third-party `ps-parser` project. That provenance is doubtful, and the
   importer is not needed: STEP import covers interchange.

### Can an agent script the model? (the owner's question)
Yes, and the design is better for agents than a code-defined model would be.
- The model is **declarative and parametric**: a JSON feature history with
  named, expression-driven dimensions. It is not imperative code like
  CadQuery or OpenSCAD scripts. An agent edits it through typed verbs
  (`create_sketch`, `add_extrude`, `edit_feature`, `set_variables`, …) or by
  writing the JSON directly.
- That beats free-form code for agents in three ways. Every edit is validated
  by a rebuild and rolled back when it fails. Diffs are small and semantic,
  which suits a commitment ledger. A human can still open the same document and
  drag a sketch point or double-click a dimension.
- Code is still available as an escape hatch. replicad itself is a
  code-CAD API (JS, CadQuery-like), so a Choir rebuild could add a sandboxed
  `script` feature type that returns a replicad shape. It would be cached and
  named like any other feature, for geometry the feature set cannot express.
- Headless execution works today: `ModelBuilder` runs in Node with
  `replicad-opencascadejs` (`tests/examples.test.ts`). An agent-only CAD desk
  does not need a browser. Caveat: the OCCT wasm is Emscripten output with JS
  glue, so on the Choir computer it needs a JS runtime process (Node or Bun)
  in the capsule. It cannot run from Go via wazero without real effort.

### App-agent design (Choir "CAD desk")
- **State on the computer:** each part is a document object holding the
  feature-history JSON (`parametric-part` format), with blobs such as imported
  STEP stored as content-addressed objects. Each verb records a semantic event
  (feature added, dimension changed, variable set) on the tape. Derived
  artifacts (STEP, STL, drawing PDF, thumbnail renders, mass properties) are
  cache objects keyed by the history hash, never canonical state. Revisions are
  the history; rollback is the existing rollback bar.
- **Direct manipulation:** sketch drawing and dragging with live solve, Smart
  Dimension, double-clicking a feature or face to edit it, rollback-bar drag,
  suppress/hide, view orientation, measure, section view.
- **Verbs for other desks and external agents:** `get_model`, `get_sketch`,
  `find_geometry` (faces and edges by name and signature), `measure`,
  `mass_properties`, `create_sketch`, `edit_sketch`, `add_<feature>`,
  `edit_feature`, `delete_features`, `set_variables` (the main parametric
  handle: "make Width 120"), `set_rollback`, `render_view` (a screenshot for
  vision self-checks), `export {step|stl|pdf}`, `import_step`, and
  `make_variant` (fork the part with different variables).
- **Calls to other desks:** research (datasheets, standard sizes such as ISO
  bolt or bearing dimensions and material properties, cited as evidence on the
  variables they drive); Texture (a design-rationale document and a
  dimensioned drawing embedded in a report); engineering (generate a
  manufacturing or 3D-print pipeline, run FEA or slicing in a capsule).
- **Why an agent beats a plain app:** the agent turns intent ("M5 clearance
  holes on a 40 mm bolt circle, 3 mm wall") into constrained, dimensioned
  geometry, checks itself by rebuild, measure and render, keeps design intent as
  named variables tied to research evidence, and produces variant families.
  Users still have the full direct-manipulation surface for the last 10%.

### State as typed objects in the object graph (owner addition)

**Grounding first.** I read `internal/objectgraph/object.go`, `store.go`,
`registry.go` and `dolt_store.go`. An `Object` has a canonical id
(`obj:<kind>:<owner>:<suffix>`), a kind, owner and computer, a `VersionID`, a
`ContentHash`, a `Body`, JSON `Metadata`, and `Tombstone`/`SupersededBy`.
`Edge`s are typed. Each kind is registered with an identity mode (content hash
or external key) and a `Versioned` flag. Existing practice for versioned
documents is a stable head object (`choir.texture_document`, external key)
plus content-addressed revision objects (`choir.texture_revision`, versioned)
joined by `document_revision` and `revision_parent` edges. Two consequences
matter for the design:

- **History** comes from the revision chain, and Dolt commit history sits
  underneath it.
- **Dolt branches are not exposed by `objectgraph` today.** `DoltStore` does
  not commit per write and has no branch API. So "variants as branches" means
  revision lineages in the graph now. Real Dolt branches would need new store
  work.

I follow the same head-plus-revision pattern below and reuse existing kinds
and edges wherever they fit, as the Autopaper design asks ("existing types
only … unless we need to").

**Object types:**

| Object | Kind (new unless noted) | Identity | Body / metadata |
|---|---|---|---|
| Part (stable handle) | `choir.cad_part` | external key | metadata: name, units, current head revision, material |
| Part revision | `choir.cad_part_revision`, `Versioned: true` | content hash | body: the whole `parametric-part` JSON (features, sketches, variables, rollback); metadata: history hash, feature count, bbox, volume, mass, rebuild status |
| Derived artifacts (STEP, STL, drawing PDF/SVG, renders) | existing `choir.artifact_blob` + `choir.artifact_manifest` | content hash | keyed by the revision's history hash; cache, never canonical |
| Imported STEP / DXF / reference photo | existing `choir.artifact_blob` (or `choir.web_capture` when fetched from the web) | content hash | the `blobs` map in their document becomes edges to these objects |

**Where their model maps cleanly, and where it does not:**

- **The part revision is the right unit.** Their document is already plain
  JSON, posted unchanged to the kernel, and identified by a chained history
  hash. That is exactly a content-addressed revision object, and their
  per-step cache key could be the revision's content hash.
- **Individual features and sketches should *not* be separate objects.** The
  history is order-dependent, and topological names (`f2:cap1`,
  `f2:side:base_l0`) are only meaningful within one ordered history. Splitting
  features into objects would turn every edit into a multi-object conditional
  batch with no gain. Inside the revision body, each feature's id gives
  agents and events a stable address (`<revision>#f7`), and edge metadata can
  point at it.
- **Global variables stay in the body.** Their *provenance* goes on edges: a
  `cites` edge from the revision to a research `choir.web_capture`
  (datasheet, ISO table), with metadata `{variable: "BoltDia", value: 5.5}`.
  That makes design intent auditable without a variable kind.
- **Undo:** in-app undo stays local and snapshot-based (their `DocStore`).
  The graph gets one revision per *accepted transaction*: one assistant
  message (already squashed into one undo step), or one user "commit" at save
  or idle time. Never one per keystroke or drag. "Undo" across sessions means
  moving the head back to a prior revision, which leaves a visible event.
- **Variants:** a variant is a new `choir.cad_part` head whose first revision
  has a `derived_from` edge to the base revision, with metadata carrying the
  variable overrides. A variable-only variant is cheap: same history, a
  different `variables` block. "Try a variant" is the agent's `make_variant`.
  Merging back is adopting a revision, not a three-way merge. Dolt branches
  could later carry experimental sessions, but the graph lineage is
  sufficient and already supported.

**Links to other apps' objects** (existing edge kinds):

- An invoice, quote or inventory line `references` a part revision (a pinned
  revision, not the head, so a quote never silently changes).
- A Texture document `transcludes` a drawing artifact or render. A design
  rationale Texture `references` the part.
- A circuits board revision → the part: an enclosure part `derived_from` the
  board outline. The PCB outline becomes a sketch via DXF.
- A cad drawing `derived_from` a part revision (2D projection).
- A research capture → a part revision via `cites` on the variables it drove.
- Run provenance: the revision is `was_generated_by` the run of the desk or
  agent that made it (existing provenance edges).

**Autopaper object graph (published) vs private:**

- **Private by default:** all part heads and revisions, assistant transcripts
  and attachments (customer drawings, photos), and supplier or cost variables.
- **Published only through the existing publication layer**
  (`choir.publication`, `publication_version`, `publication_policy`,
  `consent_record`, owner-approved): a chosen part revision as an
  open-hardware object, plus its STEP/STL/drawing artifact manifests and
  provenance. The policy must decide one thing per publication: whether to
  publish the **editable feature history** (the revision body: full design
  intent, remixable) or **only the derived geometry** (STEP/mesh: usable but
  opaque). Other computers adopt it onto their tapes and fork it with
  `derived_from`. That is "remix a published part" for free.

### Priority: **high**
It is the only app in this group with a real engine, a clean declarative
model, and an agent tool contract that already works, and it matches the
owner's "CAD operated mostly by agents". **Effort, Choir way:** about 2–3 weeks
for a first agent-driven slice: lift `model/`, `sketch/`, `kernel/` and the
`assistant/` tool layer; run the kernel headless in a capsule JS runtime for
the desk; build a Svelte window that hosts their WebGL renderer, or a minimal
viewer to start; store parts as computer documents with semantic events. Full
interactive sketch editing at their level is another 3–5 weeks.

---

## 2. cad (2D drafting, AutoCAD-style)

### What it is, stack, size
- A 2D drafting app with AutoCAD command-line semantics: lines, polylines with
  bulges, arcs, ellipses, splines, text and mtext, six dimension types,
  leaders, hatches with island detection, blocks and inserts, layers,
  linetypes and lineweights, plus object snaps, polar tracking and grips.
- Vanilla JS, WebGL2, ~32.9k lines (135 files). Dependencies: `earcut` (ISC),
  `rbush` (MIT), `lucide` (ISC). All permissive.
- **The geometry kernel is custom** (`src/core/geom/`: intersect, offset, trim,
  fillet, join, break, explode, spline, ellipse, bulge, nearest, snaps,
  measure). No CAD library is used.

### Quality
- A real engine. `src/core/schema.js` documents an exact DXF-shaped entity
  model. `src/core/drawing.js` is a transactional, immutable-entity store with
  undo and redo and change events. `src/core/spatial.js` is an R-tree index.
  The hatch boundary detection (`src/core/hatch/boundary.js`, 500 lines) and
  the robust-geometry tests (`tests/geom-robust.test.js`) suggest care.
- **DXF R2000 ASCII reader and writer** (`src/io/dxf/reader.js` 2.5k lines,
  `writer.js` 1.3k lines) run in a worker and warn honestly about what they
  skip: binary DXF, polyface meshes, xref content, variable-width polylines.
- About 450 test cases, including command integration tests
  (`tests/commands.test.js`) that drive the real Editor with typed input:
  `t.type('LINE', '0,0', '@10,0', '@0,10', '@-10,0', 'C')`.
- The editor (`src/editor/editor.js`, 1.5k lines) uses async prompts
  (`await ed.getPoint(...)`). That fits humans and the command line. An agent
  could drive the same text protocol, but a direct JSON entity API on
  `Drawing` would be cleaner for agents.

### Worth taking
- **DXF import/export** (`src/io/dxf/`): the most portable piece. Choir's
  Parametric rebuild could use it for DXF sketch import and drawing export, and
  the assistant already accepts DXF attachments.
- **2D geometry kernel** (`src/core/geom/`) and the hatch and dimension code,
  if Choir ever needs 2D drafting.
- The entity schema (`src/core/schema.js`) as a reference model.

### App-agent design
- State: a drawing document (the JSON in `schema.js`, stored compressed as
  `.cad`) with per-transaction events (`{ids, tables, header, blocks}` are
  already emitted).
- Verbs: `add_entities`, `modify` (move/rotate/scale/offset/trim/fillet by id),
  `set_layer_props`, `insert_block`, `dimension`, `hatch_region`,
  `run_command` (the AutoCAD text protocol, already agent-friendly),
  `import_dxf`, `export {dxf|pdf|svg}`, `query` (by layer, type or bounds).
- Other desks: research for floor plans and standards; Texture for embedded
  drawings; parametric for the 2D projections of 3D parts.
- Agent advantage: moderate. 2D drafting is mostly direct manipulation, and an
  agent mainly helps with bulk edits, cleanup, and converting a photo or
  description into a plan.

### State as typed objects in the object graph (owner addition)

- **Types:** `choir.cad_drawing` (external key, the head) and
  `choir.cad_drawing_revision` (content hash, versioned; body: the
  `schema.js` drawing JSON, compressed as their `.cad` already is), linked by
  the existing `document_revision` and `revision_parent` edges. **Blocks**
  become their own content-hashed objects, `choir.cad_block`, and drawing
  revisions point to them with `references`. A title block, door symbol or
  fastener symbol then becomes a reusable library object shared across
  drawings, and it is publishable.
- **Mapping:**
  - *Clean:* their `Drawing` is already transactional with immutable entities
    and emits `{ids, tables, header, blocks}` change records, so one committed
    transaction (or a batch at save or idle) becomes one revision. The
    change record is a natural event payload.
  - *Not clean:* individual entities. Drawings hold tens of thousands of
    entities in R-tree-indexed chunks, so per-entity objects would be absurd.
    Entities stay in the revision body, addressed by `<revision>#<id>`.
  - Layers, linetypes and styles stay in the body too, except shared standard
    tables, which could be content-hashed objects like blocks.
- **Versioning for free:** revision history replaces their 500-step
  in-memory undo across sessions. Variants are `derived_from` lineages, as for
  parametric.
- **Links:**
  - a drawing `derived_from` a parametric part revision (2D projection);
  - an imported DXF stored as `choir.artifact_blob` that the first revision is
    `derived_from`;
  - Texture documents `transclude` SVG/PDF exports;
  - a floor plan `cites` research captures (site survey, regulations).
- **Published vs private:** drawings stay private. Block libraries and
  selected drawing revisions (with PDF/DXF artifact manifests) can be
  published through the publication layer, so other computers adopt the
  blocks into their own libraries.

### Priority: **low** (take DXF I/O as a library: **medium**)
Parametric already makes 2D drawing sheets, so a standalone 2D drafting desk
adds little in the short term. **Effort:** lifting DXF I/O plus the geom core
as a module takes about 3–5 days. A full Choir 2D drafting app takes 3–4 weeks.

---

## 3. circuits (schematic capture and PCB layout, Altium/KiCad-style)

### What it is, stack, size
- Schematic capture (multi-sheet, ERC, netlist) plus PCB layout (routing,
  vias, 2/4/6 layers, copper pours with thermals, DRC, a grid autorouter, a 3D
  board view), with Gerber X2, Excellon, BOM, pick-and-place, netlist and
  fabrication-zip outputs, and **KiCad import** (`.kicad_sch`, `.kicad_pcb`,
  `.kicad_pro`).
- Vanilla JS and WebGL2 (SDF-instanced shapes), ~25.7k lines of JS, plus a
  **Rust kernel compiled to wasm** (`wasm/kernel/src/*.rs`, 2.3k lines) for
  pours, DRC and autorouting. Rust dependencies: `i_overlay` 9.0.1 (MIT, polygon
  boolean ops) and its `i_*` siblings, plus `libm`. JS dependencies: earcut,
  fflate, rbush, lucide. All permissive.

### Quality
- A real engine, smaller than parametric's. All units are integer nanometres.
  `src/core/store.js` is an immutable object store with an R-tree, transactions
  and undo. `src/model/connectivity.js` builds nets with a union-find over
  wires, pins, labels and ports across sheets. `src/model/sync.js` turns
  schematic-to-board updates into a reviewable change list, a good pattern for
  agent-proposed changes. The footprint generators
  (`src/model/library/footprints.js`) are parametric (SOIC, QFN, QFP, DIP,
  chip, SOT, headers). The library is original, not copied from KiCad's
  CC-BY-SA libraries.
- The autorouter (`route.rs`) is a basic grid A* with an 8-direction MST per
  net and a via cost. That is a demo-class router, not a production one.
- Tests: about 220 JS cases (KiCad import, exporters, library, kernel, render)
  plus about 17 Rust `#[test]`s.
- **Supply-chain flag:** `src/kernel/kernel.wasm` (470 KB) is **committed
  prebuilt**. Nothing shows it matches the Rust source. If Choir takes this,
  rebuild it from source; never ship the committed binary.

### Worth taking
- **Gerber X2 and Excellon writers** (`src/io/gerber.js`, `excellon.js`) and
  the **KiCad s-expression importer** (`src/io/kicad/`).
- **Netlist connectivity** and the **schematic-to-board change list**.
- **Footprint wizards.**
- The project data model (`src/model/schema.js`): one JSON project embedding
  the component definitions it uses, so it is reproducible.

### App-agent design
- State: a `circuits-project` document (schematic sheets, board, embedded
  library) with per-transaction events; Gerbers are derived artifacts.
- Verbs: `place_part`, `connect` (pin-to-pin or to a net label), `set_value`,
  `run_erc`, `sync_to_board` (returns the change list for approval), `route`
  (net or all), `pour`, `run_drc`, `export_fab`, `import_kicad`,
  `create_component_from_datasheet`.
- Other desks: research is the main one: datasheet fetching, pinouts, part
  selection, availability, all as cited evidence. Engineering for firmware.
  Texture for design-review documents.
- Agent advantage: high in principle, because netlists are text-shaped (the
  SKiDL/atopile insight) and datasheet-to-symbol-and-footprint is tedious work
  agents can own. But the hard part (placement and routing quality) is not
  solved here.

### State as typed objects in the object graph (owner addition)

- **Types:**
  - `choir.circuit_project` (external key) and `choir.circuit_project_revision`
    (content hash, versioned; body: the schematic sheets and board documents
    of the `circuits-project` JSON);
  - **`choir.circuit_component`** (content hash; body: symbol, footprint,
    pin map, 3D body, params);
  - **`choir.circuit_netlist`** (content hash; derived, `derived_from` the
    revision);
  - fabrication outputs (Gerber, Excellon, BOM, pick-and-place zip) as
    `choir.artifact_manifest` plus blobs.
- **Mapping:** this is the cleanest fit in the group.
  - Their project **embeds every component definition** so it stays
    reproducible ("editing the library never silently changes a finished
    board"). Content-hashed component objects give that property directly:
    the revision `references` exact component hashes, and the library is just
    the set of component objects the owner has. No copying is needed.
  - Schematic sheets and the board are separate `ObjectStore`s in their code.
    They could become separate revisioned objects, but one project revision
    keeps schematic and board sync atomic, so I would keep one body.
  - The netlist should be a derived object: other agents (research for part
    selection, an inventory app, a BOM costing spreadsheet) query nets and
    parts without loading the board.
- **Versioning for free:** revision history replaces their "earlier versions
  kept on every save". Their `sync.js` schematic-to-board **change list** is
  exactly a proposed revision that an owner or agent accepts, so it maps onto
  a precommitment, not a direct write. Variants are `derived_from` lineages
  (for example "the same board with the cheaper LDO").
- **Links:**
  - a component `cites` the research `choir.web_capture` of its datasheet;
  - BOM lines `reference` component objects, and inventory or invoicing
    lines in other apps `reference` the same components;
  - the board outline links to a parametric enclosure part (`derived_from`
    either way);
  - Texture design reviews `transclude` schematic SVGs;
  - KiCad imports are `artifact_blob`s that the first revision is
    `derived_from`.
- **Published vs private:** projects stay private. **Component objects are
  the high-value publishable unit**: a verified symbol, footprint and 3D body
  with its datasheet citation, adopted by other computers, which avoids the
  KiCad-library licensing question by being one's own work. Open-hardware
  releases (a project revision plus fab manifest) go through the publication
  layer with owner consent.

### Priority: **low**
A niche audience, and the hard engine parts are demo-grade. Take the
exporters and the KiCad importer later if electronics becomes a use case.
**Effort:** a schematic and netlist agent desk with KiCad import/export takes
2–3 weeks. Real PCB layout is months.

---

## 4. building-design (Revit/ArchiCAD-style BIM)

**Empty placeholder.** The directory holds only `icon.png` and `icon-128.png`,
with no source, README or package.json. The root `README.md` lists it as
"Building Design: Autodesk Revit, ArchiCAD, Vectorworks", with "Demo URL not
yet provided". **Nothing to take. Priority: n/a.** If BIM ever matters, it
would be a parametric-desk variant over IFC (e.g. web-ifc/IfcOpenShell). This
repo offers no help there. Object-graph note for that future: IFC is already
an object graph (GUID-identified elements with typed relations). It would map
onto a building head and revision object, with storeys, spaces or elements as
objects only where other apps need to reference them, for example a space in
a property-management lease, or a wall type in a cost spreadsheet.

---

## 5. api-client (Postman/Insomnia-style HTTP client)

### What it is, stack, size
- HTTP, GraphQL and WebSocket request builder with collections, environments
  and variables, auth (Bearer, Basic, API key, OAuth2 client-credentials and
  password grants, AWS SigV4), pre-request and test scripts, a collection
  runner with data files, history, cookies, import/export, and code generation
  for 17 targets.
- Vanilla JS, ~34.4k lines. Dependencies: CodeMirror 6 and Lezer (MIT),
  `yaml` (ISC), `lucide` (ISC). All permissive.

### Quality
- The most thoroughly tested app in the group: about 790 test cases (core,
  curl, codegen, http1, websocket, scripts, import, app and dialog DOM tests),
  plus Playwright e2e.
- Real pieces:
  - `src/core/http1.js`: a raw HTTP/1.1 client over Puter's socket relay. It
    sends any header, does not follow redirects on its own, and keeps response
    headers in order.
  - `src/core/curl.js`: a cURL parser.
  - `src/core/io/`: importers for Postman v2.0/2.1, OpenAPI 3, Swagger 2, HAR
    and Insomnia v4.
  - `src/core/codegen.js`: the 17 code generators.
  - `src/core/aws-sigv4.js`: AWS signing.
- The script sandbox (`src/core/scripts/sandbox.js`) is well designed: a
  sandboxed opaque-origin iframe, plus a per-run Blob Worker so runaway scripts
  can be terminated, plus a CSP with `connect-src 'none'`, so the network is
  reachable only through a host-mediated `send()`.
- The README claim that "TLS runs in your browser" depends on Puter's
  `puter.net.tls.TLSSocket` and cannot be verified from this repo.

### Worth taking
- **Importers** (OpenAPI, Postman, HAR, Insomnia, cURL) and **code
  generation**: self-contained, DOM-free, well tested.
- **The sandbox pattern**, for any Choir surface that runs user or agent JS in
  the browser.
- The workspace data model (`src/core/model.js`: collection, folder, request,
  environment, variable), which is plain JSON.

### App-agent design
- State: collections and environments as computer documents. Request and
  response exchanges are event or evidence records (status, timing, hashed
  bodies). Secrets go in Choir's credential authority, not in variables.
- Verbs: `import_spec` (OpenAPI → collection), `send` (through the gateway),
  `run_collection` (as an acceptance probe), `assert`, `generate_client
  {lang}`, `diff_responses`, `record_as_evidence`.
- Other desks: research (call public APIs as cited evidence); engineering
  ("collection as acceptance test" for deployed services, which fits Choir's
  staging-acceptance doctrine); management (scheduled health probes).
- **Authority flag:** an agent that can send arbitrary requests with arbitrary
  headers, cookies and auth is an egress, SSRF and credential surface. In
  Choir terms that is red-class (gateway/provider calls). Every send must go
  through the gateway's egress policy, never a raw socket relay.
- Agent advantage: medium. The agent writes requests and tests from an OpenAPI
  spec and keeps responses as evidence.

### State as typed objects in the object graph (owner addition)

- **Types:**
  - `choir.api_collection` (external key) and `choir.api_collection_revision`
    (content hash, versioned; body: the folder and request tree from
    `core/model.js`);
  - `choir.api_environment` (external key; variables **without secret
    values**);
  - **reuse `choir.web_capture` for exchanges** where possible. A response from
    a public API is a capture: URL, fetch time, and body as a content blob.
    Request metadata (method, redacted headers, status, timing) goes in
    capture metadata.
  - Only exchanges that are not captures (authenticated, private-service
    calls) would need a new kind, `choir.api_exchange`, and those stay
    private.
- **Mapping:**
  - *Clean:* collections are a JSON tree with auth and script inheritance
    from parents, so one revision per collection keeps inheritance coherent.
    Requests are addressed as `<revision>#<request id>` rather than as
    objects.
  - *Not clean:*
    - **secrets:** their variables carry `secret` values. In Choir those must
      be handles into the credential authority, never object bodies, because
      Dolt history would keep them forever.
    - **cookies and history:** per-device data, not shared state.
- **Versioning for free:** collection history and diffs ("what changed in
  the acceptance suite"). Variants are `derived_from` lineages (staging and
  production collections).
- **Links:**
  - a collection `derived_from` the `choir.web_capture` of the OpenAPI spec it
    was imported from;
  - collection-runner results become `choir.verifier_attestation` objects that
    `attests_to` a deploy or run, fitting Choir's acceptance-proof doctrine;
  - captures of API responses feed research evidence, and spreadsheet ranges
    and Texture paragraphs `cite` them.
- **Published vs private:** environments, private exchanges, cookies and
  history stay private. Collections can be published with secrets stripped
  and private hosts redacted, enforced by `publication_policy`. Public-API
  captures enter the Autopaper object graph through the normal capture and
  publication path like any other source.

### Priority: **medium-low**
Useful as an engineering and research tool and as an acceptance-probe surface,
but not a flagship. **Effort:** a Choir desk that reuses the importers and
codegen, with gateway-mediated send, takes about 1.5–2 weeks. A full Postman
UI is another 2–3 weeks.

---

## Group summary

**Top pick: parametric.** It has a real B-rep engine (OCCT via replicad), an
original constraint solver, a clean declarative JSON feature-history model with
expression-driven variables and stable face and edge naming, and an AI tool
contract (rebuild, verify, auto-undo on failure, one undo step per message)
that is close to a ready-made desk verb set. The kernel already runs headless
in Node. For the owner's agent-operated CAD this is the right base: the
feature history is the agent's language, replicad code can be an escape-hatch
feature type, and humans keep direct manipulation.

**Reusable pieces across apps:**
- The **transactional immutable-object store with undo and change events**.
  It appears three times in near-identical form: `cad/src/core/drawing.js`,
  `circuits/src/core/store.js`, and `parametric/src/model/document.ts`
  (snapshot-based). This is the natural seam for Choir: its change events
  become ledger events, which gives one pattern for every app desk.
- **File formats:** DXF R2000 reader/writer (cad), STEP/STL via OCCT
  (parametric), Gerber X2 and Excellon plus the KiCad importer (circuits),
  OpenAPI, Postman, HAR, Insomnia and cURL importers plus codegen (api-client).
- **The sandboxed script runner** (api-client) for running agent or user code
  in the browser.
- **The assistant pattern** (parametric `src/assistant/`): a host interface
  that is satisfiable headless, every change verified by rebuild, a part
  snapshot sent with each message, and "names and attachments are data, not
  instructions". It is a template for every app agent.

**The object-graph pattern for all four apps (owner addition):**
- **Head and revision objects.** Each app document is a stable head object
  plus content-addressed revision objects, the existing
  `texture_document`/`texture_revision` pattern. One revision is written per
  accepted transaction. In-app fine-grained undo stays local.
- **Content-hashed shared parts are the unit of reuse.** Leaf objects
  (`cad_block`, `circuit_component`) are what other apps reference, and they
  are what gets published to the Autopaper object graph.
- **Derived outputs use existing kinds.** STEP, Gerber and PDF become
  existing `artifact_blob`/`artifact_manifest` objects.
- **Cross-app links use existing edge kinds**: `references`, `cites`,
  `derived_from`, `transcludes`, `was_generated_by`.
- **New kinds needed, at most:** `cad_part(_revision)`,
  `cad_drawing(_revision)`, `cad_block`, `circuit_project(_revision)`,
  `circuit_component`, `circuit_netlist`, `api_collection(_revision)`,
  `api_environment`.
- **Gap in Choir:** `objectgraph` exposes no Dolt branches today. Variants are
  modeled as `derived_from` revision lineages until branches are exposed.
- **Rule:** secrets never go in object bodies, because Dolt history is
  permanent.

**Alarms:**
- **LGPL-2.1 OCCT wasm** in parametric. It is fine when shipped as a
  separately replaceable module, as they do; keep it that way.
- **SolidWorks/Parasolid importer** (`parametric/src/formats/sldprt/`): reverse
  engineered, and it bundles a Siemens-origin Parasolid schema file relabelled
  as MIT. Exclude it.
- **Committed prebuilt `circuits/src/kernel/kernel.wasm`**: unverifiable
  binary. Rebuild from source if taken.
- **api-client egress:** an arbitrary-header HTTP sender is a red-class
  authority surface in Choir.
- building-design is an empty placeholder.
- All of this is AI-generated in one style. It is good, not slop, but untested
  by me (no execution). Before adoption, run the parametric kernel tests in a
  capsule.
