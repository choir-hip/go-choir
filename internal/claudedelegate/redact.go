package claudedelegate

import (
	"bufio"
	"bytes"
	"io"
)

// Redacted replaces the token wherever a run's output carries it.
const Redacted = "[redacted:claude-oauth-token]"

// MaxLineBytes bounds one buffered output line. Claude Code truncates tool
// output well below this; a longer line is dropped (never written raw).
const MaxLineBytes = 64 << 20

// redactLine replaces every occurrence of secret in line.
func redactLine(line, secret []byte) []byte {
	if len(secret) == 0 || !bytes.Contains(line, secret) {
		return line
	}
	return bytes.ReplaceAll(line, secret, []byte(Redacted))
}

// pumpLines reads r line by line, redacts each whole line, and hands it to
// emit (newline included when present). Redacting whole lines means a token
// can never be split across two writes. A line over MaxLineBytes is replaced
// by a marker. It returns when r is exhausted or fails.
func pumpLines(r io.Reader, secret []byte, emit func(line []byte, dropped int)) {
	br := bufio.NewReaderSize(r, 1<<20)
	var acc []byte
	dropping := false
	dropped := 0
	for {
		chunk, err := br.ReadSlice('\n')
		if len(chunk) > 0 {
			if dropping {
				dropped += len(chunk)
			} else if len(acc)+len(chunk) > MaxLineBytes {
				dropping = true
				dropped = len(acc) + len(chunk)
				acc = acc[:0]
			} else {
				acc = append(acc, chunk...)
			}
		}
		lineDone := err == nil || (err != bufio.ErrBufferFull && (len(acc) > 0 || dropping))
		if lineDone {
			if dropping {
				emit(nil, dropped)
			} else if len(acc) > 0 {
				emit(redactLine(acc, secret), 0)
			}
			acc, dropping, dropped = acc[:0], false, 0
		}
		if err != nil && err != bufio.ErrBufferFull {
			return
		}
	}
}

// ContainsSecret reports whether r carries secret anywhere, reading in
// bounded chunks with an overlap so a match across a chunk edge is found.
func ContainsSecret(r io.Reader, secret []byte) (bool, error) {
	if len(secret) == 0 {
		return false, nil
	}
	buf := make([]byte, 0, (1<<20)+len(secret))
	chunk := make([]byte, 1<<20)
	for {
		n, err := r.Read(chunk)
		if n > 0 {
			buf = append(buf, chunk[:n]...)
			if bytes.Contains(buf, secret) {
				return true, nil
			}
			if keep := len(secret) - 1; len(buf) > keep {
				buf = append(buf[:0], buf[len(buf)-keep:]...)
			}
		}
		if err == io.EOF {
			return false, nil
		}
		if err != nil {
			return false, err
		}
	}
}
