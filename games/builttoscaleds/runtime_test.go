package builttoscaleds

import (
	"math"
	"testing"
)

func TestBuiltToScaleDSTimingFromSpawnBlocks(t *testing.T) {
	ev := blockEvt{beat: 72.5, length: 0.75}
	assertRuntimeNear(t, spawnBeat(ev), 71.75)
	assertRuntimeNear(t, windupBeat(ev), 74.75)
	assertRuntimeNear(t, hitBeat(ev), 75.5)
	noteLen, endLen := noteLengths(ev.length, false)
	assertRuntimeNear(t, noteLen, 0.75)
	assertRuntimeNear(t, endLen, 0.75)
}

func TestBuiltToScaleDSStaccatoAndPitch(t *testing.T) {
	noteLen, endLen := noteLengths(0.75, true)
	assertRuntimeNear(t, noteLen, 0.5)
	assertRuntimeNear(t, endLen, 0.5)
	assertRuntimeNear(t, semitonePitch(12), 2)
}

func TestBuiltToScaleDSBlockFrameMatchesUnityConstants(t *testing.T) {
	ev := blockEvt{beat: 72.5, length: 0.75}
	frame := blockAnimFrame(ev, hitBeat(ev), 0.5)
	if math.Abs(frame-blockHitFrame) > 0.5 {
		t.Fatalf("frame at hit = %.3f, want near %.3f", frame, blockHitFrame)
	}
}

func assertRuntimeNear(t *testing.T, got, want float64) {
	t.Helper()
	if math.Abs(got-want) > 0.0001 {
		t.Fatalf("got %v, want %v", got, want)
	}
}
