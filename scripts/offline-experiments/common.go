// Command offline-experiments runs the replay experiments E1 to E3 of
// docs/evidence/offline-experiments-2026-11-24/README.md over flat files that
// scripts/offline-experiments/export wrote on the build VM. It reads files and
// writes files; it touches no database, calls no model and makes no network call.
//
//	go run ./scripts/offline-experiments e1 DATA OUT
//	go run ./scripts/offline-experiments e2 DATA OUT
//	go run ./scripts/offline-experiments e3 sample DATA OUT
//	go run ./scripts/offline-experiments e3 score OUT
package main

import (
	"bufio"
	"bytes"
	"encoding/csv"
	"fmt"
	"io"
	"math"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/yusefmosiah/go-choir/internal/outlet"
)

// The rules fixed in ledger claims C215 to C217 before any run.
const (
	groupCos     = 0.84 // centroid cosine to join an open group
	groupQuiet   = 48 * time.Hour
	groupMaxAge  = 48 * time.Hour
	coverCos     = 0.795 // a story passage this near a group centroid covers it
	frontSize    = 60
	leadRunes    = 2400
	epsilonQ     = 0.001
	defaultWindw = 3 * time.Hour
)

type capture struct {
	ID, Host, URLKey, CanonKey, SourceID, Title, TextHash, URL string
	Fetched                                                    time.Time
	Vec                                                        []float32
	Lead                                                       string
	Origin                                                     string
}

func must(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func openCSV(path string) (*csv.Reader, *os.File) {
	f, err := os.Open(path)
	must(err)
	r := csv.NewReader(bufio.NewReaderSize(f, 1<<20))
	r.FieldsPerRecord = -1
	r.LazyQuotes = true
	return r, f
}

func readCSV(path string, fn func(rec []string)) {
	r, f := openCSV(path)
	defer f.Close()
	for {
		rec, err := r.Read()
		if err == io.EOF {
			return
		}
		must(err)
		fn(rec)
	}
}

func parseTime(s string) time.Time {
	t, err := time.Parse(time.RFC3339Nano, s)
	if err != nil {
		return time.Time{}
	}
	return t.UTC()
}

func normalize(v []float32) []float32 {
	var s float64
	for _, x := range v {
		s += float64(x) * float64(x)
	}
	if s == 0 {
		return v
	}
	n := float32(1 / math.Sqrt(s))
	out := make([]float32, len(v))
	for i, x := range v {
		out[i] = x * n
	}
	return out
}

func parseVec(s string) []float32 {
	parts := strings.Split(strings.Trim(s, "[]"), ",")
	v := make([]float32, len(parts))
	for i, p := range parts {
		x, err := strconv.ParseFloat(strings.TrimSpace(p), 32)
		must(err)
		v[i] = float32(x)
	}
	return normalize(v)
}

func dot(a, b []float32) float64 {
	var s float32
	for i := range a {
		s += a[i] * b[i]
	}
	return float64(s)
}

// loadLeads reads bodies.bin: frames "H <hash> <n>\n" and n bytes (n = -1 for a missing file).
func loadLeads(path string) map[string]string {
	f, err := os.Open(path)
	must(err)
	defer f.Close()
	br := bufio.NewReaderSize(f, 1<<20)
	leads := map[string]string{}
	for {
		line, err := br.ReadBytes('\n')
		if err == io.EOF && len(line) == 0 {
			return leads
		}
		must(err)
		fs := bytes.Fields(line)
		if len(fs) != 3 || string(fs[0]) != "H" {
			must(fmt.Errorf("bodies: bad frame %q", line))
		}
		n, err := strconv.Atoi(string(fs[2]))
		must(err)
		if n < 0 {
			continue
		}
		buf := make([]byte, n)
		_, err = io.ReadFull(br, buf)
		must(err)
		leads[string(fs[1])] = leadOf(buf)
	}
}

// leadOf is the first passage of the page text: whole paragraphs (the capture
// projection joins blocks with a blank line) packed up to about 1,800 runes,
// clipped to 2,400. It follows internal/capturefts.Passages without importing it.
func leadOf(prefix []byte) string {
	text := strings.ToValidUTF8(string(prefix), "")
	var out []string
	n := 0
	for _, p := range strings.Split(text, "\n\n") {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		out = append(out, p)
		n += utf8.RuneCountInString(p)
		if n >= 1800 {
			break
		}
	}
	lead := strings.Join(out, "\n\n")
	if utf8.RuneCountInString(lead) > leadRunes {
		lead = string([]rune(lead)[:leadRunes])
	}
	return lead
}

// loadArrivals reads the export and returns the arrivals in arrival order:
// captures with a vector, minus a capture whose canonical key or text hash an
// earlier one has. dropped counts what the copy check removed.
func loadArrivals(dir string) (arr []capture, dropped int) {
	var caps []capture
	readCSV(filepath.Join(dir, "captures.csv"), func(r []string) {
		if len(r) < 10 {
			must(fmt.Errorf("captures: %d fields", len(r)))
		}
		caps = append(caps, capture{ID: r[0], Fetched: parseTime(r[1]), Host: r[3], URLKey: r[4], CanonKey: r[5],
			SourceID: r[7], Title: r[8], TextHash: strings.TrimPrefix(r[9], "sha256:")})
	})
	vecs := map[string][]float32{}
	readCSV(filepath.Join(dir, "vectors.csv"), func(r []string) {
		if len(r) >= 2 {
			vecs[r[0]] = parseVec(r[1])
		}
	})
	urls := map[string]string{}
	readCSV(filepath.Join(dir, "urls.csv"), func(r []string) {
		if len(r) >= 2 {
			urls[r[0]] = r[1]
		}
	})
	leads := loadLeads(filepath.Join(dir, "bodies.bin"))
	seenCanon, seenText := map[string]bool{}, map[string]bool{}
	for _, c := range caps {
		v := vecs[c.ID]
		if v == nil {
			continue
		}
		if (c.CanonKey != "" && seenCanon[c.CanonKey]) || (c.TextHash != "" && seenText[c.TextHash]) {
			dropped++
			continue
		}
		seenCanon[c.CanonKey], seenText[c.TextHash] = true, true
		c.Vec, c.URL, c.Lead = v, urls[c.ID], leads[c.TextHash]
		arr = append(arr, c)
	}
	assignOrigins(arr)
	return arr, dropped
}

// ---- origins: copied from the offline slice's rung 2 (lane/offline-slice-r0,
// internal/offlineslice/groups.go) and the event layer's wireOrigin
// (internal/agentcore/event_layer.go), so that this run uses the same rule.

var wireTag = regexp.MustCompile(`(?i)\((reuters|ap|afp|bloomberg|dpa|upi|ansa|efe|xinhua|kyodo|yonhap|pti|ians|tass|anadolu)\)|^\s*(reuters|afp|ap)\s+[-—–]`)

func wireOrigin(texts ...string) string {
	for _, t := range texts {
		head := []rune(t)
		if len(head) > 300 {
			head = head[:300]
		}
		if m := wireTag.FindStringSubmatch(string(head)); m != nil {
			name := strings.ToLower(m[1])
			if name == "" {
				name = strings.ToLower(m[2])
			}
			if name == "ap" {
				name = "associated-press"
			}
			return "wire:" + name
		}
	}
	return ""
}

var linkRe = regexp.MustCompile(`(?i)https?://(?:www\.\s)?(?:\s)?[^\s/]+[^\s]*`)

var shorteners = map[string]bool{
	"dlvr.it": true, "bit.ly": true, "t.co": true, "ow.ly": true, "tinyurl.com": true, "buff.ly": true, "lnkd.in": true,
	"ift.tt": true, "trib.al": true, "wp.me": true, "apne.ws": true, "reut.rs": true, "cnn.it": true, "nyti.ms": true,
	"flip.it": true, "goo.gl": true, "w.st": true, "f24.my": true, "aje.io": true, "bbc.in": true, "cnb.cx": true, "nyp.st": true, "econ.st": true, "politi.co": true, "is.gd": true, "shorturl.at": true, "fb.me": true, "youtu.be": true, "rb.gy": true,
}

func accountOf(host, path string) string {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if host == "bsky.app" && len(parts) >= 2 && parts[0] == "profile" {
		return "bsky.app/" + strings.ToLower(parts[1])
	}
	if len(parts) >= 1 && strings.HasPrefix(parts[0], "@") {
		return host + "/" + strings.ToLower(parts[0])
	}
	if len(parts) >= 2 && parts[0] == "users" {
		return host + "/@" + strings.ToLower(parts[1])
	}
	return host
}

func linkedOutlet(text, ownHost string) string {
	for _, m := range linkRe.FindAllString(text, 3) {
		m = strings.Join(strings.Fields(m), "")
		u, err := url.Parse(m)
		if err != nil {
			continue
		}
		h := strings.TrimPrefix(strings.ToLower(u.Hostname()), "www.")
		if h == "" || shorteners[h] || h == strings.TrimPrefix(ownHost, "www.") {
			continue
		}
		if o := outlet.Of(m); o != "" {
			return o
		}
	}
	return ""
}

func originOf(c capture, wire string) string {
	if wire != "" {
		return wire
	}
	if o := outlet.Of(c.URL); o != "" {
		return o
	}
	u, err := url.Parse(strings.TrimSpace(c.URL))
	if err != nil || u.Hostname() == "" {
		return strings.ToLower(c.Host)
	}
	host := strings.ToLower(u.Hostname())
	if o := linkedOutlet(c.Lead, host); o != "" {
		return o
	}
	return accountOf(host, u.Path)
}

var leadingTags = regexp.MustCompile(`^\s*((?:#\s?[\pL\pN_]+\s*){1,4})`)

func assignOrigins(arr []capture) {
	wire := make([]string, len(arr))
	base := make([]string, len(arr))
	labels := map[string]string{}
	for i, c := range arr {
		wire[i] = wireOrigin(c.Lead, c.Title)
		base[i] = originOf(c, wire[i])
		if !strings.HasPrefix(c.SourceID, "social:") || !strings.Contains(base[i], "/") {
			if lab, _, ok := strings.Cut(base[i], "."); ok && !strings.HasPrefix(base[i], "wire:") {
				labels[strings.ToLower(lab)] = base[i]
			}
		}
	}
	for i := range arr {
		arr[i].Origin = base[i]
		if wire[i] == "" && strings.HasPrefix(arr[i].SourceID, "social:") {
			if m := leadingTags.FindStringSubmatch(arr[i].Lead); m != nil {
				for _, tag := range strings.Split(m[1], "#") {
					l := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(tag), " ", ""))
					if o, ok := labels[l]; ok && l != "" {
						arr[i].Origin = o
						break
					}
				}
			}
		}
	}
}

// ---- grouping (rule of C215): arrival order, centroid cosine, 48 h quiet and 48 h age, no size cap.

type group struct {
	Opened, Last time.Time
	Members      []int // indexes into the arrivals slice, in arrival order
	sum          []float32
	centre       []float32
}

func (g *group) add(i int, a *capture) {
	g.Members = append(g.Members, i)
	g.Last = a.Fetched
	if g.sum == nil {
		g.sum = make([]float32, len(a.Vec))
	}
	for d, x := range a.Vec {
		g.sum[d] += x
	}
	g.centre = nil
}

func (g *group) centroid() []float32 {
	if g.centre == nil {
		g.centre = normalize(g.sum)
	}
	return g.centre
}

// groupArrivals groups the arrivals at idx (indexes into arr, in arrival order).
func groupArrivals(arr []capture, idx []int) []group {
	var groups []group
	var open []int
	for _, i := range idx {
		a := &arr[i]
		keep := open[:0]
		for _, gi := range open {
			g := &groups[gi]
			if a.Fetched.Sub(g.Last) <= groupQuiet && a.Fetched.Sub(g.Opened) <= groupMaxAge {
				keep = append(keep, gi)
			}
		}
		open = keep
		best, bestCos := -1, -2.0
		for _, gi := range open {
			if c := dot(a.Vec, groups[gi].centroid()); c > bestCos {
				best, bestCos = gi, c
			}
		}
		if best >= 0 && bestCos >= groupCos {
			groups[best].add(i, a)
			continue
		}
		g := group{Opened: a.Fetched}
		g.add(i, a)
		groups = append(groups, g)
		open = append(open, len(groups)-1)
	}
	return groups
}

func allIdx(n int) []int {
	out := make([]int, n)
	for i := range out {
		out[i] = i
	}
	return out
}

func originsOf(arr []capture, members []int, from, to time.Time) map[string]bool {
	m := map[string]bool{}
	for _, i := range members {
		t := arr[i].Fetched
		if t.After(from) && !t.After(to) {
			m[arr[i].Origin] = true
		}
	}
	return m
}

func trunc(s string, n int) string {
	s = strings.Join(strings.Fields(s), " ")
	r := []rune(s)
	if len(r) > n {
		return string(r[:n]) + "..."
	}
	return s
}

func wilson(k, n int) (lo, hi float64) {
	if n == 0 {
		return 0, 1
	}
	z := 1.96
	p := float64(k) / float64(n)
	d := 1 + z*z/float64(n)
	c := p + z*z/(2*float64(n))
	h := z * math.Sqrt(p*(1-p)/float64(n)+z*z/(4*float64(n)*float64(n)))
	return (c - h) / d, (c + h) / d
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: offline-experiments e1|e2|e3 ...")
		os.Exit(2)
	}
	switch os.Args[1] {
	case "e1":
		if len(os.Args) != 4 {
			fmt.Fprintln(os.Stderr, "usage: e1 DATA OUT")
			os.Exit(2)
		}
		runE1(os.Args[2], os.Args[3])
	case "e1b":
		runE1b(os.Args[2], os.Args[3])
	case "e2":
		if len(os.Args) != 4 {
			fmt.Fprintln(os.Stderr, "usage: e2 DATA OUT")
			os.Exit(2)
		}
		runE2(os.Args[2], os.Args[3])
	case "e3":
		runE3(os.Args[2:])
	default:
		fmt.Fprintln(os.Stderr, "usage: offline-experiments e1|e2|e3 ...")
		os.Exit(2)
	}
}
