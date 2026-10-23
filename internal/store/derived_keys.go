package store

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
)

// The og_key_index side table (internal/objectgraph/derived_keys.go) as the
// Store drives it: its writers are the projector (project.go), the
// vocabulary migration and revert (vocab_migrate_og.go), and the direct
// object-graph writers; its online validation pass is started here, after the
// listener is up.

var (
	// derivedKeyRecheckEvery is how often the background loop looks for a
	// family that was withdrawn (a stale read, a failed maintenance).
	derivedKeyRecheckEvery = 30 * time.Second
	// derivedKeyRepairRecheckEvery is the least time between passes run only
	// because a family with a malformed row was written since (it may have
	// been repaired): a family whose malformed row stays costs one pass in
	// that time, not one per write.
	derivedKeyRepairRecheckEvery = 2 * time.Minute
)

const (
	// derivedKeyPageSize is the number of og_objects rows one validation
	// transaction compares: small enough that the engine is held for a few
	// tens of milliseconds, large enough that a store of 100,000 rows is
	// 500 pages.
	derivedKeyPageSize = 200
	// derivedKeyDutyCycle caps the share of time the pass may hold the
	// engine; the desks' reads fit between pages.
	derivedKeyDutyCycle = 0.25
)

// derivedKeys returns the side-table state, or nil on a store with no object
// graph (every maintainer treats nil as "do nothing").
func (s *Store) derivedKeys() *objectgraph.DerivedKeys {
	if s == nil || s.ogStore == nil {
		return nil
	}
	return s.ogStore.DerivedKeys()
}

// ValidateDerivedKeys runs one validation pass over every family, holding the
// engine one page at a time. Tests and StartDerivedKeyValidation call it.
func (s *Store) ValidateDerivedKeys(ctx context.Context, opt objectgraph.DerivedKeyValidation) error {
	if s == nil || s.ogStore == nil {
		return fmt.Errorf("runtime store: object graph not initialized")
	}
	return s.ogStore.ValidateDerivedKeys(ctx, opt)
}

// StartDerivedKeyValidation validates the derived-key families in the
// background until ctx ends, again whenever a family is withdrawn, and again
// when a family kept on the JSON path by a malformed row has been written
// since its last pass (at most every derivedKeyRepairRecheckEvery). Call it
// once, after the listener is up and the runtime has started: it is not part of
// the boot, and until a family is validated its reads use the JSON path, as
// they did before the side table existed. It holds the engine one page at a
// time, logs its progress, and never fails the process.
func (s *Store) StartDerivedKeyValidation(ctx context.Context) {
	if s == nil {
		return
	}
	repairGap := derivedKeyRepairRecheckEvery
	go func() {
		var lastPass time.Time
		for ctx.Err() == nil {
			d := s.derivedKeys()
			if d == nil {
				return
			}
			if !allFamiliesValidated(d) || (anyRepairPending(d) && time.Since(lastPass) >= repairGap) {
				start := time.Now()
				err := s.ValidateDerivedKeys(ctx, objectgraph.DerivedKeyValidation{
					PageSize:  derivedKeyPageSize,
					DutyCycle: derivedKeyDutyCycle,
				})
				lastPass = time.Now()
				if err != nil && ctx.Err() == nil {
					log.Printf("store: derived keys validation pass failed after %s, will retry: %v", time.Since(start).Round(time.Millisecond), err)
				}
			}
			if !allFamiliesValidated(d) {
				// A pass that ended with a family withdrawn waits before the
				// next one, so a persistent writer fault cannot spin it.
				select {
				case <-ctx.Done():
					return
				case <-time.After(derivedKeyRecheckEvery):
				}
				continue
			}
			if anyRepairPending(d) {
				// Written since its pass while malformed: run the next pass
				// once the least gap has passed.
				select {
				case <-ctx.Done():
					return
				case <-time.After(repairGap - time.Since(lastPass)):
				}
				continue
			}
			select {
			case <-ctx.Done():
				return
			case <-d.Wake():
			case <-time.After(derivedKeyRecheckEvery):
			}
		}
	}()
}

var derivedKeyFamilies = []string{objectgraph.KeyActorWakeOutboxProjected, objectgraph.KeyRunAgentID, objectgraph.KeyCommitmentAddressee, objectgraph.KeySourceRefDocID}

func allFamiliesValidated(d *objectgraph.DerivedKeys) bool {
	for _, name := range derivedKeyFamilies {
		if !d.Validated(name) {
			return false
		}
	}
	return true
}

func anyRepairPending(d *objectgraph.DerivedKeys) bool {
	for _, name := range derivedKeyFamilies {
		if d.RepairPending(name) {
			return true
		}
	}
	return false
}
