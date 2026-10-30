package yaegikernel

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"unicode/utf8"
)

// LR lever 6 (docs/archive/reports/lr-lever6-cell-output-design-2026-10-14.md): a
// desk cell's printed output over cellOutputBound characters reaches the
// transcript as a head, a marker line and a tail; the whole text stays in the
// worker session, read back with choir.CellOut(handle). Value views (listed
// names, a bare final expression) are not prints and are never cut here.
const (
	// cellOutputBound is the largest print, in characters (runes), shown
	// whole: the measured threshold in the note.
	cellOutputBound = 8000
	// cellOutputHead and cellOutputTail are the runes a cut view shows.
	cellOutputHead = 6000
	cellOutputTail = 1500
	// cellOutputKept is how many cut outputs a session keeps; the oldest is
	// evicted first.
	cellOutputKept = 8
)

// cellOutputStore keeps a session's last cellOutputKept cut outputs. It has
// its own mutex because cell code can start goroutines that call CellOut.
type cellOutputStore struct {
	mu sync.Mutex
	// instance names this worker session (8 hex characters, minted on first
	// use): a handle from another session is refused, never answered with
	// this session's bytes.
	instance string
	// next counts this session's cut outputs; the newest handle is
	// "<instance>:<next>". Cells that are not cut do not advance it.
	next int
	kept []keptCellOutput
}

type keptCellOutput struct {
	n    int
	text string
}

// instanceLocked returns the session instance id, minting it once.
func (c *cellOutputStore) instanceLocked() string {
	if c.instance == "" {
		var b [4]byte
		if _, err := rand.Read(b[:]); err != nil {
			// crypto/rand does not fail on supported platforms; a fixed id
			// would still refuse every foreign handle but its own.
			c.instance = "00000000"
		} else {
			c.instance = hex.EncodeToString(b[:])
		}
	}
	return c.instance
}

// keepCellOutput is the CellHooks.KeepOutput of a desk session: a print over
// the bound is stored whole and returned as its cut view; anything else is
// returned unchanged.
func (s *ChoirScope) keepCellOutput(stdout string) string {
	total := utf8.RuneCountInString(stdout)
	if s == nil || total <= cellOutputBound {
		return stdout
	}
	c := &s.cellOuts
	c.mu.Lock()
	instance := c.instanceLocked()
	c.next++
	n := c.next
	c.kept = append(c.kept, keptCellOutput{n: n, text: stdout})
	if len(c.kept) > cellOutputKept {
		c.kept = append([]keptCellOutput(nil), c.kept[len(c.kept)-cellOutputKept:]...)
	}
	c.mu.Unlock()

	head := runePrefix(stdout, cellOutputHead)
	tail := runeSuffix(stdout, cellOutputTail)
	handle := instance + ":" + strconv.Itoa(n)
	marker := fmt.Sprintf("… stdout truncated: %s chars; %s not shown. Read all of it with out, err := choir.CellOut(%q); out is a string to slice or search in your next cell.",
		groupDigits(total), groupDigits(total-cellOutputHead-cellOutputTail), handle)
	return head + "\n" + marker + "\n" + tail
}

// CellOut returns the whole printed output of a cut cell by the handle its
// marker line names. It never panics: an unknown, evicted, foreign or
// malformed handle returns an error that says which.
func (s *ChoirScope) CellOut(handle string) (out string, err error) {
	defer func() {
		if r := recover(); r != nil {
			out, err = "", fmt.Errorf("handle %s could not be read: %v", quoteHandle(handle), r)
		}
	}()
	if s == nil {
		return "", fmt.Errorf("handle %s: this session keeps no cell outputs", quoteHandle(handle))
	}
	h := strings.TrimSpace(handle)
	instance, nText, ok := strings.Cut(h, ":")
	n, nerr := strconv.Atoi(nText)
	if !ok || !isLowerHex8(instance) || nerr != nil || n < 1 || strconv.Itoa(n) != nText {
		return "", fmt.Errorf("%s is not a cell output handle; a handle looks like \"k3f9a2b1:17\", as a cut cell result's marker line names it", quoteHandle(handle))
	}
	c := &s.cellOuts
	c.mu.Lock()
	defer c.mu.Unlock()
	self := c.instanceLocked()
	if instance != self {
		return "", fmt.Errorf("handle %s is from a session that ended; this session is %s; re-run the cell", h, self)
	}
	for _, k := range c.kept {
		if k.n == n {
			return k.text, nil
		}
	}
	if n > c.next {
		return "", fmt.Errorf("handle %s names no cut output of this session (it has cut %d)", h, c.next)
	}
	return "", fmt.Errorf("handle %s was evicted; only the last %d cut outputs are kept (this session has cut %d); re-run the cell", h, cellOutputKept, c.next)
}

func isLowerHex8(s string) bool {
	if len(s) != 8 {
		return false
	}
	for i := 0; i < len(s); i++ {
		if !(s[i] >= '0' && s[i] <= '9' || s[i] >= 'a' && s[i] <= 'f') {
			return false
		}
	}
	return true
}

// quoteHandle quotes a handle for an error, bounded so a garbage argument
// is not echoed whole into the cell result.
func quoteHandle(handle string) string {
	const max = 40
	if utf8.RuneCountInString(handle) > max {
		return strconv.Quote(runePrefix(handle, max)) + "…"
	}
	return strconv.Quote(handle)
}

// runePrefix returns the first n runes of s.
func runePrefix(s string, n int) string {
	i := 0
	for count := 0; i < len(s) && count < n; count++ {
		_, size := utf8.DecodeRuneInString(s[i:])
		i += size
	}
	return s[:i]
}

// runeSuffix returns the last n runes of s.
func runeSuffix(s string, n int) string {
	i := len(s)
	for count := 0; i > 0 && count < n; count++ {
		_, size := utf8.DecodeLastRuneInString(s[:i])
		i -= size
	}
	return s[i:]
}

// runeBoundary returns the largest index <= max that does not split a
// UTF-8 sequence of s.
func runeBoundary(s string, max int) int {
	if max >= len(s) {
		return len(s)
	}
	for max > 0 && !utf8.RuneStart(s[max]) {
		max--
	}
	return max
}

// groupDigits renders n with thousands separators (103,782).
func groupDigits(n int) string {
	s := strconv.Itoa(n)
	neg := strings.HasPrefix(s, "-")
	s = strings.TrimPrefix(s, "-")
	var b strings.Builder
	for i, r := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			b.WriteByte(',')
		}
		b.WriteRune(r)
	}
	if neg {
		return "-" + b.String()
	}
	return b.String()
}
