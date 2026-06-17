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

func TestBuiltToScaleDSPianoLoopParams(t *testing.T) {
	assertRuntimeNear(t, pianoEndBeat(12, 0.75), 12.75)
	assertRuntimeNear(t, pianoEndBeat(12, -0.25), 12)
	assertRuntimeNear(t, pianoFadeSec, 0.1)
}

func TestBuiltToScaleDSBlockFrameMatchesUnityConstants(t *testing.T) {
	ev := blockEvt{beat: 72.5, length: 0.75}
	frame := blockAnimFrame(ev, hitBeat(ev), 0.5)
	if math.Abs(frame-blockHitFrame) > 0.5 {
		t.Fatalf("frame at hit = %.3f, want near %.3f", frame, blockHitFrame)
	}
}

func TestBuiltToScaleDSBlockFrameSnapsCriticalUnityFrames(t *testing.T) {
	ev := blockEvt{beat: 72.5, length: 0.75}
	secPerBeat := 0.5
	targetFrame := 7.1
	spawnTimeOffset := spawnFrameOffset / blockFramesPerSecond
	secondsPerFrame := 1 / blockFramesPerSecond
	secondsToHitFrame := secondsPerFrame * blockHitFrame
	secondsToHitBeat := secPerBeat*5*ev.length + spawnTimeOffset
	speedMult := secondsToHitFrame / secondsToHitBeat
	secondsPastSpawn := targetFrame / (blockFramesPerSecond * speedMult)
	beat := spawnBeat(ev) + (secondsPastSpawn-spawnTimeOffset)/secPerBeat

	if got := blockAnimFrame(ev, beat, secPerBeat); got != 8 {
		t.Fatalf("critical frame %.2f snapped to %.3f, want 8", targetFrame, got)
	}
}

func assertRuntimeNear(t *testing.T, got, want float64) {
	t.Helper()
	if math.Abs(got-want) > 0.0001 {
		t.Fatalf("got %v, want %v", got, want)
	}
}
