package computerversion

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// DoltPromotionAdapter implements the tag-based promotion operations that the
// TLA+ promotion protocol spec models. It connects to a live embedded Dolt
// workspace and provides typed operations for fork, commit, promote, and
// rollback.
//
// Tag-based approach (interim, pending Phase D branch rewrite):
//
// The 2026-07-07 branch-isolation experiment used *sql.DB and concluded that
// DOLT_CHECKOUT in embedded mode is a no-op. The D-PROMO Phase A settlement
// test (`TestDoltEmbeddedBranchIsolationPinnedConnection`, -count=10) showed
// that the apparent no-op was a database/sql connection-pooling artifact:
// DOLT_CHECKOUT affects only the DoltSession of the connection that executes
// it. On a pinned *sql.Conn (db.Conn(ctx)) all statements share one session,
// so DOLT_BRANCH / DOLT_CHECKOUT / DOLT_MERGE / DOLT_TAG / DOLT_RESET provide
// deterministic branch isolation and rollback in embedded mode.
//
// Until Phase D rewrites this adapter to branch-based operations (one pinned
// connection per candidate), the adapter remains tag-based for safety:
//
//   - Fork: record the current HEAD as a fork tag (DOLT_TAG).
//     The fork tag is the rollback target.
//   - Commit: commit candidate changes (DOLT_COMMIT).
//     Each capsule transaction batch produces one commit.
//   - Promote: tag the current HEAD as the promotion certificate (DOLT_TAG).
//     The promotion tag IS part of the ArtifactProgramRef.
//   - Rollback: reset the working set to the fork tag (DOLT_RESET --hard).
//     This restores the active computer's state to the pre-promotion point.
//
// TLA+ spec mapping (interim tag mapping):
//
//   - ForkCandidate(c, a) → Fork(candidateID)
//   - CapsuleTxn(c) → Commit(message)  [called per capsule transaction batch]
//   - Commit(c) → Promote(candidateID, forkTag)
//   - AutoRevert(c) → Rollback(forkTag)
//
// Phase D rewrite target (after D-PROMO settlement passed):
//
//   - Fork: DOLT_BRANCH per candidate + DOLT_CHECKOUT onto the candidate branch
//     on a pinned *sql.Conn.
//   - Commit: DOLT_COMMIT on the candidate branch.
//   - Promote: DOLT_MERGE('candidate') into main, then DOLT_TAG('promote-...').
//     These must be two resumable steps; promotion atomicity lives at the
//     route-flip layer, not in a single SQL transaction.
//   - Rollback: DOLT_RESET('--hard') to the fork tag on the same pinned
//     connection, or a route flip.
//
// Until that rewrite lands, this adapter does NOT use DOLT_BRANCH or
// DOLT_CHECKOUT. Candidate isolation in the tag-only interim comes from the
// single-writer discipline and the store's shared embedded connection.
type DoltPromotionAdapter struct {
	// DB is an existing Dolt database connection. If set, the adapter uses
	// it directly instead of opening its own connection. This is required
	// when integrating with the runtime store, which already holds the
	// single writable connection to the embedded Dolt workspace.
	DB *sql.DB

	// WorkspacePath and Database are used to open a new connection when
	// DB is nil. This is primarily used by tests and standalone tools.
	WorkspacePath string
	Database      string
	CommitName    string
	CommitEmail   string
}

// ForkRecord records the fork point for a candidate promotion. The fork tag
// is the rollback target: if the promotion fails the health window, the
// adapter resets to this tag.
type ForkRecord struct {
	ForkTag    string `json:"fork_tag"`    // e.g. "fork:candidate-1:1690000000"
	ForkCommit string `json:"fork_commit"` // HASHOF('HEAD') at fork time
}

// PromotionRecord records the promotion certificate. The promotion tag IS
// part of the ArtifactProgramRef — it is the tamper-evident reference to
// the promoted state.
type PromotionRecord struct {
	PromotionTag string `json:"promotion_tag"` // e.g. "promote:candidate-1:1690000001"
	MergeCommit  string `json:"merge_commit"`  // HASHOF('HEAD') at promotion time
	ForkTag      string `json:"fork_tag"`      // the fork tag for rollback
}

// Fork records the current HEAD as a fork tag for the given candidate.
// The fork tag is unique per candidate and includes a timestamp to avoid
// collisions with prior forks of the same candidate.
//
// This maps to the TLA+ ForkCandidate action: it captures the active
// computer's current artifact head as the fork point (promoForkTag).
func (a DoltPromotionAdapter) Fork(ctx context.Context, candidateID string) (*ForkRecord, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	candidateID = strings.TrimSpace(candidateID)
	if candidateID == "" {
		return nil, fmt.Errorf("dolt promotion: candidate ID is required")
	}

	db, connector, err := a.open()
	if err != nil {
		return nil, err
	}
	defer a.close(db, connector)

	// Get the current HEAD commit hash.
	var head string
	if err := db.QueryRowContext(ctx, "SELECT HASHOF('HEAD')").Scan(&head); err != nil {
		return nil, fmt.Errorf("dolt promotion fork: query HEAD: %w", err)
	}

	// Create a fork tag at the current HEAD.
	forkTag := fmt.Sprintf("fork-%s-%d", candidateID, time.Now().Unix())
	if _, err := db.ExecContext(ctx, fmt.Sprintf("CALL DOLT_TAG('%s')", forkTag)); err != nil {
		return nil, fmt.Errorf("dolt promotion fork: create tag %q: %w", forkTag, err)
	}

	return &ForkRecord{
		ForkTag:    forkTag,
		ForkCommit: head,
	}, nil
}

// Commit records candidate changes as a Dolt commit. The message should
// identify the capsule and transaction batch. Returns the new HEAD commit
// hash.
//
// This maps to the TLA+ CapsuleTxn action: each capsule transaction batch
// appends to the candidate's branch as a commit. In embedded mode without
// branch isolation, the commit goes to the main branch directly.
func (a DoltPromotionAdapter) Commit(ctx context.Context, message string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	message = strings.TrimSpace(message)
	if message == "" {
		return "", fmt.Errorf("dolt promotion commit: message is required")
	}

	db, connector, err := a.open()
	if err != nil {
		return "", err
	}
	defer a.close(db, connector)

	// Commit all staged and unstaged changes.
	if _, err := db.ExecContext(ctx, fmt.Sprintf("CALL DOLT_COMMIT('-Am', '%s')", message)); err != nil {
		// "nothing to commit" is not an error — return the current HEAD.
		if !strings.Contains(err.Error(), "nothing to commit") {
			return "", fmt.Errorf("dolt promotion commit: %w", err)
		}
	}

	var head string
	if err := db.QueryRowContext(ctx, "SELECT HASHOF('HEAD')").Scan(&head); err != nil {
		return "", fmt.Errorf("dolt promotion commit: query HEAD: %w", err)
	}
	return head, nil
}

// Promote tags the current HEAD as the promotion certificate for the given
// candidate. The promotion tag IS part of the ArtifactProgramRef. The fork
// tag is recorded for rollback.
//
// This maps to the TLA+ Commit action: it creates the merge tag
// (promoMergeTag) that identifies the promoted state. The tag is
// tamper-evident and content-addressed via the Dolt commit hash.
func (a DoltPromotionAdapter) Promote(ctx context.Context, candidateID string, forkTag string) (*PromotionRecord, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	candidateID = strings.TrimSpace(candidateID)
	if candidateID == "" {
		return nil, fmt.Errorf("dolt promotion: candidate ID is required")
	}
	forkTag = strings.TrimSpace(forkTag)
	if forkTag == "" {
		return nil, fmt.Errorf("dolt promotion: fork tag is required")
	}

	db, connector, err := a.open()
	if err != nil {
		return nil, err
	}
	defer a.close(db, connector)

	// Get the current HEAD commit hash (the state being promoted).
	var head string
	if err := db.QueryRowContext(ctx, "SELECT HASHOF('HEAD')").Scan(&head); err != nil {
		return nil, fmt.Errorf("dolt promotion promote: query HEAD: %w", err)
	}

	// Create the promotion tag at the current HEAD.
	promotionTag := fmt.Sprintf("promote-%s-%d", candidateID, time.Now().Unix())
	if _, err := db.ExecContext(ctx, fmt.Sprintf("CALL DOLT_TAG('%s')", promotionTag)); err != nil {
		return nil, fmt.Errorf("dolt promotion promote: create tag %q: %w", promotionTag, err)
	}

	return &PromotionRecord{
		PromotionTag: promotionTag,
		MergeCommit:  head,
		ForkTag:      forkTag,
	}, nil
}

// Rollback resets the working set to the fork tag, restoring the active
// computer's state to the pre-promotion point. This is the atomic rollback
// operation: DOLT_RESET --hard to the fork tag.
//
// This maps to the TLA+ AutoRevert action: it restores the active
// computer's artifact head to the pre-merge fork tag (promoForkTag).
// After rollback, the active computer's state is exactly what it was
// at fork time.
func (a DoltPromotionAdapter) Rollback(ctx context.Context, forkTag string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	forkTag = strings.TrimSpace(forkTag)
	if forkTag == "" {
		return fmt.Errorf("dolt promotion rollback: fork tag is required")
	}

	db, connector, err := a.open()
	if err != nil {
		return err
	}
	defer a.close(db, connector)

	// Verify the fork tag exists before resetting.
	var tagHash string
	if err := db.QueryRowContext(ctx, fmt.Sprintf("SELECT HASHOF('%s')", forkTag)).Scan(&tagHash); err != nil {
		return fmt.Errorf("dolt promotion rollback: fork tag %q not found: %w", forkTag, err)
	}

	// Reset the working set to the fork tag.
	if _, err := db.ExecContext(ctx, fmt.Sprintf("CALL DOLT_RESET('--hard', '%s')", forkTag)); err != nil {
		return fmt.Errorf("dolt promotion rollback: reset to %q: %w", forkTag, err)
	}

	return nil
}

// GetTagHash returns the commit hash that a tag points to. This is used
// to verify promotion certificates and fork tags.
func (a DoltPromotionAdapter) GetTagHash(ctx context.Context, tagName string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	tagName = strings.TrimSpace(tagName)
	if tagName == "" {
		return "", fmt.Errorf("dolt promotion: tag name is required")
	}

	db, connector, err := a.open()
	if err != nil {
		return "", err
	}
	defer a.close(db, connector)

	var hash string
	if err := db.QueryRowContext(ctx, fmt.Sprintf("SELECT HASHOF('%s')", tagName)).Scan(&hash); err != nil {
		return "", fmt.Errorf("dolt promotion: tag %q not found: %w", tagName, err)
	}
	return hash, nil
}

// open returns a database connection for the adapter. If the adapter has
// a shared DB connection, it returns that without closing it. Otherwise,
// it opens a new connection to the workspace (and the caller must close it).
func (a DoltPromotionAdapter) open() (*sql.DB, interface{ Close() error }, error) {
	if a.DB != nil {
		// Use the shared connection; return a nil connector to signal
		// that the caller should not close it.
		return a.DB, nil, nil
	}

	workspace := strings.TrimSpace(a.WorkspacePath)
	if workspace == "" {
		return nil, nil, fmt.Errorf("dolt promotion: workspace path is required")
	}
	database := strings.TrimSpace(a.Database)
	if database == "" {
		return nil, nil, fmt.Errorf("dolt promotion: database is required")
	}
	commitName := strings.TrimSpace(a.CommitName)
	if commitName == "" {
		commitName = "Choir"
	}
	commitEmail := strings.TrimSpace(a.CommitEmail)
	if commitEmail == "" {
		commitEmail = "system@choir.local"
	}
	return openDoltWorkspace(workspace, database, commitName, commitEmail)
}

// close closes the database and connector if they were opened by open().
// If the adapter is using a shared DB connection, this is a no-op.
func (a DoltPromotionAdapter) close(db *sql.DB, connector interface{ Close() error }) {
	if a.DB != nil {
		// Shared connection — do not close.
		return
	}
	_ = db.Close()
	if connector != nil {
		_ = connector.Close()
	}
}
