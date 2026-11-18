package podcastworker

// The fingerprint through the real codec: the same audio encoded twice to MP3
// at a low bitrate, the second with an ad in front, still locates; and the
// MP3 the encoder wrote is seen as constant bitrate. Skipped without ffmpeg.

import (
	"bytes"
	"encoding/binary"
	"math"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/podcastfp"
)

func speechLike(seed int64, seconds int) []int16 {
	r := rand.New(rand.NewSource(seed))
	out := make([]int16, seconds*16000)
	var phase, amp [podcastfp.Bands]float64
	for i := range out {
		if i%1600 == 0 {
			for b := range amp {
				amp[b] = math.Pow(r.Float64(), 2) * 2500
			}
		}
		var v float64
		for b := 0; b < podcastfp.Bands; b++ {
			centre := float64(podcastfp.BandBins[b]+podcastfp.BandBins[b+1]) / 2 * 15.625
			phase[b] += 2 * math.Pi * centre / 16000
			v += amp[b] * math.Sin(phase[b])
		}
		v += r.NormFloat64() * 30
		out[i] = int16(math.Max(-32768, math.Min(32767, v)))
	}
	return out
}

func writeWAV(t *testing.T, path string, pcm []int16) {
	var b bytes.Buffer
	b.WriteString("RIFF")
	binary.Write(&b, binary.LittleEndian, uint32(36+2*len(pcm)))
	b.WriteString("WAVEfmt ")
	binary.Write(&b, binary.LittleEndian, uint32(16))
	binary.Write(&b, binary.LittleEndian, uint16(1))
	binary.Write(&b, binary.LittleEndian, uint16(1))
	binary.Write(&b, binary.LittleEndian, uint32(16000))
	binary.Write(&b, binary.LittleEndian, uint32(32000))
	binary.Write(&b, binary.LittleEndian, uint16(2))
	binary.Write(&b, binary.LittleEndian, uint16(16))
	b.WriteString("data")
	binary.Write(&b, binary.LittleEndian, uint32(2*len(pcm)))
	binary.Write(&b, binary.LittleEndian, pcm)
	if err := os.WriteFile(path, b.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestFingerprintSurvivesAnMP3EncodeAndAnInsertedAd(t *testing.T) {
	ff, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("no ffmpeg")
	}
	dir := t.TempDir()
	run := func(args ...string) {
		if out, err := exec.Command(ff, append([]string{"-nostdin", "-loglevel", "error", "-y"}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("ffmpeg %v: %v %s", args, err, out)
		}
	}
	src := speechLike(1, 100)
	ad := speechLike(77, 12)[:int(9.4*16000)]
	writeWAV(t, filepath.Join(dir, "a.wav"), src)
	writeWAV(t, filepath.Join(dir, "b.wav"), append(append([]int16{}, ad...), src...))
	run("-i", filepath.Join(dir, "a.wav"), "-ar", "44100", "-b:a", "64k", filepath.Join(dir, "a.mp3"))
	run("-i", filepath.Join(dir, "b.wav"), "-ar", "44100", "-b:a", "64k", filepath.Join(dir, "b.mp3"))
	for _, n := range []string{"a", "b"} {
		info, err := podcastfp.AnalyzeMP3(filepath.Join(dir, n+".mp3"))
		if err != nil || !info.SeekableByBitrate || info.BitrateKbps != 64 {
			t.Fatalf("%s.mp3 as the encoder wrote it: %+v %v", n, info, err)
		}
	}
	track := func(n string) []byte {
		run("-i", filepath.Join(dir, n+".mp3"), "-ar", "16000", "-ac", "1", "-c:a", "pcm_s16le", filepath.Join(dir, n+"16.wav"))
		return wavTrack(filepath.Join(dir, n+"16.wav"))
	}
	ta, tb := track("a"), track("b")
	rec, err := podcastfp.Cut(ta, 0, 40, 60, podcastfp.FileInfo{})
	if err != nil {
		t.Fatal(err)
	}
	for name, p := range map[string][]byte{"head": rec.Head, "tail": rec.Tail} {
		tpl, _ := podcastfp.Unpack(p)
		m, ok := podcastfp.Locate(tpl, tb)
		at := rec.HeadAt
		if name == "tail" {
			at = rec.TailAt
		}
		if !ok || !m.Found() || math.Abs(m.Seconds()-at-9.4) > 0.5 {
			t.Fatalf("%s: found at %.2f s (stored %.2f, ad 9.4): %+v", name, m.Seconds(), at, m)
		}
	}
}
