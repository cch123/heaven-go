package spacesoccer

import (
	"math"
	"testing"

	"hsdemo/kart"
)

func loadAssets(t *testing.T) *kart.Assets {
	t.Helper()
	as, err := kart.Load("../../assets/spaceSoccer", 44100)
	if err != nil {
		t.Skipf("assets not extracted: %v", err)
	}
	return as
}

func TestSpaceSoccerAssetsContainRuntimePieces(t *testing.T) {
	as := loadAssets(t)
	for _, role := range []string{"kickerPrefab", "ballRef", "bg", "bgImage"} {
		if as.Roles[role] == "" {
			t.Fatalf("missing role %s", role)
		}
	}
	for _, ctrl := range []string{"Space Kicker", "Holder", "Flames"} {
		if _, ok := as.Controllers[ctrl]; !ok {
			t.Fatalf("missing controller %s", ctrl)
		}
	}
	for _, snd := range []string{
		"kick", "ballHit", "missNeutral", "highkicktoe1", "highkicktoe1_hit",
		"highkicktoe3", "highkicktoe3_hit", "dispenseNoise", "dispenseTumble6B",
	} {
		if len(as.Sounds[snd]) == 0 {
			t.Fatalf("missing sound %s", snd)
		}
	}
	for _, sp := range []string{"background_0", "background_1", "ball", "toe_fx", "platform_0"} {
		if _, ok := as.Sheet.Sprites[sp]; !ok {
			t.Fatalf("missing sprite %s", sp)
		}
	}
}

func TestSpaceSoccerControllerStatesHaveClips(t *testing.T) {
	as := loadAssets(t)
	allowedEmpty := map[string]bool{
		"Space Kicker/High":            true,
		"Space Kicker/HighKickRight_1": true,
	}
	for ctrlName, ctrl := range as.Controllers {
		for state, st := range ctrl.States {
			key := ctrlName + "/" + state
			if st.Clip == "" {
				if !allowedEmpty[key] {
					t.Fatalf("unexpected empty motion state %s", key)
				}
				continue
			}
			if as.Anims[st.Clip] == nil {
				t.Fatalf("%s references missing clip %s", key, st.Clip)
			}
		}
	}
}

func TestSpaceSoccerBallPathsMatchPrefabValues(t *testing.T) {
	as := loadAssets(t)
	paths := loadBallPaths(as.Extra.Components["game"].Lists["ballPaths"])
	tests := []struct {
		name     string
		duration float64
		height   float64
		end      [3]float64
	}{
		{"Dispense", 2.35, 10, [3]float64{-1, -6, 0}},
		{"Kick", 1.4, 5, [3]float64{3.77, -6.480046, 0}},
		{"HighKick", 2.3, 5.7, [3]float64{-3.5, -6, 0}},
		{"Toe", 1.35, 7.5, [3]float64{2.61, -6.08, 0}},
	}
	for _, tt := range tests {
		p, ok := paths[tt.name]
		if !ok {
			t.Fatalf("missing path %s", tt.name)
		}
		if math.Abs(p.points[0].duration-tt.duration) > 1e-6 {
			t.Fatalf("%s duration = %.6f, want %.6f", tt.name, p.points[0].duration, tt.duration)
		}
		if math.Abs(p.points[0].height-tt.height) > 1e-6 {
			t.Fatalf("%s height = %.6f, want %.6f", tt.name, p.points[0].height, tt.height)
		}
		if dist3(p.points[1].pos, tt.end) > 1e-5 {
			t.Fatalf("%s end = %v, want %v", tt.name, p.points[1].pos, tt.end)
		}
	}
}

func TestSpaceSoccerBallArcUsesVisualDuration(t *testing.T) {
	as := loadAssets(t)
	p := loadBallPaths(as.Extra.Components["game"].Lists["ballPaths"])["Dispense"]
	mid := p.posAt(1.175, 0, [2]float64{})
	if math.Abs(mid[0]-(-3.5)) > 1e-6 {
		t.Fatalf("dispense mid x = %.6f, want -3.5", mid[0])
	}
	if mid[1] <= 0 {
		t.Fatalf("dispense arc y = %.6f, want visible upward arc", mid[1])
	}
}

func TestSpaceSoccerColorsAndFormationDefaults(t *testing.T) {
	m := New().(*Module)
	bg, dots := m.bgColorsAt(0)
	if bg != defaultBG || dots != defaultDots {
		t.Fatalf("default colors = %v/%v, want %v/%v", bg, dots, defaultBG, defaultDots)
	}
	m.npcEvents = []npcEvt{{beat: 4, length: 4, preset: presetFive, choice: animEnter, ease: 0}}
	amount, x, y, z, active := m.formationAt(5)
	if amount != 5 || x != 2 || y != -0.5 || z != 1.25 {
		t.Fatalf("five-kicker formation = %d %.2f %.2f %.2f", amount, x, y, z)
	}
	if active == nil || active.anim != "Enter" {
		t.Fatalf("active enter anim = %#v, want Enter", active)
	}
}

func dist3(a, b [3]float64) float64 {
	dx, dy, dz := a[0]-b[0], a[1]-b[1], a[2]-b[2]
	return math.Sqrt(dx*dx + dy*dy + dz*dz)
}
