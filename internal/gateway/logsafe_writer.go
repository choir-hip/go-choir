package gateway

import (
	"bytes"
	"io"
)

// SafeLogWriter wraps the process log's output so one log call is always one
// line: any newline, carriage return or other control character inside a
// record is written as an escape (\n, \r, \xNN), and the record ends in
// exactly one newline. It is the backstop under the per-value cleaning (logField,
// logText): whatever text a guest or an upstream gets into a record, it cannot
// start a second line that a log-reading script would take for a row
// (docs/problems/gateway-log-lines-carry-raw-guest-strings-2026-11-22.md).
func SafeLogWriter(w io.Writer) io.Writer { return &safeLogWriter{w: w} }

type safeLogWriter struct{ w io.Writer }

func (s *safeLogWriter) Write(p []byte) (int, error) {
	body := bytes.TrimSuffix(p, []byte("\n"))
	out := make([]byte, 0, len(body)+1)
	const hex = "0123456789abcdef"
	for _, b := range body {
		switch {
		case b == '\n':
			out = append(out, '\\', 'n')
		case b == '\r':
			out = append(out, '\\', 'r')
		case b == '\t':
			out = append(out, ' ')
		case b < 0x20 || b == 0x7f:
			out = append(out, '\\', 'x', hex[b>>4], hex[b&0xf])
		default:
			out = append(out, b)
		}
	}
	out = append(out, '\n')
	if _, err := s.w.Write(out); err != nil {
		return 0, err
	}
	return len(p), nil
}
