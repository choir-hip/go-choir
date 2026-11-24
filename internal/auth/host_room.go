package auth

import (
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"time"

	"github.com/yusefmosiah/go-choir/internal/hostroom"
)

// Sign-up is bounded by what the host can hold (internal/hostroom): free disk,
// less the reserve, less one footprint for each account that holds a passkey
// and has no computer yet, must fit one more footprint. The reading has two
// parts, both read when the decision is made: the free disk (statfs on the
// directory auth.db lives in, the same filesystem the computers are on) and
// the file vmctl writes every half minute naming the accounts that have a
// computer. Either missing or stale, sign-up is closed; so is a plan that is
// not configured or did not load whole, in every environment (development and
// tests give a plan; there is no unbounded mode).

// ErrRoomUnreadable: the reading the bound needs is missing or stale. The
// person is told to try again; nobody is admitted on a guess.
var ErrRoomUnreadable = errors.New("the host's room cannot be read")

type queryRower interface {
	Query(query string, args ...any) (*sql.Rows, error)
}

// RoomConfig is the plan and where its readings are (AUTH_ROOM_*).
type RoomConfig struct {
	ReserveGiB   uint64
	FootprintGiB uint64
	// OwnersPath is the file vmctl writes (hostroom.WriteOwners).
	OwnersPath string
	// Err is why the plan did not load whole (a part missing or malformed).
	// Sign-up is closed while it is set; the rest of the service runs.
	Err string
}

type hostRoom struct {
	plan       hostroom.Plan
	ownersPath string
	statPath   string
	// misconfigured, when set, closes sign-up: there is no usable plan.
	misconfigured string
	free          func(path string) (uint64, error)
	now           func() time.Time
}

func newHostRoom(cfg *RoomConfig, dbPath string) *hostRoom {
	if cfg == nil {
		return &hostRoom{misconfigured: "no room plan is configured"}
	}
	if cfg.Err != "" {
		return &hostRoom{misconfigured: cfg.Err}
	}
	return &hostRoom{
		plan:       hostroom.Plan{ReserveBytes: cfg.ReserveGiB << 30, FootprintBytes: cfg.FootprintGiB << 30},
		ownersPath: cfg.OwnersPath,
		statPath:   filepath.Dir(dbPath),
		free:       hostroom.FreeBytes,
		now:        time.Now,
	}
}

// admit decides whether one more account may be made. It reads the accounts
// that hold a passkey through q, so called with the transaction that is about
// to insert the account, two finishes cannot both take the last place.
func (r *hostRoom) admit(q queryRower) error {
	if r == nil {
		return fmt.Errorf("%w: no room plan", ErrRoomUnreadable)
	}
	if r.misconfigured != "" {
		return fmt.Errorf("%w: %s", ErrRoomUnreadable, r.misconfigured)
	}
	if !r.plan.Valid() {
		return fmt.Errorf("%w: no footprint is configured", ErrRoomUnreadable)
	}
	free, err := r.free(r.statPath)
	if err != nil {
		return fmt.Errorf("%w: free disk: %v", ErrRoomUnreadable, err)
	}
	owners, err := hostroom.ReadOwners(r.ownersPath, r.now(), hostroom.MaxOwnersAge)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrRoomUnreadable, err)
	}
	// Only accounts whose address was proved: an older account whose address
	// never was cannot sign in or open a computer (it signs in to nothing
	// until recovery proves it), so it holds no place.
	rows, err := q.Query(`SELECT DISTINCT c.user_id FROM credentials c JOIN users u ON u.id = c.user_id WHERE u.email_verified_at IS NOT NULL`)
	if err != nil {
		return fmt.Errorf("%w: accounts: %v", ErrRoomUnreadable, err)
	}
	defer rows.Close()
	var holders []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return fmt.Errorf("%w: accounts: %v", ErrRoomUnreadable, err)
		}
		holders = append(holders, id)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("%w: accounts: %v", ErrRoomUnreadable, err)
	}
	if r.plan.Places(free, r.plan.Held(holders, owners)) < 1 {
		return ErrRegistrationClosed
	}
	return nil
}
