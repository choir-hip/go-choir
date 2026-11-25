package main

import (
	"flag"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/auth"
)

// runDeletionOps is `auth deletion-ops`, the operator's two tools for an
// account deletion that is stuck (docs/runbooks/account-deletion-stuck.md):
//
//	auth deletion-ops list  -db <auth.db>
//	auth deletion-ops rerun -db <auth.db> -job <job id> -steps tape[,gateway,...]
//
// list prints one line per open deletion by job id, never by person: state,
// when its purge was due, attempts, the steps done and the last error. rerun
// forgets the named steps of one job so the purge runner does them again on its
// next pass (every step is idempotent). Neither finalizes a deletion, deletes a
// record or touches a user: forcing a purge past a refusal is an owner act and
// is not a command here. Exit 0 done; 1 refused; 2 usage or read error.
func runDeletionOps(args []string, out io.Writer) int {
	if len(args) == 0 || (args[0] != "list" && args[0] != "rerun") {
		fmt.Fprintln(out, "usage: auth deletion-ops list -db <auth.db> | rerun -db <auth.db> -job <job id> -steps <step[,step...]>")
		return 2
	}
	fs := flag.NewFlagSet("deletion-ops "+args[0], flag.ContinueOnError)
	fs.SetOutput(out)
	dbPath := fs.String("db", "", "the auth.db of the running service")
	job := fs.String("job", "", "the job id (del-...)")
	steps := fs.String("steps", "", "comma-separated steps to run again: mail, data, computers, gateway, tape")
	if err := fs.Parse(args[1:]); err != nil {
		return 2
	}
	if *dbPath == "" {
		fmt.Fprintln(out, "-db is required")
		return 2
	}
	store, err := auth.OpenStore(*dbPath)
	if err != nil {
		fmt.Fprintln(out, err)
		return 2
	}
	defer func() { _ = store.Close() }()
	open, err := store.OpenAccountDeletions(0)
	if err != nil {
		fmt.Fprintln(out, err)
		return 2
	}
	if args[0] == "list" {
		now := time.Now().UTC()
		for _, d := range open {
			var done []string
			for name := range d.Steps {
				done = append(done, name)
			}
			sort.Strings(done)
			fmt.Fprintf(out, "job=%s state=%s due=%s overdue=%s attempts=%d steps_done=%s last_error=%q\n",
				d.JobID, d.State, d.PurgeAfter.UTC().Format(time.RFC3339), now.Sub(d.PurgeAfter).Round(time.Minute), d.Attempts, strings.Join(done, ","), d.LastError)
		}
		fmt.Fprintf(out, "open deletions: %d\n", len(open))
		return 0
	}
	if *job == "" || *steps == "" {
		fmt.Fprintln(out, "rerun needs -job and -steps")
		return 2
	}
	var names []string
	for _, n := range strings.Split(*steps, ",") {
		n = strings.TrimSpace(n)
		switch n {
		case "mail", "data", "computers", "gateway", "tape":
			names = append(names, n)
		default:
			fmt.Fprintf(out, "refused: %q is not a step that can be run again (mail, data, computers, gateway, tape)\n", n)
			return 1
		}
	}
	for _, d := range open {
		if d.JobID != *job {
			continue
		}
		if d.State != auth.DeletionPurging {
			fmt.Fprintf(out, "refused: job %s is %s, not purging; there is nothing to run again\n", d.JobID, d.State)
			return 1
		}
		if err := store.ClearDeletionSteps(d.UserID, names...); err != nil {
			fmt.Fprintln(out, err)
			return 2
		}
		fmt.Fprintf(out, "job %s: steps %s will run again on the purge runner's next pass\n", d.JobID, strings.Join(names, ","))
		return 0
	}
	fmt.Fprintf(out, "refused: no open deletion has job id %q\n", *job)
	return 1
}
