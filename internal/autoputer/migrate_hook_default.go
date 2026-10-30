//go:build !cutover_migrate

package autoputer

import "context"

// migrateLegacyStoreIfNeeded is a no-op until WP-F's guest migrator package
// (internal/migrate/guestdolt) is on the integration branch; migrate_hook.go
// (tag cutover_migrate) holds the real hook. FP0 deletes this file and the tag
// at merge.
func migrateLegacyStoreIfNeeded(ctx context.Context, storePath string) error { return nil }
