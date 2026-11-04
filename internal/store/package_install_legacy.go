package store

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
)

// Legacy install records. Before MA2.4 the install kind was named for the
// deleted release-adopt flow (kind choir.release_adoption, body field
// adoption_id). Owner rule: install is the only name. Tapes written before
// the rename still hold records of the old kind, and the tape is replayed, not
// rewritten, so the old kind and field stay readable:
//
//   - reads fold both kinds (GetPackageInstall, ListPackageInstalls), the new
//     kind preferred;
//   - decodePackageInstall accepts the old body field;
//   - writes use the new kind only.
//
// Nothing is moved or tombstoned: a code revert loses at most the installs
// written after deploy. The physical move is a later slice, after a soak and
// proved on d@e first.
//
// This file is the only place the old names live (heresy detector exception).
const legacyInstallKind = objectgraph.ObjectKind("choir.release_adoption")

// decodePackageInstall decodes a record of either kind; the old body names
// the id adoption_id.
func decodePackageInstall(obj objectgraph.Object) (PackageInstall, error) {
	var install PackageInstall
	if err := ogDecode(obj, &install); err != nil {
		return PackageInstall{}, err
	}
	if strings.TrimSpace(install.InstallID) == "" {
		var legacy struct {
			ID string `json:"adoption_id"`
		}
		if err := json.Unmarshal(obj.Body, &legacy); err != nil {
			return PackageInstall{}, fmt.Errorf("store: unmarshal install body: %w", err)
		}
		install.InstallID = legacy.ID
	}
	return install, nil
}
