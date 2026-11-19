package researchtools

// SearchCoverage states which parts of a corpus search actually ran. The
// research brief once inferred "the corpus holds nothing" from the absence of
// failure notes in prose, and four review rounds found it overstating in new
// ways each time. The brief now claims no absence at all; it states what was
// found and which parts were searched, and this object is where it reads the
// second. Each field is true only where that part ran and returned without
// error; the zero value, and any path nobody set, means not searched. It makes
// no claim that a part covered everything: a part that ran may have seen only
// some pages (the window of the newest captures; an index still catching up).
type SearchCoverage struct {
	// PageText: corpusd's page-text search answered. PageTextIndexed is the
	// number of pages its index holds, as corpusd reported it; it is compared
	// with nothing.
	PageText        bool  `json:"page_text"`
	PageTextIndexed int64 `json:"page_text_indexed"`
	// Titles: the title half ran over the capture index's term table, which
	// answered. Labels: Jev's labels were also read from the store (they are
	// read separately, and a failed read leaves Titles true and Labels false).
	Titles bool `json:"titles"`
	Labels bool `json:"labels"`
	// Window: the window of the newest captures was listed and matched by
	// title (the fallback when the index cannot answer).
	Window bool `json:"window"`
	// NoTitleWords: the query had no word the title half can match (stop
	// words alone), so Titles and Labels are false though the index answered.
	NoTitleWords bool `json:"no_title_words"`
	// Published: the published-story search ran and returned without error.
	// PublishedCapped: its scan stopped at its cap, so older pieces may be
	// missing (the pieces it did return are real).
	Published       bool `json:"published"`
	PublishedCapped bool `json:"published_capped"`
}

// coverage is the captures part of a search's coverage.
func (i corpusSearchInfo) coverage() SearchCoverage {
	return SearchCoverage{PageText: i.TextUsed, PageTextIndexed: i.TextIndexed, Titles: i.TitlesSearched,
		Labels: i.TitlesSearched && i.LabelsRead, Window: i.WindowSearched, NoTitleWords: i.NoTitleWords}
}
