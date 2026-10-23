package taperepack

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/yusefmosiah/go-choir/internal/tapestore"
)

// KeepSet is a set of digests the delete must never unlink: the digests the
// platform's tape sweep keeps because a database record names them
// (tapeExtraRefQueries: closures, programs, checkpoints and their receipts,
// route projection certificates), which this tool cannot read. A listed
// digest is kept in every namespace. A nil *KeepSet keeps nothing.
type KeepSet struct{ m map[string]struct{} }

// Has reports whether digest is listed.
func (k *KeepSet) Has(digest string) bool {
	if k == nil {
		return false
	}
	_, ok := k.m[digest]
	return ok
}

// Len is the number of distinct digests listed.
func (k *KeepSet) Len() int {
	if k == nil {
		return 0
	}
	return len(k.m)
}

// ParseKeepDigests reads one 64-character lowercase hex digest per line, each
// line ended by "\n" (the last may lack it). Anything else, blank lines,
// carriage returns, spaces, two digests on a line, is an error that names the
// line: a keep list that is half understood is not a keep list. An input with
// no digest is an error unless emptyOK, so an empty file (a failed export)
// cannot pass for "nothing to keep".
func ParseKeepDigests(r io.Reader, emptyOK bool) (*KeepSet, error) {
	br := bufio.NewReaderSize(r, 4096)
	k := &KeepSet{m: map[string]struct{}{}}
	for n := 1; ; n++ {
		line, err := br.ReadSlice('\n')
		if errors.Is(err, bufio.ErrBufferFull) {
			return nil, fmt.Errorf("taperepack: keep digests: line %d is too long", n)
		}
		if err != nil && !errors.Is(err, io.EOF) {
			return nil, fmt.Errorf("taperepack: keep digests: %w", err)
		}
		eof := errors.Is(err, io.EOF)
		if !eof {
			line = line[:len(line)-1]
		} else if len(line) == 0 {
			break
		}
		if d := string(line); !tapestore.ValidDigest(d) {
			return nil, fmt.Errorf("taperepack: keep digests: line %d is not a 64-character lowercase hex digest", n)
		} else {
			k.m[d] = struct{}{}
		}
		if eof {
			break
		}
	}
	if len(k.m) == 0 && !emptyOK {
		return nil, errors.New("taperepack: keep digests: the file names no digest (pass --keep-digests-empty-ok only if the platform's keep set is known to be empty)")
	}
	return k, nil
}

// LoadKeepDigests parses the keep file at path.
func LoadKeepDigests(path string, emptyOK bool) (*KeepSet, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("taperepack: keep digests: %w", err)
	}
	defer f.Close()
	return ParseKeepDigests(f, emptyOK)
}
