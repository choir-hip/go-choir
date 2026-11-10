# Switching models without a deploy (2026-11-09)

Status: design r2, not built (r1 reviewed by Devin and a Sonnet seat, both
"build with changes"; owner rulings of 2026-11-09 folded in; section 8). Class of the design: green. It is a build order
for section 3 of
[provider-vault-and-model-policy-design-2026-10-22.md](../provider-vault-and-model-policy-design-2026-10-22.md)
(the host model policy), which was designed and never built. It does not
replace that design; it cuts the smallest slice that lets the owner switch
models from a command.

Owner, 2026-11-09, after two pool changes in one day each needed a release
(crouton retired; Devin's SWE model made the lead): "then we should improve
the system so we can switch models programmatically".

## 1. What a switch takes today

Read from the code at 3c8f43d0b.

| Change | Path today |
|---|---|
| One role's model or effort, one computer | `choir policy set` (validated, kept revision, rollback). Seconds. |
| Which model leads a pool, one computer | Not possible. A policy file can name a pool or pin one model; it cannot define or reorder a pool (`unknownPool`, `model_policy.go`). A pin loses the pool's fallbacks. |
| Which model leads a pool, every computer | Edit the Go literal `modelcatalog.Pools()`, then a full release. Hours. |
| Effort for a role, every computer | Edit `defaultPolicyText`, bump `platform-policy-version`, release. Overwrites every file not marked owner-edited. |
| A new pool | Code and a release. |
| A model the catalog does not list | Catalog code, then a gateway and a computer release. |

Nothing set once on the host reaches every computer's choice without a
release. The paper's file is owner-edited and names `pool:default` for
every role, so today the only way to move the paper is to change what
`pool:default` means in code.

## 2. The frame

Three things are mixed in one Go file and change at different rates:

- **What a model can do** (the catalog: context, output limit, images, tool
  quirks). It is learned by probing a model and is wrong if guessed.
  It stays in code for now: a wrong entry breaks calls in ways a command
  cannot check.
- **Which models a pool holds and in what order.** This is an operating
  choice that changed twice today. It becomes data.
- **Which pool or model a role uses, and at what effort.** Already data,
  per computer.

So the work is to move pool membership out of the binary, at two scopes:
one computer, and the fleet.

## 3. The design (r2)

### 3.1 One pool schema, one resolver

A pool is the same object wherever it is written, with the fields
`modelcatalog.PoolMember` has today:

    [pools.default]
    members = [
      { model = "devin/swe-2-flash", weight = 1 },
      { model = "opencode-go/iceman-contributor-free", fallback_only = true },
      { model = "bedrock/claude-haiku-5-5", fallback_only = true },
    ]
    failover = true

A plain list of strings is shorthand for "the first member leads with
weight 1, the rest are fallbacks in order". Shadowing a platform pool that
is weighted or has `failover = false` with the shorthand is refused: the
section must say what it means.

One resolver answers "what are pool X's members" for every reader, in this
order: the computer's own file, then the fleet document (if the computer
follows the fleet, 3.3), then the compiled pools. Every reader goes through
it: `Assign`, `Selections`, `ContinuationChain`, `RoleChain`, `unknownPool`
and the overlay parser all read `catalogPools` today
(`model_policy.go:230,258,374,482,794`), and a reader left on the compiled
map would split the answer. The resolver is synchronous and reads a cache;
the fetch (3.2) refreshes the cache beside it and never blocks a run start.

A member the local catalog does not list is refused at write time, and at
resolve time it is dropped from the pool with a diagnostic in the run
metadata. It does not invalidate the file. (r1 said today's
retired-model path does this. It does not: that path covers pinned models
only, and an uncatalogued pool member is called as it is today.)

### 3.2 A computer can define a pool (slice M1)

The policy file accepts pool sections. `choir policy set`, its revisions and
rollback work unchanged. A file with a pool section counts as owner-edited:
the CLI writes the mark, and the platform never rewrites such a file whether
or not the mark is there. One verb covers the common case:

    choir policy lead devin/swe-2-flash [--pool default]

It writes the pool as currently resolved into the file if the file does not
define it, makes that member the weight 1 lead and every other member a
fallback in its existing order, and writes through the validated path. It
refuses a pool with `failover = false` or more than one weighted member
unless `--single-lead` is given, and it refuses a paid lead without
`--allow-paid` (3.5). `choir policy get` shows, for each pool, where its
definition came from: the file, the fleet document and its version, or the
release.

### 3.3 The fleet document (slice M2)

The 10-22 design's host document, cut to: `version`, `parent`, `sha256`,
`pools` (the schema above), `default_effort`, `made_by`, `reason`. The
gateway keeps it under `/var/lib/go-choir/model-policy/` as
content-addressed files with a `current` and a `candidate` pointer, and
serves it with the model list. The compiled `Pools()` is version 1 and the
floor when the directory is empty or unreadable.

**Who follows it.** A computer follows the fleet unless its file says
otherwise:

    [fleet]
    follow = false

Owner, 2026-11-09: "paper model selection should be independent of fleet,
with an option to follow it." So the paper's file carries `follow = false`
and its own pool sections, written once by an operator act before the
first fleet change (`choir paper policy pin`: copy the pools as resolved
now into the file). From then on the paper's models change only by a
command aimed at the paper. `choir paper policy follow` removes the
sections and the line. Any other computer can do the same with `choir
policy pin` and `follow`.

**Effort.** Owner, 2026-11-09: "have reasoning effort set in fleet doc, not
silent overwrite." Effort resolves: the role's value in the computer's
file, then the file's `[defaults]`, then the document's `default_effort`,
then the release's. The platform stops writing effort into files it
creates. The overwrite at a `platform-policy-version` bump is deleted
(`ensureDefaultPolicyFile`): an unmarked file whose text is one of the
platform's own past defaults is read as "no file" and follows the document;
any other file is left as it is and read as the owner's.

**Staging a change.** `set` and `lead` write a candidate. Computers of the
test tier (d@e) read the candidate; everyone else reads `current`.
`promote` moves `current` after the candidate has served a real tool-call
turn on d@e (the existing guest verify with an explicit provider and model,
run through the operator gate; a gateway-side ping is not enough, as the
inkling gate showed). `rollback` is a new version that copies an older one.
A set that removes a pool name some file may reference is refused without
`--drop-pool`: a file naming a missing pool fails to parse today
(`model_policy.go:763-772`).

### 3.4 The commands

    choir policy lead | pin | follow            one computer (owner key)
    choir paper policy lead | pin | follow      a platform computer (operator)
    choir fleet policy get | set | lead | promote | rollback   (operator)

The fleet verbs ship with M2, the smallest set that makes its proof
possible. They are admitted by the platform operator gate and each leaves
one line in the directory's hash-chained log: who, when, the diff, the
reason. Run metadata gains `llm_policy_version` and the pool's source.

### 3.5 Paid members

The catalog marks each member free or metered (it has the prices). Any
command that would make a metered member a weighted lead prints the price
and the pool's last-hour call count and needs `--allow-paid`. This is a
confirmation on an act that spends money, not a setting. Receipt: the
8,315 Synthetic calls of 2026-11-09 that followed a pool edit.

### 3.6 What is deleted

The version-bump overwrite of guest files; effort lines in
platform-written files; pool edits as code changes (the literal stays as
version 1 and the floor).

## 4. Is this a switch?

No code path is turned on or off, and every computer runs the same code.
Which model serves a call is an operating choice, like which key is in the
vault, and each change is a recorded act with an author. The rule's intent
is that what the owner thinks is running is what runs; so the source and
version of every pool is shown by `policy get` and stamped on each run,
and nobody has to read `catalog.go` to know.

## 5. Ways it could fail

1. A bad fleet document breaks many computers at once, with no ring and no
   automatic rollback. Guards: the candidate serves d@e first and must
   complete a tool-call turn; the paper does not follow; each computer
   keeps its last good copy; rollback is one command; the compiled pools
   are the floor. Not built in this slice: automatic revert on a health
   signal (residual; the 10-22 design's auto-demotion).
2. A paid lead routes a fleet's traffic silently. Guard: 3.5.
3. A pool name is dropped and files that name it stop parsing. Guard:
   `--drop-pool`.
4. A reader left on the compiled map gives a second answer. Guard: one
   resolver; a test that greps for other readers of the compiled pools.
5. A file pool goes stale when a member is retired. The member is dropped
   at resolve time with a diagnostic.
6. A shorthand section silently flattens a weighted or no-failover pool.
   Refused (3.1).
7. The gateway is down or slow at a run start. The resolver reads the
   cache; the fetch has a short timeout and never blocks.
8. A lead change moves every thread, and every Texture document, to a new
   model once, and each loses its cache. The command says so.
9. Two operators write at once. Compare-and-swap on the parent version.
10. A computer on an older release meets a member its catalog lacks: it
    drops the member (5); the gateway refuses an uncatalogued call anyway.

## 6. Build order and proof

- **M1** (orange; one seat): the schema, the resolver over file and
  compiled pools, pool sections, `policy lead | pin`, provenance in `policy
  get`. Proof on d@e: lead a different member, the verify call is served by
  it, rollback restores the old lead.
- **M2** (red, gateway and routing; two seats): the document with candidate
  and current, the fetch and cache, `[fleet] follow`, effort resolution and
  the overwrite's deletion, the fleet verbs, the paid guard, the log. Proof
  on staging: pin the paper; set a candidate lead; d@e's next run uses it
  and the paper's does not; promote; an ordinary computer's next run uses
  it; rollback.
- Later, from the 10-22 design: stages and admission by computer class,
  expiring overrides, automatic demotion, the catalog as data with a probe
  suite, the vault, the admin page.

## 7. Open questions

None blocking. Whether d@e alone is enough of a soak before `promote`, or
the owner's computers should read the candidate too.

## 8. Review record (r1)

Devin and a Sonnet seat, both "build with changes". Adopted: one schema and
one resolver for both scopes; weights, fallback flags and failover carried;
a file with pools is never overwritten; the quota chain and every other
reader go through the resolver; the unknown-member claim corrected and a
drop path specified; staged candidate before the fleet; the paid guard;
dropped pool names guarded; a tool-call verify on a real computer; pool
provenance shown; the fleet verbs moved into M2; the example model is one
the catalog lists once the Devin provider lands. Not adopted: cutting
`default_effort` (both seats), overruled by the owner's ruling, with the
file migration it needs now specified in 3.3; automatic revert on a health
signal (Sonnet), deferred and named as a residual in 5.1.
