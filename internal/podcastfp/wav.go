package podcastfp

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

// TrackFromWAV reads a 16 kHz mono 16-bit PCM WAV stream (as `ffmpeg -ar 16000
// -ac 1 -c:a pcm_s16le` writes it, with or without extra chunks) and returns
// its frame track. Memory is one frame plus the track.
func TrackFromWAV(r io.Reader) ([]byte, error) {
	var hdr [12]byte
	if _, err := io.ReadFull(r, hdr[:]); err != nil {
		return nil, err
	}
	if string(hdr[:4]) != "RIFF" || string(hdr[8:]) != "WAVE" {
		return nil, errors.New("podcastfp: not a WAV stream")
	}
	var b Builder
	for {
		var ck [8]byte
		if _, err := io.ReadFull(r, ck[:]); err != nil {
			return nil, fmt.Errorf("podcastfp: no data chunk: %w", err)
		}
		size := int64(binary.LittleEndian.Uint32(ck[4:]))
		switch string(ck[:4]) {
		case "fmt ":
			f := make([]byte, size)
			if _, err := io.ReadFull(r, f); err != nil {
				return nil, err
			}
			if size < 16 || binary.LittleEndian.Uint16(f[0:]) != 1 || binary.LittleEndian.Uint16(f[2:]) != 1 ||
				binary.LittleEndian.Uint32(f[4:]) != SampleRate || binary.LittleEndian.Uint16(f[14:]) != 16 {
				return nil, errors.New("podcastfp: the WAV is not 16 kHz mono 16-bit PCM")
			}
		case "data":
			// A streamed WAV may carry 0 or 0xFFFFFFFF: read to the end.
			var src io.Reader = r
			if size != 0 && size != 0xFFFFFFFF {
				src = io.LimitReader(r, size)
			}
			buf := make([]byte, 64<<10)
			var carry []byte
			for {
				n, err := src.Read(buf)
				p := append(carry, buf[:n]...)
				even := len(p) &^ 1
				if _, werr := b.Write(p[:even]); werr != nil {
					return nil, werr
				}
				carry = append([]byte(nil), p[even:]...)
				if err == io.EOF {
					return b.Track(), nil
				}
				if err != nil {
					return nil, err
				}
			}
		default:
			if _, err := io.CopyN(io.Discard, r, size+size&1); err != nil {
				return nil, err
			}
		}
	}
}
