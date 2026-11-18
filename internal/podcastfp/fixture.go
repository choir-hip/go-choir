package podcastfp

// FixturePCM is the conformance signal, integer arithmetic only so any
// language reproduces it exactly: a linear congruential noise (x = x*1664525 +
// 1013904223 mod 2^32, seed 1, advanced before each sample), taken as
// (x>>16)-32768, scaled by an envelope of 1,2,4,8,16,8,4,2 (one value per
// 1600 samples, repeating) and divided by 64 (truncating toward zero), plus a
// 400 Hz square wave of +3000 and -3000 (20 samples each). n samples.
func FixturePCM(n int) []int16 {
	env := [8]int32{1, 2, 4, 8, 16, 8, 4, 2}
	x := uint32(1)
	out := make([]int16, n)
	for i := range out {
		x = x*1664525 + 1013904223
		noise := int32(x>>16) - 32768
		v := noise * env[(i/1600)%8] / 64
		if (i/20)%2 == 0 {
			v += 3000
		} else {
			v -= 3000
		}
		out[i] = int16(v)
	}
	return out
}

// minBoundaryDistance reports how close, in quantiser steps, any value of the
// frames of pcm is to a rounding boundary: a fixture whose values are all far
// from a boundary quantises the same in any IEEE double implementation.
func minBoundaryDistance(pcm []int16) float64 {
	min := 1.0
	for off := 0; off+FrameLen <= len(pcm); off += Hop {
		for _, db := range frameValues(pcm[off : off+FrameLen]) {
			v := (db-DBFloor)*Scale + 0.5
			if v < 0 || v > 255 {
				continue
			}
			d := v - float64(int64(v))
			if d > 0.5 {
				d = 1 - d
			}
			if d < min {
				min = d
			}
		}
	}
	return min
}
