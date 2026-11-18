package types

import "time"

// BodyListQuery is a list of one computer's newest commitment records by what
// their body holds: every Match path (a JSON path such as
// "$.subject_refs.record_kind") equals its value, every Absent path is missing
// or empty, and, when Since is not zero, the row was last written at or after
// it. All three are applied in the query, before its limit, so the limit
// counts only rows that match.
type BodyListQuery struct {
	Match  map[string]string
	Absent []string
	Since  time.Time
}
