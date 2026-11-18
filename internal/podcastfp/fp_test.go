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
	start, end := 40.0, 52.0
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
	rec, _ := Cut(Frames(signal(1, 120)), 0, 40, 52, FileInfo{})
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

func TestCutShortClipAndBounds(t *testing.T) {
	track := Frames(signal(4, 30))
	rec, err := Cut(track, 0, 10, 18, FileInfo{})
	if err != nil {
		t.Fatal(err)
	}
	h, _ := Unpack(rec.Head)
	if len(h)/Bands != 80 || rec.HeadAt != 10 || rec.TailAt != 10 {
		t.Fatalf("an 8 s clip: %d frames, head_at %v tail_at %v", len(h)/Bands, rec.HeadAt, rec.TailAt)
	}
	if err := rec.Validate(); err != nil {
		t.Fatal(err)
	}
	if _, err := Cut(track, 0, 25, 40, FileInfo{}); err == nil {
		t.Fatal("a clip past the track was cut")
	}
}

func TestValidateRefusesBadRecords(t *testing.T) {
	rec, _ := Cut(Frames(signal(4, 30)), 0, 5, 25, FileInfo{})
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

func mp3File(t *testing.T, id3 int, frames int, kbpsAt func(i int) byte, tail int) string {
	var b bytes.Buffer
	if id3 > 0 {
		b.WriteString("ID3\x03\x00\x00")
		n := id3 - 10
		b.Write([]byte{byte(n >> 21 & 0x7F), byte(n >> 14 & 0x7F), byte(n >> 7 & 0x7F), byte(n & 0x7F)})
		b.Write(make([]byte, n))
	}
	for i := 0; i < frames; i++ {
		idx := kbpsAt(i) // 9 = 128 kbps, 10 = 160 kbps at 44.1 kHz, no padding
		length := 417
		if idx == 10 {
			length = 522
		}
		fr := make([]byte, length)
		fr[0], fr[1], fr[2], fr[3] = 0xFF, 0xFB, idx<<4, 0x00
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
	p := filepath.Join(t.TempDir(), "x.m4a")
	os.WriteFile(p, bytes.Repeat([]byte{0x00, 0x00, 0x00, 0x20, 'f', 't', 'y', 'p'}, 2000), 0o600)
	if info, _ := AnalyzeMP3(p); info.SeekableByBitrate {
		t.Fatalf("not MP3: %+v", info)
	}
}
