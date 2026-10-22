package taperepack

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// DeleteOptions configure Delete.
type DeleteOptions struct {
	Root    string
	WorkDir string
	// CensusReceipt is the digest of a census report in WorkDir
	// (census-<digest>.json). It is required, for a dry run too.
	CensusReceipt string
	// Computers narrows the delete. Empty means the computers the census
	// covered; a computer the census did not cover is refused.
	Computers []string
	// Apply unlinks. Without it Delete counts what it would unlink.
	Apply bool
	// MaxAge is how old the census may be; zero means 24 hours.
	MaxAge time.Duration
	// Batch is the number of unlinks between pauses; zero means 256.
	Batch int
	// Pause between batches; zero means 250 ms.
	Pause    time.Duration
	Rate     int
	Progress func(string)

	now func() time.Time // test seam
}

// DeleteReport is the result of a delete run.
type DeleteReport struct {
	Mode          string           `json:"mode"` // delete-dry-run or delete-apply
	Root          string           `json:"root"`
	CensusReceipt string           `json:"census_receipt"`
	VerifyReceipt string           `json:"verify_receipt"` // the fresh pass this run's verdicts came from
	Computers     []string         `json:"computers,omitempty"`
	WouldDelete   int64            `json:"would_delete"`
	Deleted       int64            `json:"deleted"`
	BytesFreed    int64            `json:"bytes_freed"`
	AlreadyGone   int64            `json:"already_gone"`
	Refused       map[string]int64 `json:"refused"`
	Log           string           `json:"log,omitempty"`
}

// Delete unlinks legacy tape files that have a verified segment copy, and
// only those. The census receipt is a gate, not a list: Delete reads the
// census named by the digest, refuses unless it hashes to that digest, is for
// this root, complete, healthy (no corrupt copy, no blocked computer, no
// legacy file that fails its hash) and recent, and then runs the whole
// verification again; each file is unlinked only when its Verdict from that
// fresh pass is deletable, at the moment the engine has just compared the
// segment copies with the legacy bytes. Dry run by default; --apply with the
// digest unlinks, in batches with a pause, logs every unlink, and stops at
// the first error. It never removes a directory.
func Delete(opts DeleteOptions) (*DeleteReport, error) {
	if opts.Root == "" {
		return nil, errors.New("taperepack: --root is required")
	}
	root := filepath.Clean(opts.Root)
	if err := checkWorkDir(opts.WorkDir, root); err != nil {
		return nil, err
	}
	if len(opts.CensusReceipt) != 64 {
		return nil, errors.New("taperepack: delete needs --census-receipt, the digest of a census report in the work directory")
	}
	if _, err := hex.DecodeString(opts.CensusReceipt); err != nil {
		return nil, errors.New("taperepack: --census-receipt is not a SHA-256 digest")
	}
	now := time.Now
	if opts.now != nil {
		now = opts.now
	}
	maxAge := opts.MaxAge
	if maxAge <= 0 {
		maxAge = 24 * time.Hour
	}
	batch, pause := opts.Batch, opts.Pause
	if batch <= 0 {
		batch = 256
	}
	if pause <= 0 {
		pause = 250 * time.Millisecond
	}

	raw, err := os.ReadFile(filepath.Join(opts.WorkDir, "census-"+opts.CensusReceipt+".json"))
	if err != nil {
		return nil, fmt.Errorf("taperepack: no census report for that digest in the work directory: %w", err)
	}
	if sum := sha256.Sum256(raw); hex.EncodeToString(sum[:]) != opts.CensusReceipt {
		return nil, errors.New("taperepack: the census report does not hash to the digest given")
	}
	var census Report
	if err := json.Unmarshal(raw, &census); err != nil {
		return nil, fmt.Errorf("taperepack: census report: %w", err)
	}
	if census.Version != reportVersion || census.Mode != "census" {
		return nil, fmt.Errorf("taperepack: the report is a %q report of version %d, not a census", census.Mode, census.Version)
	}
	same, err := sameDir(census.Root, root)
	if err != nil || !same {
		return nil, fmt.Errorf("taperepack: the census is for %s, not %s", census.Root, root)
	}
	if !census.Complete || !census.Healthy() {
		return nil, errors.New("taperepack: the census is not complete and healthy (a corrupt copy, a blocked computer, a segment error or a legacy file that fails its hash); nothing is deleted on it")
	}
	if census.Covered() == 0 {
		return nil, errors.New("taperepack: the census covered no file; nothing to delete")
	}
	finished, err := time.Parse(time.RFC3339, census.Finished)
	if err != nil {
		return nil, fmt.Errorf("taperepack: census time: %w", err)
	}
	if age := now().Sub(finished); age > maxAge || age < -5*time.Minute {
		return nil, fmt.Errorf("taperepack: the census finished %s ago; run a new census (limit %s)", age.Round(time.Second), maxAge)
	}
	computers := opts.Computers
	if len(census.Filter) > 0 {
		covered := map[string]bool{}
		for _, c := range census.Filter {
			covered[c] = true
		}
		for _, c := range computers {
			if !covered[c] {
				return nil, fmt.Errorf("taperepack: the census did not cover computer %s", c)
			}
		}
		if len(computers) == 0 {
			computers = census.Filter
		}
	}

	rep := &DeleteReport{Mode: "delete-dry-run", Root: root, CensusReceipt: opts.CensusReceipt, Computers: computers, Refused: map[string]int64{}}
	var logf *os.File
	var logw *bufio.Writer
	if opts.Apply {
		rep.Mode = "delete-apply"
		rep.Log = filepath.Join(opts.WorkDir, fmt.Sprintf("delete-%d.jsonl", now().UnixNano()))
		if logf, err = os.OpenFile(rep.Log, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600); err != nil {
			return nil, err
		}
		defer logf.Close()
		logw = bufio.NewWriter(logf)
	}
	sinceBatch := 0
	flushLog := func() error {
		if logw == nil {
			return nil
		}
		if err := logw.Flush(); err != nil {
			return err
		}
		return logf.Sync()
	}
	onVerdict := func(v Verdict) error {
		if !v.Deletable() {
			rep.Refused[v.Class()]++
			return nil
		}
		if !opts.Apply {
			rep.WouldDelete++
			return nil
		}
		size, err := unlinkCovered(root, v)
		switch {
		case errors.Is(err, os.ErrNotExist):
			rep.AlreadyGone++
			return nil
		case err != nil:
			return err
		}
		rep.Deleted++
		rep.BytesFreed += size
		line, _ := json.Marshal(struct {
			NS        string   `json:"ns"`
			Digest    string   `json:"digest"`
			Computers []string `json:"computers"`
			Bytes     int64    `json:"bytes"`
		}{nsName(v.NS()), v.Digest(), v.Computers(), size})
		if _, err := logw.Write(append(line, '\n')); err != nil {
			return err
		}
		if sinceBatch++; sinceBatch >= batch {
			sinceBatch = 0
			if err := flushLog(); err != nil {
				return err
			}
			time.Sleep(pause)
		}
		return nil
	}
	verify, err := Run(Options{Root: root, WorkDir: opts.WorkDir, Computers: computers, Rate: opts.Rate, Progress: opts.Progress, OnVerdict: onVerdict})
	if ferr := flushLog(); err == nil {
		err = ferr
	}
	if err != nil {
		return rep, err
	}
	rep.VerifyReceipt = verify.Receipt
	return rep, nil
}

func sameDir(a, b string) (bool, error) {
	ra, err := resolvePath(a)
	if err != nil {
		return false, err
	}
	rb, err := resolvePath(b)
	if err != nil {
		return false, err
	}
	return ra == rb, nil
}
