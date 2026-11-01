package store

// dropSourceCache drops the clean page cache of a store's source state file
// and its WAL after a job that read the whole file (the compaction copy).
// A var so a test can see the call. DONTNEED also drops hot pages the daemon
// will refault from disk; the owner chose more disk reads over held cache
// ("More disk reads, less memory usage"). Best effort, never fails the job.
var dropSourceCache = func(statePath string) {
	dropFileCache(statePath)
	dropFileCache(statePath + "-wal")
}
