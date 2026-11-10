package deskprotocol

// Version is the desk protocol version, MAJOR.MINOR.PATCH. It names the lock
// the current source must equal: lock/<Version>.json. It moves when the
// extracted surface moves, by at least the bump the change classifies as.
const Version = "18.3.0"

// SurfaceSchema is the schema id of a lock file.
const SurfaceSchema = "choir.desk-protocol-surface/v1"
