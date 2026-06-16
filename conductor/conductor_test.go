package conductor

import (
	"math"
	"testing"

	"hsdemo/riq"
)

func TestMinigamePitchMapsOutputClockToSongTime(t *testing.T) {
	out := 0.0
	c := New(&riq.Beatmap{Tempos: []riq.TempoChange{{Beat: 0, BPM: 60}}}, nil)
	c.SetClock(func() float64 { return out })

	out = 4
	c.pos = c.realPosition()
	c.SetMinigamePitch(0.25)

	out = 5
	assertNear(t, c.realPosition(), 4.25)

	c.pos = c.realPosition()
	c.SetMinigamePitch(1)

	out = 6
	assertNear(t, c.realPosition(), 5.25)
}

func TestMinigamePitchRebaseUsesSmoothSongPosition(t *testing.T) {
	out := 4.05
	c := New(&riq.Beatmap{Tempos: []riq.TempoChange{{Beat: 0, BPM: 60}}}, nil)
	c.SetClock(func() float64 { return out })

	c.pos = 4
	c.SetMinigamePitch(0.5)
	assertNear(t, c.realPosition(), 4)
}

func assertNear(t *testing.T, got, want float64) {
	t.Helper()
	if math.Abs(got-want) > 1e-9 {
		t.Fatalf("got %.12f, want %.12f", got, want)
	}
}
