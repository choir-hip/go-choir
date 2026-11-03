# The Claude delegate relay compares a VM id with a computer-id grant list (2026-11-03)

Date: 2026-11-03. Problem 25 of the data system cutover ledger. Mutation
class of the fix: red (gateway credentials and the provider relay), tier 2
review. Problem first; no fix in this commit.

## The problem in one sentence

The gateway's Claude delegate relay looks the calling computer up in its
grant list by the id its bearer authenticates as, which is the VM id, while
the grant list (`GATEWAY_ANTHROPIC_RELAY_IDENTITIES`, fed from
`claudeDelegateComputers` in `nix/node-b.nix`) holds product computer ids,
so no computer ever matches and no computer can delegate.

## Evidence

- `GET /api/runtime/observability?computer_id=...` answers
  `"claude_delegate": {"available": false, "reason": "the gateway withheld
  the delegate: this computer has no Claude delegate grant"}` for
  `computer-2bf3cab1e3ebf3eb24a515c4603cf954` (luna) and
  `computer-a875023a6379c41f6ddb438f02e76a02` (claude). Both are in
  `GATEWAY_ANTHROPIC_RELAY_IDENTITIES` in the running gateway's
  environment (gateway started 16:11:46Z).
- On Node B the gateway identity registry,
  `/var/lib/go-choir/gateway-identities.json`, is keyed by VM id, and each
  entry's `ComputerID` field holds that VM id. Luna's entry is
  `vm-774a03f8…`, issued 16:15:51Z at its boot. (Field names and ids only;
  no token hashes were read.)
- Code: vmctl issues the bearer with `issueGatewayToken(own.VMID)`
  (`internal/vmctl/ownership.go`); `IdentityRegistry.IssueCredential` stores
  that id; `Handler.authenticateBearer` returns it; `AnthropicRelay.grantState`
  and `HandleAnthropicRelay` (`relay.identities[computerID]`) look it up in
  a map of product computer ids. The env comment on
  `AnthropicRelayIdentitiesEnv` still says "the ids vmctl issues bearers
  for", which is the VM id; the deployed list names product ids.

## Effect

Every delegate-capable capsule opens as `ecosystem_proxy` instead of
`claude` mode; no self-development arm can delegate to Claude. Nothing is
widened: the failure is closed.

## Fix direction

Compare the same kind of id on both sides, through the authority that
already knows the VM-to-computer mapping: vmctl names the product computer
id when it asks the gateway to issue (or import) a VM's bearer, the
gateway records it on the identity, and the relay reads the grant, the
budget and the in-flight bound by that id while the bearer, the pin and
every other gateway route keep the VM id. Identities issued before the fix
carry no product computer id and fail closed with a reason that says a
cold boot re-issues the credential.
