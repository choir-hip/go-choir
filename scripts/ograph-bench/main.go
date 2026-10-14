// Command ograph-bench generates synthetic Store B-shaped data (captures,
// edges, claims, entities, events) as CSV and times retrieval queries against
// a LOCAL dolt sql-server. It exists for
// docs/reports/object-graph-retrieval-research-2026-10-14.md.
//
// It never touches a platform database: run refuses any DSN whose host is
// not loopback.
//
//	go run ./scripts/ograph-bench gen -out DIR [-captures 1000000 ...]
//	go run ./scripts/ograph-bench run -dsn 'root@tcp(127.0.0.1:3399)/bench' -params DIR/params.json -suite S
package main

import (
	"crypto/sha256"
	"database/sql"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/go-sql-driver/mysql"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: ograph-bench gen|run ...")
		os.Exit(2)
	}
	switch os.Args[1] {
	case "gen":
		gen(os.Args[2:])
	case "run":
		run(os.Args[2:])
	case "postings":
		postings(os.Args[2:])
	default:
		fmt.Fprintln(os.Stderr, "usage: ograph-bench gen|run|postings ...")
		os.Exit(2)
	}
}

// ---------------------------------------------------------------- vocabulary

var (
	latinOnsets   = []string{"b", "c", "d", "f", "g", "h", "k", "l", "m", "n", "p", "r", "s", "t", "v", "w", "z", "br", "cr", "dr", "st", "tr", "pl", "gr", "sh", "ch"}
	latinVowels   = []string{"a", "e", "i", "o", "u", "ai", "ea", "ou", "io"}
	latinCodas    = []string{"", "", "n", "r", "s", "t", "l", "m", "nd", "rt", "st", "x"}
	cyrOnsets     = []string{"б", "в", "г", "д", "ж", "з", "к", "л", "м", "н", "п", "р", "с", "т", "ф", "х", "ч", "ш"}
	cyrVowels     = []string{"а", "е", "и", "о", "у", "ы", "я", "ю"}
	shortTokens   = []string{"AI", "EU", "UK", "US", "UN", "IQ", "5G", "F1", "GM", "VR"}
	commonEnglish = []string{"the", "of", "and", "in", "to", "for", "on", "after", "says", "new", "over", "with", "from", "report", "power", "strikes", "model", "release", "election", "court", "minister", "attack", "deal", "talks", "market", "launch", "war", "crisis", "data", "chip", "security", "energy", "price", "vote", "ban", "law", "plan", "rise", "fall", "record"}
)

type vocab struct {
	latin []string
	cyr   []string
	zipf  *rand.Zipf
	rng   *rand.Rand
}

func newVocab(rng *rand.Rand, n int) *vocab {
	v := &vocab{rng: rng}
	seen := map[string]bool{}
	for _, w := range commonEnglish {
		seen[w] = true
		v.latin = append(v.latin, w)
	}
	for len(v.latin) < n {
		syl := 1 + rng.Intn(3)
		var b strings.Builder
		for i := 0; i < syl; i++ {
			b.WriteString(latinOnsets[rng.Intn(len(latinOnsets))])
			b.WriteString(latinVowels[rng.Intn(len(latinVowels))])
			b.WriteString(latinCodas[rng.Intn(len(latinCodas))])
		}
		w := b.String()
		if len(w) < 3 || seen[w] {
			continue
		}
		seen[w] = true
		v.latin = append(v.latin, w)
	}
	for len(v.cyr) < n/8 {
		var b strings.Builder
		for i := 0; i < 2+rng.Intn(2); i++ {
			b.WriteString(cyrOnsets[rng.Intn(len(cyrOnsets))])
			b.WriteString(cyrVowels[rng.Intn(len(cyrVowels))])
		}
		v.cyr = append(v.cyr, b.String())
	}
	v.zipf = rand.NewZipf(rng, 1.07, 2, uint64(len(v.latin)-1))
	return v
}

func (v *vocab) word() string { return v.latin[v.zipf.Uint64()] }

func (v *vocab) cjk(n int) string {
	var b strings.Builder
	for i := 0; i < n; i++ {
		b.WriteRune(rune(0x4E00 + v.rng.Intn(3000)))
	}
	return b.String()
}

// sentence returns a title-like string in the given language.
func (v *vocab) sentence(lang string, words int) string {
	switch lang {
	case "zh", "ja":
		return v.cjk(words * 2)
	case "ru", "uk":
		parts := make([]string, words)
		for i := range parts {
			parts[i] = v.cyr[v.rng.Intn(len(v.cyr))]
		}
		return strings.Join(parts, " ")
	}
	parts := make([]string, words)
	for i := range parts {
		if v.rng.Intn(25) == 0 {
			parts[i] = shortTokens[v.rng.Intn(len(shortTokens))]
			continue
		}
		w := v.word()
		if i == 0 || v.rng.Intn(6) == 0 {
			w = strings.ToUpper(w[:1]) + w[1:]
		}
		parts[i] = w
	}
	return strings.Join(parts, " ")
}

func pickLang(rng *rand.Rand) string {
	r := rng.Intn(100)
	switch {
	case r < 70:
		return "en"
	case r < 76:
		return "de"
	case r < 81:
		return "fr"
	case r < 85:
		return "es"
	case r < 89:
		return "ru"
	case r < 92:
		return "uk"
	case r < 96:
		return "zh"
	case r < 98:
		return "ja"
	default:
		return "ar"
	}
}

func hashID(parts ...string) string {
	h := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	return hex.EncodeToString(h[:])
}

// ---------------------------------------------------------------------- gen

type params struct {
	Owner          string     `json:"owner"`
	PaperComputer  string     `json:"paper_computer"`
	CanonicalURLs  []string   `json:"canonical_urls"`
	CaptureIDs     []string   `json:"capture_ids"`
	CommonWords    []string   `json:"common_words"`
	MidWords       []string   `json:"mid_words"`
	RareWords      []string   `json:"rare_words"`
	Phrases        []string   `json:"phrases"`
	BlockWords     []string   `json:"block_words"`
	Speakers       []string   `json:"speakers"`
	BusySpeakers   []string   `json:"busy_speakers"`
	Propositions   []string   `json:"propositions"`
	Events         []string   `json:"events"`
	Aliases        []string   `json:"aliases"`
	AliasTypos     []string   `json:"alias_typos"`
	EntitySets     [][]string `json:"entity_sets"`
	EntitySetTimes []string   `json:"entity_set_times"`
	EndTime        string     `json:"end_time"`
}

type csvOut struct {
	f *os.File
	w *csv.Writer
}

func newCSV(dir, name string, header []string) *csvOut {
	f, err := os.Create(filepath.Join(dir, name))
	must(err)
	w := csv.NewWriter(f)
	must(w.Write(header))
	return &csvOut{f: f, w: w}
}

func (c *csvOut) row(rec ...string) { must(c.w.Write(rec)) }
func (c *csvOut) close() {
	c.w.Flush()
	must(c.w.Error())
	must(c.f.Close())
}

func must(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

const tsLayout = "2006-01-02 15:04:05"

func gen(args []string) {
	fs := flag.NewFlagSet("gen", flag.ExitOnError)
	out := fs.String("out", "", "output directory")
	nCap := fs.Int("captures", 1_000_000, "web captures")
	nOther := fs.Int("other-objects", 500_000, "non-capture og_objects rows")
	nEdges := fs.Int("edges", 10_000_000, "og_edges rows")
	nBlockCaps := fs.Int("block-captures", 100_000, "newest captures that get block text rows")
	nClaims := fs.Int("claims", 5_000_000, "claims")
	nEntities := fs.Int("entities", 500_000, "entities")
	nEvents := fs.Int("events", 300_000, "world events")
	nTrigramAliases := fs.Int("trigram-aliases", 200_000, "aliases indexed in the trigram table")
	seed := fs.Int64("seed", 20261014, "rng seed")
	must(fs.Parse(args))
	if *out == "" {
		must(fmt.Errorf("-out required"))
	}
	must(os.MkdirAll(*out, 0o755))
	rng := rand.New(rand.NewSource(*seed))
	v := newVocab(rng, 40_000)
	end := time.Date(2026, 10, 14, 15, 0, 0, 0, time.UTC)
	spanSec := int64(float64(*nCap) / 4500.0 * 86400) // ~4,500 captures/day
	p := params{Owner: "owner-paper", PaperComputer: "computer-paper", EndTime: end.Format(time.RFC3339)}
	owners := []string{"owner-paper", "owner-a", "owner-b", "owner-c", "owner-d"}
	computers := []string{"computer-paper"}
	for i := 0; i < 19; i++ {
		computers = append(computers, fmt.Sprintf("computer-%02d", i))
	}
	sources := make([]string, 40)
	hosts := make([]string, 40)
	for i := range sources {
		sources[i] = fmt.Sprintf("rss:source_%02d", i)
		hosts[i] = fmt.Sprintf("www.outlet%02d.example", i)
	}

	objects := newCSV(*out, "og_objects.csv", []string{"canonical_id", "object_kind", "owner_id", "computer_id", "version_id", "content_hash", "body_ref", "body_size", "metadata", "created_at", "updated_at", "tombstone", "superseded_by"})
	capIdx := newCSV(*out, "capture_index.csv", []string{"canonical_id", "owner_id", "computer_id", "url_hash", "canonical_url", "host", "source_id", "lang", "fetched_at", "published_at", "title", "title_ci"})
	blocks := newCSV(*out, "capture_blocks.csv", []string{"capture_id", "block_ord", "lang", "text"})

	captureIDs := make([]string, *nCap)
	captureTimes := make([]time.Time, *nCap)
	firstBlockCap := *nCap - *nBlockCaps
	for i := 0; i < *nCap; i++ {
		// i increases with time: capture i is fetched at end - (n-i)/n * span
		at := end.Add(-time.Duration(float64(spanSec)*float64(*nCap-i)/float64(*nCap)) * time.Second).Add(time.Duration(rng.Intn(60)) * time.Second)
		src := rng.Intn(len(sources))
		lang := pickLang(rng)
		title := v.sentence(lang, 6+rng.Intn(8))
		path := strings.ToLower(strings.ReplaceAll(v.sentence("en", 4), " ", "-"))
		curl := fmt.Sprintf("https://%s/%d/%s-%d", hosts[src], at.Year(), path, i)
		docHash := hashID("doc", strconv.Itoa(i))[:32]
		id := "autopaper:capture:" + curl + "#" + docHash
		captureIDs[i] = id
		captureTimes[i] = at
		owner, comp := "owner-paper", "computer-paper"
		if rng.Intn(10) >= 7 {
			owner = owners[1+rng.Intn(len(owners)-1)]
			comp = computers[1+rng.Intn(len(computers)-1)]
		}
		meta := map[string]any{
			"schema_version": "choir.web_capture.v1", "url": curl, "canonical_url": curl, "title": title,
			"fetched_at": at.Format(time.RFC3339Nano), "content_blob_id": "blob:" + hashID("c", id)[:40],
			"extracted_text_blob_id": "blob:" + hashID("t", id)[:40], "source_id": sources[src], "source_type": "rss",
			"item_id": hashID("item", id)[:16], "published_at": at.Add(-time.Duration(rng.Intn(7200)) * time.Second).Format(time.RFC3339Nano),
			"language": lang, "body_kind": "article", "blocks_blob_id": "blob:" + hashID("b", id)[:40], "extractor": "blocks.v3",
			"doc_hash": docHash, "block_count": 4 + rng.Intn(20), "media_type": "text/html", "raw_bytes": 20000 + rng.Intn(200000),
		}
		mb, _ := json.Marshal(meta)
		ts := at.Format(tsLayout)
		objects.row(id, "choir.web_capture", owner, comp, "v1", hashID("h", id)[:64], "cas/"+hashID("r", id)[:40], strconv.Itoa(1000+rng.Intn(60000)), string(mb), ts, ts, "0", "")
		capIdx.row(id, owner, comp, hashID(curl)[:32], curl, hosts[src], sources[src], lang, ts, at.Add(-time.Hour).Format(tsLayout), title, title)
		if i >= firstBlockCap {
			nb := 6 + rng.Intn(5)
			for b := 0; b < nb; b++ {
				blocks.row(id, strconv.Itoa(b), lang, v.sentence(lang, 30+rng.Intn(50)))
			}
		}
		if i%max(1, *nCap/200) == 0 && len(p.CanonicalURLs) < 200 {
			p.CanonicalURLs = append(p.CanonicalURLs, curl)
			p.CaptureIDs = append(p.CaptureIDs, id)
		}
		if i >= firstBlockCap && (i-firstBlockCap)%max(1, *nBlockCaps/400) == 0 && len(p.Phrases) < 100 && lang == "en" {
			words := strings.Fields(title)
			if len(words) >= 4 {
				p.Phrases = append(p.Phrases, strings.Join(words[1:4], " "))
			}
		}
	}
	otherKinds := []string{"choir.capture_label", "choir.support_judgement", "choir.publication_version", "choir.source_item", "choir.citation_check", "choir.texture_mirror"}
	for i := 0; i < *nOther; i++ {
		k := otherKinds[rng.Intn(len(otherKinds))]
		id := fmt.Sprintf("obj:%s:%s", k, hashID("o", strconv.Itoa(i))[:32])
		at := end.Add(-time.Duration(rng.Int63n(spanSec)) * time.Second).Format(tsLayout)
		meta := fmt.Sprintf(`{"run_id":"run-%d","subject":"%s"}`, rng.Intn(200000), captureIDs[rng.Intn(*nCap)])
		objects.row(id, k, owners[rng.Intn(len(owners))], computers[rng.Intn(len(computers))], "v1", hashID("h", id)[:64], "-", "0", meta, at, at, "0", "")
	}
	objects.close()
	capIdx.close()
	blocks.close()

	// Entities and aliases.
	ents := newCSV(*out, "entities.csv", []string{"entity_id", "qid", "entity_type", "label"})
	aliases := newCSV(*out, "entity_aliases.csv", []string{"alias_norm", "entity_id", "lang", "alias"})
	trig := newCSV(*out, "alias_trigrams.csv", []string{"gram", "alias_norm", "entity_id"})
	entityIDs := make([]string, *nEntities)
	trigSeen := map[string]bool{}
	nTrig := 0
	for i := 0; i < *nEntities; i++ {
		eid := fmt.Sprintf("ent:%08d", i)
		qid := ""
		if rng.Intn(3) > 0 {
			qid = fmt.Sprintf("Q%d", 100+i*7)
		}
		entityIDs[i] = eid
		label := strings.ToUpper(v.word()[:1]) + v.word()[1:] + " " + strings.ToUpper(v.word()[:1]) + v.word()[1:]
		types := []string{"person", "organization", "place", "product", "work", "law"}
		ents.row(eid, qid, types[rng.Intn(len(types))], label)
		langs := []string{"en", "de", "fr", "ru", "zh", "ar", "es", "uk"}
		na := 1 + rng.Intn(7)
		for a := 0; a < na; a++ {
			lang := langs[a%len(langs)]
			alias := label
			if a > 0 {
				alias = v.sentence(lang, 2)
			}
			norm := strings.ToLower(alias)
			key := norm + "\x00" + eid + "\x00" + lang
			if trigSeen[key] {
				continue
			}
			trigSeen[key] = true
			aliases.row(norm, eid, lang, alias)
			if a == 0 && nTrig < *nTrigramAliases {
				nTrig++
				for _, g := range trigrams(norm) {
					trig.row(g, norm, eid)
				}
				if len(p.Aliases) < 200 && nTrig%max(1, *nTrigramAliases/200) == 0 {
					p.Aliases = append(p.Aliases, norm)
					p.AliasTypos = append(p.AliasTypos, typo(rng, norm))
				}
			}
		}
	}
	ents.close()
	aliases.close()
	trig.close()

	// Events and participants. Entity popularity is Zipf (a few ministries
	// and companies appear everywhere).
	entZipf := rand.NewZipf(rng, 1.15, 4, uint64(*nEntities-1))
	evs := newCSV(*out, "world_events.csv", []string{"event_id", "start_at", "end_at", "label", "scale", "parent_event_id"})
	parts := newCSV(*out, "event_participants.csv", []string{"entity_id", "start_at", "event_id", "role"})
	eventIDs := make([]string, *nEvents)
	eventTimes := make([]time.Time, *nEvents)
	for i := 0; i < *nEvents; i++ {
		eid := fmt.Sprintf("ev:%08d", i)
		at := end.Add(-time.Duration(rng.Int63n(spanSec)) * time.Second)
		eventIDs[i] = eid
		eventTimes[i] = at
		parent := ""
		if rng.Intn(3) == 0 && i > 0 {
			parent = eventIDs[rng.Intn(i)]
		}
		evs.row(eid, at.Format(tsLayout), at.Add(time.Duration(1+rng.Intn(72))*time.Hour).Format(tsLayout), v.sentence("en", 5), "event", parent)
		np := 3 + rng.Intn(6)
		seen := map[uint64]bool{}
		var set []string
		for k := 0; k < np; k++ {
			e := entZipf.Uint64()
			if seen[e] {
				continue
			}
			seen[e] = true
			roles := []string{"agent", "patient", "place", "other"}
			parts.row(entityIDs[e], at.Format(tsLayout), eid, roles[rng.Intn(len(roles))])
			set = append(set, entityIDs[e])
		}
		if i%max(1, *nEvents/100) == 0 && len(p.EntitySets) < 100 {
			p.EntitySets = append(p.EntitySets, set)
			p.EntitySetTimes = append(p.EntitySetTimes, at.Format(tsLayout))
			p.Events = append(p.Events, eid)
		}
	}
	evs.close()
	parts.close()

	// Claims. Speakers Zipf over entities; propositions Zipf-shared.
	claims := newCSV(*out, "claims.csv", []string{"claim_id", "capture_id", "speaker_entity_id", "proposition_id", "event_id", "modality", "stance", "selection", "uttered_at", "fetched_at", "deadline", "settled", "lang", "gloss"})
	nProps := *nClaims / 3
	propZipf := rand.NewZipf(rng, 1.05, 3, uint64(nProps-1))
	speakerCount := map[string]int{}
	for i := 0; i < *nClaims; i++ {
		ci := *nCap - 1 - int(math.Abs(rng.NormFloat64())*float64(*nCap)/2.5)%*nCap
		if ci < 0 {
			ci = 0
		}
		cap := captureIDs[ci]
		at := captureTimes[ci]
		spk := entityIDs[entZipf.Uint64()]
		speakerCount[spk]++
		prop := fmt.Sprintf("prop:%08d", propZipf.Uint64())
		ev := eventIDs[rng.Intn(*nEvents)]
		r := rng.Intn(100)
		mod := "assertion"
		deadline := ""
		switch {
		case r < 7:
			mod = "forecast"
		case r < 13:
			mod = "commitment"
		case r < 17:
			mod = "intent"
		case r < 30:
			mod = "attribution"
		}
		if mod == "forecast" || mod == "commitment" {
			deadline = at.Add(time.Duration(rng.Intn(240*24)) * time.Hour).Format(tsLayout)
		}
		stance := []string{"affirms", "affirms", "affirms", "denies", "hedges"}[rng.Intn(5)]
		sel := "scheduled"
		if rng.Intn(5) == 0 {
			sel = "targeted"
		}
		settled := "0"
		if deadline != "" && rng.Intn(3) == 0 {
			settled = "1"
		}
		lang := pickLang(rng)
		claims.row(fmt.Sprintf("x:%s#c%d", hashID("x", cap)[:24], i), cap, spk, prop, ev, mod, stance, sel,
			at.Add(-time.Duration(rng.Intn(86400))*time.Second).Format(tsLayout), at.Format(tsLayout), deadline, settled, lang, v.sentence("en", 10+rng.Intn(10)))
		if i%max(1, *nClaims/100) == 0 && len(p.Propositions) < 100 {
			p.Propositions = append(p.Propositions, prop)
		}
	}
	claims.close()
	type sc struct {
		id string
		n  int
	}
	var scs []sc
	for k, n := range speakerCount {
		scs = append(scs, sc{k, n})
	}
	sort.Slice(scs, func(i, j int) bool { return scs[i].n > scs[j].n })
	for i := 0; i < 20 && i < len(scs); i++ {
		p.BusySpeakers = append(p.BusySpeakers, scs[i].id)
	}
	for i := 0; i < 100; i++ {
		p.Speakers = append(p.Speakers, scs[len(scs)/10+rng.Intn(len(scs)/2)].id)
	}

	// Edges: found_by / reports / refers_to / cites / covers / carried_by.
	edges := newCSV(*out, "og_edges.csv", []string{"edge_id", "from_id", "to_id", "kind", "metadata", "created_at", "tombstone"})
	edgeKinds := []string{"reports", "refers_to", "found_by", "cites", "covers", "carried_by", "derived_from", "returned"}
	for i := 0; i < *nEdges; i++ {
		k := edgeKinds[rng.Intn(len(edgeKinds))]
		ci := rng.Intn(*nCap)
		from := captureIDs[ci]
		var to string
		switch k {
		case "reports", "covers":
			to = eventIDs[rng.Intn(*nEvents)]
		case "refers_to":
			to = entityIDs[entZipf.Uint64()]
		case "carried_by":
			to = "outlet:" + hosts[rng.Intn(len(hosts))]
		default:
			to = captureIDs[rng.Intn(*nCap)]
		}
		edges.row(hashID("e", strconv.Itoa(i))[:40], from, to, k, `{"p":0.9,"judge":"system:extract"}`, captureTimes[ci].Format(tsLayout), "0")
	}
	edges.close()

	// Word ranks for FT queries (latin vocab is Zipf-ranked by index).
	for i := 0; i < 20; i++ {
		p.CommonWords = append(p.CommonWords, v.latin[14+i])              // very frequent content words
		p.MidWords = append(p.MidWords, v.latin[500+rng.Intn(2000)])      // mid frequency
		p.RareWords = append(p.RareWords, v.latin[20000+rng.Intn(19000)]) // rare
		p.BlockWords = append(p.BlockWords, v.latin[200+rng.Intn(3000)])
	}
	pb, _ := json.MarshalIndent(p, "", " ")
	must(os.WriteFile(filepath.Join(*out, "params.json"), pb, 0o644))
	fmt.Printf("generated %d captures (%d with blocks), %d other objects, %d edges, %d claims, %d entities (%d trigram aliases), %d events; span %.0f days\n",
		*nCap, *nBlockCaps, *nOther, *nEdges, *nClaims, *nEntities, nTrig, *nEvents, float64(spanSec)/86400)
}

func trigrams(s string) []string {
	r := []rune("  " + s + " ")
	seen := map[string]bool{}
	var out []string
	for i := 0; i+3 <= len(r); i++ {
		g := string(r[i : i+3])
		if !seen[g] {
			seen[g] = true
			out = append(out, g)
		}
	}
	return out
}

func longestWord(s string) string {
	best := ""
	for _, w := range strings.Fields(s) {
		if len([]rune(w)) > len([]rune(best)) {
			best = w
		}
	}
	return best
}

func typo(rng *rand.Rand, s string) string {
	r := []rune(s)
	if len(r) < 4 {
		return s
	}
	i := 1 + rng.Intn(len(r)-2)
	r[i], r[i+1] = r[i+1], r[i]
	return string(r)
}

// ---------------------------------------------------------------------- run

type query struct {
	name  string
	sql   string
	args  func(i int) []any
	iters int
}

func run(args []string) {
	fs := flag.NewFlagSet("run", flag.ExitOnError)
	dsn := fs.String("dsn", "", "LOCAL dolt sql-server DSN (loopback only)")
	paramsPath := fs.String("params", "", "params.json from gen")
	suite := fs.String("suite", "baseline", "baseline|indexed|ledger|fts|asof")
	only := fs.String("only", "", "comma-separated query names to run")
	itersOverride := fs.Int("iters", 0, "override iterations per query")
	must(fs.Parse(args))
	cfg, err := mysql.ParseDSN(*dsn)
	must(err)
	host := cfg.Addr
	if h, _, ok := strings.Cut(cfg.Addr, ":"); ok {
		host = h
	}
	if host != "127.0.0.1" && host != "localhost" && host != "::1" {
		must(fmt.Errorf("refusing non-loopback DSN host %q: this harness runs against a local dolt only", host))
	}
	raw, err := os.ReadFile(*paramsPath)
	must(err)
	var p params
	must(json.Unmarshal(raw, &p))
	db, err := sql.Open("mysql", *dsn)
	must(err)
	db.SetMaxOpenConns(1)
	defer db.Close()
	end, _ := time.Parse(time.RFC3339, p.EndTime)
	pick := func(xs []string, i int) string { return xs[i%len(xs)] }
	day := func(d int) string { return end.Add(-time.Duration(d) * 24 * time.Hour).Format(tsLayout) }
	inList := func(xs []string) (string, []any) {
		ph := strings.TrimSuffix(strings.Repeat("?,", len(xs)), ",")
		a := make([]any, len(xs))
		for i, x := range xs {
			a[i] = x
		}
		return ph, a
	}

	suites := map[string][]query{
		// Today's shapes against og_objects/og_edges with only the shipped indexes.
		"baseline": {
			{"newest1000_window", `SELECT canonical_id, object_kind, owner_id, computer_id, version_id, content_hash, body, body_ref, body_size, metadata, created_at, updated_at, tombstone, superseded_by FROM og_objects WHERE object_kind='choir.web_capture' AND owner_id=? AND computer_id=? ORDER BY updated_at DESC LIMIT 1000`, func(i int) []any { return []any{p.Owner, p.PaperComputer} }, 10},
			{"url_lookup_json", `SELECT canonical_id FROM og_objects WHERE object_kind='choir.web_capture' AND JSON_UNQUOTE(JSON_EXTRACT(CAST(metadata AS JSON), '$.canonical_url')) = ? LIMIT 1`, func(i int) []any { return []any{pick(p.CanonicalURLs, i)} }, 3},
			{"title_like_all", `SELECT canonical_id FROM og_objects WHERE object_kind='choir.web_capture' AND JSON_UNQUOTE(JSON_EXTRACT(CAST(metadata AS JSON), '$.title')) LIKE ? LIMIT 50`, func(i int) []any { return []any{"%" + pick(p.RareWords, i) + "%"} }, 3},
			{"get_by_id", `SELECT metadata FROM og_objects WHERE canonical_id = ?`, func(i int) []any { return []any{pick(p.CaptureIDs, i)} }, 50},
			{"edges_from", `SELECT edge_id, to_id, kind FROM og_edges WHERE from_id = ?`, func(i int) []any { return []any{pick(p.CaptureIDs, i)} }, 50},
			{"edges_to_kind", `SELECT from_id FROM og_edges WHERE to_id = ? AND kind = 'reports' LIMIT 200`, func(i int) []any { return []any{pick(p.Events, i)} }, 50},
			{"edges_kind_scan", `SELECT COUNT(*) FROM og_edges WHERE kind = 'found_by' AND created_at >= ?`, func(i int) []any { return []any{day(1)} }, 3},
		},
		// After typed/generated columns and composite indexes.
		"indexed": {
			{"newest1000_typed", `SELECT canonical_id, title FROM capture_index WHERE computer_id=? ORDER BY fetched_at DESC LIMIT 1000`, func(i int) []any { return []any{p.PaperComputer} }, 20},
			{"url_lookup_typed", `SELECT canonical_id FROM capture_index WHERE url_hash = ? AND canonical_url = ? ORDER BY fetched_at DESC LIMIT 1`, func(i int) []any { u := pick(p.CanonicalURLs, i); return []any{hashID(u)[:32], u} }, 100},
			{"newest1000_window_composite", `SELECT canonical_id, object_kind, owner_id, computer_id, version_id, content_hash, body, body_ref, body_size, metadata, created_at, updated_at, tombstone, superseded_by FROM og_objects WHERE object_kind='choir.web_capture' AND owner_id=? AND computer_id=? ORDER BY updated_at DESC LIMIT 1000`, func(i int) []any { return []any{p.Owner, p.PaperComputer} }, 20},
			// Dolt has no reverse index scan feeding a LIMIT: ORDER BY ... DESC
			// LIMIT N is a TopN over the whole equality range. A time lower
			// bound turns it into a short range.
			{"newest1000_typed_bounded_24h", `SELECT canonical_id, title FROM capture_index WHERE computer_id=? AND fetched_at >= ? ORDER BY fetched_at DESC LIMIT 1000`, func(i int) []any { return []any{p.PaperComputer, day(1)} }, 20},
			{"newest1000_window_composite_bounded_24h", `SELECT canonical_id, object_kind, owner_id, computer_id, version_id, content_hash, body, body_ref, body_size, metadata, created_at, updated_at, tombstone, superseded_by FROM og_objects WHERE object_kind='choir.web_capture' AND owner_id=? AND computer_id=? AND updated_at >= ? ORDER BY updated_at DESC LIMIT 1000`, func(i int) []any { return []any{p.Owner, p.PaperComputer, day(1)} }, 20},
			{"doc_hash_functional_index", `SELECT canonical_id FROM og_objects WHERE JSON_UNQUOTE(JSON_EXTRACT(metadata, '$.doc_hash')) = ? LIMIT 5`, func(i int) []any { id := pick(p.CaptureIDs, i); return []any{id[strings.LastIndex(id, "#")+1:]} }, 100},
			{"url_lookup_gencol", `SELECT canonical_id FROM og_objects WHERE og_meta_canonical_url = ? LIMIT 1`, func(i int) []any { return []any{pick(p.CanonicalURLs, i)} }, 100},
			{"window_source_day", `SELECT canonical_id, title FROM capture_index WHERE source_id = ? AND fetched_at >= ? ORDER BY fetched_at DESC LIMIT 200`, func(i int) []any { return []any{fmt.Sprintf("rss:source_%02d", i%40), day(1)} }, 50},
			{"edges_to_kind_composite", `SELECT from_id FROM og_edges WHERE to_id = ? AND kind = 'reports' LIMIT 200`, func(i int) []any { return []any{pick(p.Events, i)} }, 50},
			{"edges_kind_time_composite", `SELECT COUNT(*) FROM og_edges WHERE kind = 'found_by' AND created_at >= ?`, func(i int) []any { return []any{day(1)} }, 10},
		},
		// Full-text over typed columns (natural language mode only in Dolt 2.1.9).
		"fts": {
			{"title_ft_rare", `SELECT canonical_id FROM capture_index WHERE MATCH(title_ci) AGAINST(?) LIMIT 50`, func(i int) []any { return []any{pick(p.RareWords, i)} }, 20},
			{"title_ft_mid", `SELECT canonical_id FROM capture_index WHERE MATCH(title_ci) AGAINST(?) LIMIT 50`, func(i int) []any { return []any{pick(p.MidWords, i)} }, 20},
			{"title_ft_common", `SELECT canonical_id FROM capture_index WHERE MATCH(title_ci) AGAINST(?) LIMIT 50`, func(i int) []any { return []any{pick(p.CommonWords, i)} }, 10},
			{"title_ft_ranked_mid", `SELECT canonical_id, MATCH(title_ci) AGAINST(?) s FROM capture_index WHERE MATCH(title_ci) AGAINST(?) ORDER BY s DESC LIMIT 20`, func(i int) []any { w := pick(p.MidWords, i); return []any{w, w} }, 10},
			{"title_ft_two_words", `SELECT canonical_id, MATCH(title_ci) AGAINST(?) s FROM capture_index WHERE MATCH(title_ci) AGAINST(?) ORDER BY s DESC LIMIT 20`, func(i int) []any { w := pick(p.MidWords, i) + " " + pick(p.RareWords, i); return []any{w, w} }, 10},
			{"title_ft_recent", `SELECT canonical_id FROM capture_index WHERE MATCH(title_ci) AGAINST(?) AND fetched_at >= ? LIMIT 50`, func(i int) []any { return []any{pick(p.MidWords, i), day(7)} }, 10},
			// Dolt 2.1.9 uses the FULLTEXT index only when MATCH is the whole
			// WHERE clause; a CTE that is only the MATCH, joined back by
			// primary key, keeps the index (see EXPLAIN PLAN in the report).
			{"title_ft_recent_cte", `WITH m AS (SELECT canonical_id FROM capture_index WHERE MATCH(title_ci) AGAINST(?)) SELECT c.canonical_id FROM m JOIN capture_index c ON c.canonical_id = m.canonical_id WHERE c.fetched_at >= ? LIMIT 50`, func(i int) []any { return []any{pick(p.MidWords, i), day(7)} }, 10},
			{"title_like_typed_scan", `SELECT canonical_id FROM capture_index WHERE title_ci LIKE ? LIMIT 50`, func(i int) []any { return []any{"%" + pick(p.RareWords, i) + "%"} }, 5},
			{"block_ft_word", `SELECT DISTINCT capture_id FROM capture_blocks WHERE MATCH(text) AGAINST(?) LIMIT 50`, func(i int) []any { return []any{pick(p.BlockWords, i)} }, 10},
			{"block_phrase_ft_then_like", `SELECT DISTINCT capture_id FROM capture_blocks WHERE MATCH(text) AGAINST(?) AND text LIKE ? LIMIT 50`, func(i int) []any { ph := pick(p.Phrases, i); return []any{ph, "%" + ph + "%"} }, 10},
			{"block_phrase_cte", `WITH m AS (SELECT capture_id, block_ord FROM capture_blocks WHERE MATCH(text) AGAINST(?)) SELECT DISTINCT b.capture_id FROM m JOIN capture_blocks b ON b.capture_id = m.capture_id AND b.block_ord = m.block_ord WHERE b.text LIKE ? LIMIT 50`, func(i int) []any { ph := pick(p.Phrases, i); return []any{ph, "%" + ph + "%"} }, 10},
			// Natural-language MATCH ORs its words, so a phrase's common words
			// widen the candidate set; matching only the phrase's longest word
			// (a cheap stand-in for its rarest) and verifying with LIKE is the
			// shape a caller would actually ship.
			{"block_phrase_cte_longest_word", `WITH m AS (SELECT capture_id, block_ord FROM capture_blocks WHERE MATCH(text) AGAINST(?)) SELECT DISTINCT b.capture_id FROM m JOIN capture_blocks b ON b.capture_id = m.capture_id AND b.block_ord = m.block_ord WHERE b.text LIKE ? LIMIT 50`, func(i int) []any { ph := pick(p.Phrases, i); return []any{longestWord(ph), "%" + ph + "%"} }, 10},
			{"block_phrase_like_scan", `SELECT DISTINCT capture_id FROM capture_blocks WHERE text LIKE ? LIMIT 50`, func(i int) []any { return []any{"%" + pick(p.Phrases, i) + "%"} }, 3},
		},
		// The ledger: claims, entities, events.
		"ledger": {
			{"claims_by_speaker_window", `SELECT claim_id, proposition_id, fetched_at FROM claims WHERE speaker_entity_id = ? AND fetched_at >= ? ORDER BY fetched_at DESC LIMIT 100`, func(i int) []any { return []any{pick(p.Speakers, i), day(30)} }, 50},
			{"claims_by_busy_speaker_window", `SELECT claim_id, proposition_id, fetched_at FROM claims WHERE speaker_entity_id = ? AND fetched_at >= ? ORDER BY fetched_at DESC LIMIT 100`, func(i int) []any { return []any{pick(p.BusySpeakers, i), day(30)} }, 20},
			{"claims_by_proposition", `SELECT claim_id, speaker_entity_id, stance FROM claims WHERE proposition_id = ?`, func(i int) []any { return []any{pick(p.Propositions, i)} }, 50},
			{"proposition_support_by_outlet", `SELECT ci.host, COUNT(*) FROM claims c JOIN capture_index ci ON ci.canonical_id = c.capture_id WHERE c.proposition_id = ? AND c.stance = 'affirms' GROUP BY ci.host`, func(i int) []any { return []any{pick(p.Propositions, i)} }, 30},
			{"settlement_due", `SELECT claim_id, speaker_entity_id, deadline FROM claims WHERE settled = 0 AND modality IN ('forecast','commitment') AND deadline BETWEEN ? AND ? AND selection = 'scheduled' ORDER BY deadline LIMIT 500`, func(i int) []any { return []any{day(1), day(0)} }, 20},
			{"speaker_box_counts", `SELECT modality, settled, COUNT(*) FROM claims WHERE speaker_entity_id = ? GROUP BY modality, settled`, func(i int) []any { return []any{pick(p.BusySpeakers, i)} }, 10},
			{"alias_exact", `SELECT entity_id, lang FROM entity_aliases WHERE alias_norm = ?`, func(i int) []any { return []any{pick(p.Aliases, i)} }, 100},
			{"alias_prefix", `SELECT entity_id, alias FROM entity_aliases WHERE alias_norm LIKE ? LIMIT 20`, func(i int) []any { a := pick(p.Aliases, i); return []any{a[:min(5, len(a))] + "%"} }, 50},
			{"alias_trigram_fuzzy", "", nil, 30}, // built below
			{"same_event_candidates", "", nil, 30},
			{"story_sources_claims", `SELECT c.claim_id, c.speaker_entity_id, c.proposition_id FROM og_edges e JOIN claims c ON c.capture_id = e.from_id WHERE e.to_id = ? AND e.kind = 'reports' LIMIT 500`, func(i int) []any { return []any{pick(p.Events, i)} }, 30},
			// Same question; the plain JOIN above becomes a HashJoin over all of
			// claims, an IN subquery a MergeJoin over two full indexes; the CTE
			// becomes a LookupJoin.
			{"story_sources_claims_cte", `WITH caps AS (SELECT from_id FROM og_edges WHERE to_id = ? AND kind = 'reports') SELECT c.claim_id, c.speaker_entity_id, c.proposition_id FROM caps JOIN claims c ON c.capture_id = caps.from_id LIMIT 500`, func(i int) []any { return []any{pick(p.Events, i)} }, 30},
			{"event_timeline_for_entity", `SELECT ep.event_id, ep.start_at FROM event_participants ep WHERE ep.entity_id = ? AND ep.start_at >= ? ORDER BY ep.start_at DESC LIMIT 100`, func(i int) []any { return []any{pick(p.Speakers, i), day(90)} }, 50},
		},
	}
	// Fuzzy alias: count shared trigrams of the typo against the trigram table.
	for qi, q := range suites["ledger"] {
		switch q.name {
		case "alias_trigram_fuzzy":
			suites["ledger"][qi].args = func(i int) []any {
				g := trigrams(pick(p.AliasTypos, i))
				a := make([]any, len(g))
				for k := range g {
					a[k] = g[k]
				}
				return a
			}
			suites["ledger"][qi].sql = "dynamic"
		case "same_event_candidates":
			suites["ledger"][qi].sql = "dynamic"
		}
	}
	asOf := []query{
		{"claims_speaker_head", `SELECT COUNT(*) FROM claims WHERE speaker_entity_id = ?`, func(i int) []any { return []any{pick(p.BusySpeakers, i)} }, 10},
		{"claims_speaker_asof_c1", `SELECT COUNT(*) FROM claims AS OF 'c1' WHERE speaker_entity_id = ?`, func(i int) []any { return []any{pick(p.BusySpeakers, i)} }, 10},
		{"claim_history_row", `SELECT commit_hash, settled, commit_date FROM dolt_history_claims WHERE claim_id = ?`, nil, 5},
	}
	suites["asof"] = asOf

	qs, ok := suites[*suite]
	if !ok {
		must(fmt.Errorf("unknown suite %q", *suite))
	}
	want := map[string]bool{}
	for _, n := range strings.Split(*only, ",") {
		if n != "" {
			want[n] = true
		}
	}
	if *suite == "asof" {
		var cid string
		_ = db.QueryRow(`SELECT claim_id FROM claims WHERE settled = 1 LIMIT 1`).Scan(&cid)
		qs[2].args = func(int) []any { return []any{cid} }
	}
	fmt.Printf("%-34s %6s %10s %10s %10s %8s\n", "query", "iters", "p50_ms", "p95_ms", "max_ms", "rows")
	for _, q := range qs {
		if len(want) > 0 && !want[q.name] {
			continue
		}
		iters := q.iters
		if *itersOverride > 0 {
			iters = *itersOverride
		}
		var durs []float64
		rowsSeen := 0
		var qerr error
		for i := 0; i < iters; i++ {
			text := q.sql
			var a []any
			switch q.name {
			case "alias_trigram_fuzzy":
				a = q.args(i)
				ph := strings.TrimSuffix(strings.Repeat("?,", len(a)), ",")
				text = `SELECT entity_id, alias_norm, COUNT(*) n FROM alias_trigrams WHERE gram IN (` + ph + `) GROUP BY entity_id, alias_norm ORDER BY n DESC LIMIT 10`
			case "same_event_candidates":
				set := p.EntitySets[i%len(p.EntitySets)]
				t, _ := time.Parse(tsLayout, p.EntitySetTimes[i%len(p.EntitySetTimes)])
				ph, ea := inList(set)
				text = `SELECT event_id, COUNT(*) n_shared FROM event_participants WHERE entity_id IN (` + ph + `) AND start_at BETWEEN ? AND ? GROUP BY event_id ORDER BY n_shared DESC LIMIT 20`
				a = append(ea, t.Add(-72*time.Hour).Format(tsLayout), t.Add(72*time.Hour).Format(tsLayout))
			default:
				a = q.args(i)
			}
			start := time.Now()
			rows, err := db.Query(text, a...)
			if err != nil {
				qerr = err
				break
			}
			n := 0
			for rows.Next() {
				n++
			}
			rows.Close()
			durs = append(durs, float64(time.Since(start).Microseconds())/1000)
			rowsSeen += n
		}
		if qerr != nil {
			fmt.Printf("%-34s ERROR %v\n", q.name, qerr)
			continue
		}
		sort.Float64s(durs)
		pct := func(f float64) float64 {
			return durs[int(math.Min(float64(len(durs)-1), math.Ceil(f*float64(len(durs)))-1))]
		}
		fmt.Printf("%-34s %6d %10.2f %10.2f %10.2f %8.1f\n", q.name, len(durs), pct(0.5), pct(0.95), durs[len(durs)-1], float64(rowsSeen)/float64(len(durs)))
	}
}
