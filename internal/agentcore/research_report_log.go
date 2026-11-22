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
// captured, and whether the packet's Notes carry the "adds:" and "missing:" lines
// (what the packet adds, what the corpus lacked). Counts only: never a query or a
// claim. The proof script reads the final line of each run.

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
	adds, missing := false, false
	for _, n := range packet.Notes {
		switch l := strings.ToLower(strings.TrimSpace(n)); {
		case strings.HasPrefix(l, "adds:"):
			adds = true
		case strings.HasPrefix(l, "missing:"):
			missing = true
		}
	}
	return fmt.Sprintf("research report run=%s final=%t sources=%d from_corpus=%d by_run=%d no_capture=%d claims=%d adds=%t gap=%t",
		runID, final, len(packet.Sources), corpus, byRun, none, len(packet.Claims), adds, missing)
}

func logResearchReport(runID, deskAgentID, rawPacket string, final bool) {
	if line := researchReportLine(runID, deskAgentID, rawPacket, final); line != "" {
		log.Printf("runtime: %s", line)
	}
}
