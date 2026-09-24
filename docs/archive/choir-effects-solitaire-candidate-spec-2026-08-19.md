# Solitaire Candidate Specification & Decision-Policy Manifest: Reversible Effects Excursion

**Date:** August 19, 2026  
**Author:** Choir Engineering  
**Mission Document:** `docs/definitions/choir-supervised-self-development-effects-2026-08-11.md`  
**Pre-A Checkpoint Baseline:** `99949fe2e16d3c4c446838c0e59517b108accecab7afefd9329c3a6c4a1209f7`  
**Decision Policy:** `reversible-selfdev-v1` (Digest: `c34ddf073aecaacc307f375d6f2e398798350d7a48c8d3c2e7c6d10248b394d7`)

---

## 1. Candidate Overview and Ontology

The Solitaire candidate is a minimal, self-contained reversible source change authored by an autonomous CoSuper capsule. It provides a headless REST API and durable database persistence for Klondike Solitaire.

### 1.1 Non-Negotiable Candidate Invariants
1. **API-Only:** The candidate ships zero browser UI. The computer-surface frontend is per-computer (`C15`/`I25`) and remains outside the updater-controlled release.
2. **Additive-Only Schema:** Database tables are created using `CREATE TABLE IF NOT EXISTS`. No existing platform or runtime tables are altered or dropped.
3. **No Protected Surface Touches:** Solitaire code touches no authentication/session handlers, no event or decision projection paths, no provider/gateway routing, and no Texture documents.
4. **Capsule Authorship:** Every required ref in the `CapsuleEffectBundle` (`SourceTreeRef`, `BuildRecipeRef`, `DependencyToolchainRefs`, `TestReceipts`, and `RuntimeArtifactRef`) binds receipts generated directly from the authoring capsule execution.

---

## 2. Data Model and Schema Definition

Solitaire persistence uses two dedicated tables in the VM-local embedded Dolt workspace:

```sql
CREATE TABLE IF NOT EXISTS solitaire_games (
    game_id         VARCHAR(255) PRIMARY KEY,
    owner_id        VARCHAR(255) NOT NULL,
    computer_id     VARCHAR(255) NOT NULL DEFAULT '',
    state           VARCHAR(64) NOT NULL DEFAULT 'in_progress',
    deck_seed       BIGINT NOT NULL DEFAULT 0,
    tableau_json    LONGTEXT NOT NULL DEFAULT '[]',
    foundations_json LONGTEXT NOT NULL DEFAULT '[]',
    stock_json      LONGTEXT NOT NULL DEFAULT '[]',
    waste_json      LONGTEXT NOT NULL DEFAULT '[]',
    score           INT NOT NULL DEFAULT 0,
    moves_count     INT NOT NULL DEFAULT 0,
    created_at      DATETIME NOT NULL,
    updated_at      DATETIME NOT NULL,
    INDEX idx_solitaire_owner (owner_id, computer_id)
);

CREATE TABLE IF NOT EXISTS solitaire_moves (
    move_id         VARCHAR(255) PRIMARY KEY,
    game_id         VARCHAR(255) NOT NULL,
    owner_id        VARCHAR(255) NOT NULL,
    computer_id     VARCHAR(255) NOT NULL DEFAULT '',
    move_seq        BIGINT NOT NULL,
    move_type       VARCHAR(64) NOT NULL,
    from_pile       VARCHAR(64) NOT NULL,
    to_pile         VARCHAR(64) NOT NULL,
    cards_json      LONGTEXT NOT NULL,
    created_at      DATETIME NOT NULL,
    INDEX idx_solitaire_game_moves (game_id, move_seq)
);
```

---

## 3. Headless Play REST API Specification

The API mounts under `/api/solitaire` on the guest autoputer:

| Method | Endpoint | Description | Request Body | Response Body |
| :--- | :--- | :--- | :--- | :--- |
| `POST` | `/api/solitaire/games` | Initialize and deal a new game | `{"seed": 12345}` | `SolitaireGameState` |
| `GET` | `/api/solitaire/games/{id}` | Query current game state | *None* | `SolitaireGameState` |
| `POST` | `/api/solitaire/games/{id}/moves` | Execute a move | `{"from": "waste", "to": "foundation", "card_count": 1}` | `SolitaireMoveResult` |
| `GET` | `/api/solitaire/games/{id}/history` | Retrieve move history | *None* | `[]SolitaireMove` |

---

## 4. The Correction Spine: Pre-Declared Defect and Falsification

To prove the correction spine rather than a simple gate pass, the candidate lifecycle walks a two-step promotion-falsification sequence:

### 4.1 Candidate A: Pre-Declared Rule Defect
* **Pre-declared Defect:** In Candidate A, foundation validation checks card rank ascending from Ace, but fails to check suit matching (e.g., accepting a 2 of Spades onto an Ace of Hearts).
* **Gate Pass Property:** Candidate A's own capsule unit test suite tests standard legal moves and winning sequences, which all pass. Candidate A passes all local and capsule tests and successfully freezes into Bundle A.
* **Promotion & Real State:** Bundle A promotes under `reversible-selfdev-v1` consensus policy, restarts healthy, and serves the headless API.
* **Falsification:** Admissible post-promotion evidence is submitted: an automated play sequence that plays an off-suit card to foundation. Candidate A's state machine accepts the illegal move, generating falsification proof.

### 4.2 Candidate B: Rule Repair & Supersession
* **Correction:** Candidate B fixes the foundation validator to strictly enforce matching suit (`card.Suit == foundation.Suit && card.Rank == foundation.TopRank + 1`).
* **Supersession:** Candidate B is authored, frozen, and proposed. Its proposal event cites Candidate A's falsification receipt.
* **Verification:** Candidate B promotes. Replaying the falsification sequence is now refused with HTTP 400 (`invalid move: suit mismatch`).

---

## 5. Total Restore Verification

Following successful execution and supersession of Candidate B, the entire excursion is restored back to the pre-A baseline:
1. Issue `choir computer restore --computer computer-03335285269bdba4f94377e56879f9e6 --checkpoint 99949fe2e16d3c4c446838c0e59517b108accecab7afefd9329c3a6c4a1209f7`.
2. Verify that `/api/solitaire` returns HTTP 404.
3. Verify that `solitaire_games` and `solitaire_moves` tables are absent from live Dolt state.
4. Verify that live Dolt content witness matches `99949fe2`'s recorded content root `c302f6d9e570f8755936be9c178d9a8e16ccf417551a5181b5d8be5a0637c903`.
5. Verify that the canonical event tape retains the complete forward record of Candidate A proposal, promotion, falsification, Candidate B supersession, and the restore intent.

---

## 6. Decision Policy Manifest Binding

Candidate A and B bind to `reversible-selfdev-v1.json`:
* **Policy ID:** `reversible-selfdev-v1`
* **Policy Digest:** `c34ddf073aecaacc307f375d6f2e398798350d7a48c8d3c2e7c6d10248b394d7`
* **Eligible Seats:**
  1. `cosuper-author` (`agent_profile: CoSuper`, authoring domain, recused from verification, non-quorum)
  2. `capsule-verifier` (`independent_verifier: capsule_exec_receipts`, verification domain, counts toward quorum)
  3. `independent-reviewer` (`agent_profile: not_authoring_CoSuper`, verification domain, counts toward quorum)
* **Quorum:** Global accept minimum: 2; Verification accept minimum: 2.
* **Human Seat:** `absent` (no per-candidate human gate).
* **Recovery:** `tape_recovery_restore_to_checkpoint_taken_before_A_promoted` (Pre-A Checkpoint `99949fe2`).
