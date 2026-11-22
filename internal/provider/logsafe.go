package provider

import (
	"strconv"
	"strings"
)

// LogField cleans a request- or upstream-supplied identifier for a log line's
// key=value field: no space, control character or non-ASCII byte, at most max
// bytes, "-" when nothing is left. A model name, reasoning effort, artifact
// ref or an upstream's echo of one can carry spaces, "=" and newlines; raw,
// they forge fields and whole lines that the money and status scripts parse
// (docs/problems/gateway-log-lines-carry-raw-guest-strings-2026-11-22.md).
func LogField(s string, max int) string {
	v := strings.Map(func(c rune) rune {
		if c <= ' ' || c > 0x7e {
			return -1
		}
		return c
	}, s)
	if len(v) > max {
		v = v[:max]
	}
	if v == "" {
		return "-"
	}
	return v
}

// LogText is free text from a guest or an upstream (an error body, a reason)
// as one log token: bounded to max bytes, then Go-quoted, so a newline, a
// quote or a fake "gateway: ..." line stays inside the quotes.
func LogText(s string, max int) string {
	if len(s) > max {
		s = s[:max] + "..."
	}
	return strconv.Quote(s)
}

func logModel(s string) string { return LogField(s, 96) }
