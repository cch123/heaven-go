package supersamuraislice

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"hsdemo/kmdata"
)

type superSamuraiSliceAssets struct {
	Rig         kmdata.Rig
	Roles       kmdata.Roles
	Extra       kmdata.Extra
	Anims       map[string]*kmdata.Anim
	Controllers map[string]kmdata.Controller
	Animators   kmdata.Animators
	Sounds      map[string]bool
}

func loadSuperSamuraiSliceAssets(t *testing.T) *superSamuraiSliceAssets {
	t.Helper()
	root := filepath.Join("..", "..", "assets", "superSamuraiSlice")
	as := &superSamuraiSliceAssets{
		Anims:  map[string]*kmdata.Anim{},
		Sounds: map[string]bool{},
	}
	readAssetJSON(t, filepath.Join(root, "scene.json"), &as.Rig)
	readAssetJSON(t, filepath.Join(root, "roles.json"), &as.Roles)
	readAssetJSON(t, filepath.Join(root, "extra.json"), &as.Extra)
	readAssetJSON(t, filepath.Join(root, "anims.json"), &as.Anims)
	readAssetJSON(t, filepath.Join(root, "controllers.json"), &as.Controllers)
	readAssetJSON(t, filepath.Join(root, "animators.json"), &as.Animators)

	soundRoot := filepath.Join(root, "sounds")
	if err := filepath.WalkDir(soundRoot, func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		ext := strings.ToLower(filepath.Ext(p))
		if ext != ".wav" && ext != ".ogg" {
			return nil
		}
		rel, err := filepath.Rel(soundRoot, p)
		if err != nil {
			return err
		}
		as.Sounds[strings.TrimSuffix(filepath.ToSlash(rel), ext)] = true
		return nil
	}); err != nil {
		t.Skipf("assets not extracted: %v", err)
	}
	return as
}

func readAssetJSON(t *testing.T, path string, dst any) {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Skipf("assets not extracted: %v", err)
	}
	if err := json.Unmarshal(raw, dst); err != nil {
		t.Fatalf("decode %s: %v", path, err)
	}
}

func TestSuperSamuraiSliceBindings(t *testing.T) {
	as := loadSuperSamuraiSliceAssets(t)
	wantRoles := map[string]string{
		"SamuraiShadow":   "SamuraiHolder/GameObject/Samurai/SamuraiHolder/Shadow",
		"Samurai":         "SamuraiHolder/GameObject/Samurai",
		"BG":              "BackgroundHolder/Holder/Main",
		"Water":           "BackgroundHolder/Holder/WaterHolder",
		"waterGO":         "BackgroundHolder/Holder/WaterHolder",
		"FG":              "BackgroundHolder/Holder/ForegroundHolder",
		"Fog":             "BackgroundHolder/Holder/Fog",
		"fogGO":           "BackgroundHolder/Holder/Fog",
		"Cloud":           "BackgroundHolder/Holder/CloudHolder",
		"flash":           "Flash",
		"Skateboard":      "BackgroundHolder/Holder/Skateboard/Skateboard",
		"Eagle":           "PlatformHolder/Eagle/Eagle",
		"BGAnim":          "BackgroundHolder/Holder",
		"CloudAnim":       "BackgroundHolder/Holder/CloudHolder",
		"Samurai1Anim":    "SamuraiHolder",
		"Skateboard1Anim": "BackgroundHolder/Holder/Skateboard",
		"Eagle1Anim":      "PlatformHolder/Eagle",
		"SamuraiAnim":     "SamuraiHolder/GameObject/Samurai",
		"SkateboardAnim":  "BackgroundHolder/Holder/Skateboard/Skateboard",
		"EagleAnim":       "PlatformHolder/Eagle/Eagle",
		"lightning":       "SamuraiHolder/GameObject/lightning/Main",
		"waterL":          "WaterParticle/WaterL",
		"waterR":          "WaterParticle/WaterR",
		"SmallDemon":      "demon",
		"MediumDemon":     "mediumDemon",
	}
	for role, want := range wantRoles {
		if got := as.Roles[role]; got != want {
			t.Fatalf("role %s = %q, want %q", role, got, want)
		}
	}
}

func TestSuperSamuraiSliceComponentsAndPaths(t *testing.T) {
	as := loadSuperSamuraiSliceAssets(t)
	game := requireComponent(t, as.Extra, "game")
	for field, want := range map[string]string{
		"SamuraiShadow":   "SamuraiHolder/GameObject/Samurai/SamuraiHolder/Shadow",
		"Samurai":         "SamuraiHolder/GameObject/Samurai",
		"BG":              "BackgroundHolder/Holder/Main",
		"Water":           "BackgroundHolder/Holder/WaterHolder",
		"waterGO":         "BackgroundHolder/Holder/WaterHolder",
		"FG":              "BackgroundHolder/Holder/ForegroundHolder",
		"Fog":             "BackgroundHolder/Holder/Fog",
		"fogGO":           "BackgroundHolder/Holder/Fog",
		"Cloud":           "BackgroundHolder/Holder/CloudHolder",
		"flash":           "Flash",
		"Skateboard":      "BackgroundHolder/Holder/Skateboard/Skateboard",
		"Eagle":           "PlatformHolder/Eagle/Eagle",
		"BGAnim":          "BackgroundHolder/Holder",
		"CloudAnim":       "BackgroundHolder/Holder/CloudHolder",
		"Samurai1Anim":    "SamuraiHolder",
		"Skateboard1Anim": "BackgroundHolder/Holder/Skateboard",
		"Eagle1Anim":      "PlatformHolder/Eagle",
		"SamuraiAnim":     "SamuraiHolder/GameObject/Samurai",
		"SkateboardAnim":  "BackgroundHolder/Holder/Skateboard/Skateboard",
		"EagleAnim":       "PlatformHolder/Eagle/Eagle",
		"lightning":       "SamuraiHolder/GameObject/lightning/Main",
		"waterL":          "WaterParticle/WaterL",
		"waterR":          "WaterParticle/WaterR",
		"SmallDemon":      "demon",
		"MediumDemon":     "mediumDemon",
	} {
		if got := game.Refs[field]; got != want {
			t.Fatalf("game ref %s = %q, want %q", field, got, want)
		}
	}

	// demonPaths drives every small and large demon trajectory. These values
	// come from SuperCurveObject in the Unity prefab and are easy to lose if the
	// extractor only handles flat arrays.
	paths := game.Lists["demonPaths"]
	if len(paths) != 6 {
		t.Fatalf("demonPaths len = %d, want 6", len(paths))
	}
	byName := map[string]kmdata.ComponentItem{}
	for _, p := range paths {
		byName[p.Strs["name"]] = p
	}
	for name, wantLen := range map[string]int{
		"rightUp": 3, "leftUp": 3, "rightDown": 3, "leftDown": 3,
		"CurveR": 16, "CurveL": 16,
	} {
		path, ok := byName[name]
		if !ok {
			t.Fatalf("missing demon path %s", name)
		}
		positions := path.Items["positions"]
		if len(positions) != wantLen {
			t.Fatalf("demon path %s positions len = %d, want %d", name, len(positions), wantLen)
		}
		for i, pos := range positions {
			if pos.Refs["target"] == "" {
				t.Fatalf("demon path %s position %d missing target", name, i)
			}
		}
	}
	assertNear(t, byName["rightUp"].Items["positions"][0].Nums["pos.x"], 7.765)
	assertNear(t, byName["rightUp"].Items["positions"][0].Nums["pos.y"], 2.91)
	assertNear(t, byName["rightUp"].Items["positions"][1].Nums["height"], 3)
	assertNear(t, byName["leftUp"].Items["positions"][1].Nums["height"], 2.7)
	assertNear(t, byName["rightDown"].Items["positions"][1].Nums["height"], 4.4)
	assertNear(t, byName["leftDown"].Items["positions"][1].Nums["height"], 5.5)
	assertNear(t, byName["CurveR"].Items["positions"][14].Nums["duration"], 1000)
}

func TestSuperSamuraiSliceDemonTemplates(t *testing.T) {
	as := loadSuperSamuraiSliceAssets(t)
	small := requireComponent(t, as.Extra, "demon0")
	if small.Path != "demon" {
		t.Fatalf("demon0 path = %q, want demon", small.Path)
	}
	for field, want := range map[string]string{
		"SmallDemonAnim": "demon",
		"windZone":       "demon/Holder/WindZone",
		"groundPlane":    "demon/Holder/GroundPlane",
		"Explode1":       "demon/Holder/blowup",
		"Explode2":       "demon/Holder/blowupAway",
		"Explode3":       "demon/Holder/blowupKick",
	} {
		if got := small.Refs[field]; got != want {
			t.Fatalf("demon0 ref %s = %q, want %q", field, got, want)
		}
	}

	medium := requireComponent(t, as.Extra, "demon1")
	if medium.Path != "mediumDemon" {
		t.Fatalf("demon1 path = %q, want mediumDemon", medium.Path)
	}
	for field, want := range map[string]string{
		"MediumDemonAnim": "mediumDemon",
		"windZone":        "mediumDemon/Holder/WindZone",
		"groundPlane":     "mediumDemon/Holder/GroundPlane",
		"Explode1":        "mediumDemon/Holder/blowup",
	} {
		if got := medium.Refs[field]; got != want {
			t.Fatalf("demon1 ref %s = %q, want %q", field, got, want)
		}
	}
}

func TestSuperSamuraiSliceControllersAndSounds(t *testing.T) {
	as := loadSuperSamuraiSliceAssets(t)
	wantStates := map[string][]string{
		"Samurai": {
			"Beat", "Slash00", "Slash01", "Slash02", "Kick", "Punch",
			"Guard", "Guard_Through", "Counter", "Counter_Through",
			"Slash_Through_L", "Slash_Through_R", "Run", "Pale",
			"Turn00", "Turn01", "idle",
		},
		"Holder": {
			"up", "down", "instant", "idle",
		},
		"SamuraiHolder": {
			"enter", "enter2", "exit", "instant", "idle",
		},
		"Animations/Platforms/Skateboard/Skateboard.controller": {
			"enter", "exit", "idle",
		},
		"Animations/Misc/Skateboard/Skateboard.controller": {
			"guard_FromL", "guard_FromR", "idle",
		},
		"Animations/Platforms/Eagle/Eagle.controller": {
			"enter", "exit", "idle",
		},
		"Animations/Misc/Eagle/Eagle.controller": {
			"fromL", "fromR", "idle",
		},
		"CloudHolder": {
			"fade", "idle",
		},
		"Flash": {
			"flash", "idle",
		},
		"Water": {
			"idle",
		},
		"Animations/Demon/Small/demon.controller": {
			"Break_H", "Break_K", "Break_Miss", "Break_P", "Break_V", "disappear", "idle",
		},
		"Animations/Demon/Medium/demon.controller": {
			"Attack", "Break", "GoAway", "disappear", "idle",
		},
		"Animations/Demon/Large/demon.controller": {
			"Attack", "Break", "GoAway", "Intro", "idle",
		},
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
		"BackgroundHolder/Holder":                       "Holder",
		"BackgroundHolder/Holder/CloudHolder":           "CloudHolder",
		"BackgroundHolder/Holder/Skateboard":            "Animations/Platforms/Skateboard/Skateboard.controller",
		"BackgroundHolder/Holder/Skateboard/Skateboard": "Animations/Misc/Skateboard/Skateboard.controller",
		"BackgroundHolder/Holder/WaterHolder/Water":     "Water",
		"BackgroundHolder/Holder/WaterHolder/Water (1)": "Water",
		"Flash":                            "Flash",
		"PlatformHolder/Eagle":             "Animations/Platforms/Eagle/Eagle.controller",
		"PlatformHolder/Eagle/Eagle":       "Animations/Misc/Eagle/Eagle.controller",
		"SamuraiHolder":                    "SamuraiHolder",
		"SamuraiHolder/GameObject/Samurai": "Samurai",
		"demon":                            "Animations/Demon/Small/demon.controller",
		"mediumDemon":                      "Animations/Demon/Medium/demon.controller",
	} {
		if got := as.Animators[path]; got != ctrl {
			t.Fatalf("animator %s = %q, want %q", path, got, ctrl)
		}
	}

	for _, snd := range []string{
		"SE_IAI_NEW_SWING1",
		"SE_IAI_NEW_BIRD_1",
		"SE_IAI_NEW_ENEMY_SMALL",
		"SE_IAI_NEW_ENEMY_SMALL_WATER",
		"SE_IAI_NEW_ENEMY_MID_1",
		"SE_IAI_NEW_ENEMY_MID_2",
		"SE_IAI_NEW_ENEMY_MID_3",
		"SE_IAI_NEW_ENEMY_DIE_SMALL",
		"SE_IAI_NEW_ENEMY_DIE_MID",
		"SE_IAI_NEW_HIT1",
		"SE_IAI_NEW_HIT2",
		"SE_IAI_NEW_HIT_KICK",
		"SE_IAI_NEW_HIT_MID",
		"SE_IAI_NEW_KIAI_BARRIER_BASE",
		"SE_IAI_NEW_KIAI_BARRIER_SHOCK",
		"SE_IAI_NEW_GUARD",
		"SE_IAI_NEW_OSII",
		"SE_IAI_NEW_YARARE1",
	} {
		if !as.Sounds[snd] {
			t.Fatalf("missing sound %s", snd)
		}
	}
}

func requireComponent(t *testing.T, extra kmdata.Extra, name string) kmdata.Component {
	t.Helper()
	c, ok := extra.Components[name]
	if !ok {
		t.Fatalf("missing component %s", name)
	}
	return c
}

func assertNear(t *testing.T, got, want float64) {
	t.Helper()
	if math.Abs(got-want) > 0.0001 {
		t.Fatalf("got %v, want %v", got, want)
	}
}
