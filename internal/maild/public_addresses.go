package maild

// Public addresses (owner, 2026-11-04: an app store shows who made an app by
// their Choir address; every account has one, test accounts included, and the
// sign-in email is never shown). A read-only, host-internal lookup from
// account ids to each account's PUBLIC ADDRESS. Today that is always the
// account's numbered address, "<NNN>@<primary domain>" (zero-padded to three
// digits, the root owner is 000; test numbers render as plain digits). The
// route and field are named for the public address, not the number, because
// later an account may choose one of its own addresses as its public one and
// callers (the Library) then pick it up unchanged. An account with no public
// address gets no entry, which callers treat as a defect, never as something
// to route around.

import (
	"context"
	"net/http"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/hostauth"
)

// PublicAddressesPath is the host-internal lookup route. Same transport
// gates as AuthAssignNumbersPath: loopback host-service callers only, never a
// user request.
const PublicAddressesPath = "/internal/maild/public-addresses"

const publicAddressesMaxOwners = 100

// PublicAddresses maps each account id that has an enabled numbered alias
// to the number of its numbered address, today its public address.
func (s *Store) PublicAddresses(ctx context.Context, owners []string) (map[string]int64, error) {
	out := map[string]int64{}
	if len(owners) == 0 {
		return out, nil
	}
	args := make([]any, 0, len(owners))
	marks := make([]string, 0, len(owners))
	for _, id := range owners {
		args = append(args, id)
		marks = append(marks, "?")
	}
	rows, err := s.routingDB.QueryContext(ctx, `SELECT target_id, canonical_number FROM email_aliases
		WHERE target_type = 'user' AND canonical_number IS NOT NULL AND disabled_at IS NULL
		AND target_id IN (`+strings.Join(marks, ",")+`) ORDER BY canonical_number DESC`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		var n int64
		if err := rows.Scan(&id, &n); err != nil {
			return nil, err
		}
		out[id] = n // DESC order: the lowest number (the first assigned) wins
	}
	return out, rows.Err()
}

type publicAddressesRequest struct {
	OwnerIDs []string `json:"owner_ids"`
}

// HandlePublicAddresses answers {"addresses": {owner_id: "NNN@domain"}} for
// at most 100 ids; ids with no number are absent.
func (h *Handler) HandlePublicAddresses(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	if !requireHostServiceCaller(w, r) {
		return
	}
	var in publicAddressesRequest
	if err := h.decodeJSON(r, &in); err != nil {
		writeDecodeError(w, err)
		return
	}
	if len(in.OwnerIDs) == 0 || len(in.OwnerIDs) > publicAddressesMaxOwners {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "owner_ids must hold 1 to 100 ids"})
		return
	}
	ids := make([]string, 0, len(in.OwnerIDs))
	for _, id := range in.OwnerIDs {
		if id = strings.TrimSpace(id); id != "" {
			ids = append(ids, id)
		}
	}
	numbers, err := h.store.PublicAddresses(r.Context(), ids)
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "lookup failed"})
		return
	}
	domain := strings.ToLower(strings.TrimSpace(h.cfg.PrimaryDomain))
	addresses := map[string]string{}
	for id, n := range numbers {
		addresses[id] = canonicalNumberString(n) + "@" + domain
	}
	writeJSON(w, http.StatusOK, map[string]any{"addresses": addresses})
}

// requireHostServiceCaller is the gate of AuthAssignNumbersPath.
func requireHostServiceCaller(w http.ResponseWriter, r *http.Request) bool {
	if !isMaildLoopbackHost(maildRemoteAddrHost(r)) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "host service caller required"})
		return false
	}
	if !strings.EqualFold(strings.TrimSpace(r.Header.Get("X-Internal-Caller")), "true") {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "internal caller required"})
		return false
	}
	if !hostauth.Authorize(r) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "internal credential required"})
		return false
	}
	if strings.TrimSpace(r.Header.Get("X-Authenticated-User")) != "" {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "not available to user requests"})
		return false
	}
	return true
}
