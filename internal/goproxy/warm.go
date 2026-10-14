package goproxy

import (
	"context"
	"sort"
	"strings"
	"sync"
	"time"
)

// WarmReport summarizes one warm pass.
type WarmReport struct {
	Entries  int   `json:"entries"`
	Hits     int   `json:"hits"`
	Fetched  int   `json:"fetched"`
	Failed   int   `json:"failed"`
	Mismatch int   `json:"mismatch"`
	Bytes    int64 `json:"cache_bytes"`
}

// Warm makes sure every go.sum entry is cached and matches its hash: the
// .mod for a "/go.mod" line, the .info and .zip for a module line. A
// cached file that does not match is deleted and fetched again once. Warm
// only logs; it never stops the service.
func (s *Server) Warm(ctx context.Context, sums map[string]string, workers int) WarmReport {
	if workers <= 0 {
		workers = 4
	}
	// ensure checks fetched files against the known sums, so this go.sum
	// becomes part of them before anything is fetched.
	s.addKnown(sums)
	keys := make([]string, 0, len(sums))
	for k := range sums {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var (
		mu  sync.Mutex
		rep WarmReport
		wg  sync.WaitGroup
	)
	jobs := make(chan string)
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for key := range jobs {
				res := s.warmOne(ctx, key, sums[key])
				mu.Lock()
				rep.Entries++
				switch res {
				case OutcomeHit:
					rep.Hits++
				case OutcomeFetched:
					rep.Fetched++
				case OutcomeHashMismatch:
					rep.Mismatch++
				default:
					rep.Failed++
				}
				mu.Unlock()
			}
		}()
	}
	for _, k := range keys {
		select {
		case jobs <- k:
		case <-ctx.Done():
		}
		if ctx.Err() != nil {
			break
		}
	}
	close(jobs)
	wg.Wait()
	rep.Bytes = s.cache.bytes()
	return rep
}

func (s *Server) warmOne(ctx context.Context, key, want string) string {
	start := time.Now()
	rec := Record{Event: "warm", Source: "warm"}
	finish := func(outcome, detail string) string {
		rec.Time, rec.Outcome, rec.Detail = start.UTC(), outcome, detail
		rec.DurationMS = time.Since(start).Milliseconds()
		if outcome != OutcomeHit {
			s.record(rec)
		}
		return outcome
	}
	if ctx.Err() != nil {
		return finish(OutcomeUpstreamError, ctx.Err().Error())
	}
	mod, ver, ok := strings.Cut(key, " ")
	if !ok {
		return finish(OutcomeRefused, "malformed go.sum key "+key)
	}
	kind := kindZip
	if v, isMod := strings.CutSuffix(ver, "/go.mod"); isMod {
		kind, ver = kindMod, v
	}
	emod, err1 := escape(mod)
	ever, err2 := escape(ver)
	if err1 != nil || err2 != nil || checkEscapedModulePath(emod) != nil || !isCanonicalEscapedVersion(ever) {
		return finish(OutcomeRefused, "unservable go.sum entry "+key)
	}
	base := emod + "/@v/" + ever
	main := request{kind: kind, module: emod, version: ever}
	if kind == kindZip {
		main.upstreamPath = base + ".zip"
		// The .info sits beside the zip; go mod download writes it too.
		info := request{kind: kindInfo, module: emod, version: ever, upstreamPath: base + ".info", cacheRel: "cache/" + base + ".info"}
		if _, err := s.ensure(info); err != nil {
			rec.Path = "/" + info.upstreamPath
			return finish(OutcomeUpstreamError, err.Error())
		}
	} else {
		main.upstreamPath = base + ".mod"
	}
	main.cacheRel = "cache/" + main.upstreamPath
	rec.Path = "/" + main.upstreamPath

	outcome := OutcomeFetched
	if path, hit := s.cache.lookup(main.cacheRel); hit {
		got, err := hashFile(kind, path)
		if err == nil && got == want {
			s.cache.touch(main.cacheRel)
			return finish(OutcomeHit, "")
		}
		// Corrupt or tampered on disk: drop it and fetch again.
		s.cache.remove(main.cacheRel)
		rec.Detail = "cached copy did not match go.sum; refetching"
		s.record(Record{Time: time.Now().UTC(), Event: "warm", Source: "warm", Path: rec.Path, Outcome: OutcomeHashMismatch, Detail: rec.Detail})
	}
	if _, err := s.ensure(main); err != nil {
		if _, mismatch := err.(*hashMismatchError); mismatch {
			return finish(OutcomeHashMismatch, err.Error())
		}
		return finish(OutcomeUpstreamError, err.Error())
	}
	s.cache.touch(main.cacheRel)
	return finish(outcome, "")
}
