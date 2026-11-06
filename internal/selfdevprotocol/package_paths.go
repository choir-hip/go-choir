package selfdevprotocol

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/packagepath"
)

// BundlePackagePaths lists the package paths a frozen bundle's ordered file
// effects change under /workspace/platform (forced modularity M2, door 3,
// docs/design/forced-modularity-build-2026-11-04.md section 2). corpusd is
// the only authority the computer does not sign, so it reads the bundle
// itself: a hand-built bundle whose platform change carries app source is
// refused there even if it never reached the host build. An error means the
// bundle does not decode.
func BundlePackagePaths(bundle []byte) ([]string, error) {
	var fields struct {
		Effects []struct {
			Path string `json:"path"`
		} `json:"ordered_file_effects"`
	}
	decoder := json.NewDecoder(bytes.NewReader(bundle))
	if err := decoder.Decode(&fields); err != nil {
		return nil, errors.New("the bundle does not decode")
	}
	var paths []string
	for _, effect := range fields.Effects {
		p := strings.TrimLeft(strings.TrimSpace(effect.Path), "/")
		rel, ok := strings.CutPrefix(p, "workspace/platform/")
		if !ok {
			continue
		}
		if packagepath.RootOf(rel) != "" {
			paths = append(paths, rel)
		}
	}
	return paths, nil
}

// RefuseBundlePackagePaths is the refusal text for a platform bundle that
// changes a package path, or "".
func RefuseBundlePackagePaths(bundle []byte) string {
	paths, err := BundlePackagePaths(bundle)
	if err != nil {
		return packagepath.ReasonPackagePath + ": " + err.Error()
	}
	if len(paths) == 0 {
		return ""
	}
	return packagepath.ReasonPackagePath + ": the bundle's platform change carries the package paths " + packagepath.Shown(paths, 5) + "; the source stack carries platform operations only"
}
