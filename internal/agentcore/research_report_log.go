package agentcore

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/deskmanifest"
	"github.com/yusefmosiah/go-choir/internal/researchtools"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// What a research run leaves behind in the corpus, as a count (owner, 2026-11-22: "the
// goal of any run shouldnt just be to write the best story, but to be building on and
// using our corpus, and then the story adds to our corpus"). One console line per
// research Report packet: how many of the sources it cites were in the corpus before
// the run (a capture the run did not make with a Read of its own) and how many the run
// captured, and whether the packet's Notes carry an "adds:" line and a "missing:" line that says
// something (what the packet adds, what the corpus lacked); gap is true only for the
// latter, and gap_text is its words, clipped. Counts, and the words of the "missing:" line (the run's own words about the corpus,
// clipped to 160 runes and quoted): never a query or a claim of the packet. The proof script reads the final line of each run.

// researchReportLine is the console line for one research Report packet, "" when the
// packet does not decode or is not a research desk's.
func researchReportLine(runID, deskAgentID, rawPacket string, final bool) string {
	if !deskmanifest.IsResearchInbox(strings.TrimSpace(deskAgentID)) || strings.TrimSpace(rawPacket) == "" {
		return ""
	}
	var packet types.SourcePacket
	if json.Unmarshal([]byte(rawPacket), &packet) != nil {
		return ""
	}
	corpus, byRun, none := 0, 0, 0
	for _, src := range packet.Sources {
		switch id := strings.TrimSpace(src.Target.CaptureID); {
		case id == "":
			none++
		case researchtools.CaptureMadeByRun(runID, id):
			byRun++
		default:
			corpus++
		}
	}
	adds, gap := false, ""
	// A note may hold several lines ("adds: ...\nmissing: ..."): each is read alone.
	for _, note := range packet.Notes {
		for _, n := range strings.Split(note, "\n") {
			switch l := strings.ToLower(strings.TrimSpace(n)); {
			case strings.HasPrefix(l, "adds:"):
				adds = true
			case strings.HasPrefix(l, "missing:") && gap == "":
				gap = gapText(strings.TrimSpace(n)[len("missing:"):])
			}
		}
	}
	return fmt.Sprintf("research report run=%s final=%t sources=%d from_corpus=%d by_run=%d no_capture=%d claims=%d adds=%t gap=%t gap_text=%q",
		runID, final, len(packet.Sources), corpus, byRun, none, len(packet.Claims), adds, gap != "", gap)
}

// gapTextRunes clips the words a Report gives for what the corpus lacked, so a log line
// stays one short line whatever the run wrote.
const gapTextRunes = 160

// gapPlaceholders are what a "missing:" line says when it names no gap: nothing, "none",
// and the literal the worked cell wrote until 2026-11-22 ("missing: what the graph
// lacked"), which made 55 of 56 Reports count as naming a gap.
var gapPlaceholders = map[string]bool{"": true, "none": true, "nothing": true, "n/a": true, "what the graph lacked": true}

// gapText is the words after "missing:", on one line and clipped, or "" when they name no
// gap (empty, "none", or the old placeholder). They are the run's own words about the
// corpus, never article text.
func gapText(after string) string {
	text := strings.Join(strings.Fields(after), " ")
	if gapPlaceholders[strings.ToLower(strings.TrimRight(text, " .;,"))] {
		return ""
	}
	if r := []rune(text); len(r) > gapTextRunes {
		text = strings.TrimSpace(string(r[:gapTextRunes])) + "…"
	}
	return text
}

func logResearchReport(runID, deskAgentID, rawPacket string, final bool) {
	if line := researchReportLine(runID, deskAgentID, rawPacket, final); line != "" {
		log.Printf("runtime: %s", line)
	}
}
