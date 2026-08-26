package platform

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/yusefmosiah/go-choir/internal/filecas"
)

// GCFileChunks removes stale chunks not reachable from the latest root or a
// root recorded within grace. It only ever walks this computer's chunk prefix.
func (s *Service) GCFileChunks(ctx context.Context, computerID string, grace time.Duration) (removed int, err error) {
	if s == nil || s.store == nil || !safeFileCASComponent(computerID) || grace < 0 {
		return 0, fmt.Errorf("file cas: invalid GC request")
	}
	cutoff := time.Now().UTC().Add(-grace)
	roots, err := s.fileCASRootsForGC(ctx, computerID, cutoff)
	if err != nil {
		return 0, err
	}
	reachable := make(map[string]struct{})
	for _, root := range roots {
		manifestData, err := s.readBlob(root.ManifestRef)
		if err != nil {
			return 0, fmt.Errorf("file cas: read manifest %s: %w", root.Root, err)
		}
		manifest, err := filecas.ParseManifest(manifestData)
		if err != nil || manifest.ComputerID != computerID || manifest.Root != root.Root {
			return 0, fmt.Errorf("file cas: invalid manifest %s", root.Root)
		}
		for _, entry := range manifest.Files {
			for _, digest := range entry.Chunks {
				reachable[digest] = struct{}{}
			}
		}
	}
	prefix, err := s.artifactPath(filepath.Join("sha256", "file-cas-chunks", computerID))
	if err != nil {
		return 0, err
	}
	walkErr := filepath.WalkDir(prefix, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			if os.IsNotExist(walkErr) {
				return nil
			}
			return walkErr
		}
		if entry.IsDir() || !entry.Type().IsRegular() || !validFileCASDigest(entry.Name()) {
			return nil
		}
		if _, keep := reachable[entry.Name()]; keep {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if !info.ModTime().Before(cutoff) {
			return nil
		}
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return err
		}
		removed++
		return nil
	})
	if os.IsNotExist(walkErr) {
		return removed, nil
	}
	if walkErr != nil {
		return removed, fmt.Errorf("file cas: walk chunks: %w", walkErr)
	}
	return removed, nil
}

func (s *Service) fileCASRootsForGC(ctx context.Context, computerID string, cutoff time.Time) ([]FileRootRecord, error) {
	latest, err := s.store.LatestFileRoots(ctx, computerID, 1)
	if err != nil {
		return nil, err
	}
	rows, err := s.store.db.QueryContext(ctx, `SELECT computer_id,root,manifest_ref,head_sequence,created_at FROM computer_file_roots WHERE computer_id=? AND created_at>? ORDER BY created_at DESC, root DESC`, computerID, cutoff)
	if err != nil {
		return nil, fmt.Errorf("file cas: list recent roots: %w", err)
	}
	defer rows.Close()
	byRoot := make(map[string]FileRootRecord, len(latest))
	for _, root := range latest {
		byRoot[root.Root] = root
	}
	for rows.Next() {
		var root FileRootRecord
		if err := rows.Scan(&root.ComputerID, &root.Root, &root.ManifestRef, &root.HeadSequence, &root.CreatedAt); err != nil {
			return nil, err
		}
		byRoot[root.Root] = root
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	out := make([]FileRootRecord, 0, len(byRoot))
	for _, root := range byRoot {
		out = append(out, root)
	}
	return out, nil
}
