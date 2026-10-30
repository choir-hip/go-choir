package frontimportance

// The deepen measure (docs/archive/reports/autopaper-deepen-important-stories-2026-10-14.md):
// each run logs how deep the paper's important stories are, from the
// importance this run writes and the depth corpusd serves on the front
// list. It only logs; nothing reads it back.

import "sort"

const (
	// depthImportantAt is the importance from which a story counts as
	// important for the measure.
	depthImportantAt = 7
	// depthDeepSources is how many cited sources make a story deep.
	depthDeepSources = 3
)

// depthStats summarizes the important stories with known depth.
type depthStats struct {
	// Important counts non-promo stories scored depthImportantAt or more
	// whose depth corpusd reported.
	Important int
	// Deep counts those citing depthDeepSources or more sources.
	Deep int
	// MedianSources is the median of their source counts (0 with none).
	MedianSources float64
	// Thin lists the important stories citing fewer than depthDeepSources
	// sources, in front-list order.
	Thin []string
}

func summarizeDepth(stories []frontStory, scores map[string]scoreRow) depthStats {
	var out depthStats
	var counts []int
	for _, story := range stories {
		row, ok := scores[story.PublicationID]
		if !ok || row.Promo || row.Importance < depthImportantAt || story.SourceCount == nil {
			continue
		}
		n := *story.SourceCount
		out.Important++
		counts = append(counts, n)
		if n >= depthDeepSources {
			out.Deep++
		} else {
			out.Thin = append(out.Thin, story.PublicationID)
		}
	}
	if len(counts) > 0 {
		sort.Ints(counts)
		mid := len(counts) / 2
		if len(counts)%2 == 1 {
			out.MedianSources = float64(counts[mid])
		} else {
			out.MedianSources = float64(counts[mid-1]+counts[mid]) / 2
		}
	}
	return out
}
