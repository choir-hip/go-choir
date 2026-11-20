package searchquery

import (
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"
)

// NearKey is the key of a question's substance: Key after the words that only
// decorate a search with RECENCY are taken out. A wording whose NearKey equals
// an earlier one's asks the same question again, and the desk answers it from
// what the earlier search returned (docs/definitions/
// corpus-first-search-one-verb-2026-11-19.md, Landing 1b).
//
// What is removed, and only this:
//   - "today" and "tonight", and the name of today's or yesterday's weekday
//     (UTC); another weekday names another day and stays. A capitalised
//     weekday or "Today" next to another capitalised word is a proper noun
//     (Black Friday, USA Today, Friday Night Lights) and stays;
//   - today's date and yesterday's, relative to now (UTC), written as an ISO
//     date (2026-11-20), or as a month name with its day number, with or
//     without the year ("20 November 2026", "Nov 19", "1st of December" is
//     neither today nor yesterday and stays).
//
// What is kept, so that two different questions do not merge (review of the
// re-ask holds, 2026-11-20): every other word, named entity and number; any
// OTHER explicit date, month or year ("earnings 2026-10-01" and "earnings
// 2026-11-01" are different questions; so are a bare year and a month with a
// year); a site: restriction (normalised to its lower-case host: a run hunts a
// second outlet with it, and a story publishes only on two distinct outlets),
// every other operator, and a quoted phrase (an exact-phrase search verifies a
// wording; a loose search's results do not show the phrase); not, or and a
// -exclusion; acronyms in capitals; C++. Word order, case, stop words and
// punctuation do not matter, as in Key.
//
// A query with nothing left returns "": it has no near key, and never matches
// another.
func NearKey(q string) string { return NearKeyAt(q, time.Now()) }

// NearKeyAt is NearKey with the clock given.
func NearKeyAt(q string, now time.Time) string {
	today := now.UTC()
	yesterday := today.AddDate(0, 0, -1)
	var out []string
	pos := 0
	process := func(seg string) {
		out = append(out, nearSegment(seg, today, yesterday)...)
	}
	for _, m := range phraseRE.FindAllStringIndex(q, -1) {
		process(q[pos:m[0]])
		out = append(out, q[m[0]:m[1]]) // a quoted phrase is kept whole
		pos = m[1]
	}
	process(q[pos:])
	rest := strings.Join(out, " ")
	if strings.TrimSpace(rest) == "" {
		return ""
	}
	terms := Terms(rest)
	if len(terms) == 0 {
		return ""
	}
	// Key keeps a query of only stop words; a near key of those would collide
	// with every other, so it is empty.
	allStop := true
	for _, t := range terms {
		if t == "" {
			continue
		}
		if !stopWords[t] {
			allStop = false
			break
		}
	}
	if allStop && !hasAcronym(rest) {
		return ""
	}
	return strings.Join(terms, " ")
}

// SiteRestrictions are the normalised hosts of the site: operators a query
// restricts its search to (an excluded -site: is not one), in order.
func SiteRestrictions(q string) []string {
	var out []string
	for _, tok := range strings.Fields(phraseRE.ReplaceAllString(q, " ")) {
		low := strings.ToLower(strings.Trim(tok, `'()[],;!?`))
		if strings.HasPrefix(low, "-") || !strings.HasPrefix(low, "site:") {
			continue
		}
		if h := normSiteHost(strings.TrimPrefix(low, "site:")); h != "" {
			out = append(out, h)
		}
	}
	return out
}

// normSiteHost is a site: value as a lower-case host: no scheme, no www., no
// path, no port, no trailing dot.
func normSiteHost(v string) string {
	v = strings.Trim(v, `'".,;:`)
	if i := strings.Index(v, "://"); i >= 0 {
		v = v[i+3:]
	}
	if i := strings.IndexAny(v, "/?#"); i >= 0 {
		v = v[:i]
	}
	if i := strings.Index(v, ":"); i >= 0 {
		v = v[:i]
	}
	return strings.TrimPrefix(strings.Trim(v, "."), "www.")
}

type nearTok struct {
	raw   string
	low   string
	drop  bool
	month int // 1..12 when a month name
	num   int // a day (1..31) or year (1900..2100) when a number; else 0
	year  bool
	// recency marks "today" or today's or yesterday's weekday name.
	recency bool
}

func capitalised(raw string) bool {
	for _, r := range strings.TrimLeft(raw, `"'([`) {
		return unicode.IsUpper(r)
	}
	return false
}

func nearSegment(seg string, today, yesterday time.Time) []string {
	toks := strings.Fields(seg)
	ts := make([]nearTok, 0, len(toks))
	for _, raw := range toks {
		low := strings.ToLower(strings.Trim(raw, `'()[],;:!?.`))
		t := nearTok{raw: raw, low: low}
		switch {
		case low == "":
			t.drop = true
		case strings.Contains(strings.ToLower(raw), ":") && isOperatorToken(strings.ToLower(raw)):
			// Kept, with a site: value normalised to its host.
			l := strings.ToLower(strings.Trim(raw, `'()[],;!?`))
			neg := strings.HasPrefix(l, "-")
			l = strings.TrimPrefix(l, "-")
			if strings.HasPrefix(l, "site:") {
				l = "site:" + normSiteHost(strings.TrimPrefix(l, "site:"))
			}
			if neg {
				l = "-" + l
			}
			t.raw = l
		case dateRE.MatchString(strings.Trim(raw, `'()[],;!?.`)):
			if d, ok := isoDate(strings.Trim(raw, `'()[],;!?.`)); ok && (sameDay(d, today) || sameDay(d, yesterday)) {
				t.drop = true
			}
		case low == "tonight":
			t.drop = true
		case low == "today" || weekdays[low]:
			// Decided below: a recency word only when it is today's or
			// yesterday's weekday (or "today") and not part of a proper noun.
			t.recency = low == "today" || strings.EqualFold(low, today.Weekday().String()) || strings.EqualFold(low, yesterday.Weekday().String())
		case months[low] > 0:
			t.month = months[low]
		default:
			if n, y, ok := dayOrYear(low); ok {
				t.num, t.year = n, y
			}
		}
		ts = append(ts, t)
	}
	for i := range ts {
		if !ts[i].recency {
			continue
		}
		proper := capitalised(ts[i].raw) && ((i > 0 && capitalised(ts[i-1].raw)) || (i+1 < len(ts) && capitalised(ts[i+1].raw)))
		if !proper {
			ts[i].drop = true
		}
	}
	// A month name with a day number (and a year): dropped together only when
	// it is today's or yesterday's date; otherwise every word of it stays.
	for i := range ts {
		if ts[i].month == 0 {
			continue
		}
		day, year := -1, 0
		var used []int
		j := i - 1
		if j >= 0 && ts[j].low == "of" {
			j--
		}
		if j >= 0 && ts[j].num > 0 && !ts[j].year {
			day = ts[j].num
			used = append(used, j)
		}
		if k := i + 1; k < len(ts) && ts[k].num > 0 {
			if !ts[k].year && day < 0 {
				day = ts[k].num
				used = append(used, k)
				if k+1 < len(ts) && ts[k+1].year {
					year = ts[k+1].num
					used = append(used, k+1)
				}
			} else if ts[k].year {
				year = ts[k].num
				used = append(used, k)
			}
		}
		if day < 0 {
			continue // month and year, or a month word alone: part of the question
		}
		for _, d := range []time.Time{today, yesterday} {
			if int(d.Month()) == ts[i].month && d.Day() == day && (year == 0 || d.Year() == year) {
				ts[i].drop = true
				for _, u := range used {
					ts[u].drop = true
				}
			}
		}
	}
	var kept []string
	for _, t := range ts {
		if !t.drop {
			kept = append(kept, t.raw)
		}
	}
	return kept
}

func sameDay(a, b time.Time) bool {
	ay, am, ad := a.Date()
	by, bm, bd := b.Date()
	return ay == by && am == bm && ad == bd
}

// isoDate reads year-month-day (any of - / .); other orders are not read, so
// they are never folded.
func isoDate(s string) (time.Time, bool) {
	parts := strings.FieldsFunc(s, func(r rune) bool { return r == '-' || r == '/' || r == '.' })
	if len(parts) != 3 || len(parts[0]) != 4 {
		return time.Time{}, false
	}
	y, e1 := strconv.Atoi(parts[0])
	m, e2 := strconv.Atoi(parts[1])
	d, e3 := strconv.Atoi(parts[2])
	if e1 != nil || e2 != nil || e3 != nil || m < 1 || m > 12 || d < 1 || d > 31 {
		return time.Time{}, false
	}
	return time.Date(y, time.Month(m), d, 0, 0, 0, 0, time.UTC), true
}

func hasAcronym(s string) bool {
	for _, w := range words(s) {
		if isAcronym(w) {
			return true
		}
	}
	return false
}

// isOperatorToken reports a name:value token (site:x.com, -filetype:pdf).
func isOperatorToken(tok string) bool {
	_, ok := operatorTerm(tok)
	return ok
}

var weekdays = map[string]bool{"monday": true, "tuesday": true, "wednesday": true, "thursday": true, "friday": true, "saturday": true, "sunday": true}

var months = map[string]int{
	"january": 1, "february": 2, "march": 3, "april": 4, "may": 5, "june": 6,
	"july": 7, "august": 8, "september": 9, "october": 10, "november": 11, "december": 12,
	"jan": 1, "feb": 2, "mar": 3, "apr": 4, "jun": 6, "jul": 7, "aug": 8,
	"sep": 9, "sept": 9, "oct": 10, "nov": 11, "dec": 12,
}

var dayYearRE = regexp.MustCompile(`^(?:(?:[1-9]|[12]\d|3[01])(?:st|nd|rd|th)?|(?:19|20)\d\d)$`)

// dayOrYear reads a day number (1 to 31, with an ordinal suffix or not) or a
// year (1900 to 2100); year reports which.
func dayOrYear(low string) (n int, year, ok bool) {
	if !dayYearRE.MatchString(low) {
		return 0, false, false
	}
	digits := strings.TrimRight(low, "stndrh")
	v, err := strconv.Atoi(digits)
	if err != nil {
		return 0, false, false
	}
	return v, v >= 1900, true
}
