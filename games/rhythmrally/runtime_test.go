package rhythmrally

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"hsdemo/kmdata"
)

func loadRuntimeCurves(t *testing.T) map[string]kmdata.Curve {
	t.Helper()
	var extra kmdata.Extra
	raw, err := os.ReadFile(filepath.Join("..", "..", "assets", "rhythmRally", "extra.json"))
	if err != nil {
		t.Skipf("assets not extracted: %v", err)
	}
	if err := json.Unmarshal(raw, &extra); err != nil {
		t.Fatal(err)
	}
	return extra.Curves
}

func TestTargetAndBounceMatchUnitySpeeds(t *testing.T) {
	for speed, want := range map[int][2]float64{
		speedNormal:    {2, 1},
		speedFast:      {1, 0.5},
		speedSuperFast: {1, 0.5},
		speedSlow:      {4, 2},
	} {
		target, bounce := targetAndBounce(speed)
		if target != want[0] || bounce != want[1] {
			t.Fatalf("speed %d target/bounce = %.2f/%.2f, want %.2f/%.2f", speed, target, bounce, want[0], want[1])
		}
	}
}

func TestRallyIntervalMatchesOpponentReturnTiming(t *testing.T) {
	for speed, want := range map[int]float64{
		speedNormal:    4,
		speedFast:      4,
		speedSuperFast: 2,
		speedSlow:      8,
	} {
		if got := rallyInterval(speed); got != want {
			t.Fatalf("speed %d rally interval = %.2f, want %.2f", speed, got, want)
		}
	}
}

func TestBallPositionUsesServeAndReturnCurves(t *testing.T) {
	m := &Module{curves: loadRuntimeCurves(t)}
	m.ball = ballState{started: true, served: true, ballActive: true, serveBeat: 10, targetBeat: 2, speed: speedNormal}
	start := m.ballPosition(10)
	bounce := m.ballPosition(11)
	if start[2] <= 2 || bounce[1] <= start[1] {
		t.Fatalf("serve path start=%v bounce=%v should move from opponent side with arced height", start, bounce)
	}

	m.ball.served = false
	ret := m.ballPosition(12)
	if ret[2] >= -2 {
		t.Fatalf("return path at hit beat = %v, want player side z < -2", ret)
	}
}

func TestFlightTimingMatchesUnitySpeedBranches(t *testing.T) {
	for _, tc := range []struct {
		name         string
		speed        int
		served       bool
		hit, d1, d2  float64
		heightFirst  float64
		heightSecond float64
	}{
		{"normalServe", speedNormal, true, 4, 1, 1, 1.25, 1.25},
		{"normalReturn", speedNormal, false, 6, 1, 1, 1.25, 1.25},
		{"fastServe", speedFast, true, 4, 0.5, 0.5, 0.75, 0.75},
		{"fastReturn", speedFast, false, 5, 1, 2, 1.25, 2},
		{"superFastReturn", speedSuperFast, false, 5, 0.5, 0.5, 0.75, 0.75},
		{"slowReturn", speedSlow, false, 8, 2, 2, 3, 3},
	} {
		m := &Module{ball: ballState{serveBeat: 4, speed: tc.speed, served: tc.served}}
		hit, d1, d2 := m.flightTiming()
		if hit != tc.hit || d1 != tc.d1 || d2 != tc.d2 {
			t.Fatalf("%s timing = %.2f %.2f %.2f, want %.2f %.2f %.2f", tc.name, hit, d1, d2, tc.hit, tc.d1, tc.d2)
		}
		if got := m.flightHeight(tc.speed, tc.served, false); got != tc.heightFirst {
			t.Fatalf("%s first-leg height = %.2f, want %.2f", tc.name, got, tc.heightFirst)
		}
		if got := m.flightHeight(tc.speed, tc.served, true); got != tc.heightSecond {
			t.Fatalf("%s second-leg height = %.2f, want %.2f", tc.name, got, tc.heightSecond)
		}
	}
}
