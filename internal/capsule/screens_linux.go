//go:build linux

package capsule

import "context"

// stageScreens copies the capsule's screens into <temporary>/evidence/screens/
// (stageScreenFiles). Files are opened beneath the capsule root without
// following symlinks, as release files are. nil when there are none.
func (e *Executor) stageScreens(ctx context.Context, rootFD int, changes []FileChange, temporary string) ([]ScreenFile, error) {
	names, err := screenNames(changes)
	if err != nil || len(names) == 0 {
		return nil, err
	}
	return stageScreenFiles(ctx, names, func(name string) (screenInput, error) {
		in, err := openBeneath(rootFD, ScreensPrefix+name)
		if err != nil {
			return nil, err
		}
		return in, nil
	}, temporary)
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
