# Switching models without a deploy (2026-11-09)

Status: design, not built. Class of the design: green. It is a build order
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

## 3. The design

### 3.1 A computer can define a pool (slice M1)

The policy file gains pool sections:

    [pools.default]
    members = ["devin/swe-2-flash", "opencode-go/iceman-contributor-free",
               "bedrock/claude-haiku-5-5"]

The first member leads and takes every thread; the rest are fallbacks in
order. Weights are a later key (`weights = [...]`); one lead and ordered
fallbacks is what every pool is today. A pool the file defines shadows the
platform's pool of the same name on that computer; a new name defines a new
pool. Validation refuses a member the catalog does not list, an empty pool,
and a duplicate. `choir policy set` and its revisions and rollback work
unchanged. One short verb covers the common case:

    choir policy lead devin/swe-2-flash [--pool default]

It reads the file, moves or inserts that member at the front of the pool
(writing the platform's current members into the file first if the file did
not define the pool), and writes it back through the validated path.

### 3.2 The fleet has one pool document (slice M2)

The 10-22 design's host document, cut down to `version`, `parent`,
`sha256`, `pools`, `default_effort`, `made_by` and `reason`. No stages, no
per-class admission, no overrides yet. The gateway keeps it under
`/var/lib/go-choir/model-policy/` as content-addressed files with a
`current` pointer, serves it with the model list, and refuses a member the
compiled catalog does not list. The compiled `Pools()` is version 1 and the
fallback when the directory is empty or unreadable.

A computer resolves a pool in this order: its own file's `[pools.x]`, then
the host document (fetched at run start, cached about a minute, last good
copy kept on disk), then the compiled pools. Run metadata records the
document version that chose the model. The host document never writes a
guest file.

### 3.3 The commands (slice M3)

    choir fleet policy get
    choir fleet policy set --local f --reason "..."
    choir fleet policy lead <provider/model> [--pool default] --reason "..."
    choir fleet policy rollback [--to version]
    choir fleet policy verify [--pool default]

Admitted by the platform operator gate (in build:
[paper-control-cli-2026-11-09.md](paper-control-cli-2026-11-09.md)). `set`
validates, runs one real call through the new lead (the existing verify),
and only then moves the pointer; a lead that cannot answer is refused. A
rollback is a new version that copies an older one. Each change is one line
in the directory's hash-chained log: who, when, the diff, the reason.

Because the paper's file names `pool:default`, one fleet command moves the
paper too, with no key minted from a saved session.

### 3.4 What is deleted

After M2: the silent overwrite at a `platform-policy-version` bump goes
(fleet effort moves to `default_effort` in the document; a guest file is
only ever written by its owner). After a soak: pool edits in `Pools()` stop;
the literal stays only as version 1.

## 4. Is this a switch?

The contract forbids runtime switches that gate a feature. This is not one:
no code path is turned on or off, and every computer runs the same code.
Which model serves a call is an operating choice, like which key is in the
vault, and each change is a recorded act with a named author. Computers
still diverge only by what each one adopts (its own file).

## 5. Ways it could fail

1. A bad fleet document breaks every computer at once. Guard: validation
   against the catalog; a real call through the new lead before the pointer
   moves; the last good copy on each computer; one-command rollback; the
   compiled pools as the floor.
2. The gateway is down. Computers use the last good copy, then the compiled
   pools. Model calls already need the gateway, so no new fate is shared.
3. A computer on an older release meets a member its catalog lacks. It
   treats the member as unknown and skips to the next (today's
   `retired_remapped` path); the gateway is the backstop.
4. A lead change moves every thread to a new model and each loses its cache
   once. The command says so and prints the pool's last-hour call count.
5. A file-defined pool goes stale when the platform retires a member. The
   member is skipped with the visible note the retired-model path writes
   today; the pool does not fail.
6. Two operators write at once. The pointer move is compare-and-swap on the
   parent version; the loser is told to re-read.
7. A member that needs a credential the gateway lacks. Verify fails and the
   set is refused.

## 6. Build order and proof

- **M1** (orange; one review seat): pool sections, validation, `policy
  lead`. Proof on d@e: define a pool, `verify` shows the new lead served the
  call, rollback restores the old lead.
- **M2** (red, gateway and model routing; two seats): the document, the
  gateway route, guest resolution, the version in run metadata. Proof on
  staging: move the default lead with one command, see d@e's next run use
  it without a release, roll back.
- **M3** (red; rides the operator gate): the fleet verbs and the log.
- Later, from the 10-22 design and not needed for the switch: stages and
  admission per computer class, expiring overrides, the catalog as data
  with a probe suite, the vault, the admin page.

## 7. Open questions

- Whether the paper should follow the fleet lead at once or after a soak on
  d@e (the 10-22 design's admission rule). This slice has no stages, so it
  follows at once; an operator can hold the paper by defining
  `[pools.default]` in the paper's own file.
- Whether effort belongs in the fleet document or stays per file only.
