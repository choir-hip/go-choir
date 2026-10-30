package platform

import (
	"context"
	"time"

	"github.com/yusefmosiah/go-choir/internal/buildinfo"
)

// CheckpointHeartbeat records checkpointd's pass (and the problem it saw, if
// any); the jobs route reports it as the worker heartbeat.
func (s *Store) CheckpointHeartbeat(ctx context.Context, problem string) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO projection_worker_status (worker_id,heartbeat,build_commit,error_text) VALUES ('checkpoint',?,?,?) ON CONFLICT (worker_id) DO UPDATE SET heartbeat=excluded.heartbeat,build_commit=excluded.build_commit,error_text=excluded.error_text`, time.Now().UTC(), buildinfo.Commit, problem)
	return err
}
