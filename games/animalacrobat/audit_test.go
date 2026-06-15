package animalacrobat

import (
	"math"
	"testing"

	"hsdemo/kart"
	"hsdemo/kmdata"
)

func loadAnimalAssets(t *testing.T) *kart.Assets {
	t.Helper()
	as, err := kart.Load("../../assets/animalAcrobat", 44100)
	if err != nil {
		t.Skipf("assets not extracted: %v", err)
	}
	return as
}

func TestAnimalAcrobatBindingsAndTemplates(t *testing.T) {
	as := loadAnimalAssets(t)
	for _, role := range []string{
		"_elephant", "_giraffe", "_monkeysLong", "_monkeysShort", "_gorilla",
		"_scroll", "_playerMonkey", "_spotlightMain", "_partyPoppers",
	} {
		if as.Roles[role] == "" {
			t.Fatalf("missing role %s", role)
		}
	}
	for _, root := range []string{"Elephant", "Giraffe", "WhiteMonkeys", "WhiteMonkey", "Gorilla"} {
		if kart.NewTemplate(as, root) == nil {
			t.Fatalf("missing template %s", root)
		}
	}
}

func TestAnimalAcrobatControllersAndSounds(t *testing.T) {
	as := loadAnimalAssets(t)
	wantStates := map[string][]string{
		"Elephant":          {"ElephantIdle", "ElephantEar"},
		"GiraffeRoot":       {"GiraffeIdle", "GiraffeEar"},
		"Gorilla":           {"GorillaIdle", "GorillaMiss"},
		"WhiteMonkeysPivot": {"WhiteMonkeysIdle", "WhiteMonkeysSwing"},
		"FireHoop":          {"FireIdle", "FireClose"},
		"ConfettiPop":       {"PopIntro"},
		"PlayerMonkey":      {"PlayerIdle", "PlayerBop", "PlayerJump", "PlayerAir", "PlayerHang", "PlayerHanging", "PlayerLand"},
	}
	for ctrl, states := range wantStates {
		c, ok := as.Controllers[ctrl]
		if !ok {
			t.Fatalf("missing controller %s", ctrl)
		}
		for _, st := range states {
			cs, ok := c.States[st]
			if !ok {
				t.Errorf("controller %s missing state %s", ctrl, st)
				continue
			}
			if cs.Clip != "" && as.Anims[cs.Clip] == nil {
				t.Errorf("controller %s state %s references missing clip %s", ctrl, st, cs.Clip)
			}
		}
	}
	for _, snd := range []string{
		"start", "eek", "catch", "giraffeCatch", "release", "giraffeJump",
		"giraffeDrumroll", "giraffeCymbal", "turn", "land", "miss", "cracker",
		"applause", "common_nearMiss",
	} {
		if len(as.Sounds[snd]) == 0 {
			t.Fatalf("missing sound %s", snd)
		}
	}
}

func TestAnimalAcrobatObstacleComponents(t *testing.T) {
	as := loadAnimalAssets(t)
	comps := as.Extra.Components
	for _, root := range []string{"Elephant", "Giraffe", "WhiteMonkeys", "WhiteMonkey", "Gorilla"} {
		ob := obstacleComponent(comps, root)
		if ob.Path == "" {
			t.Fatalf("missing obstacle component for %s", root)
		}
		for _, key := range []string{"_rotateRoot", "_gripPoint", "_endPoint"} {
			if ob.Refs[key] == "" {
				t.Errorf("%s missing ref %s", root, key)
			}
		}
		if ob.Nums["_fullRotRange"] == 0 {
			t.Errorf("%s missing full rotation range", root)
		}
		if root != "Gorilla" {
			in := inputComponent(comps, root, root == "Giraffe")
			if in.Path == "" {
				t.Fatalf("missing input component for %s", root)
			}
			for _, key := range []string{"_monkey", "_holdParticle", "_sweatParticle", "_gripShadow", "_endShadow"} {
				if in.Refs[key] == "" {
					t.Errorf("%s missing input ref %s", root, key)
				}
			}
		}
	}
}

func TestAnimalAcrobatObstacleComponentDoesNotMatchInputFamily(t *testing.T) {
	comps := map[string]kmdata.Component{
		"obstacleInput0": {Path: "Elephant", Refs: map[string]string{"_monkey": "wrong"}},
		"obstacle0":      {Path: "Elephant", Refs: map[string]string{"_rotateRoot": "right"}},
	}
	if got := obstacleComponent(comps, "Elephant").Refs["_rotateRoot"]; got != "right" {
		t.Fatalf("obstacleComponent returned %q, want obstacle family component", got)
	}
}

func TestAnimalAcrobatBGTileManagerRefs(t *testing.T) {
	as := loadAnimalAssets(t)
	c := as.Extra.Components["bgTileManager0"]
	if c.Path == "" {
		t.Fatal("missing BGTileManager component")
	}
	first := c.Refs["_bgTileFirst"]
	second := c.Refs["_bgTileSecond"]
	if first == "" || second == "" {
		t.Fatalf("missing BGTileManager tile refs: first=%q second=%q", first, second)
	}
	ax, _ := nodePos(as, first)
	bx, _ := nodePos(as, second)
	if bx <= ax {
		t.Fatalf("bg tile distance must be positive: first=%v second=%v", ax, bx)
	}
}

func TestAnimalAcrobatBGTilePositionsMatchUnityRecycle(t *testing.T) {
	rt := bgTileRuntime{
		firstBase:    [2]float64{-2.3766, 2.57},
		secondBase:   [2]float64{55.77, 2.57},
		tileDistance: 55.77 - (-2.3766),
		ok:           true,
	}
	d := rt.tileDistance
	cases := []struct {
		name    string
		camera  float64
		firstX  float64
		secondX float64
	}{
		{name: "before first threshold", camera: d - 0.01, firstX: rt.firstBase[0], secondX: rt.secondBase[0]},
		{name: "first tile recycled", camera: d, firstX: rt.firstBase[0] + 2*d, secondX: rt.secondBase[0]},
		{name: "second tile recycled", camera: 2 * d, firstX: rt.firstBase[0] + 2*d, secondX: rt.secondBase[0] + 2*d},
		{name: "first tile recycled again", camera: 3 * d, firstX: rt.firstBase[0] + 4*d, secondX: rt.secondBase[0] + 2*d},
	}
	for _, tc := range cases {
		first, second := bgTilePositions(tc.camera, rt)
		if math.Abs(first[0]-tc.firstX) > 1e-6 || math.Abs(second[0]-tc.secondX) > 1e-6 {
			t.Fatalf("%s: got first.x=%v second.x=%v, want %v %v", tc.name, first[0], second[0], tc.firstX, tc.secondX)
		}
		if first[1] != rt.firstBase[1] || second[1] != rt.secondBase[1] {
			t.Fatalf("%s: y changed: first.y=%v second.y=%v", tc.name, first[1], second[1])
		}
	}
}

func TestAnimalAcrobatCameraTargetFollowsUnityAnimalPhases(t *testing.T) {
	elephant := &acrobatObstacle{
		kind: kindElephant,
		beat: 10, length: 4,
		spec:  obstacleSpec{holdPadding: 1, fullRotRange: 160, ease: 0},
		gripY: -2,
	}
	gorilla := &acrobatObstacle{
		kind: kindGorilla,
		beat: 14, length: 4,
		spec: obstacleSpec{},
	}
	m := &Module{
		gameNums: map[string]float64{
			"_jumpStartCameraDistance": 3,
			"_jumpDistance":            6.5,
		},
		animals: []*acrobatObstacle{elephant, gorilla},
	}

	if _, ok := m.cameraTargetAt(8.99); ok {
		t.Fatal("camera should wait until first animal pre-travel window")
	}
	assertCameraTarget(t, m, 9, 0, 0, 0)
	assertCameraTarget(t, m, 9.5, 1.5, 0, 0)

	rotDist := cameraRotationDistance(elephant)
	assertCameraTarget(t, m, 11.5, 3+rotDist*0.5, -2, 0)
	assertCameraTarget(t, m, 14, 3+rotDist+6.5, 0, 0)
}

func TestAnimalAcrobatCameraTargetGiraffeZoom(t *testing.T) {
	giraffe := &acrobatObstacle{
		kind: kindGiraffe,
		beat: 10, length: 8,
		spec:  obstacleSpec{holdPadding: 2, fullRotRange: 160, ease: 0},
		gripY: -11.27,
	}
	gorilla := &acrobatObstacle{kind: kindGorilla, beat: 18, length: 4}
	m := &Module{
		gameNums: map[string]float64{
			"_jumpStartCameraDistance": 3,
			"_jumpDistanceGiraffe":     32,
			"_giraffeCameraZoom":       6.6,
		},
		animals: []*acrobatObstacle{giraffe, gorilla},
	}

	_, ok := m.cameraTargetAt(14.4)
	if !ok {
		t.Fatal("expected giraffe release camera target")
	}
	target, _ := m.cameraTargetAt(14.4)
	wantZ := -6.6 * 0.5 * 0.5
	if math.Abs(target.z-wantZ) > 1e-6 {
		t.Fatalf("giraffe zoom z=%v, want %v", target.z, wantZ)
	}

	m.monkeyMissed = true
	target, _ = m.cameraTargetAt(14.4)
	if target.z != 0 {
		t.Fatalf("giraffe miss should suppress zoom, got z=%v", target.z)
	}
}

func assertCameraTarget(t *testing.T, m *Module, beat, wantX, wantY, wantZ float64) {
	t.Helper()
	got, ok := m.cameraTargetAt(beat)
	if !ok {
		t.Fatalf("beat %v: missing camera target", beat)
	}
	if math.Abs(got.x-wantX) > 1e-6 || math.Abs(got.y-wantY) > 1e-6 || math.Abs(got.z-wantZ) > 1e-6 {
		t.Fatalf("beat %v: camera target=(%v,%v,%v), want (%v,%v,%v)", beat, got.x, got.y, got.z, wantX, wantY, wantZ)
	}
}
