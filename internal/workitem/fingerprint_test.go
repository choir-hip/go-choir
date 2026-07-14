package workitem

import "testing"

func TestObjectiveFingerprintGoldenVectors(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name                                 string
		owner, trajectory, parent, objective string
		want                                 string
	}{
		{
			name:       "trim punctuation and case",
			owner:      " owner ",
			trajectory: " traj ",
			parent:     " parent ",
			objective:  "Hello, WORLD!! 42",
			want:       "e08f5e56fb25191351120c982385ead8068cab97c721ea1ff65386688d86803a",
		},
		{
			name:       "equivalent separators",
			owner:      "owner",
			trajectory: "traj",
			parent:     "parent",
			objective:  "  hello---world__42  ",
			want:       "e08f5e56fb25191351120c982385ead8068cab97c721ea1ff65386688d86803a",
		},
		{
			name:       "unicode letters and digits",
			owner:      "owner",
			trajectory: "traj",
			parent:     "parent",
			objective:  "CAFÉ—東京 １２３",
			want:       "5c7f9b8afae0d645a49c616c4283e06c4d3eb14227a1e33ebda93b6bfd9a1b76",
		},
		{
			name: "empty fields still have a stable hash",
			want: "709e80c88487a2411e1ee4dfb9f22a861492d20c4765150c0c794abd70f8147c",
		},
	} {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := ObjectiveFingerprint(tt.owner, tt.trajectory, tt.parent, tt.objective); got != tt.want {
				t.Fatalf("ObjectiveFingerprint() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestWireFingerprints(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name string
		got  string
		want string
	}{
		{name: "publication", got: PublicationFingerprint(" traj ", " rev "), want: "wire_publication:traj:rev"},
		{name: "publication missing trajectory", got: PublicationFingerprint(" ", "rev"), want: ""},
		{name: "publication missing revision", got: PublicationFingerprint("traj", " "), want: ""},
		{name: "story resolution", got: StoryResolutionFingerprint(" traj ", " doc "), want: "wire_story_resolution:traj:doc"},
		{name: "story missing trajectory", got: StoryResolutionFingerprint(" ", "doc"), want: ""},
		{name: "story missing document", got: StoryResolutionFingerprint("traj", " "), want: ""},
		{name: "processor decision", got: ProcessorDecisionFingerprint(" traj "), want: "wire_processor_request_resolution:traj"},
		{name: "processor decision missing trajectory", got: ProcessorDecisionFingerprint(" "), want: ""},
		{name: "source item decision", got: SourceItemDecisionFingerprint(" traj ", " source "), want: "wire_source_item_resolution:traj:source"},
		{name: "source item missing trajectory", got: SourceItemDecisionFingerprint(" ", "source"), want: ""},
		{name: "source item missing source", got: SourceItemDecisionFingerprint("traj", " "), want: ""},
	} {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if tt.got != tt.want {
				t.Fatalf("fingerprint = %q, want %q", tt.got, tt.want)
			}
		})
	}
}
