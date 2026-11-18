package podcastworker

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"time"

	"github.com/yusefmosiah/go-choir/internal/podcastfp"
)

// A TrackStore keeps, per transcribed episode, the fingerprint frame track of
// the audio the transcript was made from (about 0.5 MB for 90 minutes) and the
// file's facts. The matcher writes clips later, when the audio is long gone;
// the track is what lets it fingerprint a clip of that very file, with no
// second download. Tracks are the worker's own scratch: they are pruned by age.
type TrackStore struct {
	Dir string
}

// Track is a stored episode track.
type Track struct {
	Frames []byte
	File   podcastfp.FileInfo
}

var trackIDRe = regexp.MustCompile(`^[A-Za-z0-9_-]{1,80}$`)

// DefaultTrackDir is beside the match state, like it, so it outlives a
// hand-started process.
func DefaultTrackDir(workDir string) string {
	return filepath.Join(filepath.Dir(DefaultMatchStatePath(workDir)), "fp-tracks")
}

func (s *TrackStore) path(episodeID, ext string) (string, error) {
	if s == nil || s.Dir == "" || !trackIDRe.MatchString(episodeID) {
		return "", fmt.Errorf("track store: no directory, or %q is not an episode id", episodeID)
	}
	return filepath.Join(s.Dir, episodeID+ext), nil
}

// Save stores a result's track; a result with none stores nothing.
func (s *TrackStore) Save(episodeID string, r Result) error {
	if len(r.Track) == 0 {
		return nil
	}
	fp, err := s.path(episodeID, ".fp")
	if err != nil {
		return err
	}
	meta, _ := s.path(episodeID, ".json")
	if err := os.MkdirAll(s.Dir, 0o700); err != nil {
		return err
	}
	raw, err := json.Marshal(r.File)
	if err != nil {
		return err
	}
	// The frames last: a track with its facts is whole.
	if err := writeAtomic(meta, raw); err != nil {
		return err
	}
	return writeAtomic(fp, r.Track)
}

func writeAtomic(path string, data []byte) error {
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return err
	}
	return nil
}

// Load reads an episode's track; os.ErrNotExist when there is none.
func (s *TrackStore) Load(episodeID string) (*Track, error) {
	fp, err := s.path(episodeID, ".fp")
	if err != nil {
		return nil, err
	}
	meta, _ := s.path(episodeID, ".json")
	frames, err := os.ReadFile(fp)
	if err != nil {
		return nil, err
	}
	if len(frames) == 0 || len(frames)%podcastfp.Bands != 0 {
		return nil, errors.New("track store: a track that is not whole frames")
	}
	var file podcastfp.FileInfo
	if raw, err := os.ReadFile(meta); err == nil {
		_ = json.Unmarshal(raw, &file)
	}
	return &Track{Frames: frames, File: file}, nil
}

// Prune removes tracks older than maxAge.
func (s *TrackStore) Prune(maxAge time.Duration) (removed int) {
	if s == nil || s.Dir == "" {
		return 0
	}
	entries, _ := os.ReadDir(s.Dir)
	for _, e := range entries {
		if info, err := e.Info(); err == nil && !e.IsDir() && time.Since(info.ModTime()) > maxAge {
			if os.Remove(filepath.Join(s.Dir, e.Name())) == nil {
				removed++
			}
		}
	}
	return removed
}

// FingerprintClip cuts a clip's fingerprint record from an episode track.
func (t *Track) FingerprintClip(start, end float64) (*podcastfp.ClipRecord, error) {
	return podcastfp.Cut(t.Frames, 0, start, end, t.File)
}
