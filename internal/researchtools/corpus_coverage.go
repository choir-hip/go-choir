package researchtools

// SearchCoverage states what one corpus search actually ran. Three review
// rounds of the research brief found one defect in three forms: a program
// inferred "the corpus holds nothing" from the absence of failure notes in
// prose, and every new way a search could be incomplete defeated it silently.
// The verb now says what it searched, and a program may claim emptiness only
// on that statement. Each field is set only on the path where that search
// returned from the store without error; the zero value means not searched, so
// a path nobody set (the window fallback, a future one) never reads as run.
type SearchCoverage struct {
	// PageText: corpusd's page-text search answered. PageTextIndexed is how
	// many of the computer's Pages it covers.
	PageText        bool  `json:"page_text"`
	PageTextIndexed int64 `json:"page_text_indexed"`
	Pages           int   `json:"pages"`
	// Labels: the title-and-label half ran over the capture index (not the
	// window of the newest captures, which covers only part of the pages).
	Labels bool `json:"labels"`
	// Window: the window of the newest captures was listed and matched by
	// title and label (the fallback when the index cannot answer); it covers
	// only part of the pages, so it is never Full on its own.
	Window bool `json:"window"`
	// Published: the published-story search answered over every publication
	// (a search that failed, or that stopped at its scan cap, is false).
	Published bool `json:"published"`
}

// textIndexComplete is the one comparison of how much of a computer's pages
// the page-text index covers: the verb's own "catching up" note and
// SearchCoverage.Full both use it.
func textIndexComplete(indexed int64, pages int) bool {
	return indexed >= int64(pages)
}

// PageTextComplete reports that the page-text search ran and covers every page
// the computer holds.
func (c SearchCoverage) PageTextComplete() bool {
	return c.PageText && textIndexComplete(c.PageTextIndexed, c.Pages)
}

// Full reports that every half ran over everything: page text over every
// page, titles and labels over the index, and the published stories. Only a
// full search that found nothing means nothing is held.
func (c SearchCoverage) Full() bool {
	return c.PageTextComplete() && c.Labels && c.Published
}

// coverage is the captures part of a search's coverage.
func (i corpusSearchInfo) coverage() SearchCoverage {
	return SearchCoverage{PageText: i.TextUsed, PageTextIndexed: i.TextIndexed, Pages: i.Captures, Labels: i.LabelsSearched, Window: i.WindowSearched}
}
