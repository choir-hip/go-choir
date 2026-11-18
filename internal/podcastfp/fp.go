// Package podcastfp is the podcast clip fingerprint: a small audio signature
// that lets a phone find a clip again in a differently served copy of the
// same episode (hosts insert different ads per download, so seconds into the
// worker's file are not seconds into the phone's file).
//
// The definition is normative in docs/reference/PODCAST-CLIP-FINGERPRINT.md
// and the conformance fixture mobile/fixtures/podcast-clip-fingerprint-v1.json;
// a Swift implementation must produce the same bytes. No cgo, no dependency:
// decoding to 16 kHz mono s16le PCM is the caller's (ffmpeg's) job.
package podcastfp

import (
	"errors"
	"fmt"
	"math"
)

// The version-1 parameters. Changing any of them is a new version.
const (
	Version    = 1
	SampleRate = 16000
	FrameLen   = 1024 // samples per analysis frame
	Hop        = 1600 // samples between frame starts: 10 frames per second
	FPS        = 10   // frames per second (SampleRate / Hop)
	Bands      = 8
	// FloorPower is added to a band's mean power before the log.
	FloorPower = 1e-10
	// DBFloor is the dB value that quantises to 0; Scale maps dB to 8 bits.
	DBFloor = -100.0
	Scale   = 2.55
	// ClipFrames is the length of a head or tail fingerprint: 12 seconds.
	ClipFrames = 120
	// FoundPeak and FoundMargin are the thresholds of a found clip: a correct
	// peak measured at 0.80 or more and 0.19 or more above the second peak; a
	// wrong place at 0.51 or less.
	FoundPeak   = 0.80
	FoundMargin = 0.19
	// SecondExclusion is the half-width in frames around the peak inside which
	// a score is the same peak and not a second one.
	SecondExclusion = 15
)

// BandBins are the FFT bin edges of the 8 bands: band b sums bins
// [BandBins[b], BandBins[b+1]). Bin k is k*15.625 Hz; the edges are the
// round(f/15.625) of 100*38^(b/8) Hz (100 to 3800 Hz, log spaced).
var BandBins = [Bands + 1]int{6, 10, 16, 25, 39, 62, 98, 154, 243}

var (
	window  [FrameLen]float64
	twiddle [FrameLen / 2][2]float64
)

func init() {
	for n := range window {
		window[n] = 0.5 - 0.5*math.Cos(2*math.Pi*float64(n)/FrameLen) // periodic Hann
	}
	for k := range twiddle {
		a := -2 * math.Pi * float64(k) / FrameLen
		twiddle[k] = [2]float64{math.Cos(a), math.Sin(a)}
	}
}

// frameValues computes the unquantised dB value of each band of one frame of
// FrameLen samples.
func frameValues(s []int16) [Bands]float64 {
	var re, im [FrameLen]float64
	for n := 0; n < FrameLen; n++ {
		re[n] = float64(s[n]) / 32768.0 * window[n]
	}
	fft(&re, &im)
	var out [Bands]float64
	for b := 0; b < Bands; b++ {
		var sum float64
		for k := BandBins[b]; k < BandBins[b+1]; k++ {
			sum += re[k]*re[k] + im[k]*im[k]
		}
		mean := sum / float64(BandBins[b+1]-BandBins[b]) / (FrameLen * FrameLen)
		out[b] = 10 * math.Log10(mean+FloorPower)
	}
	return out
}

func quantise(db float64) uint8 {
	v := math.Floor((db-DBFloor)*Scale + 0.5)
	if v < 0 {
		return 0
	}
	if v > 255 {
		return 255
	}
	return uint8(v)
}

// fft is an in-place iterative radix-2 decimation-in-time FFT.
func fft(re, im *[FrameLen]float64) {
	n := FrameLen
	for i, j := 1, 0; i < n; i++ {
		bit := n >> 1
		for ; j&bit != 0; bit >>= 1 {
			j ^= bit
		}
		j ^= bit
		if i < j {
			re[i], re[j] = re[j], re[i]
			im[i], im[j] = im[j], im[i]
		}
	}
	for size := 2; size <= n; size <<= 1 {
		half, step := size/2, n/size
		for start := 0; start < n; start += size {
			for k := 0; k < half; k++ {
				wr, wi := twiddle[k*step][0], twiddle[k*step][1]
				a, b := start+k, start+k+half
				tr := re[b]*wr - im[b]*wi
				ti := re[b]*wi + im[b]*wr
				re[b], im[b] = re[a]-tr, im[a]-ti
				re[a], im[a] = re[a]+tr, im[a]+ti
			}
		}
	}
}

// Builder turns a stream of 16 kHz mono PCM into the frame track: frame k
// covers samples [k*Hop, k*Hop+FrameLen) and is emitted once complete; the
// track is Bands bytes a frame, frame-major. A tail shorter than a frame is
// dropped.
type Builder struct {
	n      int64
	cur    [FrameLen]int16
	frames []byte
}

// WriteSamples adds samples.
func (b *Builder) WriteSamples(s []int16) {
	for _, v := range s {
		off := int(b.n % Hop)
		if off < FrameLen {
			b.cur[off] = v
			if off == FrameLen-1 {
				db := frameValues(b.cur[:])
				for _, d := range db {
					b.frames = append(b.frames, quantise(d))
				}
			}
		}
		b.n++
	}
}

// Write adds little-endian s16 PCM bytes (an odd trailing byte is an error).
func (b *Builder) Write(p []byte) (int, error) {
	if len(p)%2 != 0 {
		return 0, errors.New("podcastfp: odd byte count of s16le PCM")
	}
	s := make([]int16, len(p)/2)
	for i := range s {
		s[i] = int16(uint16(p[2*i]) | uint16(p[2*i+1])<<8)
	}
	b.WriteSamples(s)
	return len(p), nil
}

// Track returns the frames so far (not copied).
func (b *Builder) Track() []byte { return b.frames }

// Frames fingerprints a whole PCM buffer.
func Frames(pcm []int16) []byte {
	var b Builder
	b.WriteSamples(pcm)
	return b.Track()
}

// Pack makes the byte layout of one fingerprint: version byte, band count
// byte, frame count (uint16, big endian), then the frames.
func Pack(frames []byte) ([]byte, error) {
	if len(frames) == 0 || len(frames)%Bands != 0 || len(frames)/Bands > 0xFFFF {
		return nil, fmt.Errorf("podcastfp: %d bytes is not a whole number of frames", len(frames))
	}
	n := len(frames) / Bands
	out := make([]byte, 0, 4+len(frames))
	out = append(out, Version, Bands, byte(n>>8), byte(n))
	return append(out, frames...), nil
}

// Unpack checks a packed fingerprint and returns its frames.
func Unpack(p []byte) ([]byte, error) {
	if len(p) < 4+Bands {
		return nil, errors.New("podcastfp: fingerprint too short")
	}
	if p[0] != Version || p[1] != Bands {
		return nil, fmt.Errorf("podcastfp: version %d with %d bands is not version %d with %d", p[0], p[1], Version, Bands)
	}
	n := int(p[2])<<8 | int(p[3])
	if n == 0 || len(p) != 4+n*Bands {
		return nil, fmt.Errorf("podcastfp: %d frames do not fit %d bytes", n, len(p))
	}
	return p[4:], nil
}

// Match is where a template was found in a window.
type Match struct {
	Pos    int     // frame of the window where the template starts
	Peak   float64 // normalised cross-correlation at Pos
	Second float64 // best score more than SecondExclusion frames from Pos
	Margin float64 // Peak - Second
}

// Found reports whether the match clears the thresholds.
func (m Match) Found() bool { return m.Peak >= FoundPeak && m.Margin >= FoundMargin }

// Seconds is the match position in seconds from the window's start.
func (m Match) Seconds() float64 { return float64(m.Pos) / FPS }

// Locate finds the template (frames, Bands bytes a frame) in the window
// (frames of the same layout, computed from PCM that starts at the window's
// own time zero) by normalised cross-correlation over all template values:
// the template's mean is removed, the window's mean and variance are taken
// per position, and gain differences vanish because the values are in dB.
// ok is false when the window is shorter than the template or empty.
func Locate(template, window []byte) (Match, bool) {
	n, m := len(template)/Bands, len(window)/Bands
	if n == 0 || m < n || len(template)%Bands != 0 || len(window)%Bands != 0 {
		return Match{}, false
	}
	var mt float64
	for _, v := range template {
		mt += float64(v)
	}
	mt /= float64(len(template))
	tz := make([]float64, len(template))
	var tt float64
	for i, v := range template {
		tz[i] = float64(v) - mt
		tt += tz[i] * tz[i]
	}
	// Prefix sums of the window's values and squares, per frame.
	P := make([]float64, m+1)
	Q := make([]float64, m+1)
	for x := 0; x < m; x++ {
		var s, s2 float64
		for b := 0; b < Bands; b++ {
			v := float64(window[x*Bands+b])
			s += v
			s2 += v * v
		}
		P[x+1], Q[x+1] = P[x]+s, Q[x]+s2
	}
	N := float64(n * Bands)
	scores := make([]float64, m-n+1)
	best := 0
	for p := range scores {
		sx, sxx := P[p+n]-P[p], Q[p+n]-Q[p]
		var dot float64
		w := window[p*Bands : (p+n)*Bands]
		for i, v := range w {
			dot += tz[i] * float64(v)
		}
		variance := sxx - sx*sx/N
		if variance > 1e-9 && tt > 1e-9 {
			scores[p] = dot / math.Sqrt(tt*variance)
		}
		if scores[p] > scores[best] {
			best = p
		}
	}
	var second float64
	for p, s := range scores {
		if p < best-SecondExclusion || p > best+SecondExclusion {
			second = math.Max(second, s)
		}
	}
	return Match{Pos: best, Peak: scores[best], Second: second, Margin: scores[best] - second}, true
}
