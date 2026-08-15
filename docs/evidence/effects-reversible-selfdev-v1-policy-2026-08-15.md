# reversible-selfdev-v1 — staging rehearsal policy freeze

**Define sub-slice.** Not implementation. Effects remain OFF.
**Parent freeze:** `docs/evidence/effects-decision-policy-schema-2026-08-15.md` (ACCEPT 2026-08-15).
**policy_id:** `reversible-selfdev-v1`
**policy_digest:** `c34ddf073aecaacc307f375d6f2e398798350d7a48c8d3c2e7c6d10248b394d7`
**Canonical encoding:** UTF-8 JSON, object keys sorted, compact separators `,` `:`, no `policy_digest` field inside the hashed bytes (digest is of the policy object below).

This is the first complete policy document. Independent review is required before any red mutation. Implement must not invent a different quorum, roster, or bound.

Irreversible email is **not** this document. `reversible-selfdev-v1` must refuse an irreversible subject.

## Frozen policy object

```json
{
  "actuator": "selfdev_materializer",
  "admissible_evidence": [
    "capsule_execution_receipt",
    "test_receipt",
    "runtime_artifact_ref",
    "source_tree_ref",
    "build_recipe_ref",
    "dependency_toolchain_refs",
    "policy_selection_receipt",
    "ballot_attestation"
  ],
  "blast_radius": [
    "this_computer_vm_local_dolt_solitaire_tables",
    "this_computer_release_pointer"
  ],
  "blast_radius_out": [
    "platform_store",
    "cycle_state",
    "other_computers",
    "trusted_outbox",
    "host_frontend_current"
  ],
  "budget": {
    "currency": 0,
    "external_sends": 0,
    "promotions_of_this_subject": 1
  },
  "capabilities": [
    "selfdev.propose",
    "selfdev.promote",
    "selfdev.restore"
  ],
  "consequence_receipt": [
    "pre_promotion_checkpoint_identity",
    "restore_receipt_if_reverted"
  ],
  "dissent": {
    "policy_blocking_unresolved": "refuse",
    "recorded_dissent_allowed": true
  },
  "effect_class": "reversible_computer_local",
  "eligible_seats": [
    {
      "counts_toward_quorum": false,
      "eligibility": "the CoSuper assigned to author the CapsuleEffectBundle under review",
      "independence_domain": "authoring",
      "kind": "agent_profile",
      "profile": "CoSuper",
      "recused_from": [
        "verification"
      ],
      "seat_id": "cosuper-author"
    },
    {
      "counts_toward_quorum": true,
      "eligibility": "verifier refs bound to this operation's capsule-exec BuildRecipeRef, TestReceipts, and RuntimeArtifactRef",
      "independence_domain": "verification",
      "kind": "independent_verifier",
      "profile": "capsule_exec_receipts",
      "recused_from": [],
      "seat_id": "capsule-verifier"
    },
    {
      "counts_toward_quorum": true,
      "eligibility": "an agent profile that did not author the bundle; same signer as cosuper-author is refuse",
      "independence_domain": "verification",
      "kind": "agent_profile",
      "profile": "not_authoring_CoSuper",
      "recused_from": [],
      "seat_id": "independent-reviewer"
    }
  ],
  "expiry": "this document is the staging rehearsal freeze; expired if superseded by a later policy_digest",
  "forbidden_capabilities": [
    "outbox.send",
    "email.send",
    "publish",
    "pay",
    "platform.store.write",
    "cycle.write"
  ],
  "human_seat": "absent",
  "inadmissible_evidence": [
    "owner_recovery_checkpoint",
    "model_panel_output_without_ballot_attestation",
    "raw_panel_output_as_event"
  ],
  "independence_domains": [
    "authoring",
    "verification"
  ],
  "owner_revocation": true,
  "policy_id": "reversible-selfdev-v1",
  "privacy": "owner",
  "quorum": {
    "abstention_counts_against_quorum": true,
    "global_accept_minimum": 2,
    "per_domain": {
      "authoring": {
        "accept_minimum": 0,
        "required_present": false
      },
      "verification": {
        "accept_minimum": 2,
        "required_present": true
      }
    },
    "weighting": "equal"
  },
  "recovery": "tape_recovery_restore_to_checkpoint_taken_before_A_promoted",
  "recusal": "author of the subject is recused from verification; silent drop of a required verification seat is refuse",
  "replacement": {
    "bench": [],
    "empty_bench_on_required_seat_loss": "refuse"
  },
  "scope": "computer_local_user_computer",
  "subject_binding": [
    "computer_id",
    "operation_id",
    "bundle_digest",
    "desired_event_head",
    "effective_event_head",
    "pending_transition_ref",
    "desired_state_commitment",
    "effective_state_commitment"
  ],
  "timeout": {
    "clock": "canonical_utc",
    "decision_window": "PT30M"
  }
}
```

## Compact canonical bytes (the digest input)

```text
{"actuator":"selfdev_materializer","admissible_evidence":["capsule_execution_receipt","test_receipt","runtime_artifact_ref","source_tree_ref","build_recipe_ref","dependency_toolchain_refs","policy_selection_receipt","ballot_attestation"],"blast_radius":["this_computer_vm_local_dolt_solitaire_tables","this_computer_release_pointer"],"blast_radius_out":["platform_store","cycle_state","other_computers","trusted_outbox","host_frontend_current"],"budget":{"currency":0,"external_sends":0,"promotions_of_this_subject":1},"capabilities":["selfdev.propose","selfdev.promote","selfdev.restore"],"consequence_receipt":["pre_promotion_checkpoint_identity","restore_receipt_if_reverted"],"dissent":{"policy_blocking_unresolved":"refuse","recorded_dissent_allowed":true},"effect_class":"reversible_computer_local","eligible_seats":[{"counts_toward_quorum":false,"eligibility":"the CoSuper assigned to author the CapsuleEffectBundle under review","independence_domain":"authoring","kind":"agent_profile","profile":"CoSuper","recused_from":["verification"],"seat_id":"cosuper-author"},{"counts_toward_quorum":true,"eligibility":"verifier refs bound to this operation's capsule-exec BuildRecipeRef, TestReceipts, and RuntimeArtifactRef","independence_domain":"verification","kind":"independent_verifier","profile":"capsule_exec_receipts","recused_from":[],"seat_id":"capsule-verifier"},{"counts_toward_quorum":true,"eligibility":"an agent profile that did not author the bundle; same signer as cosuper-author is refuse","independence_domain":"verification","kind":"agent_profile","profile":"not_authoring_CoSuper","recused_from":[],"seat_id":"independent-reviewer"}],"expiry":"this document is the staging rehearsal freeze; expired if superseded by a later policy_digest","forbidden_capabilities":["outbox.send","email.send","publish","pay","platform.store.write","cycle.write"],"human_seat":"absent","inadmissible_evidence":["owner_recovery_checkpoint","model_panel_output_without_ballot_attestation","raw_panel_output_as_event"],"independence_domains":["authoring","verification"],"owner_revocation":true,"policy_id":"reversible-selfdev-v1","privacy":"owner","quorum":{"abstention_counts_against_quorum":true,"global_accept_minimum":2,"per_domain":{"authoring":{"accept_minimum":0,"required_present":false},"verification":{"accept_minimum":2,"required_present":true}},"weighting":"equal"},"recovery":"tape_recovery_restore_to_checkpoint_taken_before_A_promoted","recusal":"author of the subject is recused from verification; silent drop of a required verification seat is refuse","replacement":{"bench":[],"empty_bench_on_required_seat_loss":"refuse"},"scope":"computer_local_user_computer","subject_binding":["computer_id","operation_id","bundle_digest","desired_event_head","effective_event_head","pending_transition_ref","desired_state_commitment","effective_state_commitment"],"timeout":{"clock":"canonical_utc","decision_window":"PT30M"}}
```

## How to verify the digest

```text
python3 -c "import hashlib,pathlib,json,re; t=pathlib.Path('docs/evidence/effects-reversible-selfdev-v1-policy-2026-08-15.md').read_text(); c=t.split('```text',1)[1].split('```',1)[0].strip(); print(hashlib.sha256(c.encode()).hexdigest())"
```

Expected: `c34ddf073aecaacc307f375d6f2e398798350d7a48c8d3c2e7c6d10248b394d7`

## Quorum in prose

- Two verification seats must **accept**: `capsule-verifier` and `independent-reviewer`.
- Authoring CoSuper is recused from verification and does **not** count toward quorum.
- Empty replacement bench: loss of a required verification seat fails closed.
- Abstention counts against quorum.
- Unresolved policy-blocking dissent fails closed.
- Decision window 30 minutes, canonical UTC.

## Bounds in prose

- Capabilities: propose / promote / restore only. Send, publish, pay, platform-store write, cycle write are forbidden.
- Scope: this user computer. Platform and cycle OUT.
- Budget: 0 external sends, 0 currency, at most one promotion of this exact subject.
- Privacy: owner.
- Blast radius: this computer's solitaire tables and this computer's release pointer. Not other computers, not host `frontend-current`, not outbox.

## Not decided here

- Concrete signer credentials for `independent-reviewer` on staging (eligibility rule is frozen; the credential binding is an implement-time join to that rule, not a new quorum).
- Canonical encoding for BallotAttestation / PolicySelectionReceipt (schema freeze already requires self-excluding SHA-256; JSON sorted-keys compact is the encoding for **this policy document**).
- `irreversible-email-v1` bytes (later slice).
