package drummerduel

import (
	"math"
	"testing"

	"hsdemo/kart"
)

func loadDrummerDuelAssets(t *testing.T) *kart.Assets {
	t.Helper()
	as, err := kart.Load("../../assets/drummerDuel", 44100)
	if err != nil {
		t.Skipf("assets not extracted: %v", err)
	}
	return as
}

func TestDrummerDuelBindings(t *testing.T) {
	as := loadDrummerDuelAssets(t)
	wantRoles := map[string]string{
		"referee":            "Referee",
		"taikoLeft":          "Taikos/TaikoL",
		"taikoRight":         "Taikos/TaikoR",
		"drummerLeft":        "Drummers/DrummerL",
		"drummerRight":       "Drummers/DrummerR",
		"refereeObj":         "Referee",
		"refereePlatformObj": "Platforms/Center",
		"cheerLeadersObj":    "Cheerleaders",
	}
	for role, want := range wantRoles {
		if got := as.Roles[role]; got != want {
			t.Fatalf("role %s = %q, want %q", role, got, want)
		}
	}

	wantLeft := []string{
		"Cheerleaders/Left/CheerleaderL",
		"Cheerleaders/Left/CheerleaderM",
		"Cheerleaders/Left/CheerleaderR",
	}
	wantRight := []string{
		"Cheerleaders/Right/CheerleaderL",
		"Cheerleaders/Right/CheerleaderM",
		"Cheerleaders/Right/CheerleaderR",
	}
	assertStringList(t, "cheerLeadersLeft", as.Extra.RefArrays["cheerLeadersLeft"], wantLeft)
	assertStringList(t, "cheerLeadersRight", as.Extra.RefArrays["cheerLeadersRight"], wantRight)
	assertStringList(t, "game.cheerLeadersLeft", as.Extra.Components["game"].RefArrays["cheerLeadersLeft"], wantLeft)
	assertStringList(t, "game.cheerLeadersRight", as.Extra.Components["game"].RefArrays["cheerLeadersRight"], wantRight)
}

func TestDrummerDuelGameComponent(t *testing.T) {
	as := loadDrummerDuelAssets(t)
	game := as.Extra.Components["game"]
	for field, want := range map[string]string{
		"referee":                  "Referee",
		"taikoLeft":                "Taikos/TaikoL",
		"taikoRight":               "Taikos/TaikoR",
		"drummerLeft":              "Drummers/DrummerL",
		"drummerRight":             "Drummers/DrummerR",
		"drummerLeftFaceMaterial":  "faceLeft",
		"drummerRightFaceMaterial": "faceRight",
	} {
		if got := game.Refs[field]; got != want {
			t.Fatalf("game ref %s = %q, want %q", field, got, want)
		}
	}
	for field, want := range map[string]float64{
		"cameraLeft":   7.538475,
		"cameraCenter": 0,
		"cameraRight":  -7.538475,
		"cameraLoc":    1,
		"isRight":      1,
	} {
		if got := game.Nums[field]; math.Abs(got-want) > 0.000001 {
			t.Fatalf("game num %s = %f, want %f", field, got, want)
		}
	}
}

func TestDrummerDuelControllersAndSounds(t *testing.T) {
	as := loadDrummerDuelAssets(t)
	wantStates := map[string][]string{
		"Cheerleader": {
			"Idle", "Bop", "Cheer1L", "Cheer1M", "Cheer1R",
			"Cheer2", "Cheer2La", "Cheer2Lb", "Cheer2Ra", "Cheer2Rb",
			"Just", "Miss",
		},
		"Drummer": {
			"Idle", "Bop", "ArmF", "ArmB", "ArmMadF", "ArmMadB",
			"Hit", "HitF", "HitB", "HitMadF", "HitMadB",
			"HitArmF", "HitArmB", "FaceIdle", "FaceAngry", "FaceWhiff",
		},
		"Referee": {
			"Idle", "Bop", "Prepare", "Left", "Right", "Finish",
			"HeadNormal", "HeadGood", "HeadBad", "Fail",
		},
		"drumAnim": {"Idle", "Hit", "Whiff"},
	}
	for ctrl, states := range wantStates {
		c, ok := as.Controllers[ctrl]
		if !ok {
			t.Fatalf("missing controller %s", ctrl)
		}
		for _, state := range states {
			cs, ok := c.States[state]
			if !ok {
				t.Errorf("controller %s missing state %s", ctrl, state)
				continue
			}
			if cs.Clip != "" && as.Anims[cs.Clip] == nil {
				t.Errorf("controller %s state %s references missing clip %s", ctrl, state, cs.Clip)
			}
		}
	}

	for path, ctrl := range map[string]string{
		"Referee":                         "Referee",
		"Taikos/TaikoL":                   "drumAnim",
		"Taikos/TaikoR":                   "drumAnim",
		"Drummers/DrummerL":               "Drummer",
		"Drummers/DrummerR":               "Drummer",
		"Cheerleaders/Left/CheerleaderL":  "Cheerleader",
		"Cheerleaders/Right/CheerleaderR": "Cheerleader",
	} {
		if got := as.Animators[path]; got != ctrl {
			t.Fatalf("animator %s = %q, want %q", path, got, ctrl)
		}
	}

	for _, snd := range []string{
		"drumLeftHit", "drumRightHit", "drumRightWhiff", "miss", "nearMiss",
		"passToLeft", "passToLeftBad", "passToRight", "angry", "grunt",
		"one", "two", "drummerDon", "drummerDo", "drummerKo",
	} {
		if len(as.Sounds[snd]) == 0 {
			t.Fatalf("missing sound %s", snd)
		}
	}
}

func TestDrummerDuelMappedMaterialsHavePalettes(t *testing.T) {
	as := loadDrummerDuelAssets(t)
	for _, n := range as.Rig.Nodes {
		if !n.Mapped || n.Mat == "" {
			continue
		}
		if _, ok := defaultPalettes[n.Mat]; !ok {
			t.Fatalf("mapped material %s on %s has no runtime palette", n.Mat, n.Path)
		}
	}
}

func TestDrummerDuelAutomaticHitVoices(t *testing.T) {
	if got := hitVoice(8, hitAuto, 10); got != "drummerDon" {
		t.Fatalf("on-beat isolated automatic voice = %s", got)
	}
	if got := hitVoice(8, hitAuto, 8.5); got != "drummerDo" {
		t.Fatalf("on-beat dense automatic voice = %s", got)
	}
	if got := hitVoice(8.5, hitAuto, 10); got != "drummerKo" {
		t.Fatalf("offbeat automatic voice = %s", got)
	}
}

func assertStringList(t *testing.T, name string, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("%s length = %d, want %d", name, len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("%s[%d] = %q, want %q", name, i, got[i], want[i])
		}
	}
}
