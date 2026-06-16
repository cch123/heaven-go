package engine

import (
	"bytes"
	"encoding/binary"
	"io"
	"math"
	"testing"
)

func TestPitchPCMReaderMainMusicFilterIsTransparent(t *testing.T) {
	pcm := stereoPCM([]int16{0, 2000, -4000, 8000, -12000, 16000, -16000, 12000})
	r := newPitchPCMReader(pcm, 1)
	out := make([]byte, len(pcm))
	n, err := r.Read(out)
	if err != nil {
		t.Fatalf("Read returned error: %v", err)
	}
	if n != len(pcm) {
		t.Fatalf("Read wrote %d bytes, want %d", n, len(pcm))
	}
	if !bytes.Equal(out, pcm) {
		t.Fatal("Main mixer snapshot should not color unpitched PCM")
	}
}

func TestPitchPCMReaderMusicFilterAttenuatesHighFrequencies(t *testing.T) {
	pcm := alternatingStereoPCM(4096, 12000)
	base := readAllPCM(t, newPitchPCMReader(pcm, 1), len(pcm))

	filtered := newPitchPCMReader(pcm, 1)
	filtered.transitionMusicFilter(10, 500, 0, 0)
	out := readAllPCM(t, filtered, len(pcm))

	// Skip the filter warm-up transient; a 500 Hz lowpass should crush a
	// frame-to-frame alternating signal near Nyquist.
	baseRMS := pcmRMS(base[512*4:])
	outRMS := pcmRMS(out[512*4:])
	if outRMS >= baseRMS*0.35 {
		t.Fatalf("filtered RMS %.2f should be well below base RMS %.2f", outRMS, baseRMS)
	}
}

func TestPitchPCMReaderSeekResetsMusicFilter(t *testing.T) {
	pcm := alternatingStereoPCM(256, 9000)
	r := newPitchPCMReader(pcm, 1)
	r.transitionMusicFilter(10, 500, 0, 0)
	_ = readAllPCM(t, r, len(pcm))

	if _, err := r.Seek(0, io.SeekStart); err != nil {
		t.Fatalf("Seek failed: %v", err)
	}
	out := readAllPCM(t, r, len(pcm))
	if !bytes.Equal(out, pcm) {
		t.Fatal("Seek should reset live music filter state to Main snapshot")
	}
}

func stereoPCM(samples []int16) []byte {
	pcm := make([]byte, len(samples)*4)
	for i, s := range samples {
		binary.LittleEndian.PutUint16(pcm[i*4:], uint16(s))
		binary.LittleEndian.PutUint16(pcm[i*4+2:], uint16(s))
	}
	return pcm
}

func alternatingStereoPCM(frames int, amp int16) []byte {
	pcm := make([]byte, frames*4)
	for i := 0; i < frames; i++ {
		v := amp
		if i%2 == 1 {
			v = -amp
		}
		binary.LittleEndian.PutUint16(pcm[i*4:], uint16(v))
		binary.LittleEndian.PutUint16(pcm[i*4+2:], uint16(v))
	}
	return pcm
}

func readAllPCM(t *testing.T, r *pitchPCMReader, size int) []byte {
	t.Helper()
	out := make([]byte, size)
	n, err := r.Read(out)
	if err != nil {
		t.Fatalf("Read returned error: %v", err)
	}
	if n != size {
		t.Fatalf("Read wrote %d bytes, want %d", n, size)
	}
	return out
}

func pcmRMS(pcm []byte) float64 {
	var sum float64
	var count int
	for i := 0; i+1 < len(pcm); i += 2 {
		v := float64(int16(binary.LittleEndian.Uint16(pcm[i:])))
		sum += v * v
		count++
	}
	if count == 0 {
		return 0
	}
	return math.Sqrt(sum / float64(count))
}
