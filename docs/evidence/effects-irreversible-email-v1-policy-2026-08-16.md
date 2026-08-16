# irreversible-email-v1 and human-required-v1 — staging rehearsal policy freeze

**Define sub-slice.** Not implementation. Effects remain OFF. No outbox is wired.
**Parent freeze:** `docs/evidence/effects-decision-policy-schema-2026-08-15.md` (ACCEPT 2026-08-15).
**Parent reducer:** `docs/evidence/effects-decision-policy-reducer-2026-08-16.md` (`a5f8bdcc`).
**Canonical encoding:** UTF-8 JSON, object keys sorted, compact separators `,` `:`, digest is of the compact policy object (no `policy_digest` field inside the hashed bytes).

Independent review is required before any red outbox mutation. Implement must not invent a different quorum, roster, recipient-binding, or dispatch contract.

`reversible-selfdev-v1` must refuse these subjects. Restore does not unsend.

## irreversible-email-v1

**policy_id:** `irreversible-email-v1`
**policy_digest:** `d83c215443638ad0cfd95eb174de61e1f203414d8ce0631cb68f0dca106c6bbc`
**effect_class:** `irreversible_external`
**human_seat:** `absent` for this acceptance
**actuator:** `trusted_outbox`

### Frozen policy object

```json
{
  "actuator": "trusted_outbox",
  "admissible_evidence": [
    "policy_selection_receipt",
    "ballot_attestation",
    "dispatch_intent_receipt",
    "provider_delivery_receipt",
    "payload_digest"
  ],
  "blast_radius": [
    "trusted_outbox_one_exact_send",
    "owner_controlled_acceptance_inbox"
  ],
  "blast_radius_out": [
    "platform_store",
    "cycle_state",
    "other_computers",
    "host_frontend_current",
    "this_computer_vm_local_dolt_solitaire_tables"
  ],
  "budget": {
    "currency": 0,
    "external_sends": 1,
    "promotions_of_this_subject": 0,
    "sends_of_this_subject": 1
  },
  "capabilities": [
    "outbox.send"
  ],
  "consequence_receipt": [
    "dispatch_intent_receipt",
    "provider_delivery_id",
    "payload_digest",
    "recipient",
    "provider_outcome",
    "uncertain_outcome_reconciliation",
    "crash_window",
    "compensation_intent_if_correction_required"
  ],
  "dispatch_contract": {
    "crash_window": "if process dies after provider acceptance and before consequence persistence, reconciliation must find or compensate the send",
    "dispatch_idempotency_key": "exact-subject dispatch; retries do not double-send",
    "dispatch_intent_receipt": "recorded before the provider call",
    "provider_outcome": [
      "accepted",
      "rejected",
      "unknown"
    ],
    "revocation_check_point": "immediately_before_dispatch",
    "uncertain_outcome_reconciliation": "required when provider outcome is unknown; partial success never greens"
  },
  "dissent": {
    "policy_blocking_unresolved": "refuse",
    "recorded_dissent_allowed": true
  },
  "effect_class": "irreversible_external",
  "eligible_seats": [
    {
      "counts_toward_quorum": false,
      "eligibility": "the CoSuper assigned to author the proposing trajectory under review",
      "independence_domain": "authoring",
      "kind": "agent_profile",
      "profile": "CoSuper",
      "recused_from": [
        "verification",
        "external_effects"
      ],
      "seat_id": "cosuper-author"
    },
    {
      "counts_toward_quorum": true,
      "eligibility": "verifier refs bound to the proposing trajectory capsule-exec receipts",
      "independence_domain": "verification",
      "kind": "independent_verifier",
      "profile": "capsule_exec_receipts",
      "recused_from": [],
      "seat_id": "capsule-verifier"
    },
    {
      "counts_toward_quorum": true,
      "eligibility": "an agent profile that did not author the subject; same signer as cosuper-author is refuse",
      "independence_domain": "verification",
      "kind": "agent_profile",
      "profile": "not_authoring_CoSuper",
      "recused_from": [],
      "seat_id": "independent-reviewer"
    },
    {
      "counts_toward_quorum": true,
      "eligibility": "an independent reviewer in domain external_effects who did not author the subject and is not the verification signer",
      "independence_domain": "external_effects",
      "kind": "independent_verifier",
      "profile": "not_authoring_not_verification_signer",
      "recused_from": [],
      "seat_id": "external-effects-reviewer"
    }
  ],
  "expiry": "this document is the staging rehearsal freeze; expired if superseded by a later policy_digest",
  "forbidden_capabilities": [
    "publish",
    "pay",
    "platform.store.write",
    "cycle.write",
    "selfdev.promote"
  ],
  "human_seat": "absent",
  "inadmissible_evidence": [
    "owner_recovery_checkpoint",
    "model_panel_output_without_ballot_attestation",
    "raw_panel_output_as_event"
  ],
  "independence_domains": [
    "authoring",
    "verification",
    "external_effects"
  ],
  "owner_revocation": true,
  "policy_id": "irreversible-email-v1",
  "privacy": "owner",
  "quorum": {
    "abstention_counts_against_quorum": true,
    "global_accept_minimum": 3,
    "per_domain": {
      "authoring": {
        "accept_minimum": 0,
        "required_present": false
      },
      "external_effects": {
        "accept_minimum": 1,
        "required_present": true
      },
      "verification": {
        "accept_minimum": 2,
        "required_present": true
      }
    },
    "weighting": "equal"
  },
  "recovery": "compensation_or_new_forward_action; restore does not unsend",
  "recusal": "author of the subject is recused from verification and external_effects; silent drop of a required seat is refuse",
  "replacement": {
    "bench": [],
    "empty_bench_on_required_seat_loss": "refuse"
  },
  "scope": "external_one_exact_email",
  "subject_binding": [
    "computer_id",
    "operation_id",
    "bundle_digest",
    "desired_event_head",
    "effective_event_head",
    "pending_transition_ref",
    "desired_state_commitment",
    "effective_state_commitment",
    "recipient",
    "payload_digest",
    "actuator",
    "acceptance_inbox"
  ],
  "timeout": {
    "clock": "canonical_utc",
    "decision_window": "PT60M"
  }
}
```

### Compact canonical bytes (the digest input)

```text
{"actuator":"trusted_outbox","admissible_evidence":["policy_selection_receipt","ballot_attestation","dispatch_intent_receipt","provider_delivery_receipt","payload_digest"],"blast_radius":["trusted_outbox_one_exact_send","owner_controlled_acceptance_inbox"],"blast_radius_out":["platform_store","cycle_state","other_computers","host_frontend_current","this_computer_vm_local_dolt_solitaire_tables"],"budget":{"currency":0,"external_sends":1,"promotions_of_this_subject":0,"sends_of_this_subject":1},"capabilities":["outbox.send"],"consequence_receipt":["dispatch_intent_receipt","provider_delivery_id","payload_digest","recipient","provider_outcome","uncertain_outcome_reconciliation","crash_window","compensation_intent_if_correction_required"],"dispatch_contract":{"crash_window":"if process dies after provider acceptance and before consequence persistence, reconciliation must find or compensate the send","dispatch_idempotency_key":"exact-subject dispatch; retries do not double-send","dispatch_intent_receipt":"recorded before the provider call","provider_outcome":["accepted","rejected","unknown"],"revocation_check_point":"immediately_before_dispatch","uncertain_outcome_reconciliation":"required when provider outcome is unknown; partial success never greens"},"dissent":{"policy_blocking_unresolved":"refuse","recorded_dissent_allowed":true},"effect_class":"irreversible_external","eligible_seats":[{"counts_toward_quorum":false,"eligibility":"the CoSuper assigned to author the proposing trajectory under review","independence_domain":"authoring","kind":"agent_profile","profile":"CoSuper","recused_from":["verification","external_effects"],"seat_id":"cosuper-author"},{"counts_toward_quorum":true,"eligibility":"verifier refs bound to the proposing trajectory capsule-exec receipts","independence_domain":"verification","kind":"independent_verifier","profile":"capsule_exec_receipts","recused_from":[],"seat_id":"capsule-verifier"},{"counts_toward_quorum":true,"eligibility":"an agent profile that did not author the subject; same signer as cosuper-author is refuse","independence_domain":"verification","kind":"agent_profile","profile":"not_authoring_CoSuper","recused_from":[],"seat_id":"independent-reviewer"},{"counts_toward_quorum":true,"eligibility":"an independent reviewer in domain external_effects who did not author the subject and is not the verification signer","independence_domain":"external_effects","kind":"independent_verifier","profile":"not_authoring_not_verification_signer","recused_from":[],"seat_id":"external-effects-reviewer"}],"expiry":"this document is the staging rehearsal freeze; expired if superseded by a later policy_digest","forbidden_capabilities":["publish","pay","platform.store.write","cycle.write","selfdev.promote"],"human_seat":"absent","inadmissible_evidence":["owner_recovery_checkpoint","model_panel_output_without_ballot_attestation","raw_panel_output_as_event"],"independence_domains":["authoring","verification","external_effects"],"owner_revocation":true,"policy_id":"irreversible-email-v1","privacy":"owner","quorum":{"abstention_counts_against_quorum":true,"global_accept_minimum":3,"per_domain":{"authoring":{"accept_minimum":0,"required_present":false},"external_effects":{"accept_minimum":1,"required_present":true},"verification":{"accept_minimum":2,"required_present":true}},"weighting":"equal"},"recovery":"compensation_or_new_forward_action; restore does not unsend","recusal":"author of the subject is recused from verification and external_effects; silent drop of a required seat is refuse","replacement":{"bench":[],"empty_bench_on_required_seat_loss":"refuse"},"scope":"external_one_exact_email","subject_binding":["computer_id","operation_id","bundle_digest","desired_event_head","effective_event_head","pending_transition_ref","desired_state_commitment","effective_state_commitment","recipient","payload_digest","actuator","acceptance_inbox"],"timeout":{"clock":"canonical_utc","decision_window":"PT60M"}}
```

### How to verify the digest

```text
python3 -c "import hashlib,pathlib; t=pathlib.Path('docs/evidence/effects-irreversible-email-v1-policy-2026-08-16.md').read_text(); c=t.split('## irreversible-email-v1')[1].split('```text',1)[1].split('```',1)[0].strip(); print(hashlib.sha256(c.encode()).hexdigest())"
```

Expected: `d83c215443638ad0cfd95eb174de61e1f203414d8ce0631cb68f0dca106c6bbc`

### Quorum in prose (strictly stronger than reversible-selfdev-v1)

- Three counting accepts: `capsule-verifier` and `independent-reviewer` in `verification` (2-of-2) **plus** `external-effects-reviewer` in a third independence domain `external_effects` (1-of-1).
- Authoring CoSuper is recused from verification **and** external_effects and does not count toward quorum.
- Empty replacement bench: loss of a required seat fails closed.
- Abstention counts against quorum.
- Unresolved policy-blocking dissent fails closed.
- Decision window PT60M, canonical UTC (longer than reversible PT30M).

### Bounds in prose

- Capabilities: `outbox.send` only. Promote, publish, pay, platform-store write, cycle write are forbidden.
- Scope: one exact external email. Platform and cycle OUT. Solitaire tables OUT.
- Budget: 1 external send of this exact subject, 0 currency, 0 promotions.
- Privacy: owner.
- Blast radius: the trusted outbox one-exact-send and the owner-controlled acceptance inbox. Not other computers, not host `frontend-current`.
- Dispatch contract is owned by this policy: revocation checked immediately before dispatch; intent receipt recorded before the provider call; idempotent exact-subject key; provider outcome accepted/rejected/unknown; unknown requires reconciliation; crash window must find or compensate; partial success never greens.
- Subject binding includes the reversible identity joins **plus** exact recipient, payload digest, actuator=`trusted_outbox`, and owner-controlled acceptance inbox.

## human-required-v1

**policy_id:** `human-required-v1`
**policy_digest:** `33f5dc442d7fd0d028b76a6792954add090dafcaff87cc2ff1a1829a08df9b96`
**human_seat:** `required`
**purpose:** prove the absent-seat refuse on the same irreversible email subject.

### Frozen policy object

```json
{
  "actuator": "trusted_outbox",
  "admissible_evidence": [
    "policy_selection_receipt",
    "ballot_attestation",
    "dispatch_intent_receipt",
    "provider_delivery_receipt",
    "payload_digest"
  ],
  "blast_radius": [
    "trusted_outbox_one_exact_send",
    "owner_controlled_acceptance_inbox"
  ],
  "blast_radius_out": [
    "platform_store",
    "cycle_state",
    "other_computers",
    "host_frontend_current",
    "this_computer_vm_local_dolt_solitaire_tables"
  ],
  "budget": {
    "currency": 0,
    "external_sends": 1,
    "promotions_of_this_subject": 0,
    "sends_of_this_subject": 1
  },
  "capabilities": [
    "outbox.send"
  ],
  "consequence_receipt": [
    "dispatch_intent_receipt",
    "provider_delivery_id",
    "payload_digest",
    "recipient",
    "provider_outcome",
    "uncertain_outcome_reconciliation",
    "crash_window",
    "compensation_intent_if_correction_required"
  ],
  "dispatch_contract": {
    "crash_window": "if process dies after provider acceptance and before consequence persistence, reconciliation must find or compensate the send",
    "dispatch_idempotency_key": "exact-subject dispatch; retries do not double-send",
    "dispatch_intent_receipt": "recorded before the provider call",
    "provider_outcome": [
      "accepted",
      "rejected",
      "unknown"
    ],
    "revocation_check_point": "immediately_before_dispatch",
    "uncertain_outcome_reconciliation": "required when provider outcome is unknown; partial success never greens"
  },
  "dissent": {
    "policy_blocking_unresolved": "refuse",
    "recorded_dissent_allowed": true
  },
  "effect_class": "irreversible_external",
  "eligible_seats": [
    {
      "counts_toward_quorum": false,
      "eligibility": "the CoSuper assigned to author the proposing trajectory under review",
      "independence_domain": "authoring",
      "kind": "agent_profile",
      "profile": "CoSuper",
      "recused_from": [
        "verification",
        "external_effects"
      ],
      "seat_id": "cosuper-author"
    },
    {
      "counts_toward_quorum": true,
      "eligibility": "verifier refs bound to the proposing trajectory capsule-exec receipts",
      "independence_domain": "verification",
      "kind": "independent_verifier",
      "profile": "capsule_exec_receipts",
      "recused_from": [],
      "seat_id": "capsule-verifier"
    },
    {
      "counts_toward_quorum": true,
      "eligibility": "an agent profile that did not author the subject; same signer as cosuper-author is refuse",
      "independence_domain": "verification",
      "kind": "agent_profile",
      "profile": "not_authoring_CoSuper",
      "recused_from": [],
      "seat_id": "independent-reviewer"
    },
    {
      "counts_toward_quorum": true,
      "eligibility": "an independent reviewer in domain external_effects who did not author the subject and is not the verification signer",
      "independence_domain": "external_effects",
      "kind": "independent_verifier",
      "profile": "not_authoring_not_verification_signer",
      "recused_from": [],
      "seat_id": "external-effects-reviewer"
    },
    {
      "counts_toward_quorum": true,
      "eligibility": "owner human seat named by this policy; absence is refuse",
      "independence_domain": "owner_human",
      "kind": "owner_human",
      "profile": "owner",
      "recused_from": [],
      "seat_id": "owner-human"
    }
  ],
  "expiry": "this document is the staging rehearsal freeze; expired if superseded by a later policy_digest",
  "forbidden_capabilities": [
    "publish",
    "pay",
    "platform.store.write",
    "cycle.write",
    "selfdev.promote"
  ],
  "human_seat": "required",
  "inadmissible_evidence": [
    "owner_recovery_checkpoint",
    "model_panel_output_without_ballot_attestation",
    "raw_panel_output_as_event"
  ],
  "independence_domains": [
    "authoring",
    "external_effects",
    "owner_human",
    "verification"
  ],
  "owner_revocation": true,
  "policy_id": "human-required-v1",
  "privacy": "owner",
  "quorum": {
    "abstention_counts_against_quorum": true,
    "global_accept_minimum": 4,
    "per_domain": {
      "authoring": {
        "accept_minimum": 0,
        "required_present": false
      },
      "external_effects": {
        "accept_minimum": 1,
        "required_present": true
      },
      "owner_human": {
        "accept_minimum": 1,
        "required_present": true
      },
      "verification": {
        "accept_minimum": 2,
        "required_present": true
      }
    },
    "weighting": "equal"
  },
  "recovery": "compensation_or_new_forward_action; restore does not unsend",
  "recusal": "author of the subject is recused from verification and external_effects; silent drop of a required seat is refuse",
  "replacement": {
    "bench": [],
    "empty_bench_on_required_seat_loss": "refuse"
  },
  "scope": "external_one_exact_email",
  "subject_binding": [
    "computer_id",
    "operation_id",
    "bundle_digest",
    "desired_event_head",
    "effective_event_head",
    "pending_transition_ref",
    "desired_state_commitment",
    "effective_state_commitment",
    "recipient",
    "payload_digest",
    "actuator",
    "acceptance_inbox"
  ],
  "timeout": {
    "clock": "canonical_utc",
    "decision_window": "PT60M"
  }
}
```

### Compact canonical bytes (the digest input)

```text
{"actuator":"trusted_outbox","admissible_evidence":["policy_selection_receipt","ballot_attestation","dispatch_intent_receipt","provider_delivery_receipt","payload_digest"],"blast_radius":["trusted_outbox_one_exact_send","owner_controlled_acceptance_inbox"],"blast_radius_out":["platform_store","cycle_state","other_computers","host_frontend_current","this_computer_vm_local_dolt_solitaire_tables"],"budget":{"currency":0,"external_sends":1,"promotions_of_this_subject":0,"sends_of_this_subject":1},"capabilities":["outbox.send"],"consequence_receipt":["dispatch_intent_receipt","provider_delivery_id","payload_digest","recipient","provider_outcome","uncertain_outcome_reconciliation","crash_window","compensation_intent_if_correction_required"],"dispatch_contract":{"crash_window":"if process dies after provider acceptance and before consequence persistence, reconciliation must find or compensate the send","dispatch_idempotency_key":"exact-subject dispatch; retries do not double-send","dispatch_intent_receipt":"recorded before the provider call","provider_outcome":["accepted","rejected","unknown"],"revocation_check_point":"immediately_before_dispatch","uncertain_outcome_reconciliation":"required when provider outcome is unknown; partial success never greens"},"dissent":{"policy_blocking_unresolved":"refuse","recorded_dissent_allowed":true},"effect_class":"irreversible_external","eligible_seats":[{"counts_toward_quorum":false,"eligibility":"the CoSuper assigned to author the proposing trajectory under review","independence_domain":"authoring","kind":"agent_profile","profile":"CoSuper","recused_from":["verification","external_effects"],"seat_id":"cosuper-author"},{"counts_toward_quorum":true,"eligibility":"verifier refs bound to the proposing trajectory capsule-exec receipts","independence_domain":"verification","kind":"independent_verifier","profile":"capsule_exec_receipts","recused_from":[],"seat_id":"capsule-verifier"},{"counts_toward_quorum":true,"eligibility":"an agent profile that did not author the subject; same signer as cosuper-author is refuse","independence_domain":"verification","kind":"agent_profile","profile":"not_authoring_CoSuper","recused_from":[],"seat_id":"independent-reviewer"},{"counts_toward_quorum":true,"eligibility":"an independent reviewer in domain external_effects who did not author the subject and is not the verification signer","independence_domain":"external_effects","kind":"independent_verifier","profile":"not_authoring_not_verification_signer","recused_from":[],"seat_id":"external-effects-reviewer"},{"counts_toward_quorum":true,"eligibility":"owner human seat named by this policy; absence is refuse","independence_domain":"owner_human","kind":"owner_human","profile":"owner","recused_from":[],"seat_id":"owner-human"}],"expiry":"this document is the staging rehearsal freeze; expired if superseded by a later policy_digest","forbidden_capabilities":["publish","pay","platform.store.write","cycle.write","selfdev.promote"],"human_seat":"required","inadmissible_evidence":["owner_recovery_checkpoint","model_panel_output_without_ballot_attestation","raw_panel_output_as_event"],"independence_domains":["authoring","external_effects","owner_human","verification"],"owner_revocation":true,"policy_id":"human-required-v1","privacy":"owner","quorum":{"abstention_counts_against_quorum":true,"global_accept_minimum":4,"per_domain":{"authoring":{"accept_minimum":0,"required_present":false},"external_effects":{"accept_minimum":1,"required_present":true},"owner_human":{"accept_minimum":1,"required_present":true},"verification":{"accept_minimum":2,"required_present":true}},"weighting":"equal"},"recovery":"compensation_or_new_forward_action; restore does not unsend","recusal":"author of the subject is recused from verification and external_effects; silent drop of a required seat is refuse","replacement":{"bench":[],"empty_bench_on_required_seat_loss":"refuse"},"scope":"external_one_exact_email","subject_binding":["computer_id","operation_id","bundle_digest","desired_event_head","effective_event_head","pending_transition_ref","desired_state_commitment","effective_state_commitment","recipient","payload_digest","actuator","acceptance_inbox"],"timeout":{"clock":"canonical_utc","decision_window":"PT60M"}}
```

Expected digest: `33f5dc442d7fd0d028b76a6792954add090dafcaff87cc2ff1a1829a08df9b96`

### Quorum in prose

Same three counting agent/verifier seats as irreversible-email-v1, **plus** `owner-human` in domain `owner_human` with `accept_minimum` 1 and `required_present` true. Global accept minimum is 4. Absence of the human seat fails closed.

## Not decided here

- Concrete owner-controlled acceptance inbox address on staging (eligibility rule is frozen; the address is an implement-time join to that rule, not a new quorum).
- Production trusted-outbox wiring (route map 7 implement, after this freeze is independently reviewed).
- Provider choice and credential binding.

## What this freeze does not do

Does not wire an outbox. Does not send email. Does not delete `external-owner:`. Does not arm effects. Does not rematerialize.
