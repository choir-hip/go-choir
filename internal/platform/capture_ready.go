package platform

// The ready list (docs/design/entity-layer-2026-11-20.md section 4.1, station
// E2). The event flush already asks corpusd one question per flush: the
// nearest held captures to the captures it queued
// (POST /internal/platform/captures/neighbours). With `ready: true` the same
// request answers a different question first: which of this computer's
// captures are ready to be judged, and what do the judges need to know about
// each. A capture is ready when it has a stored vector, or when corpusd will
// never make one (the embedder refused it, or it is owner-private), and it
// has no row in proj_capture_resolution. A capture whose vector is still to
// come is waiting, and is counted, not listed.
//
// Rolling deploy: the guest and corpusd deploy at different moments, so the
// route is extended and not replaced. An old guest sends no `ready` and gets
// exactly the old answer. A new guest against an old corpusd sends a field the
// old corpusd ignores and gets no `ready` object back, which it reads as
// "this corpusd has no ready list". Both fields are additive on both sides.
//
// Nothing here calls a model, writes a row, or changes what the paper does.

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"golang.org/x/net/publicsuffix"

	"github.com/yusefmosiah/go-choir/internal/embedder"
	"github.com/yusefmosiah/go-choir/internal/entitytext"
	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// readyScanMax bounds the unresolved captures one request reads to find the
// ready ones and count the waiting. A variable so a test can move it.
var readyScanMax = 20000

const (
	// readyListDefaultLimit and readyListMaxLimit bound how many ready
	// captures one request lists: a bound on the size of an answer, not a
	// limit on the layer (the rest is the next request's, `more` says so).
	readyListDefaultLimit = 100
	readyListMaxLimit     = 500
	// readyCandidatesPerMention bounds one mention's shortlist: the question
	// shows these and not the whole lexicon.
	readyCandidatesPerMention = 8
	// readyGroupPool bounds the rows one lookup step reads for one mention
	// before the shortlist is ranked.
	readyGroupPool = 200
	// readyLeadRunes bounds the lead passage on the wire (the vector's unit is
	// about 1,800 runes).
	readyLeadRunes = 2400
)

// Vector states of a ready capture.
const (
	ReadyVectorStored   = "stored"   // it has a vector, and neighbours
	ReadyVectorRefused  = "refused"  // the embedder refused it; it will never have one
	ReadyVectorExcluded = "excluded" // owner-private: it never gets a vector
)

// ReadyLink is an entity a capture is linked to.
type ReadyLink struct {
	EntityID string  `json:"entity_id"`
	Role     string  `json:"role"`
	P        float64 `json:"p,omitempty"`
}

// ReadyNeighbour is one of a ready capture's nearest captures with the links
// the layer already holds for it: its event and its entities.
type ReadyNeighbour struct {
	CaptureID string      `json:"capture_id"`
	Score     float64     `json:"score"`
	EventID   string      `json:"event_id,omitempty"`
	Entities  []ReadyLink `json:"entities,omitempty"`
}

// ReadyCandidate is an existing entity a mention may be.
type ReadyCandidate struct {
	EntityID string   `json:"entity_id"`
	Name     string   `json:"name"`
	Type     string   `json:"type"`
	Aliases  []string `json:"aliases,omitempty"`
	// Via is the lookup that found it, strongest first: exact, near, subset,
	// initials, neighbour.
	Via string `json:"via"`
	// Mentions and Central are the live story mention edges of the entity and
	// those central (each capped at 500): its weight in the paper.
	Mentions int `json:"mentions"`
	Central  int `json:"central"`
	// Ambiguous: the name is one many things go by (see ambiguity).
	Ambiguous bool `json:"ambiguous,omitempty"`
	// InNeighbours: the entity is linked to one of the capture's neighbours.
	InNeighbours bool `json:"in_neighbours,omitempty"`
}

// ReadyMention is one name the capture's title or lead holds, with its
// shortlist. A mention with no candidates is a proposed new entity (New);
// type is for the judge to say.
type ReadyMention struct {
	Phrase  string  `json:"phrase"`
	Key     string  `json:"key"`
	InTitle bool    `json:"in_title,omitempty"`
	InLead  bool    `json:"in_lead,omitempty"`
	Count   int     `json:"count"`
	Score   float64 `json:"score"`
	Acronym bool    `json:"acronym,omitempty"`
	// Ambiguous mentions are never settled by a single confident candidate:
	// the judge is shown every one.
	Ambiguous    bool             `json:"ambiguous,omitempty"`
	AmbiguousWhy string           `json:"ambiguous_why,omitempty"`
	New          bool             `json:"new,omitempty"`
	Candidates   []ReadyCandidate `json:"candidates"`
}

// ReadyCapture is one capture that is ready to be judged.
type ReadyCapture struct {
	CaptureID string `json:"capture_id"`
	Title     string `json:"title"`
	Lead      string `json:"lead,omitempty"`
	// Outlet is the registrable domain of the capture's URL.
	Outlet    string `json:"outlet,omitempty"`
	Language  string `json:"language,omitempty"`
	FetchedAt string `json:"fetched_at,omitempty"`
	Vector    string `json:"vector"`
	// Neighbours repeat the response's neighbour list for this capture, with
	// event and entity links; the list at Neighbours[id] of the response is
	// the same captures without them.
	Neighbours []ReadyNeighbour `json:"neighbours,omitempty"`
	Mentions   []ReadyMention   `json:"mentions"`
}

// CaptureReadyList is the `ready` object of the neighbours answer.
type CaptureReadyList struct {
	Captures []ReadyCapture `json:"captures"`
	// Waiting counts this computer's unresolved captures that are not ready:
	// eligible for a vector the worker has not stored yet. It counts the ones
	// the scan reached (Scanned), newest first.
	Waiting int `json:"waiting"`
	Scanned int `json:"scanned"`
	// More: the list stopped at its limit, or the scan at its bound, with
	// captures unread.
	More bool `json:"more"`
	// CandidatesUnavailable says why the mentions carry no candidates (the
	// computer is not platform tier, or the lexicon could not be read).
	CandidatesUnavailable string `json:"candidates_unavailable,omitempty"`
}

// readyRow is one scanned capture.
type readyRow struct {
	id, title, host, language string
	fetched                   sql.NullTime
	vector                    string // "" when not ready
	waiting                   bool
}

// readyCaptures scans the computer's unresolved, live captures newest first
// (by fetch time, the order of idx_pci_fetched, so the scan stops where the
// limit does; a capture with no fetch time is last) and splits them into the
// ready (at most limit) and the waiting. A superseded or tombstoned capture is
// left out in the statement itself: it is never judged and has no resolution
// row, so counted here it would hold a place in the scan window for good.
//
// One statement, read as a stream and closed when the limit is reached or the
// scan bound is: a full scan of readyScanMax rows is one statement (a first
// version paged by OFFSET, 40 statements and about 410,000 index entries
// read for the same 20,000 rows).
func (s *Store) readyCaptures(ctx context.Context, owner, computer string, since time.Time, limit int) (ready []readyRow, waiting, scanned int, more bool, err error) {
	if limit <= 0 {
		return nil, 0, 0, true, nil
	}
	if err = s.ensureCaptureResolutionTable(ctx); err != nil {
		return
	}
	args := []any{embedder.ModelHash, owner, computer}
	cond := ""
	if !since.IsZero() {
		cond = ` AND v.fetched_at >= ?`
		args = append(args, since.UTC())
	}
	args = append(args, readyScanMax+1)
	q := `SELECT v.capture_id, v.title, v.host, v.language, v.fetched_at,
			vc.capture_id IS NOT NULL AS has_vec, (vc.capture_id IS NOT NULL AND vc.embedding IS NULL) AS refused,
			` + vecCaptureEligibleSQL + ` AS eligible
		FROM ` + captureIndexTable + ` v
		LEFT JOIN ` + vecCaptureTable + ` vc ON vc.capture_id = v.capture_id AND vc.model = ?
		WHERE v.owner_id = ? AND v.computer_id = ?` + cond + `
			AND EXISTS (SELECT 1 FROM og_objects o WHERE o.canonical_id = v.capture_id AND NOT o.tombstone AND COALESCE(o.superseded_by, '') = '')
			AND NOT EXISTS (SELECT 1 FROM ` + captureResolutionTable + ` r WHERE r.capture_id = v.capture_id)
		ORDER BY v.fetched_at DESC NULLS LAST, v.capture_id DESC
		LIMIT ?`
	rows, qerr := s.corpus().QueryContext(ctx, q, args...)
	if qerr != nil {
		return nil, 0, 0, false, qerr
	}
	defer rows.Close()
	for rows.Next() {
		if scanned == readyScanMax {
			// One row past the bound: the scan stopped with captures unread.
			return ready, waiting, scanned, true, nil
		}
		var r readyRow
		var hasVec, refused, eligible bool
		if serr := rows.Scan(&r.id, &r.title, &r.host, &r.language, &r.fetched, &hasVec, &refused, &eligible); serr != nil {
			return nil, 0, 0, false, serr
		}
		switch {
		case !eligible:
			r.vector = ReadyVectorExcluded
		case hasVec && refused:
			r.vector = ReadyVectorRefused
		case hasVec:
			r.vector = ReadyVectorStored
		default:
			r.waiting = true
		}
		if r.waiting {
			scanned++
			waiting++
			continue
		}
		if len(ready) == limit {
			return ready, waiting, scanned, true, nil
		}
		scanned++
		ready = append(ready, r)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, 0, false, err
	}
	return ready, waiting, scanned, false, nil
}

// readyLink is an edge of a capture read for the ready list.
type readyEdge struct {
	from, to, kind, role string
	p                    float64
}

const edgeInEvent = objectgraph.EdgeKind("in_event")

// captureLinkEdges reads the live entity and event edges that start at the
// named captures.
func (s *Store) captureLinkEdges(ctx context.Context, ids []string) ([]readyEdge, error) {
	var out []readyEdge
	for start := 0; start < len(ids); start += 500 {
		chunk := ids[start:min(start+500, len(ids))]
		args := make([]any, 0, len(chunk)+2)
		args = append(args, string(edgeMentionsEntity), string(edgeInEvent))
		for _, id := range chunk {
			args = append(args, id)
		}
		rows, err := s.corpus().QueryContext(ctx, `SELECT from_id, to_id, kind, metadata FROM og_edges
			WHERE kind IN (?, ?) AND NOT tombstone AND from_id IN (?`+strings.Repeat(",?", len(chunk)-1)+`)`, args...)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var e readyEdge
			var meta string
			if err := rows.Scan(&e.from, &e.to, &e.kind, &meta); err != nil {
				_ = rows.Close()
				return nil, err
			}
			var m struct {
				Role string  `json:"role"`
				P    float64 `json:"p"`
			}
			_ = json.Unmarshal([]byte(meta), &m)
			e.role, e.p = m.Role, m.P
			out = append(out, e)
		}
		err = rows.Err()
		_ = rows.Close()
		if err != nil {
			return nil, err
		}
	}
	return out, nil
}

// registrableDomain is a URL's outlet: its registrable domain by the public
// suffix list ("news.bbc.co.uk" is "bbc.co.uk"), or the host itself when the
// list has no answer (an address, a single label).
func registrableDomain(host string) string {
	host = strings.TrimPrefix(strings.ToLower(strings.TrimSpace(host)), "www.")
	if host == "" {
		return ""
	}
	if d, err := publicsuffix.EffectiveTLDPlusOne(host); err == nil {
		return d
	}
	return host
}

// hostOfURL is the host of raw, "" when it has none.
func hostOfURL(raw string) string {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return ""
	}
	return u.Hostname()
}

// CaptureReadyList answers a neighbours request with `ready` set: the ready
// captures of the computer with their neighbours and entity candidates, then
// the neighbours answer for them (and for any capture the request named).
func (s *Service) CaptureReadyList(ctx context.Context, req CaptureNeighboursRequest) (CaptureNeighboursResponse, error) {
	if s == nil || s.store == nil {
		return CaptureNeighboursResponse{}, fmt.Errorf("capture ready list: nil store")
	}
	owner, computer := strings.TrimSpace(req.OwnerID), strings.TrimSpace(req.ComputerID)
	if owner == "" || computer == "" {
		return CaptureNeighboursResponse{}, fmt.Errorf("%w: owner_id and computer_id are required", errBadNeighbours)
	}
	limit := req.ReadyLimit
	switch {
	case limit <= 0:
		limit = readyListDefaultLimit
	case limit > readyListMaxLimit:
		limit = readyListMaxLimit
	}
	var since time.Time
	if v := strings.TrimSpace(req.ReadySince); v != "" {
		for _, layout := range []string{time.RFC3339, "2006-01-02"} {
			if t, err := time.Parse(layout, v); err == nil {
				since = t
				break
			}
		}
		if since.IsZero() {
			return CaptureNeighboursResponse{}, fmt.Errorf("%w: ready_since %q is not a day or a time", errBadNeighbours, v)
		}
	}
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, captureNeighboursDeadline)
		defer cancel()
	}
	if !s.store.VectorsReady() {
		if err := s.store.ensureVectorTables(ctx); err != nil && !s.store.VectorsReady() {
			// Without the vector tables nothing can be called ready.
			res := CaptureNeighboursResponse{Neighbours: map[string][]CaptureNeighbour{}, Unavailable: "the vector tables are not ready"}
			return res, nil
		}
	}
	// The ids the request names are served as the old route served them. The
	// ready captures take the room that is left under the route's bound, so a
	// request the old route answered is never refused for asking the list too.
	seen := map[string]bool{}
	var ids []string
	for _, id := range req.CaptureIDs {
		if id = strings.TrimSpace(id); id != "" && !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
	}
	if room := captureNeighboursMaxIDs - len(ids); room < limit {
		limit = max(room, 0)
	}
	rows, waiting, scanned, more, err := s.store.readyCaptures(ctx, owner, computer, since, limit)
	if err != nil {
		return CaptureNeighboursResponse{}, err
	}
	nreq := req
	for _, r := range rows {
		if r.vector == ReadyVectorStored && !seen[r.id] {
			seen[r.id] = true
			ids = append(ids, r.id)
		}
	}
	nreq.CaptureIDs = ids
	res, err := s.store.CaptureNeighbours(ctx, nreq)
	if err != nil {
		return res, err
	}
	list, err := s.assembleReady(ctx, owner, computer, rows, res.Neighbours)
	if err != nil {
		return res, err
	}
	list.Waiting, list.Scanned, list.More = waiting, scanned, more
	res.Ready = list
	return res, nil
}

// assembleReady fills the ready captures: text from the capture objects,
// links of the neighbours, and the mentions with their candidates.
func (s *Service) assembleReady(ctx context.Context, owner, computer string, rows []readyRow, neighbours map[string][]CaptureNeighbour) (*CaptureReadyList, error) {
	list := &CaptureReadyList{Captures: []ReadyCapture{}}
	if len(rows) == 0 {
		return list, nil
	}
	ids := make([]string, len(rows))
	for i, r := range rows {
		ids[i] = r.id
	}
	objs, err := s.store.captureObjectsByIDAny(ctx, ids)
	if err != nil {
		return nil, err
	}
	// The links of every neighbour, one read.
	var nids []string
	nseen := map[string]bool{}
	for _, r := range rows {
		for _, n := range neighbours[r.id] {
			if !nseen[n.CaptureID] {
				nseen[n.CaptureID] = true
				nids = append(nids, n.CaptureID)
			}
		}
	}
	edges, err := s.store.captureLinkEdges(ctx, nids)
	if err != nil {
		return nil, err
	}
	// The lexicon, when this computer may read it.
	var lex *entityLexicon
	tier := s.relationTier
	if tier == nil {
		tier = platformComputer
	}
	if platform, terr := tier(computer); terr != nil || !platform {
		list.CandidatesUnavailable = "entity candidates are answered for a platform computer only"
	} else if og := s.ogStore(); og == nil {
		list.CandidatesUnavailable = "store unavailable"
	} else if x, lerr := s.entityLexicon(ctx, og, owner); lerr != nil {
		list.CandidatesUnavailable = "the entity lexicon could not be read"
	} else {
		lex = x
	}
	var byCanonical map[string]lexiconEntity
	if lex != nil {
		byCanonical = lex.byCanonical()
	}
	linksOf := map[string][]readyEdge{}
	for _, e := range edges {
		linksOf[e.from] = append(linksOf[e.from], e)
	}

	type pending struct {
		item  *ReadyCapture
		raws  []rawMention
		known map[string]bool // entity ids linked to this capture's neighbours
	}
	var pend []pending
	countIDs := map[string]bool{}
	for _, r := range rows {
		item := ReadyCapture{CaptureID: r.id, Title: strings.TrimSpace(r.title), Language: r.language, Vector: r.vector, Mentions: []ReadyMention{}}
		if r.fetched.Valid {
			item.FetchedAt = r.fetched.Time.UTC().Format(time.RFC3339)
		}
		item.Outlet = registrableDomain(r.host)
		if obj, ok := objs[r.id]; ok {
			if meta, merr := objectgraph.WebCaptureMetadataFromObject(obj); merr == nil {
				if t := strings.TrimSpace(meta.Title); t != "" {
					item.Title = t
				}
				if item.Outlet == "" {
					item.Outlet = registrableDomain(hostOfURL(meta.URL))
				}
				hash := strings.TrimPrefix(strings.TrimSpace(meta.ExtractedTextBlobID), "sha256:")
				if len(hash) != 64 {
					hash = ""
				}
				if lead, lerr := captureLeadPassage(s.captureBodies, hash); lerr == nil {
					item.Lead = clipRunes(lead, readyLeadRunes)
				}
			}
		}
		known := map[string]bool{}
		for _, n := range neighbours[r.id] {
			rn := ReadyNeighbour{CaptureID: n.CaptureID, Score: n.Score}
			for _, e := range linksOf[n.CaptureID] {
				switch objectgraph.EdgeKind(e.kind) {
				case edgeInEvent:
					rn.EventID = e.to
				case edgeMentionsEntity:
					if ent, ok := byCanonical[e.to]; ok {
						rn.Entities = append(rn.Entities, ReadyLink{EntityID: ent.EntityID, Role: e.role, P: e.p})
						if e.role == types.EntityRoleCentral || e.role == types.EntityRoleIncidental {
							known[ent.EntityID] = true
						}
					}
				}
			}
			sort.Slice(rn.Entities, func(i, j int) bool { return rn.Entities[i].EntityID < rn.Entities[j].EntityID })
			item.Neighbours = append(item.Neighbours, rn)
		}
		p := pending{item: nil, known: known}
		list.Captures = append(list.Captures, item)
		if lex != nil {
			p.raws = lex.mentionsFor(item.Title, item.Lead, known)
			for _, m := range p.raws {
				for _, c := range m.cands {
					if e, ok := lex.entity(c.id); ok {
						countIDs[e.Canonical] = true
					}
				}
			}
		}
		pend = append(pend, p)
	}
	for i := range pend {
		pend[i].item = &list.Captures[i]
	}
	if lex == nil {
		return list, nil
	}
	counts, err := s.entityMentionCounts(ctx, countIDs)
	if err != nil {
		return nil, err
	}
	for _, p := range pend {
		for _, m := range p.raws {
			p.item.Mentions = append(p.item.Mentions, lex.finishMention(m, counts, p.known))
		}
	}
	return list, nil
}

// ---- mentions and candidates, no model ----

// rawCand is a candidate before its weights are known.
type rawCand struct{ id, via string }

// rawMention is a mention and the entity ids its lookups found, in the
// design's order.
type rawMention struct {
	phrase, key string
	stats       entitytext.Stats
	acronym     bool
	hit         bool
	cands       []rawCand
	ambiguous   bool
	why         string
}

// notEntityWords are one-word forms that are not named things, however they
// are capitalised: demonyms and adjectives of place or party, and the common
// nouns the story layer's baseline found attached as entities
// (docs/evidence/entity-layer-baseline-hand-read-2026-11-20.md). The list is
// short on purpose: the long tail belongs to the judge's "not an entity" and
// the learned stoplist. A mention of one of these is neither looked up nor
// proposed as a new entity.
var notEntityWords = func() map[string]bool {
	m := map[string]bool{}
	for _, w := range []string{
		// demonyms and adjectives
		"american", "british", "canadian", "chinese", "democratic", "emirati", "european", "french", "german",
		"haitian", "indian", "iranian", "israeli", "italian", "japanese", "korean", "mexican", "palestinian",
		"russian", "saudi", "somali", "spanish", "turkish", "ukrainian", "african", "asian", "australian",
		"brazilian", "egyptian", "syrian", "polish", "irish", "scottish", "welsh", "swiss", "dutch",
		// common nouns, roles and fragments seen as entities
		"health", "pedestrian", "vote", "national", "officer", "women", "men", "energy", "city", "driver",
		"wife", "husband", "season", "match", "nothing", "lawyer", "bill", "opens", "channel", "communications",
		"americas", "spice", "ipo", "police", "officials", "voters", "residents", "students", "teachers",
		"workers", "doctors", "company", "government", "minister", "president", "federal", "state", "local",
	} {
		m[w] = true
	}
	return m
}()

// notEntityKey: a mention that is one of notEntityWords by itself.
func notEntityKey(key string) bool { return !strings.Contains(key, " ") && notEntityWords[key] }

// genericInstitutions are names of a kind of body, not of one: every country
// has a parliament and a supreme court. A mention of one is ambiguous
// whatever one entity the lexicon holds under it.
var genericInstitutions = func() map[string]bool {
	m := map[string]bool{}
	for _, w := range []string{
		"parliament", "supreme court", "city council", "high court", "senate", "congress", "legislature",
		"national assembly", "assembly", "council", "court", "ministry", "committee", "commission", "cabinet",
		"central bank", "federal court", "district court", "court of appeal", "court of appeals",
		"constitutional court", "house of representatives", "house of commons", "county council",
		"school board", "town council", "police", "army", "navy", "military", "defense ministry", "foreign ministry",
	} {
		m[w] = true
	}
	return m
}()

// ambiguity says whether a mention's name is one many things go by, and why:
// the key is filed under more than one entity, it is a generic institution
// name, or other entities' names are built on it (a one-word name two or more
// others hold: "Washington" in "George Washington", "Washington Post").
func (x *entityLexicon) ambiguity(key string, exact []string) (bool, string) {
	switch {
	case len(exact) > 1:
		return true, "the name is filed under more than one entity"
	case genericInstitutions[key]:
		return true, "a generic institution name: many countries and cities have one"
	case len(exact) == 1 && !strings.Contains(key, " ") && len(x.gaz.KeysWith(key)) >= 2:
		return true, "other entities' names are built on this name"
	}
	return false, ""
}

// mentionsFor reads a capture's title and lead and shortlists the existing
// entities each name may be. known are the entity ids linked to the capture's
// neighbours. Lookups, in this order (the judge leans to the first choice):
// the exact key, near forms, token subsets, initials, then the neighbours'
// entities whose names share a word with the mention.
func (x *entityLexicon) mentionsFor(title, lead string, known map[string]bool) []rawMention {
	doc := entitytext.Parse(title, lead)
	hits := x.gaz.Match(doc, nil)
	props := entitytext.Propose(doc, nil)

	byKey := map[string]*rawMention{}
	var order []string
	add := func(m rawMention) {
		if _, ok := byKey[m.key]; ok {
			return
		}
		byKey[m.key] = &m
		order = append(order, m.key)
	}
	for _, h := range hits {
		if notEntityKey(h.Key) {
			continue
		}
		add(rawMention{phrase: h.Phrase, key: h.Key, stats: h.Stats, hit: true})
	}
	var propKeys []string
	for _, p := range props {
		if _, ok := byKey[p.Key]; ok {
			continue
		}
		// The learned stoplist applies to what is proposed, never to an
		// entity's own name (as in the story layer's candidates read).
		if x.stopped(p.Key) || notEntityKey(p.Key) {
			continue
		}
		add(rawMention{phrase: p.Phrase, key: p.Key, stats: p.Stats, acronym: p.Acronym})
		propKeys = append(propKeys, p.Key)
	}
	// A hit's form inside a proposal ("Kowalczyk" in "Marta Kowalczyk") is
	// that proposal's near form: one mention, with the longer name.
	drop := map[string]bool{}
	for _, pk := range propKeys {
		for k, m := range byKey {
			if m.hit && affixWords(strings.Fields(pk), strings.Fields(k)) {
				drop[k] = true
			}
		}
	}

	var out []rawMention
	for _, key := range order {
		if drop[key] {
			continue
		}
		m := byKey[key]
		exact := x.gaz.Lookup(key)
		m.ambiguous, m.why = x.ambiguity(key, exact)
		seen := map[string]bool{}
		push := func(ids []string, via string) {
			if len(ids) > readyGroupPool {
				ids = ids[:readyGroupPool]
			}
			for _, id := range ids {
				if seen[id] {
					continue
				}
				if _, ok := x.entity(id); !ok {
					continue
				}
				seen[id] = true
				m.cands = append(m.cands, rawCand{id: id, via: via})
			}
		}
		push(exact, "exact")
		push(x.gaz.Near(key), "near")
		push(x.gaz.Subset(key), "subset")
		push(x.gaz.Within(key), "subset")
		push(x.gaz.Initials(key), "initials")
		// The neighbours' entities that share a word with the mention.
		words := strings.Fields(key)
		var near []string
		for id := range known {
			e, ok := x.entity(id)
			if !ok || seen[id] {
				continue
			}
			if sharesOpenWord(words, strings.Fields(entitytext.Key(e.Name))) {
				near = append(near, id)
			}
		}
		sort.Strings(near)
		push(near, "neighbour")
		if m.hit && len(m.cands) == 0 {
			continue // a hit that resolved to no live entity: nothing to ask
		}
		out = append(out, *m)
	}
	return out
}

// sharesOpenWord: the two names share a word that is not short or closed
// ("Hegseth" in "Pete Hegseth").
func sharesOpenWord(a, b []string) bool {
	for _, x := range a {
		if utf8.RuneCountInString(x) < 3 {
			continue
		}
		for _, y := range b {
			if x == y && !entitytextClosed(x) {
				return true
			}
		}
	}
	return false
}

func entitytextClosed(w string) bool { return !entitytext.Matchable(w) }

// finishMention ranks a mention's candidates (within each lookup, the more
// central and mentioned first), bounds the shortlist and fills the entity
// facts.
func (x *entityLexicon) finishMention(m rawMention, counts map[string]entityCount, known map[string]bool) ReadyMention {
	rm := ReadyMention{Phrase: m.phrase, Key: m.key, InTitle: m.stats.InTitle, InLead: m.stats.InLead,
		Count: m.stats.Count, Score: m.stats.Score, Acronym: m.acronym, Ambiguous: m.ambiguous, AmbiguousWhy: m.why,
		Candidates: []ReadyCandidate{}}
	rank := map[string]int{"exact": 0, "near": 1, "subset": 2, "initials": 3, "neighbour": 4}
	cs := append([]rawCand(nil), m.cands...)
	weight := func(id string) int {
		if e, ok := x.entity(id); ok {
			return counts[e.Canonical].central*1000 + counts[e.Canonical].mentions
		}
		return 0
	}
	sort.SliceStable(cs, func(i, j int) bool {
		if rank[cs[i].via] != rank[cs[j].via] {
			return rank[cs[i].via] < rank[cs[j].via]
		}
		wi, wj := weight(cs[i].id), weight(cs[j].id)
		if wi != wj {
			return wi > wj
		}
		return cs[i].id < cs[j].id
	})
	if len(cs) > readyCandidatesPerMention {
		cs = cs[:readyCandidatesPerMention]
	}
	for _, c := range cs {
		e, ok := x.entity(c.id)
		if !ok {
			continue
		}
		cnt := counts[e.Canonical]
		amb, _ := x.ambiguity(entitytext.Key(e.Name), x.gaz.Lookup(entitytext.Key(e.Name)))
		rm.Candidates = append(rm.Candidates, ReadyCandidate{
			EntityID: e.EntityID, Name: e.Name, Type: e.Type, Aliases: e.Aliases, Via: c.via,
			Mentions: cnt.mentions, Central: cnt.central, Ambiguous: amb || m.ambiguous, InNeighbours: known[e.EntityID],
		})
	}
	rm.New = len(rm.Candidates) == 0
	return rm
}

// ---- weights ----

// entityCount is an entity's live story mentions, each capped.
type entityCount struct{ mentions, central int }

const (
	entityCountCap     = 500
	entityCountTTL     = 10 * time.Minute
	entityCountMaxKept = 50000
)

type entityCountEntry struct {
	at time.Time
	entityCount
}

// entityMentionCounts reads the live story mention edges of the named entity
// rows (canonical ids), through a ten-minute cache so a flush a minute does
// not read a hub's edges again. A probe is two indexed reads capped at 500.
func (s *Service) entityMentionCounts(ctx context.Context, canonicals map[string]bool) (map[string]entityCount, error) {
	out := make(map[string]entityCount, len(canonicals))
	now := time.Now()
	var need []string
	s.readyCounts.mu.Lock()
	for id := range canonicals {
		if e, ok := s.readyCounts.m[id]; ok && now.Sub(e.at) < entityCountTTL {
			out[id] = e.entityCount
		} else {
			need = append(need, id)
		}
	}
	s.readyCounts.mu.Unlock()
	sort.Strings(need)
	for start := 0; start < len(need); start += 100 {
		chunk := need[start:min(start+100, len(need))]
		vals := make([]string, len(chunk))
		args := make([]any, 0, len(chunk))
		for i, id := range chunk {
			vals[i] = "(?::text)"
			args = append(args, id)
		}
		q := `SELECT i.id,
			(SELECT COUNT(*) FROM (SELECT 1 FROM og_edges e WHERE e.to_id = i.id AND e.kind = 'mentions_entity' AND NOT e.tombstone
				AND starts_with(e.from_id, 'obj:choir.publication:') LIMIT ` + fmt.Sprint(entityCountCap) + `) a),
			(SELECT COUNT(*) FROM (SELECT 1 FROM og_edges e WHERE e.to_id = i.id AND e.kind = 'mentions_entity' AND NOT e.tombstone
				AND starts_with(e.from_id, 'obj:choir.publication:') AND position('"role":"central"' in e.metadata) > 0 LIMIT ` + fmt.Sprint(entityCountCap) + `) b)
			FROM (VALUES ` + strings.Join(vals, ",") + `) AS i(id)`
		rows, err := s.store.corpus().QueryContext(ctx, q, args...)
		if err != nil {
			return nil, fmt.Errorf("entity mention counts: %w", err)
		}
		for rows.Next() {
			var id string
			var c entityCount
			if err := rows.Scan(&id, &c.mentions, &c.central); err != nil {
				_ = rows.Close()
				return nil, err
			}
			out[id] = c
		}
		err = rows.Err()
		_ = rows.Close()
		if err != nil {
			return nil, err
		}
	}
	s.readyCounts.mu.Lock()
	if s.readyCounts.m == nil || len(s.readyCounts.m) > entityCountMaxKept {
		s.readyCounts.m = map[string]entityCountEntry{}
	}
	for _, id := range need {
		s.readyCounts.m[id] = entityCountEntry{at: now, entityCount: out[id]}
	}
	s.readyCounts.mu.Unlock()
	return out, nil
}
