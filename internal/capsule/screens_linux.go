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
