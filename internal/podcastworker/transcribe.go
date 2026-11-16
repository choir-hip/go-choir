package podcastworker

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const (
	feedFetchDeadline = 60 * time.Second
	userAgent         = "choir-podcast-worker/1"
	// maxDownload bounds one enclosure; a news episode is tens of MB.
	maxDownload = 600 << 20
	// wavBytesPerSecond is 16 kHz, mono, 16-bit.
	wavBytesPerSecond = 32000.0
)

// FetchFeed downloads and parses one feed.
func FetchFeed(ctx context.Context, client *http.Client, feedURL, show string) ([]Episode, error) {
	// One slow feed cannot hold the poll: each fetch has its own deadline.
	ctx, cancel := context.WithTimeout(ctx, feedFetchDeadline)
	defer cancel()
	u, err := url.Parse(feedURL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return nil, fmt.Errorf("feed %q is not an http(s) URL", feedURL)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, feedURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch feed %s: %w", feedURL, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch feed %s: status %d", feedURL, resp.StatusCode)
	}
	return ParseFeed(feedURL, resp.Body, show)
}

// DefaultMinFreeBytes is the free space the worker insists on before it starts
// a download: 15 GiB. The worker runs on a shared host; a manner, not a limit
// on model calls. The downloaded audio is deleted as soon as the episode's
// transcription ends, whatever its outcome.
const DefaultMinFreeBytes = 15 << 30

// DefaultWorkDirMaxBytes bounds what the worker's own episode directories may
// hold: one episode's download and WAV are tens to hundreds of MB, so 2 GiB is
// several episodes' worth, and only a crash can leave more than one behind.
const DefaultWorkDirMaxBytes = 2 << 30

// ErrLowDisk is the refusal to start a download for lack of free space.
var ErrLowDisk = errors.New("not enough free disk space to start a download")

// Transcriber turns an enclosure URL into timed segments with ffmpeg and
// parakeet.cpp's CLI.
type Transcriber struct {
	FFmpeg, Parakeet, Model, VAD string
	Threads                      int
	WorkDir                      string
	// MaxMinutes limits the audio transcribed from the start of an episode;
	// 0 means all of it.
	MaxMinutes int
	HTTP       *http.Client
	// MinFreeBytes is the free space the filesystem of WorkDir must have before
	// a download starts (0 means DefaultMinFreeBytes; a negative value turns the
	// check off for a test). FreeBytes reads it; a test replaces it.
	MinFreeBytes int64
	FreeBytes    func(dir string) (uint64, error)
	// WorkDirMaxBytes is the most the worker's own episode directories in
	// WorkDir may hold before a download is refused (0 means
	// DefaultWorkDirMaxBytes; negative turns the check off).
	WorkDirMaxBytes int64
}

func (t *Transcriber) workDir() string {
	if t.WorkDir != "" {
		return t.WorkDir
	}
	return os.TempDir()
}

// SweepWorkDir removes the episode directories a crashed or killed run left
// in the work directory. They are the worker's own scratch and nothing else.
func (t *Transcriber) SweepWorkDir() (removed int, err error) {
	entries, err := os.ReadDir(t.workDir())
	if err != nil {
		return 0, err
	}
	for _, e := range entries {
		if e.IsDir() && strings.HasPrefix(e.Name(), "episode-") {
			if rerr := os.RemoveAll(filepath.Join(t.workDir(), e.Name())); rerr == nil {
				removed++
			} else if err == nil {
				err = rerr
			}
		}
	}
	return removed, err
}

// ownedBytes is the size of the worker's own episode directories.
func (t *Transcriber) ownedBytes() int64 {
	entries, err := os.ReadDir(t.workDir())
	if err != nil {
		return 0
	}
	var total int64
	for _, e := range entries {
		if !e.IsDir() || !strings.HasPrefix(e.Name(), "episode-") {
			continue
		}
		_ = filepath.Walk(filepath.Join(t.workDir(), e.Name()), func(_ string, info os.FileInfo, err error) error {
			if err == nil && info.Mode().IsRegular() {
				total += info.Size()
			}
			return nil
		})
	}
	return total
}

// DescribeFreeSpace is a log line about the filesystem holding dir.
func DescribeFreeSpace(dir string) string {
	if dir == "" {
		dir = os.TempDir()
	}
	free, err := freeBytes(dir)
	if err != nil {
		return fmt.Sprintf("free space of %s unknown: %v", dir, err)
	}
	return fmt.Sprintf("free space of %s: %.1f GiB", dir, float64(free)/(1<<30))
}

// httpClient is the guarded client every outside fetch goes through, unless a
// test supplies one.
func (t *Transcriber) httpClient() *http.Client {
	if t.HTTP != nil {
		return t.HTTP
	}
	t.HTTP = SafeClient(30 * time.Minute)
	return t.HTTP
}

// checkDisk refuses when the work directory's filesystem is short of space.
func (t *Transcriber) checkDisk() error {
	dir := t.workDir()
	if limit := t.WorkDirMaxBytes; limit >= 0 {
		if limit == 0 {
			limit = DefaultWorkDirMaxBytes
		}
		if held := t.ownedBytes(); held >= limit {
			return fmt.Errorf("%w: the work directory %s holds %.1f GiB of its own episodes, the bound is %.1f GiB", ErrLowDisk, dir, float64(held)/(1<<30), float64(limit)/(1<<30))
		}
	}
	min := t.MinFreeBytes
	if min < 0 {
		return nil
	}
	if min == 0 {
		min = DefaultMinFreeBytes
	}
	free := t.FreeBytes
	if free == nil {
		free = freeBytes
	}
	got, err := free(dir)
	if err != nil {
		return fmt.Errorf("read the free space of %s: %w", dir, err)
	}
	if got < uint64(min) {
		return fmt.Errorf("%w: %.1f GiB free in %s, %.1f GiB needed", ErrLowDisk, float64(got)/(1<<30), dir, float64(min)/(1<<30))
	}
	return nil
}

// Engine names the engine and model for the transcript's record.
func (t *Transcriber) Engine() string {
	base := strings.TrimSuffix(filepath.Base(t.Model), filepath.Ext(t.Model))
	return "parakeet.cpp " + base
}

// Result is a transcription: the segments, the seconds of audio transcribed
// and the length in seconds of the whole file as downloaded (0 when unknown).
type Result struct {
	Segments    []Segment
	Seconds     float64
	FileSeconds float64
	// StableFile is whether the publisher served the same file (the same
	// byte length) when the download began and again when the transcription
	// ended, minutes later: nil when either probe failed. A publisher that
	// stitches ads in answers differently each time, and a clip of such a
	// file may not line up on the listener's copy.
	StableFile *bool
}

// Transcribe downloads the enclosure, converts it to 16 kHz mono and runs
// the model.
func (t *Transcriber) Transcribe(ctx context.Context, enclosureURL string) (Result, error) {
	if err := t.checkDisk(); err != nil {
		return Result{}, err
	}
	dir, err := os.MkdirTemp(t.WorkDir, "episode-")
	if err != nil {
		return Result{}, err
	}
	defer os.RemoveAll(dir)
	src := filepath.Join(dir, "in.audio")
	client := t.httpClient()
	before := ProbeLength(ctx, client, enclosureURL)
	if err := t.download(ctx, enclosureURL, src); err != nil {
		return Result{}, err
	}
	if head, err := readHead(src, 512); err == nil && looksLikePlaylist(head) {
		return Result{}, errors.New("the download is a playlist, not audio: refused")
	}
	// The length of the file as downloaded: an ad-stitched episode differs
	// from its feed's duration and from the next download, and a clip is valid
	// only for this file, so the phone checks its own length against this one.
	fileSeconds := probeSeconds(ctx, t.FFmpeg, src)
	wav := filepath.Join(dir, "in.wav")
	args := []string{"-nostdin", "-hide_banner", "-loglevel", "error", "-y", "-protocol_whitelist", "file", "-i", src}
	if t.MaxMinutes > 0 {
		args = append(args, "-t", fmt.Sprint(t.MaxMinutes*60))
	}
	args = append(args, "-ar", "16000", "-ac", "1", "-c:a", "pcm_s16le", wav)
	if out, err := exec.CommandContext(ctx, t.FFmpeg, args...).CombinedOutput(); err != nil {
		return Result{}, fmt.Errorf("ffmpeg: %v: %s", err, tail(out))
	}
	_ = os.Remove(src)
	info, err := os.Stat(wav)
	if err != nil {
		return Result{}, err
	}
	seconds := float64(info.Size()-44) / wavBytesPerSecond
	threads := t.Threads
	if threads <= 0 {
		threads = 4
	}
	cmd := exec.CommandContext(ctx, t.Parakeet, "transcribe", "--model", t.Model, "--input", wav,
		"--vad", "--vad-model", t.VAD, "--threads", fmt.Sprint(threads), "--timestamps", "--json")
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		return Result{}, fmt.Errorf("parakeet: %v: %s", err, tail(stderr.Bytes()))
	}
	parsed, err := ParseParakeet(stdout.Bytes())
	if err != nil {
		return Result{}, err
	}
	res := Result{Segments: Segments(parsed.Words), Seconds: seconds, FileSeconds: fileSeconds}
	if after := ProbeLength(ctx, client, enclosureURL); before > 0 && after > 0 {
		stable := before == after
		res.StableFile = &stable
	}
	return res, nil
}

func (t *Transcriber) download(ctx context.Context, rawURL, dst string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", userAgent)
	client := t.httpClient()
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("download: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download: status %d", resp.StatusCode)
	}
	f, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer f.Close()
	n, err := io.Copy(f, io.LimitReader(resp.Body, maxDownload+1))
	if err != nil {
		return fmt.Errorf("download: %w", err)
	}
	if n > maxDownload {
		return fmt.Errorf("download: over %d bytes", maxDownload)
	}
	return nil
}

// ProbeLength asks for one byte of the enclosure, following redirects, and
// returns the file's total length from Content-Range (or Content-Length when
// the server ignored the range). 0 means unknown.
func ProbeLength(ctx context.Context, client *http.Client, rawURL string) int64 {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return 0
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Range", "bytes=0-0")
	resp, err := client.Do(req)
	if err != nil {
		return 0
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<10))
	if resp.StatusCode == http.StatusPartialContent {
		if _, total, ok := strings.Cut(resp.Header.Get("Content-Range"), "/"); ok {
			if n, err := strconv.ParseInt(strings.TrimSpace(total), 10, 64); err == nil && n > 0 {
				return n
			}
		}
		return 0
	}
	if resp.StatusCode == http.StatusOK && resp.ContentLength > 0 {
		return resp.ContentLength
	}
	return 0
}

var durationRe = regexp.MustCompile(`Duration: (\d+):(\d+):(\d+(?:\.\d+)?)`)

// probeSeconds reads a file's duration from `ffmpeg -i`, which prints it on
// stderr and exits with an error for lack of an output. 0 means unknown.
func probeSeconds(ctx context.Context, ffmpeg, path string) float64 {
	out, _ := exec.CommandContext(ctx, ffmpeg, "-nostdin", "-hide_banner", "-protocol_whitelist", "file", "-i", path).CombinedOutput()
	m := durationRe.FindSubmatch(out)
	if m == nil {
		return 0
	}
	h, _ := strconv.ParseFloat(string(m[1]), 64)
	mi, _ := strconv.ParseFloat(string(m[2]), 64)
	sec, _ := strconv.ParseFloat(string(m[3]), 64)
	return h*3600 + mi*60 + sec
}

func readHead(path string, n int) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	buf := make([]byte, n)
	m, err := io.ReadFull(f, buf)
	if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) && !errors.Is(err, io.EOF) {
		return nil, err
	}
	return buf[:m], nil
}

func tail(b []byte) string {
	s := strings.TrimSpace(string(b))
	if len(s) > 400 {
		s = s[len(s)-400:]
	}
	return s
}
