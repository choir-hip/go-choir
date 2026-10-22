package store

// Run acceptance records the trajectory's desk path in
// metadata.authority_profile as roles joined by " > " ("texture >
// research"). After the vocabulary cutover the write guard checked the
// joined string as one token and refused every multi-desk acceptance
// (docs/problems/run-acceptance-refused-by-vocab-fence-on-multi-desk-trajectory-2026-10-22.md).
//
// Failure modes this guards: a joined path refused (the defect); a retired
// role hidden inside a path accepted (the guard opened instead of split);
// empty parts accepted (the fence skips empty values, so a split that
// forwards them would pass " >  > "); a single role refused; the path
// reading widened to other kinds, whose authority_profile is one role.

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
)

func TestVocabWriteGuardRunAcceptanceAuthorityPath(t *testing.T) {
	s := openTestStore(t)
	s.vocabCutover.Store(true)
	object := func(kind, profile string) []objectgraph.Object {
		meta, _ := json.Marshal(map[string]string{"authority_profile": profile})
		return []objectgraph.Object{{ObjectKind: objectgraph.ObjectKind(kind), Metadata: meta, Body: []byte(`{}`)}}
	}
	const acceptance = "choir.run_acceptance"
	cases := []struct {
		kind, profile string
		ok            bool
	}{
		{acceptance, "texture > research", true},
		{acceptance, "texture > research > engineering", true},
		{acceptance, "texture", true},
		{acceptance, "texture > researcher", false},
		{acceptance, "super > research", false},
		{acceptance, "texture > boss", false},
		{acceptance, " >  > ", false},
		{acceptance, "texture >  > research", false},
		{acceptance, "texture > ", false},
		{acceptance, "texture>research", false},
		{"choir.work_item", "texture > research", false},
	}
	for _, c := range cases {
		err := s.vocabWriteGuard(context.Background(), object(c.kind, c.profile), nil)
		if c.ok && err != nil {
			t.Errorf("%s authority_profile=%q refused: %v", c.kind, c.profile, err)
		}
		if !c.ok && err == nil {
			t.Errorf("%s authority_profile=%q accepted; want refused", c.kind, c.profile)
		}
	}
}
