package tracelog

// Two readers slice S2 of the design needs and S1 did not ship
// (docs/design/trace-segment-log-2026-10-22.md, 6.2): the store's
// ListEventsAfter takes a run id and no owner, and ListEventsByTrajectory
// rebuilds the newest-10,000 window of a trajectory. Ways they can fail,
// written before the code:
//
//   - the run reader forgets the owners it was not asked about (a run id is
//     shared by two owners, and the empty run id is shared by all of them);
//   - it skips a sealed segment by a summary that is keyed by owner and run;
//   - it orders by stream_seq where the page cursor is seq, or the other way;
//   - the trajectory reader returns the oldest lines where the newest are
//     wanted, or loads every line of a long trajectory to find them;
//   - either reads past a segment that retention deleted, or fails on one.

import (
	"fmt"
	"sort"
	"testing"
)

// shapedLog writes 120 lines across sealed segments and an open one: three
// owners, a run id shared by two of them, the empty run, and two trajectories.
func shapedLog(t *testing.T, dir string) (*Log, []Meta) {
	t.Helper()
	l, _ := openT(t, dir, func(o *Options) { o.SealRecords = 9 })
	var all []Meta
	for i := 1; i <= 120; i++ {
		m := meta(i)
		m.OwnerID = []string{"owner-1", "owner-2", "owner-3"}[i%3]
		m.RunID = []string{"run-a", "run-b", "", "run-a"}[i%4]
		m.TrajectoryID = []string{"traj-1", "traj-2"}[(i/5)%2]
		m.Seq = int64(i/3 + 1) // not unique: two owners share a seq
		if err := l.Append(m, lineFor(i)); err != nil {
			t.Fatal(err)
		}
		all = append(all, m)
	}
	return l, all
}

func TestByRunAnyOwnerMatchesABruteForceScan(t *testing.T) {
	dir := t.TempDir()
	l, all := shapedLog(t, dir)
	readers := map[string]*Reader{"live": l.Reader(), "static": OpenReader(dir)}
	for name, r := range readers {
		for _, run := range []string{"run-a", "run-b", "", "run-none"} {
			for _, after := range []int64{0, 1, 7, 20, 39, 40, 500} {
				for _, limit := range []int{1, 3, 1000} {
					var want []Meta
					for _, m := range all {
						if m.RunID == run && m.Seq > after {
							want = append(want, m)
						}
					}
					sort.Slice(want, func(i, j int) bool {
						if want[i].Seq != want[j].Seq {
							return want[i].Seq < want[j].Seq
						}
						return want[i].StreamSeq < want[j].StreamSeq
					})
					if len(want) > limit {
						want = want[:limit]
					}
					got, err := r.ByRunAnyOwner(run, after, limit)
					if err != nil {
						t.Fatalf("%s ByRunAnyOwner(%q,%d,%d): %v", name, run, after, limit, err)
					}
					if len(got) != len(want) {
						t.Fatalf("%s ByRunAnyOwner(%q,%d,%d) = %d records, want %d", name, run, after, limit, len(got), len(want))
					}
					for i := range got {
						if got[i].Meta.StreamSeq != want[i].StreamSeq {
							t.Fatalf("%s ByRunAnyOwner(%q,%d,%d)[%d] = stream %d, want %d", name, run, after, limit, i, got[i].Meta.StreamSeq, want[i].StreamSeq)
						}
						if string(got[i].Line) != string(lineFor(int(want[i].StreamSeq))) {
							t.Fatalf("%s ByRunAnyOwner: line %d came back as %q", name, want[i].StreamSeq, got[i].Line)
						}
					}
				}
			}
		}
	}
}

func TestByTrajectoryNewestIsTheNewestLinesNewestFirst(t *testing.T) {
	dir := t.TempDir()
	l, all := shapedLog(t, dir)
	for name, r := range map[string]*Reader{"live": l.Reader(), "static": OpenReader(dir)} {
		for _, owner := range []string{"owner-1", "owner-2", "owner-3", "owner-none"} {
			for _, traj := range []string{"traj-1", "traj-2", "traj-none"} {
				for _, limit := range []int{1, 4, 1000} {
					var want []int64
					for _, m := range all {
						if m.OwnerID == owner && m.TrajectoryID == traj {
							want = append(want, m.StreamSeq)
						}
					}
					sort.Slice(want, func(i, j int) bool { return want[i] > want[j] })
					if len(want) > limit {
						want = want[:limit]
					}
					got, err := r.ByTrajectoryNewest(owner, traj, limit)
					if err != nil {
						t.Fatalf("%s ByTrajectoryNewest(%s,%s,%d): %v", name, owner, traj, limit, err)
					}
					var gotStreams []int64
					for _, rec := range got {
						gotStreams = append(gotStreams, rec.Meta.StreamSeq)
					}
					if fmt.Sprint(gotStreams) != fmt.Sprint(want) {
						t.Fatalf("%s ByTrajectoryNewest(%s,%s,%d) = %v, want %v", name, owner, traj, limit, gotStreams, want)
					}
				}
			}
		}
	}
}

// A segment deleted by retention is skipped, not an error, by both readers.
func TestNewReadersSurviveRetention(t *testing.T) {
	dir := t.TempDir()
	l, _ := shapedLog(t, dir)
	r := l.Reader()
	if _, err := r.ByRunAnyOwner("run-a", 0, 1000); err != nil { // warm the summaries
		t.Fatal(err)
	}
	l.SetRetention(0, 1) // keep nothing that is sealed
	res, err := l.Retain()
	if err != nil || len(res.Deleted) == 0 {
		t.Fatalf("Retain deleted %v, err %v", res.Deleted, err)
	}
	if _, err := r.ByRunAnyOwner("run-a", 0, 1000); err != nil {
		t.Fatalf("ByRunAnyOwner after retention: %v", err)
	}
	if _, err := r.ByTrajectoryNewest("owner-1", "traj-1", 1000); err != nil {
		t.Fatalf("ByTrajectoryNewest after retention: %v", err)
	}
}
