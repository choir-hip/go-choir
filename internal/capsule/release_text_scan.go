package capsule

import (
	"bufio"
	"bytes"
	"errors"
	"io"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
)

// textScanWindow bounds how much of one line the text scan holds at once.
// A line longer than this (a minified bundle is one line of several MiB)
// is scanned in windows that overlap by textScanCarry bytes, so a secret
// straddling a window boundary is still matched.
//
// The carry is enough only if every match has a bounded span. The token
// patterns do (a token cut at a boundary still satisfies its minimum
// length), but authorization_bearer and credential_assignment allow an
// unbounded run of spaces and tabs between their anchors, so an attacker
// could put more than the carry of blanks between "Bearer" and the token.
// The scan therefore keeps at most textScanBlankRun bytes of any run of
// spaces and tabs: the patterns match a blank run of any length, so a
// shortened run matches wherever the full run did, and every match then
// fits inside the carry.
//
// private_key and private_key_header carry a label run, (?:[A-Z0-9 ]+ )?,
// that the blank cap cannot bound, so an inflated label could push
// "-----BEGIN" and "PRIVATE KEY-----" into different windows (Devin review
// r1). A window of an over-long line is therefore also refused when it holds
// textScanPEMMarker, a contiguous 16-byte tail every PEM private key header
// ends with. That refuses a little more than the whole-line detector would
// (the marker without a well-formed header), never less; lines within the
// window keep the whole-line detector exactly.
const (
	textScanWindow   = 1 << 20
	textScanCarry    = 4 << 10
	textScanBlankRun = 64
)

var textScanPEMMarker = []byte("PRIVATE KEY-----")

// textHasRefusalSecret reports whether any line of a text release file
// matches a refusal-grade secret pattern. Refusal-grade, not redaction-grade:
// DetectPrivateSecrets over-matches machine-generated text (vendored pdf.js
// enums, minified member accesses) just as it did compiled binaries. Lines
// have no length cap (2026-11-06: a bufio.Scanner token cap refused every
// freeze that carried a minified frontend asset).
func textHasRefusalSecret(r io.Reader) (bool, error) {
	reader := bufio.NewReaderSize(r, 64<<10)
	var line []byte
	blanks := 0
	windowed := false // this line was longer than a window
	for {
		part, err := reader.ReadSlice('\n')
		for _, b := range part {
			if b == ' ' || b == '\t' {
				if blanks++; blanks > textScanBlankRun {
					continue
				}
			} else {
				blanks = 0
			}
			line = append(line, b)
		}
		switch {
		case errors.Is(err, bufio.ErrBufferFull):
			if len(line) >= textScanWindow {
				if bytes.Contains(line, textScanPEMMarker) || len(computerevent.DetectRefusalSecrets(line)) != 0 {
					return true, nil
				}
				line = append(line[:0:0], line[len(line)-textScanCarry:]...)
				windowed = true
			}
			continue
		case err != nil && !errors.Is(err, io.EOF):
			return false, err
		}
		text := bytes.TrimSuffix(bytes.TrimSuffix(line, []byte("\n")), []byte("\r"))
		if (windowed && bytes.Contains(text, textScanPEMMarker)) || len(computerevent.DetectRefusalSecrets(text)) != 0 {
			return true, nil
		}
		if err != nil {
			return false, nil
		}
		line, blanks, windowed = line[:0], 0, false
	}
}
