package builder

import (
	"fmt"
	"path"
	"strings"
)

// PatchedChange is one path the committed patched tree changed against the
// base commit, as git recorded it (`diff --raw --no-renames`). Modes are
// git's octal modes; "000000" means absent on that side.
type PatchedChange struct {
	Path    string `json:"path"`
	OldMode string `json:"old_mode"`
	NewMode string `json:"new_mode"`
	Status  string `json:"status"`
}

// PatchRefusal is the builder's own refusal of a patched tree: a link, git
// metadata, or a file the commit does not carry.
type PatchRefusal struct {
	Path   string
	Detail string
}

func (e *PatchRefusal) Error() string {
	return "builder: patched tree refused: " + e.Detail + " (" + e.Path + ")"
}

const (
	modeSymlink = "120000"
	modeGitlink = "160000"
)

// RefuseTreeMetadata refuses a change that makes the tree mean something
// beyond its file bytes (H16): a symlink or a gitlink on either side, or a
// .gitattributes or .gitmodules file at any depth.
func RefuseTreeMetadata(changes []PatchedChange) error {
	for _, c := range changes {
		for _, mode := range []string{c.OldMode, c.NewMode} {
			switch mode {
			case modeSymlink:
				return &PatchRefusal{Path: c.Path, Detail: "the patch changes a symbolic link"}
			case modeGitlink:
				return &PatchRefusal{Path: c.Path, Detail: "the patch changes a submodule link"}
			}
		}
		switch path.Base(c.Path) {
		case ".gitattributes", ".gitmodules":
			return &PatchRefusal{Path: c.Path, Detail: "the patch changes git metadata"}
		}
	}
	return nil
}

// parseRawDiff reads `diff --raw -z --no-renames` output: records of
// ":<old mode> <new mode> <old sha> <new sha> <status>\0<path>\0".
func parseRawDiff(raw string) ([]PatchedChange, error) {
	fields := strings.Split(strings.TrimSuffix(raw, "\x00"), "\x00")
	if len(fields) == 1 && fields[0] == "" {
		return nil, nil
	}
	if len(fields)%2 != 0 {
		return nil, fmt.Errorf("builder: unreadable committed change list")
	}
	changes := make([]PatchedChange, 0, len(fields)/2)
	for i := 0; i < len(fields); i += 2 {
		meta := strings.Fields(strings.TrimPrefix(fields[i], ":"))
		if !strings.HasPrefix(fields[i], ":") || len(meta) != 5 || fields[i+1] == "" {
			return nil, fmt.Errorf("builder: unreadable committed change %q", fields[i])
		}
		changes = append(changes, PatchedChange{Path: fields[i+1], OldMode: meta[0], NewMode: meta[1], Status: meta[4]})
	}
	return changes, nil
}
