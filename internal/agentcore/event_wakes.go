package agentcore

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"log"
	"sort"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/sourcegraph"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
	"github.com/yusefmosiah/go-choir/internal/yaegikernel"
)

// Production by event (event layer V3a, §15 of
// docs/reports/event-layer-at-ingestion-design-2026-10-16.md). With
// intake.event_wakes on, a poll's arrival is held, and after each flush the
// event layer mints captures records to management:
//
//   - an event that crossed to EventMinOrigins counted origins in the flush
//     wakes management once (event-corroborated:<event>);
//   - an event that was corroborated before the flush and gained members
//     names them in a held record (event-grew:…), read on management's
//     next turn and waking no one;
//   - an event still single-origin, opened by an official source, first
//     seen more than eventAgedAfter ago, wakes management once so it can
//     ask for a second source (event-aged:<event>);
//   - captures the flush left without a membership wake management as an
//     unresolved record, which is the arrival they would have had without
//     the event layer: a Jev outage never stops production (E10).
//
// Every record id is deterministic, so a retried mint replays.

// Bounds. Vars so a test can shorten them.
var (
	eventAgedAfter   = 45 * time.Minute
	eventAgedHorizon = 24 * time.Hour
	eventWakeTimeout = 30 * time.Second
)

// eventRecordItems is the most members one event record names: one
// intake task's worth.
const eventRecordItems = yaegikernel.MaxTaskCaptures

// eventCapturesSource is the source id an event's record names.
func eventCapturesSource(eventID string) string { return "event:" + strings.TrimSpace(eventID) }

// eventUnresolvedSource is the source id of the unresolved record.
const eventUnresolvedSource = "event:unresolved"

func idsDigest(ids []string) string {
	sorted := append([]string(nil), ids...)
	sort.Strings(sorted)
	sum := sha256.Sum256([]byte(strings.Join(sorted, "\x00")))
	return hex.EncodeToString(sum[:12])
}

// eventRecordPages names an event's members, counted ones first, in
// pages of eventRecordItems; only those in want when want is set.
func eventRecordPages(v *eventView, want map[string]bool) [][]types.CaptureArrival {
	var counted, tentative []types.CaptureArrival
	for _, m := range v.memberships {
		if want != nil && !want[m.CaptureID] {
			continue
		}
		item := types.CaptureArrival{CaptureID: m.CaptureID, Title: m.Title, URL: m.URL, SourceID: m.SourceID, Tentative: m.Tentative}
		if m.Tentative {
			tentative = append(tentative, item)
		} else {
			counted = append(counted, item)
		}
	}
	var pages [][]types.CaptureArrival
	for all := append(counted, tentative...); len(all) > 0; {
		n := min(eventRecordItems, len(all))
		pages = append(pages, all[:n])
		all = all[n:]
	}
	return pages
}

func (v *eventView) captureEvent(reason string) *types.CaptureEvent {
	origins := make([]string, 0, len(v.origins))
	for o := range v.origins {
		origins = append(origins, o)
	}
	sort.Strings(origins)
	return &types.CaptureEvent{EventID: v.ev.EventID, Reason: reason, Headline: v.ev.Headline, Origins: origins,
		FirstSeen: v.ev.FirstSeen.UTC().Format(time.RFC3339)}
}

// eventLedger is what the ledger holds for one event: the capture ids its
// records named, and whether its corroboration record was committed.
type eventLedger struct {
	named  map[string]bool
	woken  bool
	loaded bool
}

// eventWake is one record a flush mints, and the event flag its commit
// sets: an event is settled, or its aged wake marked, only once its
// record committed, so a failed commit is retried by the next flush (code
// review, Devin B1).
type eventWake struct {
	req    types.CommitInboxRecordsRequest
	settle *eventView
	aged   *eventView
}

// eventWakeRequests builds the records one flush mints, from what the
// ledger holds rather than from transitions seen in memory (panel round 2,
// Devin B1): a corroborated event with no corroboration record gets one,
// and a woken event's members no record named go in held growth records.
// So a crash between a membership write and its record leaves a gap the
// next flush fills. An event corroborated before enabledAt (the setting's
// time) reached management on unheld arrivals, so only its members
// resolved since are named. Members past one record's bound go on held
// growth pages. Events with a member within eventAgedHorizon are checked;
// an event is settled until it gains a member once its check minted
// nothing or every record it minted committed. The caller holds idx.mu.
func (rt *Runtime) eventWakeRequests(ctx context.Context, ownerID, computerID string, idx *eventIndex, report eventResolveReport, enabledAt, now time.Time) []eventWake {
	var wakes []eventWake
	add := func(sourceID, recordID string, held bool, event *types.CaptureEvent, items []types.CaptureArrival, settle, aged *eventView) {
		req, err := capturesRecordRequest(ownerID, computerID, sourceID, recordID, held, event, items)
		if err != nil {
			log.Printf("event layer: record %s: %v", recordID, err)
			return
		}
		wakes = append(wakes, eventWake{req: req, settle: settle, aged: aged})
	}
	var due []*eventView
	for _, v := range idx.events {
		if v.settled || !v.corroborated() || now.Sub(v.last) > eventAgedHorizon {
			continue
		}
		due = append(due, v)
	}
	sort.Slice(due, func(i, j int) bool { return due[i].ev.EventID < due[j].ev.EventID })
	for _, v := range due {
		ledger, err := rt.eventLedgerOf(ctx, ownerID, computerID, v.ev.EventID)
		if err != nil {
			log.Printf("event layer: ledger of %s: %v", v.ev.EventID, err)
			continue
		}
		source := eventCapturesSource(v.ev.EventID)
		grew := func(pages [][]types.CaptureArrival) {
			for _, page := range pages {
				ids := make([]string, len(page))
				for i, item := range page {
					ids[i] = item.CaptureID
				}
				add(source, "event-grew:"+v.ev.EventID+":"+idsDigest(ids), true, v.captureEvent(types.CaptureEventGrew), page, v, nil)
			}
		}
		before := len(wakes)
		// An event corroborated before the setting was on reached
		// management on its unheld arrivals: it is woken already, and
		// only its later members are named.
		if !ledger.woken && !v.corroboratedAt.Before(enabledAt) {
			pages := eventRecordPages(v, nil)
			if len(pages) > 0 {
				add(source, "event-corroborated:"+v.ev.EventID, false, v.captureEvent(types.CaptureEventCorroborated), pages[0], v, nil)
				grew(pages[1:])
			}
		} else {
			want := map[string]bool{}
			for _, m := range v.memberships {
				if !ledger.named[m.CaptureID] && !m.ResolvedAt.Before(enabledAt) {
					want[m.CaptureID] = true
				}
			}
			if len(want) > 0 {
				grew(eventRecordPages(v, want))
			}
		}
		if len(wakes) == before {
			v.settled = true
		}
	}
	official := rt.officialSources()
	if len(official) > 0 {
		var aged []*eventView
		for _, v := range idx.events {
			age := now.Sub(v.ev.FirstSeen)
			if v.agedWoken || v.corroborated() || v.ev.FirstSeen.Before(enabledAt) || age < eventAgedAfter || age > eventAgedHorizon || !official[v.openerSource()] {
				continue
			}
			aged = append(aged, v)
		}
		sort.Slice(aged, func(i, j int) bool { return aged[i].ev.EventID < aged[j].ev.EventID })
		for _, v := range aged {
			if pages := eventRecordPages(v, nil); len(pages) > 0 {
				add(eventCapturesSource(v.ev.EventID), "event-aged:"+v.ev.EventID, false, v.captureEvent(types.CaptureEventOfficial), pages[0], nil, v)
			}
		}
	}
	// Captures the flush left go out in records of one task's worth each.
	for _, req := range unresolvedRequests(ownerID, computerID, types.CaptureEventUnresolved, report.Left) {
		wakes = append(wakes, eventWake{req: req})
	}
	return wakes
}

// unresolvedRequests names inputs to management in records of one task's
// worth each: the arrival they would have had without the event layer.
func unresolvedRequests(ownerID, computerID, reason string, inputs []eventInput) []types.CommitInboxRecordsRequest {
	var reqs []types.CommitInboxRecordsRequest
	for left := inputs; len(left) > 0; {
		n := min(eventRecordItems, len(left))
		items := make([]types.CaptureArrival, 0, n)
		ids := make([]string, 0, n)
		for _, in := range left[:n] {
			id := strings.TrimSpace(in.Item.CaptureID)
			items = append(items, types.CaptureArrival{CaptureID: id, Title: strings.TrimSpace(in.Item.Title), URL: strings.TrimSpace(in.Item.URL),
				PublishedAt: strings.TrimSpace(in.Item.PublishedAt), Language: strings.TrimSpace(in.Item.Language), Script: strings.TrimSpace(in.Item.Script), SourceID: in.SourceID})
			ids = append(ids, id)
		}
		left = left[n:]
		recordID := "event-" + reason + ":" + idsDigest(ids)
		req, err := capturesRecordRequest(ownerID, computerID, eventUnresolvedSource, recordID, false, &types.CaptureEvent{Reason: reason}, items)
		if err != nil {
			log.Printf("event layer: record %s: %v", recordID, err)
			continue
		}
		reqs = append(reqs, req)
	}
	return reqs
}

// eventLedgerOf reads an event's records from its conversation with
// management.
func (rt *Runtime) eventLedgerOf(ctx context.Context, ownerID, computerID, eventID string) (eventLedger, error) {
	management := agentprofile.Management + ":" + strings.TrimSpace(ownerID)
	conversation := types.InboxConversationID(management, types.CaptureArrivalSenderInbox(eventCapturesSource(eventID)))
	recs, err := rt.store.ConversationRecords(ctx, ownerID, computerID, conversation)
	if err != nil {
		return eventLedger{}, err
	}
	out := eventLedger{named: map[string]bool{}, loaded: true}
	for _, rec := range recs {
		if rec.Captures == nil || rec.Captures.Event == nil {
			continue
		}
		if rec.RecordID == "event-corroborated:"+eventID {
			out.woken = true
		}
		for _, item := range rec.Captures.Items {
			out.named[item.CaptureID] = true
		}
	}
	return out, nil
}

// openerSource is the source of the capture that opened the event.
func (v *eventView) openerSource() string {
	for _, m := range v.memberships {
		if m.Opened || m.CaptureID == v.ev.OpenedBy {
			return m.SourceID
		}
	}
	return ""
}

// officialSources are the registry's official sources by id.
func (rt *Runtime) officialSources() map[string]bool {
	if rt.researchDeps == nil || rt.researchDeps.Sources == nil {
		return nil
	}
	out := map[string]bool{}
	for _, src := range rt.researchDeps.Sources.Sources {
		if src.Official {
			out[src.ID] = true
		}
	}
	return out
}

// mintEventWakes commits a flush's records when intake.event_wakes is on.
// After a failed resolution (resolveErr set), the inputs without a
// membership are left unresolved; a capture a chunk resolved before the
// failure waits on its event like any other (code review, Devin B2). A
// record already committed (a replay with other content) is a conflict
// and counts as committed. An event's flag is set only for records that
// committed, so a failed commit is retried by the next flush.
func (rt *Runtime) mintEventWakes(layerCtx context.Context, ownerID, computerID string, inputs []eventInput, report eventResolveReport, resolveErr error) {
	ctx, cancel := context.WithTimeout(layerCtx, eventWakeTimeout)
	defer cancel()
	if !rt.computerSettingOn(ctx, ownerID, computerID, store.SettingIntakeEventWakes) {
		return
	}
	setting, err := rt.store.GetComputerSetting(ctx, ownerID, computerID, store.SettingIntakeEventWakes)
	if err != nil {
		log.Printf("event layer: read %s: %v", store.SettingIntakeEventWakes, err)
		return
	}
	idx := rt.eventIndexFor(ownerID, computerID)
	idx.mu.Lock()
	var wakes []eventWake
	loadErr := idx.loadLocked(ctx, rt.store, ownerID, computerID)
	if resolveErr != nil {
		report = eventResolveReport{}
		for _, in := range inputs {
			if loadErr != nil || idx.byCapture[strings.TrimSpace(in.Item.CaptureID)] == "" {
				report.Left = append(report.Left, in)
			}
		}
	}
	if loadErr != nil {
		// The index is unreadable: only the captures left go out.
		log.Printf("%v", loadErr)
		for _, req := range unresolvedRequests(ownerID, computerID, types.CaptureEventUnresolved, report.Left) {
			wakes = append(wakes, eventWake{req: req})
		}
	} else {
		wakes = rt.eventWakeRequests(ctx, ownerID, computerID, idx, report, setting.UpdatedAt, time.Now().UTC())
	}
	idx.mu.Unlock()
	failed := map[*eventView]bool{}
	var committed []eventWake
	for _, w := range wakes {
		if err := rt.commitCapturesRecord(ctx, w.req); err != nil && !errors.Is(err, store.ErrLifecycleCommandConflict) {
			log.Printf("event layer: %v", err)
			failed[w.settle] = true
			continue
		}
		committed = append(committed, w)
		rec := w.req.Records[0]
		log.Printf("event layer: %s: %d captures to management (held %v)", rec.RecordID, len(rec.Captures.Items), rec.Captures.Held)
	}
	idx.mu.Lock()
	for _, w := range committed {
		if w.settle != nil && !failed[w.settle] {
			w.settle.settled = true
		}
		if w.aged != nil {
			w.aged.agedWoken = true
		}
	}
	idx.mu.Unlock()
}

// The held-capture sweep (panel round 2: Gemini B1, Devin B2). A held
// arrival's captures can be stranded: a crash or a stop drops the queue
// before Jev resolves them, or the setting is turned off while they wait
// on their events. A source poll (a live act, never a boot) sweeps at most
// every eventSweepEvery: it reads the newest eventSweepScan records of
// management's inbox, and releases each held capture older than
// eventHeldGrace that no event record named and that is not waiting on an
// event, in records like the unresolved ones, oldest first, at most
// eventSweepMax a sweep. A released capture is never put to Jev again,
// so a capture that crashed resolution cannot crash it twice. With
// event_wakes on, a capture with a membership is waiting, not stranded;
// with it off, every unnamed held capture is released, which is the
// rollback's drain. A record a sweep mints names its captures, so the next
// sweep finds them named.
var (
	eventSweepEvery = 5 * time.Minute
	eventHeldGrace  = 10 * time.Minute
	eventSweepScan  = int64(1000)
	eventSweepMax   = 48
)

func (rt *Runtime) sweepHeldCaptures(ctx context.Context, ownerID, computerID string, now time.Time) {
	if rt == nil || rt.store == nil {
		return
	}
	eventsOn := rt.computerSettingOn(ctx, ownerID, computerID, store.SettingIntakeEvents)
	wakesOn := rt.computerSettingOn(ctx, ownerID, computerID, store.SettingIntakeEventWakes)
	if !eventsOn && !wakesOn {
		return
	}
	key := eventKey(ownerID, computerID)
	rt.events.mu.Lock()
	if rt.events.lastSweep == nil {
		rt.events.lastSweep = map[string]time.Time{}
	}
	if last, ok := rt.events.lastSweep[key]; rt.events.stopped || (ok && now.Sub(last) < eventSweepEvery) {
		rt.events.mu.Unlock()
		return
	}
	rt.events.mu.Unlock()

	management := agentprofile.Management + ":" + strings.TrimSpace(ownerID)
	head, err := rt.store.InboxHead(ctx, ownerID, computerID, management)
	if err != nil {
		log.Printf("event layer: sweep: head of %s: %v", management, err)
		return
	}
	recs, err := rt.store.ListInboxRecords(ctx, ownerID, computerID, management, max(0, head-eventSweepScan), head)
	if err != nil {
		log.Printf("event layer: sweep: %v", err)
		return
	}
	// A sweep counts once its scan read: a failed read retries at the
	// next poll (code review, Devin).
	rt.events.mu.Lock()
	rt.events.lastSweep[key] = now
	rt.events.mu.Unlock()
	named := map[string]bool{}
	for _, rec := range recs {
		if rec.Captures != nil && rec.Captures.Event != nil {
			for _, item := range rec.Captures.Items {
				named[item.CaptureID] = true
			}
		}
	}
	var waiting map[string]string
	if eventsOn && wakesOn {
		idx := rt.eventIndexFor(ownerID, computerID)
		idx.mu.Lock()
		if err := idx.loadLocked(ctx, rt.store, ownerID, computerID); err != nil {
			idx.mu.Unlock()
			log.Printf("event layer: sweep: %v", err)
			return
		}
		waiting = make(map[string]string, len(idx.byCapture))
		for c, e := range idx.byCapture {
			waiting[c] = e
		}
		idx.mu.Unlock()
	}
	var release []eventInput
	held, stranded := 0, 0
	for _, rec := range recs {
		if rec.Captures == nil || !rec.Captures.Held || rec.Captures.Event != nil {
			continue
		}
		at, err := time.Parse(time.RFC3339Nano, rec.Provenance.CommittedAt)
		if err != nil || now.Sub(at) > eventAgedHorizon {
			continue
		}
		for _, item := range rec.Captures.Items {
			if named[item.CaptureID] {
				continue
			}
			held++
			if waiting[item.CaptureID] != "" || now.Sub(at) < eventHeldGrace {
				continue
			}
			stranded++
			named[item.CaptureID] = true
			if len(release) < eventSweepMax {
				release = append(release, eventInput{SourceID: rec.Captures.SourceID, At: at, Item: sourcegraph.CapturedItem{CaptureID: item.CaptureID,
					Title: item.Title, URL: item.URL, PublishedAt: item.PublishedAt, Language: item.Language, Script: item.Script}})
			}
		}
	}
	reason := types.CaptureEventUnresolved
	if !wakesOn {
		reason = types.CaptureEventReleased
	}
	for _, req := range unresolvedRequests(ownerID, computerID, reason, release) {
		if err := rt.commitCapturesRecord(ctx, req); err != nil && !errors.Is(err, store.ErrLifecycleCommandConflict) {
			log.Printf("event layer: sweep: %v", err)
		}
	}
	rt.events.mu.Lock()
	if rt.events.tally == nil {
		rt.events.tally = map[string]yaegikernel.HeldCaptures{}
	}
	rt.events.tally[key] = yaegikernel.HeldCaptures{Waiting: held - stranded, Released: len(release), SweptAt: now.Format(time.RFC3339)}
	rt.events.mu.Unlock()
	if held > 0 {
		log.Printf("event layer: sweep: %d held captures unnamed, %d stranded, %d released", held, stranded, len(release))
	}
}

// heldCapturesTally is the last sweep's count, or nil before one ran.
func (rt *Runtime) heldCapturesTally(ownerID, computerID string) *yaegikernel.HeldCaptures {
	rt.events.mu.Lock()
	defer rt.events.mu.Unlock()
	t, ok := rt.events.tally[eventKey(ownerID, computerID)]
	if !ok {
		return nil
	}
	return &t
}
