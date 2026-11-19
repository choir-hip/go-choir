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
// running/ at start is moved to interrupted/ and logged. Nothing here deletes
// done/, refused/ or interrupted/ files; the artifact collector sweeps named
// namespaces only and never reads this directory (artifact_gc.go
// artifactGCNamespaces); the operator deletes them after reading the record.

import (
	"context"
	"encoding/json"
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
		if err := os.Rename(filepath.Join(s.vectorSpoolDir("running"), name), filepath.Join(s.vectorSpoolDir("interrupted"), name)); err != nil {
			log.Printf("corpusd vectors inbox: %s was running at start and could not be moved to interrupted/: %v", name, err)
			continue
		}
		log.Printf("corpusd vectors inbox: %s was running when corpusd stopped; moved to interrupted/ and not run again (move it back to inbox/ to retry)", name)
	}
}

// ProcessVectorInbox imports the files in the inbox that are old enough, one at
// a time, oldest first. A file waits while a job of the route runs. It returns
// the files it took.
func (s *Service) ProcessVectorInbox(ctx context.Context) (taken int) {
	if err := s.ensureVectorInboxDirs(); err != nil {
		return 0
	}
	entries, err := os.ReadDir(s.vectorSpoolDir("inbox"))
	if err != nil {
		return 0
	}
	type cand struct {
		name string
		mod  time.Time
	}
	var cands []cand
	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(name, ".jsonl") || e.IsDir() {
			continue // *.part and anything else is left alone
		}
		fi, err := os.Lstat(filepath.Join(s.vectorSpoolDir("inbox"), name))
		if err != nil || time.Since(fi.ModTime()) < vectorInboxMinAge {
			continue
		}
		cands = append(cands, cand{name, fi.ModTime()})
	}
	sort.Slice(cands, func(i, j int) bool { return cands[i].mod.Before(cands[j].mod) })
	for _, c := range cands {
		if ctx.Err() != nil {
			return taken
		}
		job, ok := s.beginVectorJob("inbox", c.name)
		if !ok {
			return taken // the route's job runs; the file waits
		}
		taken++
		s.importInboxFile(ctx, job, c.name)
	}
	return taken
}

// importInboxFile claims name (a rename into running/ is the claim: a second
// corpusd cannot also take it), imports it, and moves it to done/ or refused/
// with the job record beside it.
func (s *Service) importInboxFile(ctx context.Context, job *VectorImportJob, name string) {
	src := filepath.Join(s.vectorSpoolDir("inbox"), name)
	run := filepath.Join(s.vectorSpoolDir("running"), name)
	// Judged before the claim, on the link itself: a symlink or any other
	// non-regular file is moved to refused/ as it is and never opened.
	fi, lerr := os.Lstat(src)
	if err := os.Rename(src, run); err != nil {
		s.finishVectorJob(job, VectorImport{}, fmt.Errorf("could not claim %s: %w", name, err))
		return
	}
	var res VectorImport
	var err error
	switch {
	case lerr != nil:
		err = fmt.Errorf("%w: %v", ErrVectorImportRefused, lerr)
	case !fi.Mode().IsRegular():
		err = fmt.Errorf("%w: %s is not a regular file (a symlink or special file; not followed)", ErrVectorImportRefused, name)
	case fi.Size() > vectorImportMaxBytes:
		err = fmt.Errorf("%w: %s is %d bytes, over the %d the import takes", ErrVectorImportRefused, name, fi.Size(), vectorImportMaxBytes)
	default:
		res, err = s.ImportVectorFile(ctx, run)
	}
	if ctx.Err() != nil && err != nil {
		// Shutdown mid-import: the file stays in running/ and is moved to
		// interrupted/ at the next start.
		s.finishVectorJob(job, res, err)
		return
	}
	out := s.finishVectorJob(job, res, err)
	dest := "done"
	if out.State != "done" {
		dest = "refused"
	}
	if rerr := os.Rename(run, filepath.Join(s.vectorSpoolDir(dest), name)); rerr != nil {
		log.Printf("corpusd vectors inbox: %s finished %s but could not be moved to %s/: %v", name, out.State, dest, rerr)
		return
	}
	rec, _ := json.MarshalIndent(out, "", "  ")
	if werr := os.WriteFile(filepath.Join(s.vectorSpoolDir(dest), name+".job.json"), append(rec, '\n'), 0o640); werr != nil {
		log.Printf("corpusd vectors inbox: job record of %s not written: %v", name, werr)
	}
}
