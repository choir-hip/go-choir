# Paper control from the CLI (2026-11-09)

Status: design, not built. Class of the design: green. It adds a CLI layer
and a build order to
[platform-operator-account-2026-11-06.md](platform-operator-account-2026-11-06.md)
section 5; it does not replace it.

Owner, 2026-11-09: "it's not the paper model policy control mechanism so
much as the key to modify the paper directly. And I think we need to build
a mechanism for the paper's control. And it might be that basically I just
need to have an account that logs into the paper and I can manually do it in
the GUI. But I'd rather not that only. I'd rather it be something that can
be done over the CLI."

## 1. The problem

Today's path is in [the runbook](../runbooks/paper-control.md). The defect
is the authority, not the verbs: the paper's owner is an account nobody can
sign in to, so the only key that can act on the paper is minted by
replaying a saved browser session on one laptop, every 7 days. The owner
cannot act from their own account, in a browser or from a shell.

## 2. The choice

The owner's own account acts on the paper as platform operator (the 11-06
design, already chosen). One authority then serves both surfaces:

- **CLI:** `choir login` as the owner, then `choir paper <verb>`. The CLI
  sends the owner's own key; the proxy gate
  `authorizePlatformComputerAct` admits it for computers in
  `platform_computers`, and each act leaves an operator receipt on the
  paper's tape.
- **GUI:** the same gate admits the owner's browser session, so a later
  operator page (11-06 section 6) needs no second mechanism. The owner does
  not sign in *as* the paper; they act *on* it as operator.

Rejected: making the owner a second owner of the paper (the computer model
has one owner; a platform computer owned by a person's account breaks the
"no person behind it" property the fleet rules lean on), and keeping the
test owner's key with nicer CLI verbs (the laptop session stays the single
point of failure).

## 3. The verbs

| Verb | Route | Slice |
|---|---|---|
| `choir paper status` | `GET /api/platform/operator/computers` (build, vmctl state, last publish, schedules, policy resolve) | O2 |
| `choir paper policy get` | reads `System/model-policy.toml` and the resolve for each role | O2 |
| `choir paper policy set --local f` | validates, keeps the old file as a numbered revision, writes, reads back, prints the diff and the receipt id | O3 |
| `choir paper policy rollback [--to n]` | restores a kept revision | O3 |
| `choir paper policy verify [--role r]` | the real-call verify | O3 |
| `choir paper schedule list/start/stop` | the source schedules | O4 |
| `choir paper restart --reason` | a planned, marker-backed refresh | O5 |
| `choir paper key --ttl 2h` | a short-lived bearer for a script | O5 |

`paper` is a name for "the platform computer the roster calls the paper";
`--computer <id>` reaches any other platform computer. The policy verbs
also work with an ordinary owner key on the owner's own computers, which is
how they are proved on d@e before the operator gate exists.

Server pieces the verbs need, all in O3: a validate step on the policy
write (parse, known roles, known pools; refuse rather than fall back
silently), the previous file kept as a revision beside it, and a
`platform.computer.model_policy` record on the tape.

## 4. Build order

1. Policy verbs on the owner-key path, with validation and kept revisions
   (orange; proved on d@e). Useful at once: the runbook's six steps become
   one command, still with the test owner's key.
2. O2 then O3 behind the operator gate (orange, provider routing). From
   here the owner's own login controls the paper's policy.
3. O4, O5, then retire `qa_renew_key.mjs` for the paper and delete the
   saved session (red for O5).

## 5. What needs the owner's hands

One thing, once: the bootstrap in 11-06 section 4.1 (naming the operator
account in the `CHOIR_PLATFORM_OPERATORS` secret; it is empty today). The
build of step 1 needs nothing.
