package store

// ProjectionBatchTrimBoundary is the highest sequence whose finalized
// projection_batch_recorded row the trim at head is defined to delete.
func ProjectionBatchTrimBoundary(head uint64) uint64 { return projectionBatchTrimBoundary(head) }

// EventIndexWitnessFilter (STUB) is the SQL predicate keeping the rows of
// computer_event_index the replay probe compares.
func EventIndexWitnessFilter(boundary uint64) string { return "" }
