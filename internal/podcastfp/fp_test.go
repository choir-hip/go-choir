package podcastfp

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"flag"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"testing"
)

var updateFixture = flag.Bool("update-fixture", false, "rewrite mobile/fixtures/podcast-clip-fingerprint-v1.json")

const fixturePath = "../../mobile/fixtures/podcast-clip-fingerprint-v1.json"

type fixtureFile struct {
	Description string `json:"description"`
	Version     int    `json:"version"`
	SampleRate  int    `json:"sample_rate"`
	FrameLen    int    `json:"frame_len"`
	Hop         int    `json:"hop"`
	BandBins    []int  `json:"band_bins"`
	Samples     int    `json:"samples"`
	FirstPCM    []int  `json:"first_samples"`
	FramesHex   string `json:"frames_hex"`
	// Pack of frames [Locate.From, Locate.To), and where Locate must find it.
	Locate struct {
		From    int     `json:"from"`
		To      int     `json:"to"`
		PackHex string  `json:"pack_hex"`
		Pos     int     `json:"pos"`
		MinPeak float64 `json:"min_peak"`
	} `json:"locate"`
}

func buildFixture() fixtureFile {
	var f fixtureFile
	f.Description = "Podcast clip fingerprint v1 conformance (docs/reference/PODCAST-CLIP-FINGERPRINT.md). Generate `samples` s16 samples by the documented integer formula, build the frames, compare frames_hex byte for byte; pack frames [from,to) and compare pack_hex; locate that pack in the whole frame track and expect pos and a peak of at least min_peak."
	f.Version, f.SampleRate, f.FrameLen, f.Hop = Version, SampleRate, FrameLen, Hop
	f.BandBins = BandBins[:]
	f.Samples = 48000
	pcm := FixturePCM(f.Samples)
	for _, v := range pcm[:16] {
		f.FirstPCM = append(f.FirstPCM, int(v))
	}
	frames := Frames(pcm)
	f.FramesHex = hex.EncodeToString(frames)
	f.Locate.From, f.Locate.To = 6, 18
	p, _ := Pack(frames[6*Bands : 18*Bands])
	f.Locate.PackHex = hex.EncodeToString(p)
	f.Locate.Pos, f.Locate.MinPeak = 6, 0.99
	return f
}

func TestFixtureIsReproducedExactly(t *testing.T) {
	want := buildFixture()
	if d := minBoundaryDistance(FixturePCM(want.Samples)); d < 1e-3 {
		t.Fatalf("a fixture value lies %.2g of a step from a rounding boundary: another implementation could round it the other way; change the fixture", d)
	}
	if *updateFixture {
		raw, _ := json.MarshalIndent(want, "", " ")
		if err := os.WriteFile(filepath.FromSlash(fixturePath), append(raw, '\n'), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	raw, err := os.ReadFile(filepath.FromSlash(fixturePath))
	if err != nil {
		t.Fatal(err)
	}
	var got fixtureFile
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	if got.FramesHex != want.FramesHex || got.Locate.PackHex != want.Locate.PackHex || got.Samples != want.Samples {
		t.Fatalf("the fixture file differs from what the code makes; rerun with -update-fixture if the change is meant (and bump the version)")
	}
	frames, _ := hex.DecodeString(got.FramesHex)
	pk, _ := hex.DecodeString(got.Locate.PackHex)
	tpl, err := Unpack(pk)
	if err != nil {
		t.Fatal(err)
	}
	m, ok := Locate(tpl, frames)
	if !ok || m.Pos != got.Locate.Pos || m.Peak < got.Locate.MinPeak {
		t.Fatalf("locate on the fixture: %+v ok=%v", m, ok)
	}
}

// signal makes speech-like test audio: every 0.1 s each of the 8 bands gets
// its own random level, carried by a sinusoid at the band's centre, plus a
// little noise.
func signal(seed int64, seconds int) []int16 {
	r := rand.New(rand.NewSource(seed))
	out := make([]int16, seconds*SampleRate)
	var phase [Bands]float64
	var amp [Bands]float64
	for i := range out {
		if i%1600 == 0 {
			for b := range amp {
				amp[b] = math.Pow(r.Float64(), 2) * 2500
			}
		}
		var v float64
		for b := 0; b < Bands; b++ {
			centre := (float64(BandBins[b]) + float64(BandBins[b+1])) / 2 * 15.625
			phase[b] += 2 * math.Pi * centre / SampleRate
			v += amp[b] * math.Sin(phase[b])
		}
		v += r.NormFloat64() * 30
		out[i] = int16(math.Max(-32768, math.Min(32767, v)))
	}
	return out
}

func degrade(pcm []int16, gain float64, noise float64, seed int64) []int16 {
	r := rand.New(rand.NewSource(seed))
	out := make([]int16, len(pcm))
	for i, v := range pcm {
		out[i] = int16(math.Max(-32768, math.Min(32767, float64(v)*gain+r.NormFloat64()*noise)))
	}
	return out
}

func TestLocateFindsAShiftedNoisyQuieterClip(t *testing.T) {
	src := signal(1, 120)
	start, end := 40.0, 70.0
	track := Frames(src)
	rec, err := Cut(track, 0, start, end, FileInfo{})
	if err != nil {
		t.Fatal(err)
	}
	// The listener's file: a 7.3 s ad before the clip, half the level, noise.
	ad := signal(99, 8)[:int(7.3*SampleRate)]
	served := append(append([]int16{}, ad...), src...)
	served = degrade(served, 0.5, 40, 7)
	win := Frames(served[int(20*SampleRate):]) // the phone fetched from 20 s on
	head, _ := Unpack(rec.Head)
	m, ok := Locate(head, win)
	if !ok || !m.Found() {
		t.Fatalf("head not found: %+v", m)
	}
	got := 20 + m.Seconds() - rec.HeadAt // offset of the served file against the stored one
	if math.Abs(got-7.3) > 0.5 {
		t.Fatalf("offset %.2f s, want 7.3", got)
	}
}

func TestLocateRefusesAClipThatIsNotThere(t *testing.T) {
	rec, _ := Cut(Frames(signal(1, 120)), 0, 40, 70, FileInfo{})
	head, _ := Unpack(rec.Head)
	m, ok := Locate(head, Frames(signal(2, 100)))
	if !ok || m.Found() || m.Peak >= FoundPeak {
		t.Fatalf("a clip absent from the window was found: %+v", m)
	}
}

func TestAnAdInsideTheClipPutsHeadAndTailAtDifferentOffsets(t *testing.T) {
	src := signal(3, 120)
	rec, _ := Cut(Frames(src), 0, 40, 70, FileInfo{})
	cut := 55 * SampleRate
	served := append(append(append([]int16{}, src[:cut]...), signal(98, 10)[:5*SampleRate]...), src[cut:]...)
	win := Frames(served)
	head, _ := Unpack(rec.Head)
	tail, _ := Unpack(rec.Tail)
	mh, _ := Locate(head, win)
	mt, _ := Locate(tail, win)
	if !mh.Found() || !mt.Found() {
		t.Fatalf("heads %+v tail %+v", mh, mt)
	}
	oh, ot := mh.Seconds()-rec.HeadAt, mt.Seconds()-rec.TailAt
	if math.Abs(oh) > 0.2 || math.Abs(ot-5) > 0.2 {
		t.Fatalf("offsets head %.2f tail %.2f, want 0 and 5", oh, ot)
	}
}

func TestCutRefusesAClipUnder24SecondsAndOneOutsideTheTrack(t *testing.T) {
	track := Frames(signal(4, 60))
	for _, c := range [][2]float64{{10, 18}, {10, 33.9}} {
		if _, err := Cut(track, 0, c[0], c[1], FileInfo{}); err == nil {
			t.Fatalf("a %.1f s clip was cut", c[1]-c[0])
		}
	}
	rec, err := Cut(track, 0, 10, 34, FileInfo{})
	if err != nil {
		t.Fatal(err)
	}
	h, _ := Unpack(rec.Head)
	tl, _ := Unpack(rec.Tail)
	if len(h)/Bands != 120 || len(tl)/Bands != 120 || rec.HeadAt != 10 || rec.TailAt != 22 {
		t.Fatalf("a 24 s clip: head %d frames, tail %d, head_at %v tail_at %v", len(h)/Bands, len(tl)/Bands, rec.HeadAt, rec.TailAt)
	}
	if err := rec.Validate(); err != nil {
		t.Fatal(err)
	}
	if _, err := Cut(track, 0, 50, 75, FileInfo{}); err == nil {
		t.Fatal("a clip past the track was cut")
	}
}

func TestValidateRefusesBadRecords(t *testing.T) {
	rec, _ := Cut(Frames(signal(4, 40)), 0, 5, 35, FileInfo{})
	bad := *rec
	bad.Head = append([]byte{2}, bad.Head[1:]...)
	if bad.Validate() == nil {
		t.Fatal("version 2 head accepted")
	}
	bad = *rec
	bad.Tail = bad.Tail[:len(bad.Tail)-1]
	if bad.Validate() == nil {
		t.Fatal("a truncated tail accepted")
	}
	bad = *rec
	short, _ := Pack(Frames(signal(4, 3))[:10*Bands])
	bad.Head = short
	if bad.Validate() == nil {
		t.Fatal("a 10-frame head accepted")
	}
	bad = *rec
	bad.SeekableByBitrate = true
	if bad.Validate() == nil {
		t.Fatal("seekable without a bitrate accepted")
	}
	bad = *rec
	bad.Start = math.NaN()
	if bad.Validate() == nil {
		t.Fatal("NaN start accepted")
	}
}

func wavBytes(pcm []int16, extra bool) []byte {
	var b bytes.Buffer
	data := make([]byte, 2*len(pcm))
	for i, v := range pcm {
		binary.LittleEndian.PutUint16(data[2*i:], uint16(v))
	}
	var body bytes.Buffer
	body.WriteString("WAVE")
	f := make([]byte, 16)
	binary.LittleEndian.PutUint16(f[0:], 1)
	binary.LittleEndian.PutUint16(f[2:], 1)
	binary.LittleEndian.PutUint32(f[4:], SampleRate)
	binary.LittleEndian.PutUint32(f[8:], SampleRate*2)
	binary.LittleEndian.PutUint16(f[12:], 2)
	binary.LittleEndian.PutUint16(f[14:], 16)
	body.WriteString("fmt ")
	binary.Write(&body, binary.LittleEndian, uint32(16))
	body.Write(f)
	if extra {
		body.WriteString("LIST")
		binary.Write(&body, binary.LittleEndian, uint32(5))
		body.WriteString("abcde\x00") // odd size plus pad byte
	}
	body.WriteString("data")
	binary.Write(&body, binary.LittleEndian, uint32(len(data)))
	body.Write(data)
	b.WriteString("RIFF")
	binary.Write(&b, binary.LittleEndian, uint32(body.Len()))
	b.Write(body.Bytes())
	return b.Bytes()
}

func TestTrackFromWAVEqualsFramesOfThePCM(t *testing.T) {
	pcm := signal(5, 6)
	want := Frames(pcm)
	for _, extra := range []bool{false, true} {
		got, err := TrackFromWAV(bytes.NewReader(wavBytes(pcm, extra)))
		if err != nil || !bytes.Equal(got, want) {
			t.Fatalf("extra chunk %v: err %v, equal %v", extra, err, bytes.Equal(got, want))
		}
	}
	if _, err := TrackFromWAV(bytes.NewReader([]byte("not a wav at all"))); err == nil {
		t.Fatal("garbage accepted")
	}
}

func mp3File(t *testing.T, id3 int, frames int, kbpsAt func(i int) byte, tail int, padded ...bool) string {
	isPadded := len(padded) > 0 && padded[0]
	padded_ := isPadded
	var b bytes.Buffer
	if id3 > 0 {
		b.WriteString("ID3\x03\x00\x00")
		n := id3 - 10
		b.Write([]byte{byte(n >> 21 & 0x7F), byte(n >> 14 & 0x7F), byte(n >> 7 & 0x7F), byte(n & 0x7F)})
		b.Write(make([]byte, n))
	}
	for i := 0; i < frames; i++ {
		idx := kbpsAt(i) // 9 = 128 kbps, 10 = 160 kbps at 44.1 kHz
		length, pad := 417, byte(0)
		if idx == 10 {
			length = 522
		}
		if padded_ && i%2 == 1 { // 128 kbps at 44.1 kHz: 417.96 bytes, so 417 and 418 alternate
			length, pad = length+1, 2
		}
		fr := make([]byte, length)
		fr[0], fr[1], fr[2], fr[3] = 0xFF, 0xFB, idx<<4|pad, 0x00
		b.Write(fr)
	}
	b.Write(make([]byte, tail))
	p := filepath.Join(t.TempDir(), "x.mp3")
	if err := os.WriteFile(p, b.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestAnalyzeMP3(t *testing.T) {
	cbr := func(int) byte { return 9 }
	info, err := AnalyzeMP3(mp3File(t, 2048, 300, cbr, 128))
	if err != nil || !info.SeekableByBitrate || info.BitrateKbps != 128 || info.FirstFrameOffset != 2048 || info.Bytes != 2048+300*417+128 {
		t.Fatalf("CBR with ID3 and an ID3v1 tail: %+v %v", info, err)
	}
	info, _ = AnalyzeMP3(mp3File(t, 0, 300, cbr, 0))
	if !info.SeekableByBitrate || info.FirstFrameOffset != 0 {
		t.Fatalf("CBR without a tag: %+v", info)
	}
	info, _ = AnalyzeMP3(mp3File(t, 0, 300, func(i int) byte {
		if i > 150 {
			return 10
		}
		return 9
	}, 0))
	if info.SeekableByBitrate || info.BitrateKbps != 0 {
		t.Fatalf("a bitrate change is not CBR: %+v", info)
	}
	info, _ = AnalyzeMP3(mp3File(t, 0, 300, cbr, 100000))
	if info.SeekableByBitrate {
		t.Fatalf("a lost sync long before the end is not CBR: %+v", info)
	}
	info, _ = AnalyzeMP3(mp3File(t, 10, 300, cbr, 128, true))
	if !info.SeekableByBitrate || info.BitrateKbps != 128 || info.Bytes != 10+150*417+150*418+128 {
		t.Fatalf("CBR with padded frames (417 and 418 bytes alternating): %+v", info)
	}
	p := filepath.Join(t.TempDir(), "x.m4a")
	os.WriteFile(p, bytes.Repeat([]byte{0x00, 0x00, 0x00, 0x20, 'f', 't', 'y', 'p'}, 2000), 0o600)
	if info, _ := AnalyzeMP3(p); info.SeekableByBitrate {
		t.Fatalf("not MP3: %+v", info)
	}
}

// naiveFrames is a second, deliberately plain implementation written from the
// spec page alone: an O(N^2) DFT with cos and sin per term, band edges from
// 100*38^(b/8) Hz, its own dB and quantiser. The fixture must come out of it
// as well as out of Frames.
func naiveFrames(pcm []int16) []byte {
	var edges [9]int
	for b := 0; b <= 8; b++ {
		edges[b] = int(math.Floor(100*math.Pow(38, float64(b)/8)/15.625 + 0.5))
	}
	var out []byte
	for k := 0; (k+0)*1600+1024 <= len(pcm); k++ {
		x := make([]float64, 1024)
		for n := range x {
			w := 0.5 - 0.5*math.Cos(2*math.Pi*float64(n)/1024)
			x[n] = float64(pcm[k*1600+n]) / 32768 * w
		}
		for b := 0; b < 8; b++ {
			sum := 0.0
			for bin := edges[b]; bin < edges[b+1]; bin++ {
				var re, im float64
				for n, v := range x {
					a := 2 * math.Pi * float64(bin) * float64(n) / 1024
					re += v * math.Cos(a)
					im -= v * math.Sin(a)
				}
				sum += re*re + im*im
			}
			mean := sum / float64(edges[b+1]-edges[b]) / (1024.0 * 1024.0)
			db := 10 * math.Log(mean+1e-10) / math.Ln10
			q := int(math.Floor((db+100)*2.55 + 0.5))
			if q < 0 {
				q = 0
			}
			if q > 255 {
				q = 255
			}
			out = append(out, byte(q))
		}
	}
	return out
}

func TestFixtureIsReproducedByAnIndependentNaiveReference(t *testing.T) {
	raw, err := os.ReadFile(filepath.FromSlash(fixturePath))
	if err != nil {
		t.Fatal(err)
	}
	var f fixtureFile
	if err := json.Unmarshal(raw, &f); err != nil {
		t.Fatal(err)
	}
	for b, e := range []int{6, 10, 16, 25, 39, 62, 98, 154, 243} {
		if f.BandBins[b] != e {
			t.Fatalf("band edge %d: fixture %d, spec table %d", b, f.BandBins[b], e)
		}
	}
	got := naiveFrames(FixturePCM(f.Samples))
	want, _ := hex.DecodeString(f.FramesHex)
	if !bytes.Equal(got, want) {
		for i := range got {
			if got[i] != want[i] {
				t.Fatalf("naive reference differs from the fixture at byte %d (frame %d band %d): %d against %d", i, i/8, i%8, got[i], want[i])
			}
		}
		t.Fatalf("naive reference has %d bytes, fixture %d", len(got), len(want))
	}
	for b := 0; b <= 8; b++ {
		if e := int(math.Floor(100*math.Pow(38, float64(b)/8)/15.625 + 0.5)); e != BandBins[b] {
			t.Fatalf("BandBins[%d] = %d, the formula gives %d", b, BandBins[b], e)
		}
	}
	pk, _ := hex.DecodeString(f.Locate.PackHex)
	n := f.Locate.To - f.Locate.From
	manual := append([]byte{1, 8, byte(n >> 8), byte(n)}, want[f.Locate.From*8:f.Locate.To*8]...)
	if !bytes.Equal(pk, manual) {
		t.Fatal("pack_hex is not the version, band count, frame count and frames")
	}
}
