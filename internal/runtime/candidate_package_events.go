package runtime

import (
	"context"
	"encoding/json"
	"log"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

func emitCandidatePackagePromotionEvent(ctx context.Context, s *store.Store, ownerID, traceID string, kind types.EventKind, phase string, payload map[string]any) {
	if s == nil {
		return
	}
	traceID = strings.TrimSpace(traceID)
	if traceID == "" {
		return
	}
	data, err := json.Marshal(payload)
	if err != nil {
		log.Printf("runtime: marshal app promotion event: %v", err)
		return
	}
	evRec := &types.EventRecord{
		EventID:      uuid.NewString(),
		OwnerID:      ownerID,
		TrajectoryID: traceID,
		Timestamp:    time.Now().UTC(),
		Kind:         kind,
		Phase:        phase,
		Payload:      data,
	}
	if err := s.AppendEvent(ctx, evRec); err != nil {
		log.Printf("runtime: persist app promotion event %s: %v", evRec.EventID, err)
	}
}
