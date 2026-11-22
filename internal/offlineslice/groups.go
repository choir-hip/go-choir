package offlineslice

import (
	"math"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/outlet"
)

// LinkMode says how an arrival is compared with a group.
type LinkMode int

const (
	// LinkCentroid: cosine to the group's centroid.
	LinkCentroid LinkMode = iota
	// LinkAnyMember: the best cosine to any member (single link).
	LinkAnyMember
)

// GItem is one arrival to group.
type GItem struct {
	At  time.Time
	Vec []float32
}

// Group is a set of arrivals about one thing, built incrementally.
type Group struct {
	Opened  time.Time
	Last    time.Time
	Members []int // indexes into the items, in arrival order
	sum     []float32
	centre  []float32
}

func (g *Group) add(i int, it GItem) {
	g.Members = append(g.Members, i)
	g.Last = it.At
	if g.sum == nil {
		g.sum = make([]float32, len(it.Vec))
	}
	for d, x := range it.Vec {
		g.sum[d] += x
	}
	g.centre = normalize(g.sum)
}

func dot(a, b []float32) float64 {
	var s float32
	for i := range a {
		s += a[i] * b[i]
	}
	return float64(s)
}

// GroupArrivals groups items (in arrival order) incrementally. An arrival
// joins the open group it is nearest to when that cosine is at least G, else
// it opens a group. A group closes when closeAfter passes with no new member;
// a closed group is not joined. Nothing after an arrival is read.
func GroupArrivals(items []GItem, G float64, mode LinkMode, closeAfter time.Duration) []Group {
	return GroupArrivalsCapped(items, G, mode, closeAfter, 0, 0)
}

// GroupArrivalsCapped is GroupArrivals with two bounds on chaining: a group
// takes no member once it is maxAge old (0: no bound) or holds maxSize members
// (0: no bound).
func GroupArrivalsCapped(items []GItem, G float64, mode LinkMode, closeAfter, maxAge time.Duration, maxSize int) []Group {
	var groups []Group
	var open []int // indexes into groups
	for i, it := range items {
		keep := open[:0]
		for _, gi := range open {
			g := &groups[gi]
			if it.At.Sub(g.Last) <= closeAfter && (maxAge == 0 || it.At.Sub(g.Opened) <= maxAge) && (maxSize == 0 || len(g.Members) < maxSize) {
				keep = append(keep, gi)
			}
		}
		open = keep
		best, bestCos := -1, -2.0
		for _, gi := range open {
			g := &groups[gi]
			var c float64
			switch mode {
			case LinkAnyMember:
				c = -2
				for _, m := range g.Members {
					if s := dot(it.Vec, items[m].Vec); s > c {
						c = s
					}
				}
			default:
				c = dot(it.Vec, g.centre)
			}
			if c > bestCos {
				best, bestCos = gi, c
			}
		}
		if best >= 0 && bestCos >= G {
			groups[best].add(i, it)
			continue
		}
		g := Group{Opened: it.At}
		g.add(i, it)
		groups = append(groups, g)
		open = append(open, len(groups)-1)
	}
	return groups
}

// BestThreshold picks the cosine T that best separates pos (arrivals that
// should count as matched) from neg: matched means cosine >= T, and the chosen
// T maximises Youden's J = share of pos matched - share of neg matched. It
// returns T, the share of pos matched and the share of neg matched.
func BestThreshold(pos, neg []float64) (T, tpr, fpr float64) {
	cands := append(append([]float64(nil), pos...), neg...)
	sort.Float64s(cands)
	bestJ := math.Inf(-1)
	for _, c := range cands {
		var p, n int
		for _, x := range pos {
			if x >= c {
				p++
			}
		}
		for _, x := range neg {
			if x >= c {
				n++
			}
		}
		tp, fp := float64(p)/float64(len(pos)), float64(n)/float64(len(neg))
		if j := tp - fp; j > bestJ+1e-12 || (math.Abs(j-bestJ) <= 1e-12 && c > T) {
			bestJ, T, tpr, fpr = j, c, tp, fp
		}
	}
	return
}

// Knee returns the index of the point of the curve farthest from the straight
// line between its ends, after scaling both axes to [0,1] (the "kneedle"
// reading of a knee).
func Knee(xs, ys []float64) int {
	n := len(xs)
	if n < 3 {
		return 0
	}
	x0, x1, y0, y1 := xs[0], xs[n-1], ys[0], ys[n-1]
	best, bi := -1.0, 0
	for i := range xs {
		x := (xs[i] - x0) / (x1 - x0)
		y := (ys[i] - y0) / (y1 - y0)
		if d := math.Abs(y - x); d > best {
			best, bi = d, i
		}
	}
	return bi
}

// ReachTime is the time from the first member until the members' distinct
// origins number n; ok is false when they never do. ats are in arrival order.
func ReachTime(ats []time.Time, origins []string, n int) (time.Duration, bool) {
	seen := map[string]bool{}
	for i, o := range origins {
		seen[o] = true
		if len(seen) >= n {
			return ats[i].Sub(ats[0]), true
		}
	}
	return 0, false
}

var linkRe = regexp.MustCompile(`(?i)https?://(?:www\.\s)?(?:\s)?[^\s/]+[^\s]*`)

// shorteners cannot be resolved from stored data to the publisher.
var shorteners = map[string]bool{
	"dlvr.it": true, "bit.ly": true, "t.co": true, "ow.ly": true, "tinyurl.com": true, "buff.ly": true, "lnkd.in": true,
	"ift.tt": true, "trib.al": true, "wp.me": true, "apne.ws": true, "reut.rs": true, "cnn.it": true, "nyti.ms": true,
	"flip.it": true, "goo.gl": true, "w.st": true, "f24.my": true, "aje.io": true, "bbc.in": true, "cnb.cx": true, "nyp.st": true, "econ.st": true, "politi.co": true, "is.gd": true, "shorturl.at": true, "fb.me": true, "youtu.be": true, "rb.gy": true,
}

// accountOf is the account a social post URL names, with its instance.
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

// linkedOutlet is the outlet of the first article link in a post's text, or "".
func linkedOutlet(text, ownHost string) string {
	for _, m := range linkRe.FindAllString(text, 3) {
		m = strings.Join(strings.Fields(m), "") // the extractor splits links with spaces
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

// OriginOf is a capture's distinct origin, with no model: a wire copy is its
// wire service (the event layer's wireOrigin, passed in as wire); a capture
// whose URL names an outlet is that outlet (internal/outlet, the rule the
// event layer and the publish gate use); a social post is the outlet of the
// article it links when that link can be read from the stored text, else the
// account (instance and handle).
func OriginOf(c Capture, rawURL string, wire string) string {
	if wire != "" {
		return wire
	}
	if o := outlet.Of(rawURL); o != "" {
		return o
	}
	u, err := url.Parse(strings.TrimSpace(rawURL))
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

// OriginOfRelays is OriginOf with one more rule for relays: a post that opens
// with a hashtag naming an outlet (labels maps an outlet's name, lower case
// without spaces, to its outlet key) is that outlet, however the account is
// named and whether or not the link resolves. Relay accounts say whom they
// relay in their first hashtag ("# BBC # News ...").
func OriginOfRelays(c Capture, rawURL string, wire string, labels map[string]string) string {
	if wire == "" && strings.HasPrefix(c.SourceID, "social:") {
		if m := leadingTags.FindStringSubmatch(c.Lead); m != nil {
			for _, tag := range strings.Split(m[1], "#") {
				l := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(tag), " ", ""))
				if o, ok := labels[l]; ok && l != "" {
					return o
				}
			}
		}
	}
	return OriginOf(c, rawURL, wire)
}
