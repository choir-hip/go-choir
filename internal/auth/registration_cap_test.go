package auth

import (
	"fmt"
	"net/http"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/hostroom"
)

// Sign-up is bounded by what the host can hold: free disk, less the reserve,
// less the room already promised to accounts that have no computer yet, must
// fit one more computer (internal/hostroom). The decision is made inside the
// transaction that makes the account. Failure modes guarded, written before
// the code:
//  1. (retired 2026-11-24) two finishes both taking the last place: an
//     account with no computer holds nothing (owner: "dont count any account
//     without a computer"), so finishes racing before their computers exist
//     are bounded only by the free reading, a named residual;
//  2. a missing, stale or unreadable reading admitting anyone (closed, never
//     open), at the mail, the begin and the finish alike;
//  3. an account that already has a passkey being closed out of signing in;
//  4. a refused sign-up leaving a user, a key or a passkey behind;
//  5. an account with no computer holding room, or a computer that exists
//     not holding the gap to a footprint;
//  6. the waitlist keyed by a spelling (two spellings, two rows), unbounded
//     for a stranger (a whole IPv6 /64 is one source), kept past its TTL when
//     nobody is refused any more, or a refused person told they are listed
//     when the row was not kept;
//  7. a backlog of records with no computer (898 on staging, unproved or
//     not) holding a footprint each, so that they close sign-up for everyone;
//  8. the promise released when a computer appears, while its disk is a
//     fraction of the footprint it grows to (the room then grows as computers
//     open);
//  9. no room plan configured, or a partial one, leaving sign-up unbounded
//     (every environment closes; dev and tests give a plan).

const gib = uint64(1) << 30

type roomFixture struct {
	mu     sync.Mutex
	free   uint64
	path   string
	broken bool
}

func (f *roomFixture) freeBytes(string) (uint64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.broken {
		return 0, fmt.Errorf("statfs: no such device")
	}
	return f.free, nil
}

func (f *roomFixture) setFree(g uint64) { f.mu.Lock(); f.free = g * gib; f.mu.Unlock() }

// owners writes the file vmctl writes: these users have a computer that holds
// a whole footprint already.
func (f *roomFixture) owners(t *testing.T, at time.Time, ids ...string) {
	t.Helper()
	var os []hostroom.Owner
	for _, id := range ids {
		os = append(os, hostroom.Owner{UserID: id, DiskBytes: 9 * gib})
	}
	f.ownersWith(t, at, os...)
}

// ownersWith writes the file with the disk each computer holds.
func (f *roomFixture) ownersWith(t *testing.T, at time.Time, os ...hostroom.Owner) {
	t.Helper()
	if err := hostroom.WriteOwners(f.path, at, os); err != nil {
		t.Fatal(err)
	}
}

// withRoom gives h a bound of reserve 100 GiB and a footprint of 9 GiB, with
// freeGiB free, and a fresh owners file that lists owners (each holding a
// whole footprint).
func withRoom(t *testing.T, h *Handler, freeGiB uint64, owners ...string) *roomFixture {
	t.Helper()
	f := &roomFixture{free: freeGiB * gib, path: filepath.Join(t.TempDir(), "computer-owners.json")}
	f.owners(t, time.Now(), owners...)
	h.room = &hostRoom{
		plan:       hostroom.Plan{ReserveBytes: 100 * gib, FootprintBytes: 9 * gib},
		ownersPath: f.path,
		statPath:   t.TempDir(),
		free:       f.freeBytes,
		now:        time.Now,
	}
	return f
}

// openRoomForTests is the plan a test server runs with: a petabyte free, so
// the room never decides a test that is about something else.
func openRoomForTests(t *testing.T, h *Handler) {
	t.Helper()
	withRoom(t, h, 1<<20)
}

func addPasskey(t *testing.T, h *Handler, userID, email string) {
	t.Helper()
	if _, err := h.store.CreateUser(userID, email); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	if err := h.store.CreateCredential(&Credential{
		ID: "cred-" + userID, UserID: userID, PublicKey: []byte("k"), AttestationType: "none",
		Transport: `["internal"]`, AAGUID: make([]byte, 16), Flags: "{}", CreatedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatalf("CreateCredential: %v", err)
	}
}

// Failure 9: a handler with no plan, or a plan that did not load whole,
// refuses sign-up in every environment; sign-in is unaffected.
func TestNoRoomPlanClosesSignUpEverywhere(t *testing.T) {
	for name, room := range map[string]*hostRoom{
		"no plan":      newHostRoom(nil, "/x/auth.db"),
		"partial plan": newHostRoom(&RoomConfig{Err: "AUTH_ROOM_OWNERS_PATH is missing"}, "/x/auth.db"),
		"nil room":     nil,
	} {
		t.Run(name, func(t *testing.T) {
			h := appEnv(t)
			h.room = room
			rec := registerBeginFor(t, h, "closed@example.com")
			if rec.Code != http.StatusServiceUnavailable {
				t.Fatalf("sign-up with %s = %d %s, want 503", name, rec.Code, rec.Body.String())
			}
			var users int
			_ = h.store.db.QueryRow("SELECT COUNT(*) FROM users").Scan(&users)
			if users != 0 {
				t.Fatalf("a refused sign-up left %d users", users)
			}
		})
	}
}

// Failures 2 and 4: no reading, a stale reading and an unreadable disk close
// every step, and nothing is left behind.
func TestAMissingOrStaleReadingClosesSignUp(t *testing.T) {
	for _, tc := range []struct {
		name  string
		spoil func(t *testing.T, h *Handler, f *roomFixture)
	}{
		{"no owners file", func(t *testing.T, h *Handler, f *roomFixture) {
			h.room.ownersPath = filepath.Join(t.TempDir(), "absent.json")
		}},
		{"stale owners file", func(t *testing.T, h *Handler, f *roomFixture) { f.owners(t, time.Now().Add(-10*time.Minute)) }},
		{"disk unreadable", func(t *testing.T, h *Handler, f *roomFixture) { f.broken = true }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := appEnv(t)
			f := withRoom(t, h, 300)
			tc.spoil(t, h, f)
			rec := registerBeginFor(t, h, "closed@example.com")
			if rec.Code != http.StatusServiceUnavailable {
				t.Fatalf("begin with no reading = %d %s, want 503", rec.Code, rec.Body.String())
			}
			var users, keys int
			_ = h.store.db.QueryRow("SELECT COUNT(*) FROM users").Scan(&users)
			_ = h.store.db.QueryRow("SELECT COUNT(*) FROM api_keys").Scan(&keys)
			if users != 0 || keys != 0 {
				t.Fatalf("a refused sign-up left %d users and %d keys", users, keys)
			}
		})
	}
}

// Failures 3, 4 and the finish: begun while there is room, finished after it
// is gone; the person who has an account still signs in.
func TestFinishRefusesWhenTheRoomWentAndSignInStillWorks(t *testing.T) {
	h := appEnv(t)
	f := withRoom(t, h, 160)
	first := newSoftPasskey(t)
	if rec := createAccount(t, h, "first@example.com", first, testOrigin); rec.Code != 200 {
		t.Fatalf("first = %d: %s", rec.Code, rec.Body.String())
	}
	// The first account has no computer yet: it holds nothing.
	second := newSoftPasskey(t)
	begin := decodeMap(t, registerBeginFor(t, h, "second@example.com"))
	f.setFree(100) // no room left
	fin := postJSON(h.HandleAppRegisterFinish, appFinishBody(testApp, second.create(t, begin, testOrigin)))
	if fin.Code != 403 || decodeMap(t, fin)["code"] != "registration_waitlist" {
		t.Fatalf("finish with no room = %d: %s", fin.Code, fin.Body.String())
	}
	if _, err := h.store.GetUserByEmail("second@example.com"); err == nil {
		t.Fatal("a refused finish left an account")
	}
	var keys int
	_ = h.store.db.QueryRow("SELECT COUNT(*) FROM api_keys").Scan(&keys)
	if keys != 1 {
		t.Fatalf("%d keys, want only the first account's", keys)
	}
	opts := decodeMap(t, postJSON(h.HandleAppLoginBegin, map[string]string{}))
	if rec := postJSON(h.HandleAppLoginFinish, appFinishBody(testApp, first.assert(t, opts, testOrigin))); rec.Code != 200 {
		t.Fatalf("sign-in with sign-up closed = %d", rec.Code)
	}
}

// Failure 5 and 8: an account with no computer holds nothing; once its
// computer exists it holds the gap to a footprint, so opening one does not
// make room.
func TestAComputerHoldsItsGrowthGapAndAnAccountWithoutOneHoldsNothing(t *testing.T) {
	h := appEnv(t)
	f := withRoom(t, h, 109) // one place
	if rec := createAccount(t, h, "one@example.com", newSoftPasskey(t), testOrigin); rec.Code != 200 {
		t.Fatalf("the one place: %d %s", rec.Code, rec.Body.String())
	}
	if rec := registerBeginFor(t, h, "two@example.com"); rec.Code != 200 {
		t.Fatalf("an account with no computer held the place: %d %s", rec.Code, rec.Body.String())
	}
	// Its computer opens: 0.6 GiB of disk, and the free reading falls by that.
	one, _ := h.store.GetUserByEmail("one@example.com")
	const fresh = 644245094
	f.free -= fresh
	f.ownersWith(t, time.Now(), hostroom.Owner{UserID: one.ID, DiskBytes: fresh})
	if rec := registerBeginFor(t, h, "two@example.com"); rec.Code != 403 {
		t.Fatalf("opening a computer made room: %d %s", rec.Code, rec.Body.String())
	}
	// It grows to a footprint: the free reading falls by the rest, still none.
	f.free -= 9*gib - fresh
	f.ownersWith(t, time.Now(), hostroom.Owner{UserID: one.ID, DiskBytes: 9 * gib})
	if rec := registerBeginFor(t, h, "two@example.com"); rec.Code != 403 {
		t.Fatalf("a grown computer made room: %d %s", rec.Code, rec.Body.String())
	}
	// Somebody frees disk: now there is a place.
	f.setFree(109)
	if rec := registerBeginFor(t, h, "two@example.com"); rec.Code != 200 {
		t.Fatalf("room after disk was freed: %d %s", rec.Code, rec.Body.String())
	}
}

// Ten accounts are admitted; all ten open their computers; the room then
// closes, and does not grow as they open.
func TestTenNewComputersOpenAndTheRoomDoesNotGrow(t *testing.T) {
	h := appEnv(t)
	f := withRoom(t, h, 190) // ten places
	var ids []string
	for i := 0; i < 10; i++ {
		email := fmt.Sprintf("ten%d@example.com", i)
		if rec := createAccount(t, h, email, newSoftPasskey(t), testOrigin); rec.Code != 200 {
			t.Fatalf("account %d: %d %s", i, rec.Code, rec.Body.String())
		}
		u, _ := h.store.GetUserByEmail(email)
		ids = append(ids, u.ID)
	}
	// None has a computer yet, so none holds room: the free reading is the bound.
	if rec := registerBeginFor(t, h, "eleventh@example.com"); rec.Code != 200 {
		t.Fatalf("accounts with no computer closed the room: %d %s", rec.Code, rec.Body.String())
	}
	const fresh = 644245094
	var os []hostroom.Owner
	for _, id := range ids {
		os = append(os, hostroom.Owner{UserID: id, DiskBytes: fresh})
		f.free -= fresh
	}
	f.ownersWith(t, time.Now(), os...)
	// Ten computers took ten places (ten footprints less nothing): now none.
	if rec := registerBeginFor(t, h, "eleventh@example.com"); rec.Code != 403 {
		t.Fatalf("ten computers opening made room: %d %s", rec.Code, rec.Body.String())
	}
}

// Failure 7 and 10: records of accounts with no computer (a passkey, proved or
// not, at any address) hold nothing. 898 of them, each charged a footprint,
// closed sign-up on staging for good (owner, 2026-11-24: "dont count any
// account without a computer ... we just have records of email adddresses
// where we deleted the computers for testing").
func TestAccountsWithoutAComputerHoldNothing(t *testing.T) {
	h := appEnv(t)
	withRoom(t, h, 109) // one place
	for i := 0; i < 898; i++ {
		id := fmt.Sprintf("old%d", i)
		addPasskey(t, h, id, id+"@example.com")
		if i%2 == 0 {
			if _, err := h.store.db.Exec(`UPDATE users SET email_verified_at = NULL WHERE id = ?`, id); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := h.room.admit(h.store.db); err != nil {
		t.Fatalf("898 records with no computer closed sign-up: %v", err)
	}
	if rec := createAccount(t, h, "new@choir.news", newSoftPasskey(t), testOrigin); rec.Code != 200 {
		t.Fatalf("sign-up with 898 records and a free place: %d %s", rec.Code, rec.Body.String())
	}
}

// A computer that exists holds the gap to a footprint, and a free reading
// under reserve plus one footprint is closed.
func TestAnOwnerHoldsItsGapAndLessThanAFootprintOverTheReserveIsClosed(t *testing.T) {
	h := appEnv(t)
	f := withRoom(t, h, 118) // two places
	if err := h.room.admit(h.store.db); err != nil {
		t.Fatal(err)
	}
	f.ownersWith(t, time.Now(), hostroom.Owner{UserID: "small", DiskBytes: 1 * gib}) // holds 8 GiB
	if err := h.room.admit(h.store.db); err != nil {
		t.Fatalf("118 - 100 - 8 = 10 GiB is one place: %v", err)
	}
	f.ownersWith(t, time.Now(), hostroom.Owner{UserID: "small", DiskBytes: 1 * gib}, hostroom.Owner{UserID: "small2", DiskBytes: 1 * gib}) // 16 held
	if err := h.room.admit(h.store.db); err != ErrRegistrationClosed {
		t.Fatalf("118 - 100 - 16 = 2 GiB left admitted: %v", err)
	}
	f.ownersWith(t, time.Now())
	f.setFree(108) // 8 GiB over the reserve, under the 9 GiB footprint
	if err := h.room.admit(h.store.db); err != ErrRegistrationClosed {
		t.Fatalf("less than a footprint over the reserve admitted: %v", err)
	}
}

// Failure 6.
func TestWaitlistKeyIsTheNormalisedAddressAndItIsBounded(t *testing.T) {
	h, _ := testHandlerEnv(t)
	now := time.Date(2026, 11, 24, 12, 0, 0, 0, time.UTC)
	add := func(email, src string, at time.Time) bool {
		t.Helper()
		recorded, err := h.store.AddRegistrationWaitlist(email, src, at)
		if err != nil {
			t.Fatal(err)
		}
		return recorded
	}
	if !add("Ada@Example.COM", "src-a", now) || !add(" ada@example.com ", "src-a", now.Add(time.Minute)) {
		t.Fatal("a listed address was not recorded")
	}
	entries, _ := h.store.ListRegistrationWaitlist()
	if len(entries) != 1 || entries[0].Email != "ada@example.com" || entries[0].Attempts != 2 {
		t.Fatalf("one person, two spellings: %+v", entries)
	}

	// One source cannot hold more than waitlistPerSourceMax rows, and the
	// ones it cannot hold are reported as not recorded.
	for i := 0; i < waitlistPerSourceMax+5; i++ {
		got := add(fmt.Sprintf("probe%d@x.org", i), "src-b", now)
		if want := i < waitlistPerSourceMax; got != want {
			t.Fatalf("probe %d recorded = %v, want %v", i, got, want)
		}
	}
	var fromB int
	_ = h.store.db.QueryRow("SELECT COUNT(*) FROM registration_waitlist WHERE source_hash = ?", "src-b").Scan(&fromB)
	if fromB != waitlistPerSourceMax {
		t.Fatalf("one source holds %d rows, want %d", fromB, waitlistPerSourceMax)
	}

	// The table as a whole is bounded.
	for i := 0; i < waitlistTotalMax+50; i++ {
		add(fmt.Sprintf("many%d@x.org", i), fmt.Sprintf("src-%d", i), now)
	}
	var total int
	_ = h.store.db.QueryRow("SELECT COUNT(*) FROM registration_waitlist").Scan(&total)
	if total > waitlistTotalMax {
		t.Fatalf("the waitlist holds %d rows, bound %d", total, waitlistTotalMax)
	}
	if add("one-too-many@x.org", "src-new", now) {
		t.Fatal("a new address at a full table was reported recorded")
	}

	// An address already on the list keeps counting at a full table.
	if !add("ada@example.com", "src-a", now.Add(2*time.Minute)) {
		t.Fatal("a listed address at a full table was reported not recorded")
	}
	entries, _ = h.store.ListRegistrationWaitlist()
	var ada WaitlistEntry
	for _, e := range entries {
		if e.Email == "ada@example.com" {
			ada = e
		}
	}
	if ada.Attempts != 3 {
		t.Fatalf("a listed address stopped counting at a full table: %+v", ada)
	}

	// The sweeper deletes rows past the TTL with nobody being refused: the
	// inventory says an address is deleted 30 days after its last ask.
	n, err := h.store.SweepRegistrationWaitlist(now.Add(waitlistTTL - time.Hour))
	if err != nil || n != 0 {
		t.Fatalf("a sweep inside the TTL deleted %d rows, %v", n, err)
	}
	n, err = h.store.SweepRegistrationWaitlist(now.Add(waitlistTTL + time.Minute))
	if err != nil || n == 0 {
		t.Fatalf("a sweep past the TTL deleted %d rows, %v", n, err)
	}
	entries, _ = h.store.ListRegistrationWaitlist()
	if len(entries) != 1 || entries[0].Email != "ada@example.com" {
		t.Fatalf("after the sweep the list is %d rows, want only the address asked again at +2m", len(entries))
	}
}

// A whole IPv6 /64 is one source: rotating the interface id does not make a
// fresh allowance, and a refused person is told the truth when the list could
// not keep them.
func TestWaitlistSourceIsAnIPv6SlashSixtyFour(t *testing.T) {
	if waitlistSource("2001:db8:1:2::1") != waitlistSource("2001:db8:1:2:ffff::9") {
		t.Fatal("two addresses of one /64 are two sources")
	}
	if waitlistSource("2001:db8:1:2::1") == waitlistSource("2001:db8:1:3::1") {
		t.Fatal("two /64s are one source")
	}
	if waitlistSource("203.0.113.5") == waitlistSource("203.0.113.6") {
		t.Fatal("two IPv4 addresses are one source")
	}
	h := appEnv(t)
	withRoom(t, h, 100) // no place: every sign-up is refused
	var messages []string
	for i := 0; i < waitlistPerSourceMax+4; i++ {
		rec := postJSONFrom(h.HandleRegisterMail, fmt.Sprintf("2001:db8:7:7::%x", i+1), map[string]string{"email": fmt.Sprintf("rot%d@example.com", i), "binding": newBinding(t)})
		if rec.Code != http.StatusForbidden {
			t.Fatalf("request %d = %d %s", i, rec.Code, rec.Body.String())
		}
		messages = append(messages, decodeMap(t, rec)["error"].(string))
	}
	var rows int
	_ = h.store.db.QueryRow("SELECT COUNT(*) FROM registration_waitlist").Scan(&rows)
	if rows != waitlistPerSourceMax {
		t.Fatalf("rotating one /64 put %d rows on the list, want %d", rows, waitlistPerSourceMax)
	}
	if messages[0] != registrationWaitlistMessage || messages[len(messages)-1] != registrationWaitlistFullMessage {
		t.Fatalf("a person the list could not keep was told %q", messages[len(messages)-1])
	}
}

// A plan that did not load whole closes sign-up; the service keeps running
// (sign-in, recovery and sessions do not depend on it).
func TestRoomFromEnv(t *testing.T) {
	for _, k := range []string{"AUTH_ROOM_RESERVE_GIB", "AUTH_ROOM_FOOTPRINT_GIB", "AUTH_ROOM_OWNERS_PATH"} {
		t.Setenv(k, "")
	}
	cfg, err := LoadConfig()
	if err != nil || cfg.Room != nil {
		t.Fatalf("unset: %v, %+v", err, cfg.Room)
	}
	if newHostRoom(cfg.Room, "/x/auth.db").admit(nil) == nil {
		t.Fatal("an unset plan admitted")
	}
	t.Setenv("AUTH_ROOM_RESERVE_GIB", "100")
	t.Setenv("AUTH_ROOM_FOOTPRINT_GIB", "15")
	t.Setenv("AUTH_ROOM_OWNERS_PATH", "/x/computer-owners.json")
	cfg, err = LoadConfig()
	if err != nil || cfg.Room == nil || cfg.Room.Err != "" || cfg.Room.ReserveGiB != 100 || cfg.Room.FootprintGiB != 15 || cfg.Room.OwnersPath != "/x/computer-owners.json" {
		t.Fatalf("set: %v, %+v", err, cfg.Room)
	}
	for name, set := range map[string]func(){
		"a part missing":                      func() { t.Setenv("AUTH_ROOM_OWNERS_PATH", "") },
		"a zero footprint":                    func() { t.Setenv("AUTH_ROOM_FOOTPRINT_GIB", "0") },
		"a footprint under a real computer's": func() { t.Setenv("AUTH_ROOM_FOOTPRINT_GIB", "1") },
		"a reserve under the deploy floor":    func() { t.Setenv("AUTH_ROOM_RESERVE_GIB", "59") },
		"no reserve":                          func() { t.Setenv("AUTH_ROOM_RESERVE_GIB", "0") },
		"a malformed number":                  func() { t.Setenv("AUTH_ROOM_RESERVE_GIB", "lots") },
	} {
		t.Setenv("AUTH_ROOM_RESERVE_GIB", "100")
		t.Setenv("AUTH_ROOM_FOOTPRINT_GIB", "15")
		t.Setenv("AUTH_ROOM_OWNERS_PATH", "/x/computer-owners.json")
		set()
		cfg, err := LoadConfig()
		if err != nil {
			t.Fatalf("%s stopped the service: %v", name, err)
		}
		if cfg.Room == nil || cfg.Room.Err == "" {
			t.Fatalf("%s loaded as a usable plan: %+v", name, cfg.Room)
		}
		if newHostRoom(cfg.Room, "/x/auth.db").admit(nil) == nil {
			t.Fatalf("%s admitted", name)
		}
	}
}

// A person refused at begin keeps the mailed code: when there is room the same
// code is accepted (the code is not spent by a refusal).
func TestARefusedBeginKeepsTheMailedCode(t *testing.T) {
	h := appEnv(t)
	f := withRoom(t, h, 300)
	fields, rec := proofFields(t, h, "retry@example.com")
	if fields == nil {
		t.Fatalf("mail refused with room: %d %s", rec.Code, rec.Body.String())
	}
	f.setFree(100) // the room closes between the mail and the begin
	if rec := postJSONFrom(h.HandleRegisterBegin, nextTestIP(), fields); rec.Code != http.StatusForbidden {
		t.Fatalf("begin with no room = %d %s, want 403", rec.Code, rec.Body.String())
	}
	f.setFree(300)
	if rec := postJSONFrom(h.HandleRegisterBegin, nextTestIP(), fields); rec.Code != http.StatusOK {
		t.Fatalf("the same code once there is room = %d %s", rec.Code, rec.Body.String())
	}
}

// A person refused at finish keeps the ceremony: the same passkey answer is
// accepted when there is room.
func TestARefusedFinishKeepsThePendingChallenge(t *testing.T) {
	h := appEnv(t)
	f := withRoom(t, h, 300)
	pk := newSoftPasskey(t)
	begin := decodeMap(t, registerBeginFor(t, h, "again@example.com"))
	body := appFinishBody(testApp, pk.create(t, begin, testOrigin))
	f.setFree(100)
	if rec := postJSON(h.HandleAppRegisterFinish, body); rec.Code != http.StatusForbidden {
		t.Fatalf("finish with no room = %d %s, want 403", rec.Code, rec.Body.String())
	}
	if _, err := h.store.GetUserByEmail("again@example.com"); err == nil {
		t.Fatal("a refused finish left an account")
	}
	f.setFree(300)
	if rec := postJSON(h.HandleAppRegisterFinish, body); rec.Code != http.StatusOK {
		t.Fatalf("the same answer once there is room = %d %s", rec.Code, rec.Body.String())
	}
}
