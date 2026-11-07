//go:build linux

package capsule

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"
)

// stageScreens copies the capsule's screens into <temporary>/evidence/screens/
// (stageScreenFiles). Files are opened beneath the capsule root without
// following symlinks, as release files are. nil when there are none.
func (e *Executor) stageScreens(ctx context.Context, rootFD int, changes []FileChange, temporary string, planned bool) ([]ScreenFile, error) {
	names, err := screenNames(changes)
	if err != nil || len(names) == 0 {
		return nil, err
	}
	// A screen older than the newest source change shows an older build.
	statTime := func(path string) (time.Time, error) {
		in, err := openBeneath(rootFD, strings.TrimPrefix(path, "/"))
		if err != nil {
			return time.Time{}, err
		}
		defer in.Close()
		info, err := in.Stat()
		if err != nil {
			return time.Time{}, err
		}
		return info.ModTime(), nil
	}
	oldest, newest, source, err := sourceChangeTimes(changes, statTime)
	if err != nil {
		return nil, err
	}
	// A plan bundle's before shots come from the running build on the
	// platform, not from this capsule, so the late-before rule has nothing
	// to judge (design 3.3, Freeze rule).
	if !planned {
		if err := refuseLateBeforeShots(names, oldest, func(name string) (time.Time, error) { return statTime(ScreensPrefix + name) }); err != nil {
			return nil, err
		}
	}
	return stageScreenFiles(ctx, names, func(name string) (screenInput, error) {
		in, err := openBeneath(rootFD, ScreensPrefix+name)
		if err != nil {
			return nil, err
		}
		return in, nil
	}, temporary, newest, source)
}

// stageVideo copies the capsule's one walkthrough video, if any, beside the
// screens (stageVideoFile), after them: the screens' directory is created
// exclusively. Opened beneath the capsule root without following symlinks.
func (e *Executor) stageVideo(ctx context.Context, rootFD int, changes []FileChange, temporary string) (*VideoFile, error) {
	name, err := videoName(changes)
	if err != nil || name == "" {
		return nil, err
	}
	return stageVideoFile(ctx, name, func(name string) (screenInput, error) {
		in, err := openBeneath(rootFD, ScreensPrefix+name)
		if err != nil {
			return nil, err
		}
		return in, nil
	}, temporary)
}

// readShotPlan reads and validates the engineer's shot plan, if it wrote
// one at ShotPlanFile, opened beneath the capsule root without following
// symlinks. nil for none. A plan that does not decode or validate refuses
// the Freeze with the reason; it is never dropped silently.
func (e *Executor) readShotPlan(rootFD int, changes []FileChange) (*ShotPlan, error) {
	present := false
	for _, change := range changes {
		if strings.TrimPrefix(change.Path, "/") == ShotPlanPrefix && change.Kind != ChangeDeleted {
			present = true
		}
	}
	if !present {
		return nil, nil
	}
	in, err := openBeneath(rootFD, ShotPlanPrefix)
	if err != nil {
		return nil, fmt.Errorf("capsule freeze: shot plan %s is unavailable: %w", ShotPlanFile, err)
	}
	defer in.Close()
	info, err := in.Stat()
	if err != nil || !info.Mode().IsRegular() {
		return nil, fmt.Errorf("capsule freeze: shot plan %s is not a regular file", ShotPlanFile)
	}
	raw, err := io.ReadAll(io.LimitReader(in, MaxShotPlanBytes+1))
	if err != nil {
		return nil, err
	}
	return DecodeShotPlan(raw)
}
