package podcastfp

import (
	"bufio"
	"os"
)

var (
	mpeg1L3Kbps = [16]int{0, 32, 40, 48, 56, 64, 80, 96, 112, 128, 160, 192, 224, 256, 320, 0}
	mpeg2L3Kbps = [16]int{0, 8, 16, 24, 32, 40, 48, 56, 64, 80, 96, 112, 128, 144, 160, 0}
	rate1       = [4]int{44100, 48000, 32000, 0}
)

// frameHeader parses a 4-byte MPEG audio header; ok only for layer III.
func frameHeader(h []byte) (length, kbps int, ok bool) {
	if len(h) < 4 || h[0] != 0xFF || h[1]&0xE0 != 0xE0 {
		return
	}
	ver := (h[1] >> 3) & 3 // 3 = MPEG1, 2 = MPEG2, 0 = MPEG2.5
	layer := (h[1] >> 1) & 3
	bi, si := int(h[2]>>4), int(h[2]>>2)&3
	if ver == 1 || layer != 1 || bi == 0 || bi == 15 || si == 3 {
		return
	}
	rate := rate1[si]
	table, coef := mpeg1L3Kbps, 144
	if ver != 3 {
		table, coef = mpeg2L3Kbps, 72
		rate /= 2
		if ver == 0 {
			rate /= 2
		}
	}
	kbps = table[bi]
	pad := int(h[2]>>1) & 1
	return coef*kbps*1000/rate + pad, kbps, true
}

// AnalyzeMP3 walks an MP3 file frame by frame. SeekableByBitrate is true when
// the file has an ID3v2 tag or none, then MPEG audio frames of one bitrate
// to within an ID3v1/APE-sized tail, and no tag in the middle. Anything else
// (AAC, a changing bitrate, a stitched tag, a lost sync) is false, with what
// was learned still filled in.
func AnalyzeMP3(path string) (FileInfo, error) {
	f, err := os.Open(path)
	if err != nil {
		return FileInfo{}, err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return FileInfo{}, err
	}
	info := FileInfo{Bytes: st.Size()}
	r := bufio.NewReaderSize(f, 1<<16)
	var pos int64
	h, err := r.Peek(10)
	if err == nil && string(h[:3]) == "ID3" && h[6]|h[7]|h[8]|h[9] < 0x80 {
		size := int64(h[6])<<21 | int64(h[7])<<14 | int64(h[8])<<7 | int64(h[9])
		size += 10
		if h[5]&0x10 != 0 {
			size += 10 // footer
		}
		if _, err := r.Discard(int(size)); err != nil {
			return info, nil
		}
		pos = size
	}
	info.FirstFrameOffset = pos
	var kbps0 int
	for {
		h, err := r.Peek(4)
		if err != nil {
			break
		}
		length, kbps, ok := frameHeader(h)
		if !ok || length < 4 {
			break
		}
		if kbps0 == 0 {
			kbps0 = kbps
		} else if kbps != kbps0 {
			return info, nil // not constant
		}
		if _, err := r.Discard(length); err != nil {
			pos += int64(length) // a short last frame
			break
		}
		pos += int64(length)
	}
	// Everything from the first frame to a tail of at most 4 KiB (an ID3v1 or
	// APE tag, padding) was frames of one bitrate.
	if kbps0 == 0 || info.Bytes-pos > 4096 || pos < info.FirstFrameOffset+4096 {
		return info, nil
	}
	info.BitrateKbps = kbps0
	info.SeekableByBitrate = true
	return info, nil
}
