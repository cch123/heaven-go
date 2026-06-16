package bigrockfinish

import (
	"math"
	"testing"
)

func TestBigRockFinishGuitarLoopParams(t *testing.T) {
	assertNear(t, guitarStopBeat(32, 0.125, 0), 32.0625)
	assertNear(t, guitarStopBeat(32, 0.125, 3), 32.5)
	assertNear(t, guitarFadeSec, 0.1)
	assertNear(t, guitarWhiffFade, 0.25)
}

func assertNear(t *testing.T, got, want float64) {
	t.Helper()
	if math.Abs(got-want) > 1e-9 {
		t.Fatalf("got %v, want %v", got, want)
	}
}
