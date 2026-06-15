package engine

import (
	"encoding/binary"
	"math"
	"testing"
)

func TestPitchLoopReaderLoopsPCMAtUnityPitchOne(t *testing.T) {
	pcm := makeTestPCM([]int16{100, 200, 300})
	r := newPitchLoopReader(pcm, 1)
	out := make([]byte, 5*4)
	n, err := r.Read(out)
	if err != nil || n != len(out) {
		t.Fatalf("Read = %d, %v", n, err)
	}
	got := framesFromPCM(out)
	want := []int16{100, 200, 300, 100, 200}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("frame %d = %d, want %d", i, got[i], want[i])
		}
	}
}

func TestPitchLoopReaderRampPitch(t *testing.T) {
	r := newPitchLoopReader(makeTestPCM([]int16{0, 1000, 2000, 3000}), 1)
	r.RampPitch(2, 2/SampleRate)
	out := make([]byte, 3*4)
	if _, err := r.Read(out); err != nil {
		t.Fatal(err)
	}
	r.mu.Lock()
	got := r.pitch
	active := r.rampFrames
	r.mu.Unlock()
	if active != 0 {
		t.Fatalf("ramp still active after expected duration: %.3f frames", active)
	}
	if math.Abs(got-2) > 1e-9 {
		t.Fatalf("pitch = %.6f, want 2", got)
	}
}

func makeTestPCM(frames []int16) []byte {
	pcm := make([]byte, len(frames)*4)
	for i, v := range frames {
		binary.LittleEndian.PutUint16(pcm[i*4:], uint16(v))
		binary.LittleEndian.PutUint16(pcm[i*4+2:], uint16(v))
	}
	return pcm
}

func framesFromPCM(pcm []byte) []int16 {
	out := make([]int16, len(pcm)/4)
	for i := range out {
		out[i] = int16(binary.LittleEndian.Uint16(pcm[i*4:]))
	}
	return out
}
