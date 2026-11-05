// Command deskprotocol-lock extracts the desk protocol surface, checks it
// against its lock and writes new locks (internal/deskprotocol, slice D0).
//
//	deskprotocol-lock            check: the surface equals lock/<Version>.json
//	                             and every adjacent pair of locks is bumped
//	                             enough
//	  --base REF                 also refuse any lock that existed at
//	                             merge-base(REF, HEAD) and was changed,
//	                             deleted or renamed
//	  --locks-only               check the lock files and history only; no
//	                             extraction (the base's copy runs this way, so
//	                             its own run-time tables are never judged)
//	  --write                    write the lock for Version, once, only when
//	                             the bump from the latest lock is enough
//	  --json                     print the extracted surface
//	  --root DIR                 the checkout (default: the current directory)
package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/deskprotocol"
	"github.com/yusefmosiah/go-choir/internal/deskprotocol/extract"
)

func main() {
	var (
		base      = flag.String("base", "", "git ref the lock history is checked against")
		locksOnly = flag.Bool("locks-only", false, "check lock files and history only")
		write     = flag.Bool("write", false, "write the lock for Version")
		asJSON    = flag.Bool("json", false, "print the extracted surface")
		root      = flag.String("root", ".", "the checkout")
	)
	flag.Parse()
	abs, err := filepath.Abs(*root)
	if err != nil {
		fail(err)
	}
	problems, err := run(abs, *base, *locksOnly, *write, *asJSON)
	if err != nil {
		fail(err)
	}
	if len(problems) > 0 {
		for _, p := range problems {
			fmt.Fprintln(os.Stderr, "desk-protocol:", p)
		}
		os.Exit(1)
	}
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "desk-protocol:", err)
	os.Exit(1)
}

func run(root, base string, locksOnly, write, asJSON bool) ([]string, error) {
	lockDir := filepath.Join(root, filepath.FromSlash(deskprotocol.LockDir))
	var problems []string

	if !locksOnly || write || asJSON {
		cur, err := extract.Surface(root)
		if err != nil {
			return nil, fmt.Errorf("extract: %w", err)
		}
		if len(cur.Names) < deskprotocol.MinNames || len(cur.Verbs) < deskprotocol.MinVerbs || len(cur.Desks) < deskprotocol.MinDesks {
			return nil, fmt.Errorf("extraction is below the floors (names %d/%d, verbs %d/%d, desks %d/%d): a broken extractor must not match a lock",
				len(cur.Names), deskprotocol.MinNames, len(cur.Verbs), deskprotocol.MinVerbs, len(cur.Desks), deskprotocol.MinDesks)
		}
		if asJSON {
			b, err := cur.Marshal()
			if err != nil {
				return nil, err
			}
			os.Stdout.Write(b)
			return nil, nil
		}
		locks, err := deskprotocol.LoadLocks(lockDir)
		if err != nil && !(write && os.IsNotExist(err)) {
			return nil, err
		}
		if write {
			return nil, writeLock(lockDir, locks, cur)
		}
		problems = append(problems, compare(locks, cur)...)
	}

	locks, err := deskprotocol.LoadLocks(lockDir)
	if err != nil {
		return nil, err
	}
	problems = append(problems, deskprotocol.CheckLockSet(locks)...)
	if base != "" {
		ps, err := history(root, base)
		if err != nil {
			return nil, err
		}
		problems = append(problems, ps...)
	}
	return problems, nil
}

// compare is the check proper: the current surface must equal the lock for
// Version. A difference is classified and refused; locks are never edited.
func compare(locks []*deskprotocol.Surface, cur *deskprotocol.Surface) []string {
	var at *deskprotocol.Surface
	for _, l := range locks {
		if l.Version == deskprotocol.Version {
			at = l
		}
	}
	switch {
	case at == nil:
		return []string{fmt.Sprintf("no lock for Version %s; run deskprotocol-lock --write once the bump is right", deskprotocol.Version)}
	case !at.Equal(cur):
		res := deskprotocol.Classify(at, cur)
		return []string{fmt.Sprintf("the surface differs from lock %s (%s change); locks are never edited: bump deskprotocol.Version by at least a %s step and --write a new lock:\n%s",
			at.Version, res.Level, res.Level, deskprotocol.FormatChanges(res))}
	}
	return nil
}

func writeLock(lockDir string, locks []*deskprotocol.Surface, s *deskprotocol.Surface) error {
	for _, l := range locks {
		if l.Version == s.Version {
			return fmt.Errorf("lock %s exists; locks are only ever added", s.Version)
		}
	}
	if n := len(locks); n > 0 {
		last := locks[n-1]
		got, err := deskprotocol.Bump(last.Version, s.Version)
		if err != nil {
			return err
		}
		if res := deskprotocol.Classify(last, s); got < res.Level {
			return fmt.Errorf("%s -> %s is a %s bump but the change is %s:\n%s", last.Version, s.Version, got, res.Level, deskprotocol.FormatChanges(res))
		}
	}
	b, err := s.Marshal()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(lockDir, 0o755); err != nil {
		return err
	}
	path := filepath.Join(lockDir, s.Version+".json")
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return err
	}
	if _, err := f.Write(b); err != nil {
		f.Close()
		return err
	}
	fmt.Println("wrote", path)
	return f.Close()
}

func git(root string, args ...string) (string, error) {
	cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
	}
	return strings.TrimSpace(string(out)), nil
}

// history refuses any lock present at the merge base that is now changed,
// deleted or renamed. A git failure fails the check.
func history(root, base string) ([]string, error) {
	mb, err := git(root, "merge-base", base, "HEAD")
	if err != nil {
		return nil, fmt.Errorf("lock history: %w", err)
	}
	// The locks and the golden corpus are both version-frozen.
	list, err := git(root, "ls-tree", "-r", "--name-only", mb, "--", deskprotocol.LockDir, deskprotocol.TestdataDir)
	if err != nil {
		return nil, fmt.Errorf("lock history: %w", err)
	}
	var problems []string
	for _, p := range strings.Fields(list) {
		want, err := exec.Command("git", "-C", root, "show", mb+":"+p).Output()
		if err != nil {
			return nil, fmt.Errorf("lock history: git show %s:%s: %w", mb, p, err)
		}
		got, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(p)))
		switch {
		case os.IsNotExist(err):
			problems = append(problems, fmt.Sprintf("%s existed at the merge base and is gone (deleted or renamed)", p))
		case err != nil:
			return nil, err
		case string(got) != string(want):
			problems = append(problems, fmt.Sprintf("%s existed at the merge base and was changed; locks and the golden corpus are only ever added", p))
		}
	}
	return problems, nil
}
