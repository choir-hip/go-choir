//go:build linux

package capsule

import (
	"context"
	"strings"
	"time"
)

// stageScreens copies the capsule's screens into <temporary>/evidence/screens/
// (stageScreenFiles). Files are opened beneath the capsule root without
// following symlinks, as release files are. nil when there are none.
func (e *Executor) stageScreens(ctx context.Context, rootFD int, changes []FileChange, temporary string) ([]ScreenFile, error) {
	names, err := screenNames(changes)
	if err != nil || len(names) == 0 {
		return nil, err
	}
	// A screen older than the newest source change shows an older build.
	newest, source, err := newestSourceChange(changes, func(path string) (time.Time, error) {
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
	})
	if err != nil {
		return nil, err
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
