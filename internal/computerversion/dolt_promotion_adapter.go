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
// Tag-based approach (embedded-mode compatible):
//
// The branch isolation experiment (2026-07-07) found that DOLT_CHECKOUT in
// embedded mode is a no-op for the working set — branch-based candidate
// isolation requires sql-server mode. However, DOLT_TAG, DOLT_COMMIT, and
// DOLT_RESET all work in embedded mode. So this adapter uses a tag-based
// approach:
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
// TLA+ spec mapping:
//
//   - ForkCandidate(c, a) → Fork(candidateID)
//   - CapsuleTxn(c) → Commit(message)  [called per capsule transaction batch]
//   - Commit(c) → Promote(candidateID, forkTag)
//   - AutoRevert(c) → Rollback(forkTag)
//
// This adapter does NOT use DOLT_BRANCH or DOLT_CHECKOUT. Candidate isolation
// in embedded mode must come from a different layer (e.g., separate Dolt
// databases per candidate, or application-level isolation via capsule
// overlayfs). When the platform Dolt moves to sql-server mode, this adapter
// can be extended to use branch-based isolation.
type DoltPromotionAdapter struct {
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

// open opens an embedded Dolt connection to the adapter's workspace.
func (a DoltPromotionAdapter) open() (*sql.DB, interface{ Close() error }, error) {
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

// close closes the database and connector.
func (a DoltPromotionAdapter) close(db *sql.DB, connector interface{ Close() error }) {
	_ = db.Close()
	if connector != nil {
		_ = connector.Close()
	}
}
