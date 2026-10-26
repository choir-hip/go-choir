package platform

// The learned entity lexicon and its candidates read
// (docs/problems/related-stories-no-entity-layer-2026-10-26.md, slice c0 §1,
// §2 and §6). Corpusd keeps each owner's lexicon in memory: a token-trie
// gazetteer over every entity's name (and, with the learning writes, its
// aliases), with each entity's name, type and first-seen time. It is built
// from Store B on the owner's first request, so after every corpusd restart,
// and updated in place by every write of an entity row, so a mint is matched
// by the next request with no rebuild. The lexicon assumes one corpusd
// process writes Store B, as today; a second writer's mints would reach this
// process's lexicon only at its next restart.
//
// The read answers a story's candidates: the owner's entities the text names
// (by name, or by a near form that belongs to exactly one entity), with their
// world facts, and the phrases the text proposes that are not entities yet.
// Corpusd extracts from the bounded text itself with the shared package
// (internal/entitytext), so a proposal the text does not hold is never
// offered; from the computer's proposals it takes only their rarity in the
// computer's library. Nothing is written, and the answer carries names and
// titles, never story text.

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/yusefmosiah/go-choir/internal/entitytext"
	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/types"
)

const (
	// lexiconPageSize is one page of the lexicon's build read.
	lexiconPageSize = 500
	// candidateHitsForFacts bounds the hits whose edges are read to know
	// whether they are hubs, before ranking.
	candidateHitsForFacts = 48
)

// lexiconEntity is what the lexicon keeps of one entity.
type lexiconEntity struct {
	EntityID  string
	Canonical string
	Name      string
	Type      string
	FirstSeen time.Time
	Aliases   []string
}

// entityLexicon is one owner's learned lexicon.
type entityLexicon struct {
	ready chan struct{}
	err   error

	gaz *entitytext.Gazetteer

	mu       sync.RWMutex
	entities map[string]lexiconEntity     // entity id -> entity
	stop     map[string]entityStopwordRow // key -> an active stopword
}

func (x *entityLexicon) put(e lexiconEntity) {
	x.mu.Lock()
	if _, ok := x.entities[e.EntityID]; !ok {
		x.entities[e.EntityID] = e
	}
	x.mu.Unlock()
	x.gaz.Add(e.EntityID, e.Name)
	for _, a := range e.Aliases {
		x.gaz.Add(e.EntityID, a)
	}
}

// setStop records a stopword row's state: active rows stop their key.
func (x *entityLexicon) setStop(row entityStopwordRow) {
	x.mu.Lock()
	defer x.mu.Unlock()
	if row.Active {
		x.stop[row.Key] = row
	} else {
		delete(x.stop, row.Key)
	}
}

func (x *entityLexicon) stopped(key string) bool {
	x.mu.RLock()
	defer x.mu.RUnlock()
	_, ok := x.stop[key]
	return ok
}

// stops are the active stopwords, by key.
func (x *entityLexicon) stops() []entityStopwordRow {
	x.mu.RLock()
	defer x.mu.RUnlock()
	out := make([]entityStopwordRow, 0, len(x.stop))
	for _, r := range x.stop {
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Key < out[j].Key })
	return out
}

// all is every entity of the lexicon.
func (x *entityLexicon) all() []lexiconEntity {
	x.mu.RLock()
	defer x.mu.RUnlock()
	out := make([]lexiconEntity, 0, len(x.entities))
	for _, e := range x.entities {
		out = append(out, e)
	}
	return out
}

// sprawls reports alias sprawl (slice c0 §7): the entity's name contains,
// or is contained in, another entity's name.
func (x *entityLexicon) sprawls(e lexiconEntity) bool {
	key := entitytext.Key(e.Name)
	for _, id := range x.gaz.Near(key) {
		if id != e.EntityID {
			return true
		}
	}
	for _, h := range x.gaz.Match(entitytext.Parse("", e.Name), nil) {
		if h.EntityID != e.EntityID {
			return true
		}
	}
	return false
}

func (x *entityLexicon) entity(id string) (lexiconEntity, bool) {
	x.mu.RLock()
	defer x.mu.RUnlock()
	e, ok := x.entities[id]
	return e, ok
}

func (x *entityLexicon) size() types.EntityLexiconSize {
	entities, keys := x.gaz.Size()
	x.mu.RLock()
	defer x.mu.RUnlock()
	return types.EntityLexiconSize{Entities: entities, Keys: keys, Stopwords: len(x.stop)}
}

// entityLexicons holds the lexicons by owner.
type entityLexicons struct {
	mu      sync.Mutex
	byOwner map[string]*entityLexicon
	// writeMu serialises the read-modify-write of stopword rows.
	writeMu sync.Mutex
	// now is the learning clock; nil is time.Now. A test sets it.
	now func() time.Time
}

// entityLexicon returns owner's lexicon, building it from Store B on first
// use. Concurrent first requests share one build; a failed build is
// forgotten, so the next request tries again.
func (s *Service) entityLexicon(ctx context.Context, og *ObjectGraphStore, owner string) (*entityLexicon, error) {
	s.lexicons.mu.Lock()
	if s.lexicons.byOwner == nil {
		s.lexicons.byOwner = map[string]*entityLexicon{}
	}
	x, ok := s.lexicons.byOwner[owner]
	if !ok {
		x = &entityLexicon{ready: make(chan struct{}), gaz: entitytext.NewGazetteer(), entities: map[string]lexiconEntity{}, stop: map[string]entityStopwordRow{}}
		s.lexicons.byOwner[owner] = x
	}
	s.lexicons.mu.Unlock()
	if !ok {
		x.err = buildEntityLexicon(context.WithoutCancel(ctx), og, owner, x)
		if x.err != nil {
			s.lexicons.mu.Lock()
			if s.lexicons.byOwner[owner] == x {
				delete(s.lexicons.byOwner, owner)
			}
			s.lexicons.mu.Unlock()
			log.Printf("corpusd: entity lexicon of %s: %v", owner, x.err)
		}
		close(x.ready)
	}
	select {
	case <-x.ready:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	if x.err != nil {
		return nil, x.err
	}
	return x, nil
}

func buildEntityLexicon(ctx context.Context, og *ObjectGraphStore, owner string, x *entityLexicon) error {
	after := ""
	for {
		page, err := og.ListObjectsByMetadataPage(ctx, string(ogKindEntity), "", "", owner, after, lexiconPageSize)
		if err != nil {
			return fmt.Errorf("read entities: %w", err)
		}
		for _, obj := range page {
			if obj.Tombstone {
				continue
			}
			var meta struct {
				EntityID string   `json:"entity_id"`
				Name     string   `json:"name"`
				Type     string   `json:"type"`
				Aliases  []string `json:"aliases"`
			}
			if json.Unmarshal(obj.Metadata, &meta) != nil || meta.EntityID == "" || meta.Name == "" {
				continue
			}
			x.put(lexiconEntity{EntityID: meta.EntityID, Canonical: obj.CanonicalID, Name: meta.Name, Type: meta.Type, FirstSeen: obj.CreatedAt.UTC(), Aliases: meta.Aliases})
		}
		if len(page) < lexiconPageSize {
			break
		}
		after = page[len(page)-1].CanonicalID
	}
	after = ""
	for {
		page, err := og.ListObjectsByMetadataPage(ctx, string(ogKindEntityStopword), "", "", owner, after, lexiconPageSize)
		if err != nil {
			return fmt.Errorf("read stopwords: %w", err)
		}
		for _, obj := range page {
			var row entityStopwordRow
			if obj.Tombstone || json.Unmarshal(obj.Body, &row) != nil {
				continue
			}
			x.setStop(row)
		}
		if len(page) < lexiconPageSize {
			return nil
		}
		after = page[len(page)-1].CanonicalID
	}
}

// noteEntityWritten updates owner's lexicon in place after an entity row is
// written. A lexicon not built yet reads the row when it is.
func (s *Service) noteEntityWritten(owner string, e lexiconEntity) {
	s.lexicons.mu.Lock()
	x := s.lexicons.byOwner[owner]
	s.lexicons.mu.Unlock()
	if x != nil {
		x.put(e)
	}
}

// candidate is one candidate while the answer is assembled.
type candidate struct {
	types.EntityCandidate
	rankKey string
}

// EntityCandidates answers a story's entity candidates (slice c0 §2).
func (s *Service) EntityCandidates(ctx context.Context, req types.EntityCandidatesRequest) (types.EntityCandidatesResponse, error) {
	owner, computer := strings.TrimSpace(req.OwnerID), strings.TrimSpace(req.ComputerID)
	if owner == "" || computer == "" {
		return types.EntityCandidatesResponse{}, fmt.Errorf("owner_id and computer_id are required")
	}
	if n := len(req.Proposals); n > entitytext.MaxProposals {
		return types.EntityCandidatesResponse{}, fmt.Errorf("at most %d proposals, got %d", entitytext.MaxProposals, n)
	}
	for i, p := range req.Proposals {
		if utf8.RuneCountInString(p.Phrase) > types.EntityNameMaxRunes || len(p.Aliases) > entitytext.MaxProposals {
			return types.EntityCandidatesResponse{}, fmt.Errorf("proposal %d is over its bounds", i+1)
		}
	}
	tier := s.relationTier
	if tier == nil {
		tier = platformComputer
	}
	if platform, err := tier(computer); err != nil || !platform {
		return types.EntityCandidatesResponse{}, fmt.Errorf("entity candidates are answered for a platform computer only")
	}
	og := s.ogStore()
	if og == nil {
		return types.EntityCandidatesResponse{}, fmt.Errorf("entity candidates: store unavailable")
	}
	lex, err := s.entityLexicon(ctx, og, owner)
	if err != nil {
		return types.EntityCandidatesResponse{}, fmt.Errorf("entity candidates: %w", err)
	}

	rarity := map[string]float64{}
	for _, p := range req.Proposals {
		if k := entitytext.Key(p.Phrase); k != "" && p.Rarity > 0 {
			rarity[k] = p.Rarity
		}
	}
	doc := entitytext.Parse(req.Title, req.Text)
	hits := lex.gaz.Match(doc, nil)
	props := entitytext.Propose(doc, nil)

	byID := map[string]*candidate{}
	var order []*candidate
	addHit := func(id, key string, st entitytext.Stats, phrase string, ambiguous bool) *candidate {
		if c, ok := byID[id]; ok {
			return c
		}
		e, ok := lex.entity(id)
		if !ok {
			return nil
		}
		c := &candidate{rankKey: "e:" + id, EntityCandidate: types.EntityCandidate{
			Source: types.EntityCandidateExisting, EntityID: id, Name: e.Name, Type: e.Type, Phrase: phrase,
			Count: st.Count, Sentences: st.Sentences, InTitle: st.InTitle, InLead: st.InLead,
			Rarity: st.Rarity, Score: st.Score, Ambiguous: ambiguous,
		}}
		applyRarity(c, rarity[key])
		byID[id] = c
		order = append(order, c)
		return c
	}
	hitKeys := map[string][]string{} // a hit's key -> its entity ids
	for _, h := range hits {
		addHit(h.EntityID, h.Key, h.Stats, h.Phrase, h.Ambiguous)
		hitKeys[h.Key] = appendUnique(hitKeys[h.Key], h.EntityID)
	}

	var extracted []*candidate
	for _, p := range props {
		words := strings.Fields(p.Key)
		// The proposal is a hit's own form: the hit counts it already.
		if ids, ok := hitKeys[p.Key]; ok {
			for _, id := range ids {
				if c := byID[id]; c != nil {
					c.AliasCandidates = appendAliases(c, p.Aliases...)
				}
			}
			continue
		}
		// A hit's form sits inside the proposal ("Marta Kowalczyk" around
		// "Kowalczyk"): the proposal is a near form of that one entity.
		inner := map[string]bool{}
		for k, ids := range hitKeys {
			if affixWords(words, strings.Fields(k)) {
				for _, id := range ids {
					inner[id] = true
				}
			}
		}
		if len(inner) == 1 {
			for id := range inner {
				c := byID[id]
				c.AliasCandidates = appendAliases(c, append([]string{p.Phrase}, p.Aliases...)...)
			}
			continue
		}
		// The proposal is part of exactly one entity's name ("Zelenskyy" of
		// "Volodymyr Zelenskyy"): an existing candidate under that form.
		if len(inner) == 0 {
			if near := lex.gaz.Near(p.Key); len(near) == 1 {
				if c := addHit(near[0], p.Key, p.Stats, p.Phrase, false); c != nil {
					c.AliasCandidates = appendAliases(c, append([]string{p.Phrase}, p.Aliases...)...)
					continue
				}
			}
		}
		// The learned stoplist applies only here, after every entity form
		// is matched, so it never blocks an entity's name or alias (G7).
		if lex.stopped(p.Key) {
			continue
		}
		c := &candidate{rankKey: "p:" + p.Key, EntityCandidate: types.EntityCandidate{
			Source: types.EntityCandidateExtracted, Phrase: p.Phrase, Count: p.Count, Sentences: p.Sentences,
			InTitle: p.InTitle, InLead: p.InLead, Acronym: p.Acronym, Rarity: p.Rarity, Score: p.Score,
			AliasCandidates: append([]string(nil), p.Aliases...),
		}}
		applyRarity(c, rarity[p.Key])
		extracted = append(extracted, c)
	}

	// Hubs are known before ranking: the edges of the best hits are read.
	sort.SliceStable(order, func(i, j int) bool { return candidateBefore(order[i], order[j]) })
	if len(order) > candidateHitsForFacts {
		order = order[:candidateHitsForFacts]
	}
	tuning := s.related.tune()
	edgesOf := map[string][]objectgraph.Edge{}
	for _, c := range order {
		e, _ := lex.entity(c.EntityID)
		edges, err := entityMentions(ctx, og, e.Canonical)
		if err != nil {
			return types.EntityCandidatesResponse{}, err
		}
		edgesOf[c.EntityID] = edges
		stories, central := mentionCounts(edges)
		c.Facts = &types.EntityFacts{Stories: stories, CentralStories: central, FirstSeen: e.FirstSeen}
		c.Hub = central > tuning.HubEntityStories || len(edges) >= mentionEdgeLimit
	}

	all := append(append([]*candidate(nil), order...), extracted...)
	byRank := map[string]*candidate{}
	ranks := make([]entitytext.Candidate, 0, len(all))
	for _, c := range all {
		byRank[c.rankKey] = c
		ranks = append(ranks, entitytext.Candidate{Key: c.rankKey, InTitle: c.InTitle, Hub: c.Hub, Score: c.Score})
	}
	resp := types.EntityCandidatesResponse{Candidates: []types.EntityCandidate{}, Lexicon: lex.size()}
	for _, r := range entitytext.Rank(ranks, types.EntityCandidatesMax, types.EntityCandidatesHubSlots) {
		c := byRank[r.Key]
		if c.Source == types.EntityCandidateExisting {
			if err := s.fillEntityFacts(ctx, og, lex, c, edgesOf[c.EntityID]); err != nil {
				return types.EntityCandidatesResponse{}, err
			}
		}
		resp.Candidates = append(resp.Candidates, c.EntityCandidate)
	}
	return resp, nil
}

func candidateBefore(a, b *candidate) bool {
	if a.InTitle != b.InTitle {
		return a.InTitle
	}
	if a.Score != b.Score {
		return a.Score > b.Score
	}
	return a.rankKey < b.rankKey
}

// applyRarity weights a candidate by its rarity in the computer's library,
// when the computer sent one.
func applyRarity(c *candidate, r float64) {
	if r <= 0 {
		return
	}
	if c.Rarity > 0 {
		c.Score = c.Score / c.Rarity * r
	} else {
		c.Score *= r
	}
	c.Rarity = r
}

func appendUnique(xs []string, s string) []string {
	for _, x := range xs {
		if x == s {
			return xs
		}
	}
	return append(xs, s)
}

func appendAliases(c *candidate, forms ...string) []string {
	out := c.AliasCandidates
	for _, f := range forms {
		if f == "" || entitytext.Key(f) == entitytext.Key(c.Name) {
			continue
		}
		out = appendUnique(out, f)
	}
	return out
}

// affixWords reports whether short is a strict word prefix or suffix of
// long.
func affixWords(long, short []string) bool {
	if len(short) == 0 || len(short) >= len(long) {
		return false
	}
	pre, suf := true, true
	for i, w := range short {
		if long[i] != w {
			pre = false
		}
		if long[len(long)-len(short)+i] != w {
			suf = false
		}
	}
	return pre || suf
}

// entityMentions are an entity's live mention edges, at most mentionEdgeLimit.
func entityMentions(ctx context.Context, og *ObjectGraphStore, canonical string) ([]objectgraph.Edge, error) {
	live := false
	edges, err := og.ListEdges(ctx, objectgraph.EdgeFilter{ToID: canonical, Kind: edgeMentionsEntity, Tombstone: &live, Limit: mentionEdgeLimit})
	if err != nil {
		return nil, fmt.Errorf("entity candidates: mentions of %s: %w", canonical, err)
	}
	return edges, nil
}

// mentionCounts counts the stories with a live mention, and those central.
func mentionCounts(edges []objectgraph.Edge) (stories, central int) {
	all, cen := map[string]bool{}, map[string]bool{}
	for _, e := range edges {
		all[e.FromID] = true
		if mentionRole(e) == types.EntityRoleCentral {
			cen[e.FromID] = true
		}
	}
	return len(all), len(cen)
}

// fillEntityFacts adds an existing candidate's prior stories (published
// Autopaper stories, newest mention first) and the entities central to them
// too.
func (s *Service) fillEntityFacts(ctx context.Context, og *ObjectGraphStore, lex *entityLexicon, c *candidate, edges []objectgraph.Edge) error {
	sorted := append([]objectgraph.Edge(nil), edges...)
	sort.SliceStable(sorted, func(i, j int) bool {
		if !sorted[i].CreatedAt.Equal(sorted[j].CreatedAt) {
			return sorted[i].CreatedAt.After(sorted[j].CreatedAt)
		}
		return sorted[i].FromID < sorted[j].FromID
	})
	var pubs []string
	seen := map[string]bool{}
	for _, e := range sorted {
		if !seen[e.FromID] {
			seen[e.FromID] = true
			pubs = append(pubs, e.FromID)
		}
		if len(pubs) == 2*types.EntityFactsMaxStories {
			break
		}
	}
	if len(pubs) == 0 {
		return nil
	}
	infos, err := s.relatedPublications(ctx, og, seen)
	if err != nil {
		return err
	}
	self, _ := lex.entity(c.EntityID)
	tally := map[string]int{}
	for _, p := range pubs {
		info, ok := infos[p]
		if !ok || !info.eligible {
			continue
		}
		var meta frontPublicationMeta
		_ = json.Unmarshal(info.obj.Metadata, &meta)
		c.Facts.PriorStories = append(c.Facts.PriorStories, types.EntityStoryRef{PublicationID: meta.PublicationID, Title: meta.Title})
		live := false
		mentions, err := og.ListEdges(ctx, objectgraph.EdgeFilter{FromID: p, Kind: edgeMentionsEntity, Tombstone: &live, Limit: mentionEdgeLimit})
		if err != nil {
			return fmt.Errorf("entity candidates: entities of %s: %w", p, err)
		}
		for _, m := range mentions {
			if m.ToID != self.Canonical && mentionRole(m) == types.EntityRoleCentral {
				tally[m.ToID]++
			}
		}
		if len(c.Facts.PriorStories) == types.EntityFactsMaxStories {
			break
		}
	}
	byCanonical := lex.byCanonical()
	cos := make([]string, 0, len(tally))
	for id := range tally {
		if _, ok := byCanonical[id]; ok {
			cos = append(cos, id)
		}
	}
	sort.Slice(cos, func(i, j int) bool {
		if tally[cos[i]] != tally[cos[j]] {
			return tally[cos[i]] > tally[cos[j]]
		}
		return cos[i] < cos[j]
	})
	for _, id := range cos {
		if len(c.Facts.CoEntities) == types.EntityFactsMaxCoEntities {
			break
		}
		e := byCanonical[id]
		c.Facts.CoEntities = append(c.Facts.CoEntities, types.EntityRef{EntityID: e.EntityID, Name: e.Name, Type: e.Type})
	}
	return nil
}

func (x *entityLexicon) byCanonical() map[string]lexiconEntity {
	x.mu.RLock()
	defer x.mu.RUnlock()
	out := make(map[string]lexiconEntity, len(x.entities))
	for _, e := range x.entities {
		out[e.Canonical] = e
	}
	return out
}

// HandleStoryEntityCandidates serves POST types.StoryEntityCandidatesPath
// under the bound-guest rule of the entity write: it reads the owner's
// lexicon, so only that owner's own computer asks.
func (h *Handler) HandleStoryEntityCandidates(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}
	var req types.EntityCandidatesRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Error: "invalid request body"})
		return
	}
	if err := requireInternalOrBoundGuestWrite(r.Context(), h.guestLookup, r, req.OwnerID, req.ComputerID); err != nil {
		writeJSON(w, http.StatusForbidden, apiError{Error: err.Error()})
		return
	}
	resp, err := h.service.EntityCandidates(r.Context(), req)
	if err != nil {
		log.Printf("corpusd: entity candidates for %s: %v", strings.TrimSpace(req.ComputerID), err)
		writeJSON(w, http.StatusBadRequest, apiError{Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, resp)
}
