# GET /internal/platform/captures/<id> serves any capture id to any bound guest

**Found:** 2026-11-24, in review of lane `corpus-repl` (two seats).

**Problem.** `HandleCaptureRead` (`internal/platform/captures.go`) is admitted
with `allowAnyGuest`: any guest vmctl binds to a live computer may read any
capture by id, whatever owner and computer wrote it, through `GetObject`, which
does not check `superseded_by`. The body, URL and labels come back; an
owner-private capture is protected only by the label the host's `ReadCapture`
gate (agentcore, `capture_read_gate.go`) checks after the fetch, in the desk's
own process, not by corpusd.

**Evidence.** The route's code and its comment ("any bound guest reads them
(W1b2)", captures are shared Autopaper content); the proxy carve-out in
`guest_authority.go` for GET on `/internal/platform/captures/`. A capture id
carries its owner and a hash suffix, so it is not guessable; ids reach a guest
through lists and stories.

**Belief.** The design treats captures as shared world content, and the id is
unguessable, so this is a deliberate choice and not a leak a guest can use
without an id. It is also the one place corpusd serves a capture without
checking scope, tombstone, supersession or the owner-private label itself.

**Remaining error.** Whether an id can reach a guest that should not have it
(a published story cites capture ids; a private capture cited by a published
story would be readable by id). Not measured.

**What the corpus verbs do.** The new `CorpusTexts` does not use this route:
corpusd's `/captures/texts` reads under the scope and eligibility of every
corpus list. The older route is unchanged here; narrowing it changes
`choir.ReadCapture` for every desk and is the owner's call.
