package conductor

import (
	"math"
	"testing"
	"time"

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

func TestUpdateIgnoresCoarseOutputClockWithinSyncMargin(t *testing.T) {
	const (
		tps     = 240
		seconds = 1
		quantum = 0.10
	)
	var (
		out float64
		now = time.Unix(0, 0)
	)
	c := New(&riq.Beatmap{Tempos: []riq.TempoChange{{Beat: 0, BPM: 60}}}, nil)
	c.SetClock(func() float64 { return out })
	c.now = func() time.Time { return now }
	c.playing = true
	c.lastTick = now
	c.rebaseOutputClock()

	step := time.Second / tps
	for i := 1; i <= tps*seconds; i++ {
		now = now.Add(step)
		elapsed := float64(i) / tps
		out = math.Floor(elapsed/quantum) * quantum
		c.Update()
	}

	// audio.Player.Position can be a staircase clock. Small, bounded lag from
	// that clock must not feed back into c.pos, otherwise every minigame's
	// animation curves inherit a visible speed wobble.
	want := (time.Duration(tps*seconds) * step).Seconds()
	if math.Abs(c.Time()-want) > 1e-9 {
		t.Fatalf("coarse output clock pulled song time: got %.12f, want %.12f", c.Time(), want)
	}
	if math.Abs(c.Drift()) > syncMargin {
		t.Fatalf("expected drift to remain inside deadband, got %.12f", c.Drift())
	}
}

func TestUpdateConvergesWhenOutputClockDriftExceedsMargin(t *testing.T) {
	var (
		out float64
		now = time.Unix(0, 0)
	)
	c := New(&riq.Beatmap{Tempos: []riq.TempoChange{{Beat: 0, BPM: 60}}}, nil)
	c.SetClock(func() float64 { return out })
	c.now = func() time.Time { return now }
	c.playing = true
	c.lastTick = now
	c.rebaseOutputClock()

	out = 1.0
	now = now.Add(time.Second / 240)
	c.Update()
	if math.Abs(c.Drift()) > syncMargin {
		t.Fatalf("large output drift was not brought inside margin: %.12f", c.Drift())
	}
}

func assertNear(t *testing.T, got, want float64) {
	t.Helper()
	if math.Abs(got-want) > 1e-9 {
		t.Fatalf("got %.12f, want %.12f", got, want)
	}
}
