package engine

import (
	"encoding/binary"
	"io"
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

func TestPitchPCMReaderResamplesAndTracksSourcePosition(t *testing.T) {
	r := newPitchPCMReader(makeTestPCM([]int16{0, 1000, 2000, 3000}), 0.5)
	out := make([]byte, 4*4)
	n, err := r.Read(out)
	if err != nil || n != len(out) {
		t.Fatalf("Read = %d, %v", n, err)
	}
	got := framesFromPCM(out)
	want := []int16{0, 500, 1000, 1500}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("frame %d = %d, want %d", i, got[i], want[i])
		}
	}
	if got := r.PositionSeconds(); math.Abs(got-2/float64(SampleRate)) > 1e-9 {
		t.Fatalf("PositionSeconds = %.12f, want %.12f", got, 2/float64(SampleRate))
	}
}

func TestPitchPCMReaderSeekAndEOF(t *testing.T) {
	r := newPitchPCMReader(makeTestPCM([]int16{100, 200, 300}), 1)
	if pos, err := r.Seek(4, io.SeekStart); err != nil || pos != 4 {
		t.Fatalf("Seek = %d, %v; want 4, nil", pos, err)
	}
	out := make([]byte, 2*4)
	n, err := r.Read(out)
	if err != nil || n != len(out) {
		t.Fatalf("Read = %d, %v", n, err)
	}
	got := framesFromPCM(out)
	want := []int16{200, 300}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("frame %d = %d, want %d", i, got[i], want[i])
		}
	}
	if n, err := r.Read(out[:4]); n != 0 || err != io.EOF {
		t.Fatalf("EOF Read = %d, %v; want 0, EOF", n, err)
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
