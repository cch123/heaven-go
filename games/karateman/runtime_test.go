package karateman

import (
	"math"
	"testing"

	"hsdemo/kmdata"
)

func TestKaratePotFlightCrossesHitPositionAtJudgeBeat(t *testing.T) {
	st := kmdata.Stage{
		HitPos:       [2]float64{2.862, 1.07},
		FloorY:       -2.1,
		StartOffset:  [2]float64{1.5, 0},
		StartOffsetZ: -8,
		HitOffset:    0.65,
		Slip:         0.13,
	}
	x, y, z, rot := karatePotFlight(st, 12, 0.25, 13)
	assertNear(t, x, st.HitPos[0])
	assertNear(t, y, st.HitPos[1])
	assertNear(t, z, 0)
	if math.IsNaN(rot) || math.IsInf(rot, 0) {
		t.Fatalf("rotation is not finite: %v", rot)
	}
}

func TestKarateCueLookupTablesMatchPackInUsage(t *testing.T) {
	if got := hitTypeSprite(hitRock); got != "karateman_rock" {
		t.Fatalf("rock sprite = %s", got)
	}
	if got := hitTypeSound(hitRock); got != "rockHit" {
		t.Fatalf("rock hit sound = %s", got)
	}
	if !hitTypeHeavy(hitRock) {
		t.Fatal("rock should use Joe's heavy straight punch")
	}
	if got := throwSound(29.5); got != "offbeatObjectOut" {
		t.Fatalf("offbeat throw sound = %s", got)
	}
	if got := wordClearBeat(26, 1, 4, false); got != 30 {
		t.Fatalf("hit four clear beat = %v", got)
	}
	hit, number := wordVoice(3)
	if hit != "en/hitAlt" || number != "en/threeAlt" {
		t.Fatalf("alternate hit-three voice = %s/%s", hit, number)
	}
}

func assertNear(t *testing.T, got, want float64) {
	t.Helper()
	if math.Abs(got-want) > 0.0001 {
		t.Fatalf("got %v, want %v", got, want)
	}
}
