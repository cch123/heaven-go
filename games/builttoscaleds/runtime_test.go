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

func TestBuiltToScaleDSLightPatternMatchesUnityAlternation(t *testing.T) {
	m := &Module{lights: []lightEvt{{beat: 8, auto: true}}}
	active, first, transition := m.lightPatternAt(8.25)
	if !active || !first || transition != 8 {
		t.Fatalf("auto light at 8.25 = active=%v first=%v transition=%v", active, first, transition)
	}
	active, first, transition = m.lightPatternAt(9.1)
	if !active || first || transition != 9 {
		t.Fatalf("auto light at 9.1 = active=%v first=%v transition=%v", active, first, transition)
	}
}

func TestBuiltToScaleDSManualLightsResetAfterLength(t *testing.T) {
	m := &Module{lights: []lightEvt{{beat: 4, length: 2, light: true}}}
	active, first, transition := m.lightPatternAt(5.2)
	if !active || first || transition != 5 {
		t.Fatalf("manual light inside event = active=%v first=%v transition=%v", active, first, transition)
	}
	active, first, transition = m.lightPatternAt(6.1)
	if active || !first || transition != 6 {
		t.Fatalf("manual light after end = active=%v first=%v transition=%v", active, first, transition)
	}
}

func TestBuiltToScaleDSLightTargetsAndBeltOffset(t *testing.T) {
	env := [4]float64{0.1, 0.4, 0.2, 1}
	first, second := lightTargets(true, false, env)
	if first != env || second != [4]float64{1, 1, 1, 1} {
		t.Fatalf("second light target = first %#v second %#v", first, second)
	}
	assertRuntimeNear(t, beltTextureOffset(4.33, 0.5)[1], math.Mod(-4.33*0.5, 1))
}

func assertRuntimeNear(t *testing.T, got, want float64) {
	t.Helper()
	if math.Abs(got-want) > 0.0001 {
		t.Fatalf("got %v, want %v", got, want)
	}
}
