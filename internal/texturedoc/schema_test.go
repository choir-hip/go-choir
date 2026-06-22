package texturedoc

import (
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/sourcecontract"
)

func TestValidDocProjectsNumberedRefsAndEmbeds(t *testing.T) {
	doc := validDoc()
	entities := validEntities()

	projection, err := Project(doc, entities)
	if err != nil {
		t.Fatalf("Project() error = %v", err)
	}
	if !strings.Contains(projection.Text, "The claim is grounded[1].") {
		t.Fatalf("projection text missing numbered ref: %q", projection.Text)
	}
	if !strings.Contains(projection.Text, "[source 2: Launch photo | image_preview]") {
		t.Fatalf("projection text missing source embed: %q", projection.Text)
	}
	if len(projection.SourceRefs) != 1 || projection.SourceRefs[0].SourceEntityID != "src-web" || projection.SourceRefs[0].Number != 1 {
		t.Fatalf("source refs = %#v", projection.SourceRefs)
	}
	if len(projection.SourceEmbeds) != 1 || projection.SourceEmbeds[0].SourceEntityID != "src-image" || projection.SourceEmbeds[0].TargetKind != "image" {
		t.Fatalf("source embeds = %#v", projection.SourceEmbeds)
	}
}

func TestValidatorRejectsLegacySourceSyntaxesInText(t *testing.T) {
	cases := []string{
		"raw {{source:abc}} token",
		"[Story](source:abc)",
		"[source:abc]",
		"Source: https://example.com",
		"Unresolved citation [1]",
	}

	for _, text := range cases {
		t.Run(text, func(t *testing.T) {
			doc := validDoc()
			doc.Doc.Content[0].Content = []Node{{Type: "text", Text: text}}
			err := Validate(doc, validEntities())
			if err == nil {
				t.Fatalf("Validate() succeeded for legacy syntax %q", text)
			}
		})
	}
}

func TestValidatorRejectsUnsupportedLinkMark(t *testing.T) {
	doc := validDoc()
	doc.Doc.Content[0].Content = []Node{{
		Type: "text",
		Text: "source link",
		Marks: []Mark{{
			Type: "link",
			Attrs: map[string]any{
				"href": "source:abc",
			},
		}},
	}}

	err := Validate(doc, validEntities())
	if err == nil || !strings.Contains(err.Error(), "unsupported mark type") {
		t.Fatalf("Validate() error = %v, want unsupported mark", err)
	}
}

func TestSourceRefsAndEmbedsMustResolveEntities(t *testing.T) {
	doc := validDoc()
	doc.Doc.Content[0].Content[1].Attrs["source_entity_id"] = "missing"

	err := Validate(doc, validEntities())
	if err == nil || !strings.Contains(err.Error(), "does not resolve") {
		t.Fatalf("Validate() error = %v, want unresolved source ref", err)
	}

	doc = validDoc()
	doc.Doc.Content[1].Attrs["source_entity_id"] = "missing"
	err = Validate(doc, validEntities())
	if err == nil || !strings.Contains(err.Error(), "does not resolve") {
		t.Fatalf("Validate() error = %v, want unresolved source embed", err)
	}

	doc = validDoc()
	entities := append(validEntities(), sourceEntity("src-detached", "web_url", "Detached", "numbered_ref", sourcecontract.OpenSurfaceSource))
	err = Validate(doc, entities)
	if err == nil || !strings.Contains(err.Error(), "not referenced") {
		t.Fatalf("Validate() error = %v, want detached source entity rejection", err)
	}
}

func TestSourceEmbedMustBeLeafBlock(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*Node)
	}{
		{
			name: "content",
			mutate: func(node *Node) {
				node.Content = []Node{{Type: "text", Text: "{{source:hidden}}"}}
			},
		},
		{
			name: "text",
			mutate: func(node *Node) {
				node.Text = "{{source:hidden}}"
			},
		},
		{
			name: "marks",
			mutate: func(node *Node) {
				node.Marks = []Mark{{Type: "strong"}}
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			doc := validDoc()
			embed := &doc.Doc.Content[1]
			tc.mutate(embed)
			err := Validate(doc, validEntities())
			if err == nil || !strings.Contains(err.Error(), "source_embed must be a leaf block") {
				t.Fatalf("Validate() error = %v, want source_embed leaf rejection", err)
			}
		})
	}
}

func TestSourceEntityEnumsAreValidated(t *testing.T) {
	tests := []struct {
		name   string
		mutate func([]SourceEntity)
		want   string
	}{
		{
			name: "target kind",
			mutate: func(entities []SourceEntity) {
				entities[0].Target.Kind = "youtube_token"
			},
			want: "target.kind",
		},
		{
			name: "selector kind",
			mutate: func(entities []SourceEntity) {
				entities[0].Selectors[0].Kind = "xpath"
			},
			want: "selectors[0].kind",
		},
		{
			name: "display mode",
			mutate: func(entities []SourceEntity) {
				entities[0].Display.Mode = "youtube_embed"
			},
			want: "display.mode",
		},
		{
			name: "evidence state",
			mutate: func(entities []SourceEntity) {
				entities[0].Evidence.State = "maybe"
			},
			want: "evidence.state",
		},
		{
			name: "open surface",
			mutate: func(entities []SourceEntity) {
				entities[0].Evidence.OpenSurface = "new_window"
			},
			want: "evidence.open_surface",
		},
		{
			name: "reader artifact state",
			mutate: func(entities []SourceEntity) {
				entities[0].Evidence.ReaderArtifactState = "half_ready"
			},
			want: "reader_artifact_state",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entities := validEntities()
			tt.mutate(entities)
			err := Validate(validDoc(), entities)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("Validate() error = %v, want %q", err, tt.want)
			}
		})
	}
}

func TestImageRegionSelectorIsAccepted(t *testing.T) {
	doc := validDoc()
	entities := validEntities()
	entities[1].Selectors = []SourceSelector{{
		Kind: sourcecontract.SelectorKindImageRegion,
		Data: map[string]any{
			"x":      10,
			"y":      20,
			"width":  300,
			"height": 200,
		},
	}}

	if err := Validate(doc, entities); err != nil {
		t.Fatalf("Validate() rejected image_region selector: %v", err)
	}
}

func TestMultimediaTargetsUseSourceEntitiesNotBodySyntaxes(t *testing.T) {
	entities := validEntities()
	entities = append(entities,
		sourceEntity("src-video", "video", "Demo video", "player", sourcecontract.OpenSurfaceVideo),
		sourceEntity("src-audio", "audio", "Podcast clip", "player", sourcecontract.OpenSurfaceAudio),
		sourceEntity("src-pdf", "pdf", "Report PDF", "pdf_pages", sourcecontract.OpenSurfacePDF),
		sourceEntity("src-transcript", "transcript", "Transcript", "transcript", sourcecontract.OpenSurfaceTranscript),
		sourceEntity("src-file", "file_artifact", "Attachment", "source_window", sourcecontract.OpenSurfaceFile),
	)
	doc := validDoc()
	doc.Doc.Content = append(doc.Doc.Content,
		sourceEmbed("embed-video", "src-video", "player"),
		sourceEmbed("embed-audio", "src-audio", "player"),
		sourceEmbed("embed-pdf", "src-pdf", "pdf_pages"),
		sourceEmbed("embed-transcript", "src-transcript", "transcript"),
		sourceEmbed("embed-file", "src-file", "source_window"),
	)

	projection, err := Project(doc, entities)
	if err != nil {
		t.Fatalf("Project() error = %v", err)
	}
	if len(projection.SourceEmbeds) != 6 {
		t.Fatalf("source embeds = %#v", projection.SourceEmbeds)
	}
	for _, embed := range projection.SourceEmbeds {
		switch embed.TargetKind {
		case "image", "video", "audio", "pdf", "transcript", "file_artifact":
		default:
			t.Fatalf("unexpected multimedia embed target kind %q in %#v", embed.TargetKind, projection.SourceEmbeds)
		}
	}

	doc = validDoc()
	doc.Doc.Content = append(doc.Doc.Content, Node{
		Type:  "media_embed",
		Attrs: map[string]any{"id": "media-1", "source_entity_id": "src-video"},
	})
	err = Validate(doc, entities)
	if err == nil || !strings.Contains(err.Error(), "unsupported block node type") {
		t.Fatalf("Validate() error = %v, want media_embed rejection", err)
	}
}

func TestExecutionEvidenceTargetsUseSourceEntities(t *testing.T) {
	entities := validEntities()
	executionTargets := []struct {
		id          string
		targetKind  string
		title       string
		openSurface string
	}{
		{"src-command", "command_output", "go test output", sourcecontract.OpenSurfaceSourceWindow},
		{"src-shell", "shell_session", "terminal session", sourcecontract.OpenSurfaceSourceWindow},
		{"src-diff", "diff_hunk", "runtime diff", sourcecontract.OpenSurfaceSourceWindow},
		{"src-patch", "patch", "candidate patch", sourcecontract.OpenSurfaceFile},
		{"src-test", "test_run", "focused test run", sourcecontract.OpenSurfaceSourceWindow},
		{"src-package", "app_change_package", "change package", sourcecontract.OpenSurfaceSourceWindow},
		{"src-screenshot", "screenshot", "verification screenshot", sourcecontract.OpenSurfaceImage},
		{"src-video-artifact", "video_artifact", "verification video", sourcecontract.OpenSurfaceVideo},
		{"src-benchmark", "benchmark_log", "benchmark log", sourcecontract.OpenSurfaceFile},
	}
	doc := validDoc()
	for _, target := range executionTargets {
		entities = append(entities, sourceEntity(target.id, target.targetKind, target.title, "source_window", target.openSurface))
		doc.Doc.Content = append(doc.Doc.Content, sourceEmbed("embed-"+target.id, target.id, "source_window"))
	}
	if err := Validate(doc, entities); err != nil {
		t.Fatalf("Validate() rejected execution source targets: %v", err)
	}
	projection, err := Project(doc, entities)
	if err != nil {
		t.Fatalf("Project() error = %v", err)
	}
	for _, target := range executionTargets {
		found := false
		for _, embed := range projection.SourceEmbeds {
			if embed.SourceEntityID == target.id && embed.TargetKind == target.targetKind {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("projection missing execution source target %#v in %#v", target, projection.SourceEmbeds)
		}
	}
}

func validDoc() StructuredTextureDoc {
	return StructuredTextureDoc{
		Schema: SchemaV1,
		Doc: Node{
			Type:  "doc",
			Attrs: map[string]any{"id": "doc-1"},
			Content: []Node{
				{
					Type:  "paragraph",
					Attrs: map[string]any{"id": "p-1"},
					Content: []Node{
						{Type: "text", Text: "The claim is grounded"},
						{
							Type: "source_ref",
							Attrs: map[string]any{
								"id":               "ref-1",
								"source_entity_id": "src-web",
								"display_mode":     "numbered_ref",
							},
						},
						{Type: "text", Text: "."},
					},
				},
				sourceEmbed("embed-1", "src-image", "image_preview"),
			},
		},
	}
}

func validEntities() []SourceEntity {
	return []SourceEntity{
		{
			SourceEntityID: "src-web",
			Target: SourceTarget{
				Kind: "web_url",
				URI:  "https://example.com/story",
			},
			Selectors: []SourceSelector{{
				Kind: sourcecontract.SelectorKindTextQuote,
				Data: map[string]any{"exact": "grounded"},
			}},
			Display: SourceDisplay{
				Mode:  "numbered_ref",
				Title: "Example story",
			},
			Evidence: SourceEvidence{
				State:               sourcecontract.EvidenceStateConfirms,
				OpenSurface:         sourcecontract.OpenSurfaceSource,
				ReaderArtifactState: sourcecontract.ReaderArtifactStateReady,
			},
			Provenance: SourceEntityProvenance{
				CreatedBy:    "runtime",
				SourceSystem: "test",
			},
		},
		sourceEntity("src-image", "image", "Launch photo", "image_preview", sourcecontract.OpenSurfaceImage),
	}
}

func sourceEntity(id, targetKind, title, displayMode, openSurface string) SourceEntity {
	return SourceEntity{
		SourceEntityID: id,
		Target: SourceTarget{
			Kind: targetKind,
			ID:   id + "-target",
		},
		Selectors: []SourceSelector{{
			Kind: sourcecontract.SelectorKindWholeResource,
		}},
		Display: SourceDisplay{
			Mode:  displayMode,
			Title: title,
		},
		Evidence: SourceEvidence{
			State:       sourcecontract.EvidenceStateAvailable,
			OpenSurface: openSurface,
		},
		Provenance: SourceEntityProvenance{
			CreatedBy:    "runtime",
			SourceSystem: "test",
		},
	}
}

func sourceEmbed(nodeID, sourceEntityID, displayMode string) Node {
	return Node{
		Type: "source_embed",
		Attrs: map[string]any{
			"id":               nodeID,
			"source_entity_id": sourceEntityID,
			"display_mode":     displayMode,
		},
	}
}
