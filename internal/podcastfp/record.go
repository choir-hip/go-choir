package podcastfp

import (
	"errors"
	"fmt"
	"math"
)

// FileInfo is what the worker knows of the audio file it fingerprinted.
type FileInfo struct {
	Bytes            int64   `json:"file_bytes,omitempty"`
	Seconds          float64 `json:"file_seconds,omitempty"`
	FirstFrameOffset int64   `json:"first_frame_offset,omitempty"` // bytes before the first MPEG frame (ID3v2 size)
	BitrateKbps      int     `json:"bitrate_kbps,omitempty"`
	// SeekableByBitrate: the file is MP3 at one constant bitrate from its first
	// frame to its end, so byte = first_frame_offset + seconds*bitrate_kbps*125.
	SeekableByBitrate bool `json:"seekable_by_bitrate"`
}

// ClipRecord is the fingerprint a clip carries on the platform. Every time is
// in seconds of the file the fingerprint was cut from (FileSeconds long).
type ClipRecord struct {
	Version int    `json:"version"`
	Head    []byte `json:"head"` // Pack of the first 12 s of the clip (base64 in JSON)
	Tail    []byte `json:"tail"` // Pack of the last 12 s
	// HeadAt and TailAt are where Head and Tail begin in that file, on the
	// 0.1 s frame grid. Start and End are the clip's bounds in that file.
	HeadAt float64 `json:"head_at"`
	TailAt float64 `json:"tail_at"`
	Start  float64 `json:"start"`
	End    float64 `json:"end"`
	FileInfo
}

// Cut makes a clip record from a frame track whose frame 0 begins at t0
// seconds of the file (t0 is a whole number of seconds, so frames fall on the
// 0.1 s grid). The head is the clip's first min(12 s, length) from
// round(start*10), the tail the last as many frames ending at round(end*10).
func Cut(track []byte, t0, start, end float64, file FileInfo) (*ClipRecord, error) {
	total := len(track) / Bands
	f0 := int(math.Round((start - t0) * FPS))
	f1 := int(math.Round((end - t0) * FPS))
	n := min(ClipFrames, f1-f0)
	if n < 10 || f0 < 0 || f1 > total {
		return nil, fmt.Errorf("podcastfp: clip %.1f-%.1f s is outside the %.1f s of track starting at %.1f s", start, end, float64(total)/FPS, t0)
	}
	head, err := Pack(track[f0*Bands : (f0+n)*Bands])
	if err != nil {
		return nil, err
	}
	tail, err := Pack(track[(f1-n)*Bands : f1*Bands])
	if err != nil {
		return nil, err
	}
	return &ClipRecord{
		Version: Version, Head: head, Tail: tail,
		HeadAt: t0 + float64(f0)/FPS, TailAt: t0 + float64(f1-n)/FPS,
		Start: start, End: end, FileInfo: file,
	}, nil
}

// MaxPackedBytes is the largest packed fingerprint a record may carry.
const MaxPackedBytes = 4 + ClipFrames*Bands

// Validate checks a record the way the platform does before storing it.
func (r *ClipRecord) Validate() error {
	if r.Version != Version {
		return fmt.Errorf("fingerprint version %d is not %d", r.Version, Version)
	}
	for name, p := range map[string][]byte{"head": r.Head, "tail": r.Tail} {
		if len(p) > MaxPackedBytes {
			return fmt.Errorf("fingerprint %s is over %d bytes", name, MaxPackedBytes)
		}
		if _, err := Unpack(p); err != nil {
			return fmt.Errorf("fingerprint %s: %w", name, err)
		}
	}
	for name, v := range map[string]float64{"head_at": r.HeadAt, "tail_at": r.TailAt, "start": r.Start, "end": r.End, "file_seconds": r.Seconds} {
		if math.IsNaN(v) || math.IsInf(v, 0) || v < 0 || v > 24*3600 {
			return fmt.Errorf("fingerprint %s is not a time in a day", name)
		}
	}
	if r.End <= r.Start || r.TailAt < r.HeadAt {
		return errors.New("fingerprint times are out of order")
	}
	if r.Bytes < 0 || r.FirstFrameOffset < 0 || r.BitrateKbps < 0 || r.BitrateKbps > 640 || r.FirstFrameOffset > r.Bytes && r.Bytes > 0 {
		return errors.New("fingerprint file facts are out of range")
	}
	if r.SeekableByBitrate && (r.BitrateKbps == 0 || r.Bytes == 0) {
		return errors.New("fingerprint says seekable by bitrate without a bitrate and a length")
	}
	return nil
}
