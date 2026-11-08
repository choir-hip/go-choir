package objectgraph

import "testing"

// Enforce flip S1, D3 (section 3): Texture is a supplemental reader of the two
// kinds its source cards resolve from refs a desk's record carries:
// content_item (the shell custodian's text, which Texture already renders) and
// agent_evidence (research's). It is admitted to those kinds alone: the
// custodian's other kinds, and every other desk, stay as they were.
func TestTextureIsASupplementalReaderOfContentItemAndAgentEvidenceOnly(t *testing.T) {
	admits := func(kind ObjectKind, reader Writer) bool {
		reg, err := writeRegistry.LookupKind(kind)
		if err != nil {
			t.Fatalf("%s: %v", kind, err)
		}
		return readerAdmits(reg, reader, true)
	}
	for _, kind := range []ObjectKind{"choir.content_item", "choir.agent_evidence"} {
		if !admits(kind, WriterTexture) {
			t.Errorf("Texture is not admitted to %s", kind)
		}
		for _, other := range []Writer{WriterManagement, WriterEngineering} {
			if admits(kind, other) {
				t.Errorf("%s is admitted to %s", other, kind)
			}
		}
	}
	// Not widened: the shell's and research's other kinds are still closed to
	// Texture.
	for _, kind := range []ObjectKind{"choir.media_item", "choir.podcast_subscription", "choir.run_memory_entry"} {
		reg, err := writeRegistry.LookupKind(kind)
		if err != nil {
			continue
		}
		if reg.Writer != WriterTexture && readerAdmits(reg, WriterTexture, true) && reg.Writer != WriterCorpusd {
			if reg.Writer == WriterShell || reg.Writer == WriterResearch {
				t.Errorf("Texture is admitted to %s (custodian %s)", kind, reg.Writer)
			}
		}
	}
}
