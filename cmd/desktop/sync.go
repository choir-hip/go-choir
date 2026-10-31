package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/yusefmosiah/go-choir/internal/tapefollow"
)

// desktopSync keeps a verified, read-only local copy of the signed-in
// owner's computer (tape and snapshot) under
// ~/Library/Application Support/Choir/<computer-id>/ (internal/tapefollow).
// No VM runs on the Mac, and nothing here writes to the computer: the
// follower's only request is the GET of the next bundle, made with the app's
// own session cookies (the native jar; the renderer never sees them).
//
// While signed out it waits; once /api/compute/status names the current
// computer it follows that computer, and re-reads the name every few minutes
// so a different sign-in follows its own computer. Private payloads arrive as
// the encrypted envelopes the host holds; their keys are not on the Mac yet
// (later: the computer key in the Keychain).
type desktopSync struct {
	session *desktopSession
	client  *http.Client
	// storeDir returns the store directory for a computer.
	storeDir func(computerID string) (string, error)
	// onStatus receives every status change.
	onStatus func(tapefollow.Status)

	mu         sync.Mutex
	computerID string
	dir        string
	syncer     *tapefollow.Syncer
	last       tapefollow.Status
	note       string
}

const (
	desktopSyncSignInPoll = 30 * time.Second
	desktopSyncRediscover = 10 * time.Minute
)

func newDesktopSync(session *desktopSession) *desktopSync {
	transport := session.client.Transport
	return &desktopSync{
		session: session,
		// Same jar and transport as the window's proxy, but no whole-request
		// timeout: a bootstrap streams for minutes (the follower bounds a
		// stalled stream itself).
		client:   &http.Client{Jar: session.jar, Transport: transport},
		storeDir: tapefollow.DefaultStoreDir,
	}
}

// StatusLine is the menu's line.
func (d *desktopSync) StatusLine() string {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.note != "" {
		return d.note
	}
	return d.last.Line()
}

// StoreDir is the current computer's store, "" before one is known.
func (d *desktopSync) StoreDir() string {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.dir
}

func (d *desktopSync) SyncNow() {
	d.mu.Lock()
	s := d.syncer
	d.mu.Unlock()
	if s != nil {
		s.SyncNow()
	}
}

func (d *desktopSync) setNote(note string) {
	d.mu.Lock()
	d.note = note
	st := d.last
	d.mu.Unlock()
	if d.onStatus != nil {
		d.onStatus(st)
	}
}

// Run follows until ctx ends.
func (d *desktopSync) Run(ctx context.Context) {
	for ctx.Err() == nil {
		id, err := d.currentComputer(ctx)
		if err != nil {
			d.setNote("Sync waits for sign-in")
			if !sleepCtx(ctx, desktopSyncSignInPoll) {
				return
			}
			continue
		}
		d.follow(ctx, id)
	}
}

// follow runs the syncer for id until ctx ends or the signed-in computer
// changes.
func (d *desktopSync) follow(ctx context.Context, id string) {
	dir, err := d.storeDir(id)
	if err != nil {
		d.setNote("Sync unavailable: " + err.Error())
		sleepCtx(ctx, desktopSyncRediscover)
		return
	}
	// The trust set is the repository-pinned platform key: what signs
	// choir.news's receipts. A forged bundle is refused.
	store, err := tapefollow.Open(dir, id, nil)
	if err != nil {
		d.setNote("Sync unavailable: " + err.Error())
		sleepCtx(ctx, desktopSyncSignInPoll)
		return
	}
	defer store.Close()
	syncer := &tapefollow.Syncer{
		Store:  store,
		Client: &tapefollow.Client{Backend: d.session.backend.String(), HTTP: d.client},
	}
	syncer.OnStatus = func(st tapefollow.Status) {
		d.mu.Lock()
		d.last = st
		d.note = ""
		d.mu.Unlock()
		if st.Phase == tapefollow.PhaseError || st.Phase == tapefollow.PhaseStopped {
			log.Printf("sync: %s", st.Line())
		}
		if d.onStatus != nil {
			d.onStatus(st)
		}
	}
	d.mu.Lock()
	d.computerID, d.dir, d.syncer, d.note = id, dir, syncer, ""
	d.last = syncer.Status()
	d.mu.Unlock()
	log.Printf("sync: following %s into %s", id, dir)
	if d.onStatus != nil {
		d.onStatus(syncer.Status())
	}

	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- syncer.Run(runCtx) }()
	ticker := time.NewTicker(desktopSyncRediscover)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			<-done
			return
		case err := <-done:
			// A stop retrying cannot fix (a fork, a full store): keep the
			// status, look again later.
			if err != nil && !errors.Is(err, context.Canceled) {
				sleepCtx(ctx, desktopSyncRediscover)
			}
			return
		case <-ticker.C:
			if now, err := d.currentComputer(ctx); err == nil && now != id {
				cancel()
				<-done
				return
			}
		}
	}
}

// currentComputer asks the server which computer the session's owner is on.
func (d *desktopSync) currentComputer(ctx context.Context) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, desktopAuthNetworkLimit)
	defer cancel()
	u := d.session.backend.JoinPath("api", "compute", "status")
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/json")
	resp, err := d.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("compute status %d", resp.StatusCode)
	}
	var body struct {
		CurrentComputer struct {
			ComputerID string `json:"computer_id"`
		} `json:"current_computer"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&body); err != nil {
		return "", err
	}
	id := strings.TrimSpace(body.CurrentComputer.ComputerID)
	if id == "" {
		return "", errors.New("no current computer")
	}
	return id, nil
}

func sleepCtx(ctx context.Context, d time.Duration) bool {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-t.C:
		return true
	}
}
