package platform

// The spool inbox: the Mac's first-pass file reaches corpusd as a file in a
// directory on its own disk, with no HTTP and no credential. Whoever can write
// the directory on the host already holds more than the host credential, so
// this adds no authority; the file passes exactly what the route applies
// (ImportVectorFile: refusal of another model, a bad row, a failed sample, the
// bounds) and is one job among the route's (one at a time).
//
//	<artifacts>/vector-import/inbox/        files *.jsonl wait here (copy as name.part, rename when whole)
//	<artifacts>/vector-import/running/      the file being imported
//	<artifacts>/vector-import/done/         imported; the job record beside it as <name>.job.json
//	<artifacts>/vector-import/refused/      refused or failed; same record; retried only if a person moves it back
//	<artifacts>/vector-import/interrupted/  found in running/ at start; never run again by corpusd
//
// A crash restart resumes nothing (CLAUDE.md, Restarts): a file left in
// running/ at start is moved to interrupted/ and logged. A file that could not be
// imported NOW (the server cannot answer, too few rows could be checked) is put
// back in inbox/: that is unstarted work, not resumed work.
// One corpusd is assumed: the claim cannot replace a file, but two processes taking the
// same inbox file at once is not designed for. Nothing here deletes
// done/, refused/ or interrupted/ files; the artifact collector sweeps named
// namespaces only and never reads this directory (artifact_gc.go
// artifactGCNamespaces); the operator deletes them after reading the record.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// vectorInboxMinAge is how old a file's modification time must be before it is
// taken, so a copy in progress is not read. Copy as name.part and rename.
var vectorInboxMinAge = 60 * time.Second

// vectorInboxShortDraws is how many times in a row the same inbox file may fall
// short of the sample floor in a draw that was only a subset of its rows before
// it is refused. One short draw of a big file proves nothing about the rows not
// drawn (each pass draws afresh), so it waits; the same shortfall in this many
// draws means the file is mostly uncheckable, which is as good as a proof can be
// without embedding every row. In memory: a restart starts the count again.
const vectorInboxShortDraws = 3

var vectorInboxDirNames = []string{"inbox", "running", "done", "refused", "interrupted"}

func (s *Service) vectorSpoolDir(sub string) string {
	return filepath.Join(s.artifactsRoot, vectorImportDirName, sub)
}

// ensureVectorInboxDirs creates the inbox directories with the mode of the
// route's spool.
func (s *Service) ensureVectorInboxDirs() error {
	for _, d := range vectorInboxDirNames {
		if err := os.MkdirAll(s.vectorSpoolDir(d), 0o750); err != nil {
			return err
		}
	}
	return nil
}

// recoverVectorInbox runs at start: a file in running/ was being imported when
// the process died; it is moved to interrupted/ and logged, never run again.
func (s *Service) recoverVectorInbox() {
	if err := s.ensureVectorInboxDirs(); err != nil {
		log.Printf("corpusd vectors inbox: cannot create the directories: %v", err)
		return
	}
	entries, err := os.ReadDir(s.vectorSpoolDir("running"))
	if err != nil {
		return
	}
	for _, e := range entries {
		name := e.Name()
		if _, err := moveNoReplace(filepath.Join(s.vectorSpoolDir("running"), name), s.vectorSpoolDir("interrupted"), name); err != nil {
			log.Printf("corpusd vectors inbox: %s was running at start and could not be moved to interrupted/: %v", name, err)
			continue
		}
		log.Printf("corpusd vectors inbox: %s was running when corpusd stopped; moved to interrupted/ and not run again (move it back to inbox/ to retry)", name)
	}
}

// moveNoReplace moves src into dir/name, never replacing what is there: a name
// taken is suffixed (name.2, name.3, ...). A regular file is hard-linked then
// unlinked (link fails if the destination exists, so two movers cannot both
// win); anything else (a directory, a symlink) is renamed after a check, which is
// enough for one corpusd. It returns the name used.
func moveNoReplace(src, dir, name string) (string, error) {
	fi, err := os.Lstat(src)
	if err != nil {
		return "", err
	}
	for n := 1; n < 1000; n++ {
		cand := name
		if n > 1 {
			cand = fmt.Sprintf("%s.%d", name, n)
		}
		dst := filepath.Join(dir, cand)
		if _, err := os.Lstat(dst); err == nil {
			continue
		}
		if fi.Mode().IsRegular() {
			if err := os.Link(src, dst); err != nil {
				if os.IsExist(err) {
					continue
				}
				return "", err
			}
			return cand, os.Remove(src)
		}
		return cand, os.Rename(src, dst)
	}
	return "", fmt.Errorf("no free name for %s in %s", name, dir)
}

// ProcessVectorInbox imports one file of the inbox, if one is eligible: the
// oldest that is not waiting, else the waiting one tried longest ago; none while a job of the
// route runs (the file waits). It returns how many files it brought to an outcome
// (done/ or refused/): a file that was put back because it could not be imported
// now is not counted, so the pass is not "work" and waits its ordinary interval.
// The error is the embedder being unable to answer (errEmbedderDown), so the pass
// backs off, or the inbox being unreadable.
func (s *Service) ProcessVectorInbox(ctx context.Context) (imported int, err error) {
	if err := s.ensureVectorInboxDirs(); err != nil {
		return 0, fmt.Errorf("vector inbox: %w", err)
	}
	pick, _, _, perr := s.pickVectorInboxFile()
	if perr != nil {
		return 0, perr
	}
	v := &s.store.vec
	if pick == "" || ctx.Err() != nil {
		return 0, nil
	}
	job, ok := s.beginVectorJob("inbox", pick)
	if !ok {
		return 0, nil // the route's job runs; the file waits
	}
	done, ierr := s.importInboxFile(ctx, job, pick)
	v.importMu.Lock()
	if done {
		delete(v.inboxDeferred, pick)
	} else {
		if v.inboxDeferred == nil {
			v.inboxDeferred = map[string]time.Time{}
		}
		v.inboxDeferred[pick] = time.Now()
	}
	v.importMu.Unlock()
	if done {
		return 1, nil // importInboxFile returns an error only for a file not done
	}
	return 0, ierr
}

// pickVectorInboxFile is the one rule for which inbox file is taken next: the
// oldest eligible file (not .part, old enough) that is not waiting, else the
// waiting one tried longest ago. It returns "" when there is none; waiting says
// the file was tried and put back, and lastTried when. It prunes the memory of
// files that have left the inbox.
func (s *Service) pickVectorInboxFile() (pick string, lastTried time.Time, waiting bool, err error) {
	if err := s.ensureVectorInboxDirs(); err != nil {
		return "", time.Time{}, false, fmt.Errorf("vector inbox: %w", err)
	}
	entries, rerr := os.ReadDir(s.vectorSpoolDir("inbox"))
	if rerr != nil {
		return "", time.Time{}, false, fmt.Errorf("vector inbox: %w", rerr)
	}
	type cand struct {
		name string
		mod  time.Time
	}
	var cands []cand
	present := map[string]bool{}
	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(name, ".jsonl") {
			continue // *.part and anything else is left alone
		}
		present[name] = true
		fi, lerr := os.Lstat(filepath.Join(s.vectorSpoolDir("inbox"), name))
		if lerr != nil || time.Since(fi.ModTime()) < vectorInboxMinAge {
			continue
		}
		cands = append(cands, cand{name, fi.ModTime()})
	}
	v := &s.store.vec
	v.importMu.Lock()
	for name := range v.inboxDeferred {
		if !present[name] {
			delete(v.inboxDeferred, name) // gone from the inbox
		}
	}
	for name := range v.inboxShort {
		if !present[name] {
			delete(v.inboxShort, name)
		}
	}
	deferred := map[string]time.Time{} // name -> when it was last tried
	for name, at := range v.inboxDeferred {
		deferred[name] = at
	}
	v.importMu.Unlock()
	sort.Slice(cands, func(i, j int) bool { return cands[i].mod.Before(cands[j].mod) })
	pick = ""
	for _, c := range cands {
		if _, waits := deferred[c.name]; !waits {
			pick = c.name
			break
		}
	}
	if pick == "" {
		// Only waiting files are left: try the one tried longest ago (ties keep
		// the order of cands, which sort.Slice leaves arbitrary for equal
		// modification times), so a file that can never be imported now does not
		// starve the others.
		var last time.Time
		for _, c := range cands {
			if at := deferred[c.name]; pick == "" || at.Before(last) {
				pick, last = c.name, at
			}
		}
	}
	if pick != "" {
		lastTried, waiting = deferred[pick]
	}
	return pick, lastTried, waiting, nil
}

// vectorInboxHasFileForPass says whether a running pass should yield: the inbox
// holds a file the next pass would pick, and it is not a waiter tried less than
// idle ago (a waiter that cannot be imported now is tried once per idle
// interval at most, so it cannot make passes spin). Picking is not taking: the
// take also needs beginVectorJob, so while a job of the route runs the pass
// that follows a yield takes nothing, and the file costs one early return per
// check interval until that job ends.
func (s *Service) vectorInboxHasFileForPass(idle time.Duration) bool {
	pick, last, waiting, err := s.pickVectorInboxFile()
	if err != nil || pick == "" {
		return false
	}
	return !waiting || time.Since(last) >= idle
}

// importInboxFile claims name (a move into running/ that cannot replace), imports
// it, and then follows importFate: done/ with its record, refused/ with its
// record (a fault of the file itself), or back to inbox/ untouched (the server
// cannot answer, the file cannot be checked now, or something other than the file
// failed): unstarted work, found again next pass, never resumed work.
func (s *Service) importInboxFile(ctx context.Context, job *VectorImportJob, name string) (done bool, err error) {
	src := filepath.Join(s.vectorSpoolDir("inbox"), name)
	// Judged on the link itself before the claim: a symlink or any other
	// non-regular file (a directory named x.jsonl) is moved to refused/ as it is
	// and never opened.
	fi, lerr := os.Lstat(src)
	claimed, err := moveNoReplace(src, s.vectorSpoolDir("running"), name)
	if err != nil {
		s.finishVectorJob(job, VectorImport{}, fmt.Errorf("could not claim %s: %w", name, err))
		s.forgetVectorJob(job)
		return false, nil
	}
	run := filepath.Join(s.vectorSpoolDir("running"), claimed)
	var res VectorImport
	var ierr error
	switch {
	case lerr != nil:
		ierr = fmt.Errorf("%w: %v", ErrVectorImportRefused, lerr)
	case !fi.Mode().IsRegular():
		ierr = fmt.Errorf("%w: %s is not a regular file (a symlink, a directory or a special file; not followed)", ErrVectorImportRefused, name)
	case fi.Size() > vectorImportMaxBytes:
		ierr = fmt.Errorf("%w: %s is %d bytes, over the %d the import takes", ErrVectorImportRefused, name, fi.Size(), vectorImportMaxBytes)
	default:
		res, ierr = s.ImportVectorFile(ctx, run)
		ierr = s.countShortDraw(name, ierr)
	}
	if ctx.Err() != nil && ierr != nil {
		// Shutdown mid-import: the file stays in running/ and is moved to
		// interrupted/ at the next start.
		s.finishVectorJob(job, res, ierr)
		return false, nil
	}
	out := s.finishVectorJob(job, res, ierr)
	var dest string
	switch importFate(ierr) {
	case fateDone:
		dest = "done"
	case fateRefuse:
		dest = "refused"
	default:
		// Back to inbox/ with its modification time: taken again next pass.
		if _, merr := moveNoReplace(run, s.vectorSpoolDir("inbox"), name); merr != nil {
			log.Printf("corpusd vectors inbox: %s could not be returned to inbox/ (%v); moved to interrupted/ instead", name, merr)
			_, _ = moveNoReplace(run, s.vectorSpoolDir("interrupted"), name)
		} else {
			log.Printf("corpusd vectors inbox: %s was not imported now (%v); it stays in inbox/ and is tried again next pass", name, ierr)
		}
		s.forgetVectorJob(job) // no entry for a file that waits
		if errors.Is(ierr, errEmbedderDown) {
			return false, ierr
		}
		return false, nil
	}
	final, merr := moveNoReplace(run, s.vectorSpoolDir(dest), name)
	if merr != nil {
		// The import itself is done and idempotent; the file stays in running/ and is
		// reported as interrupted at the next start (re-importing it is harmless).
		log.Printf("corpusd vectors inbox: %s finished %s but could not be moved to %s/: %v", name, out.State, dest, merr)
		return true, nil
	}
	rec, _ := json.MarshalIndent(out, "", "  ")
	recPath := filepath.Join(s.vectorSpoolDir(dest), final+".job.json")
	f, werr := os.OpenFile(recPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o640)
	if werr == nil {
		_, werr = f.Write(append(rec, '\n'))
		if cerr := f.Close(); werr == nil {
			werr = cerr
		}
	}
	if werr != nil {
		log.Printf("corpusd vectors inbox: job record of %s not written: %v", final, werr)
	}
	return true, nil
}

// countShortDraw counts, per inbox file, consecutive imports that fell short of
// the sample floor in a draw that covered only part of the file
// (errVectorImportShortDraw). At vectorInboxShortDraws the error becomes a
// refusal naming the count; any other outcome clears the count.
func (s *Service) countShortDraw(name string, ierr error) error {
	v := &s.store.vec
	v.importMu.Lock()
	defer v.importMu.Unlock()
	if !errors.Is(ierr, errVectorImportShortDraw) {
		delete(v.inboxShort, name)
		return ierr
	}
	if v.inboxShort == nil {
		v.inboxShort = map[string]int{}
	}
	v.inboxShort[name]++
	if n := v.inboxShort[name]; n >= vectorInboxShortDraws {
		delete(v.inboxShort, name)
		return fmt.Errorf("%w: the sample fell short of the floor in %d draws in a row, each of %d of the file's rows; the file is mostly uncheckable by this host (last: %v)", ErrVectorImportRefused, n, vectorImportCandidates, ierr)
	}
	return ierr
}
