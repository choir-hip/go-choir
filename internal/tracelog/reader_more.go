package tracelog

import (
	"math"
	"sort"
)

// Two readers the store's list methods need beyond the four of the design's
// S2 list (docs/design/trace-segment-log-2026-10-22.md 9.2 "S2 build notes"):
// ListEventsAfter takes a run id and no owner, and ListEventsByTrajectory
// rebuilds the newest window of a trajectory.

// ByRunAnyOwner returns the lines of every owner's run with that id (the empty
// id included) whose seq is greater than afterSeq, ascending in seq with
// stream_seq as the tie-break, at most limit (200 when limit is not positive).
// It is ByRun without the owner, as the store's run readers have no owner.
func (r *Reader) ByRunAnyOwner(run string, afterSeq int64, limit int) ([]Record, error) {
	if limit <= 0 {
		limit = 200
	}
	ls, err := r.list()
	if err != nil {
		return nil, err
	}
	var sf soft
	var cands []cand
	full, thresh := false, int64(math.MaxInt64)
	for _, f := range ls.segs {
		sum, err := r.summary(f)
		if skippable(&sf, err) {
			continue
		}
		if sum != nil {
			lo, hit := int64(math.MaxInt64), false
			for key, rng := range sum.Runs {
				if runOfKey(key) == run && rng[1] > afterSeq {
					hit, lo = true, min(lo, rng[0])
				}
			}
			if !hit || (full && lo > thresh) {
				continue
			}
		}
		err = r.withIdx(f, func(x *segIdx) {
			ru, ok := x.strs.lookup(run)
			if !ok {
				return
			}
			for k, list := range x.byRun {
				if k[1] != ru {
					continue
				}
				for _, i := range list {
					if e := x.ents[i]; e.seq > afterSeq {
						cands = append(cands, cand{f.segno, e.off, e.n, e.seq, e.stream})
					}
				}
			}
		})
		if err != nil && !skippable(&sf, err) {
			return nil, err
		}
		if len(cands) >= limit {
			sortCands(cands, true)
			cands = cands[:limit]
			full, thresh = true, cands[limit-1].seq
		}
	}
	sortCands(cands, true)
	if len(cands) > limit {
		cands = cands[:limit]
	}
	return r.finish(cands, &sf)
}

// ByTrajectoryNewest returns the owner's newest limit lines of a trajectory,
// newest first by stream_seq. ByTrajectory returns the first lines; the
// store's trajectory window is the newest.
func (r *Reader) ByTrajectoryNewest(owner, traj string, limit int) ([]Record, error) {
	if limit <= 0 {
		limit = 200
	}
	ls, err := r.list()
	if err != nil {
		return nil, err
	}
	var sf soft
	var cands []cand
	full, thresh := false, int64(math.MinInt64)
	newestFirst := func(c []cand) {
		sort.Slice(c, func(i, j int) bool {
			if c[i].stream != c[j].stream {
				return c[i].stream > c[j].stream
			}
			return c[i].off > c[j].off
		})
	}
	for i := len(ls.segs) - 1; i >= 0; i-- {
		f := ls.segs[i]
		sum, err := r.summary(f)
		if skippable(&sf, err) {
			continue
		}
		if sum != nil {
			rng, ok := sum.Trajs[runKey(owner, traj)]
			if !ok || (full && rng[1] < thresh) {
				continue
			}
		}
		err = r.withIdx(f, func(x *segIdx) {
			o, ok1 := x.strs.lookup(owner)
			t, ok2 := x.strs.lookup(traj)
			if !ok1 || !ok2 {
				return
			}
			for _, j := range x.byTraj[pair{o, t}] {
				e := x.ents[j]
				cands = append(cands, cand{f.segno, e.off, e.n, e.seq, e.stream})
			}
		})
		if err != nil && !skippable(&sf, err) {
			return nil, err
		}
		if len(cands) >= limit {
			newestFirst(cands)
			cands = cands[:limit]
			full, thresh = true, cands[limit-1].stream
		}
	}
	newestFirst(cands)
	if len(cands) > limit {
		cands = cands[:limit]
	}
	return r.finish(cands, &sf)
}

// runOfKey is the run (or trajectory) half of a summary key, which is the
// owner, a NUL and the run.
func runOfKey(key string) string {
	for i := 0; i < len(key); i++ {
		if key[i] == 0 {
			return key[i+1:]
		}
	}
	return key
}
