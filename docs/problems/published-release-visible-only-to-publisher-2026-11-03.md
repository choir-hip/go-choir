# A published package release is visible only to its publisher (2026-11-03)

Status: **open** (documented before the fix). Mutation class of the fix:
**red** (visibility and authority of shared data), review tier 2.

## The owner's ruling

Owner, 2026-11-03 about 21:10Z: "published releases mean published, for
everyone to see. thats the whole point of publishing to the library. no
such thing as published only to the computer that made it. oxymoron."
And: "adopting should be optional. its an app store model. except when
platform pushes out a new base image, that gets pushed everywhere."
Later the same evening: the Library UI says "install", not "adopt".

## What the code does today

- `choir package publish` lists a release with `scopes: ["owner"]` and
  `shared_with: {owners: [], computers: []}`
  (`internal/platform/packages.go`, `publishPackage`). The owner-act
  receipt binds `scopes: "owner"` (`PackagePublishSubject`).
- `pkgrelease.Listing.VisibleTo` admits the track's owner, named owners
  of a `shared` listing, and everyone for `public`. So a published release
  is visible to its publisher only.
- Other owners see a release only after a second owner act, `choir package
  share --owners … --confirm-paths …` (PATCH
  `/api/catalog/records/<id>/listing`), which the proxy gates on the named
  computer's `packages.share` setting (off by default) through the guest's
  `/packages/share-gate` route. The paths preview and the second secret
  scan run at share time, not at publish.
- corpusd has a host switch, `CHOIR_PACKAGE_SHARE_READS=owner`, that reads
  every listing at owner scope.
- Another owner's catalog finds tracks through the `widened` lookup key,
  which only shared or public listing versions carry.

Evidence: tonight's releases were shared among four test accounts with
`choir package share`; without that step the other three accounts' catalogs
did not list them.

## Why it is wrong

Publishing to the Library is the act of making a release available to
everyone. A release published "to your own catalog" is the oxymoron the owner
named. The share act, its setting and its host switch are a second gate on
a step the owner already took; `packages.share` is also a hidden switch,
off by default, against the standing rule that nothing ships off.

## The intended model

1. Publishing is the one act that makes a release public: every signed-in
   owner sees it in the catalog (search, history, release detail) and may
   install it. Installing stays optional (app store model).
2. Publish keeps its checks: confinement to the package root, the owner's
   authority (owner act), the secret scan, and a `--confirm-paths` preview
   of everything that becomes public. The confirmation says plainly that
   the release becomes visible to everyone.
3. Share is retired: the act refuses `share_retired` with the reason, the
   CLI command sends nothing and explains, `packages.share`, the guest's
   share gate and `CHOIR_PACKAGE_SHARE_READS` are deleted.
4. Withdraw and reinstate stay: a withdrawn (or suspended or yanked)
   release is its publisher's alone again.
5. Releases published before the ruling become public by a read-time rule:
   a listed release is visible to every signed-in owner whatever its
   stored scope set. No listing version is rewritten. The catalog's lookup
   key moves from `widened` to `record_kind`, and corpusd's boot backfill
   re-keys the existing rows (it rewrites a stale key row and adds a missing
   one).

## What becomes visible to everyone

Every field of a listed release's view, to every signed-in owner:

- the record (content-addressed, so it is served whole and verifies
  against its id): schema; package name and manifest version; track (the
  publisher's owner id); parent (kind, base commit or parent release id,
  tree); parent-patch, tree-patch and review-patch references (sha256 and
  size, not the bytes); source root, tree, changed paths and the base
  commit it was built on; manifest summary (sha256, ABI, SDK, kind and
  open counts); publisher owner id and computer id; operation id, bundle
  digest, decision event, materialization receipt, checkpoint ref and
  verifier refs; precommitments (record id, kind, question text,
  probability, settled outcome, resolving record id); the declared test
  command;
- the view: release id, canonical id (which embeds the owner id),
  published time;
- the listing: record kind and id, package, track, seq, scopes, state,
  state reason (free text the owner typed at withdraw or reinstate),
  state actor (the owner-act receipt digest), version and predecessor
  digest; `shared_with` is blanked for everyone but the publisher;
- the catalog row: package, track, head release id and seq, release count.

Not included: the owner's email or name (an owner id is a random UUID),
the patch bytes (no read route serves them), screenshots, captures or
evidence assets (a record carries none).

Fields to watch:

- the precommitment questions and the test command are free text the
  publisher wrote; they become public with the record. The publish preview
  now lists them beside the paths. Trimming them from the public view
  would break the record's verification against its id, so they stay and
  are shown before the owner confirms;
- the owner id and computer id are stable pseudonymous identifiers that
  link an owner's releases together. Neither grants access to anything;
  the computer routes check ownership. Kept;
- the operation's receipt refs point into the publisher's own tape and
  are unreadable to anyone else. Kept, as provenance.

## Remaining error

- The catalog now pages every listing version on the cloud on each
  catalog read (one keyed seek, then the rows). Fine at tonight's scale;
  a catalog index of track heads is the fix when it is not.
- Installing another owner's release is not built yet (S5); "installable
  by anyone" is the authority this change grants, not a working path.
