package samuraislicervl

import (
	"path"
	"testing"

	"hsdemo/engine"
	"hsdemo/kart"
)

func loadAuditAssets(t *testing.T) *kart.Assets {
	t.Helper()
	as, err := kart.Load("../../assets/samuraiSliceRvl", engine.SampleRate)
	if err != nil {
		t.Skipf("assets not extracted: %v", err)
	}
	return as
}

func TestExtractedSceneRolesTemplatesComponentsAndSounds(t *testing.T) {
	as := loadAuditAssets(t)
	for _, role := range []string{
		"SamuraiAnim", "fgHolder", "demonholder", "flashholder",
		"hordeSlicedPrefab", "smogEffectPrefab", "hordeDemonPrefab",
	} {
		if as.Roles[role] == "" {
			t.Fatalf("missing role %s", role)
		}
	}
	for _, root := range []string{
		"SteveHolder", "DemonHolder", "HordePrefab", "SlicedHorde", "Smog",
		"SmogParticlePrefab", "SmallDemonSlicedPrefab", "MediumDemonSliced",
		"BigDemonSliced", "HugeDemonSliced", "LightningHolder", "Flash",
	} {
		if _, ok := as.NodeIndex(root); !ok {
			t.Fatalf("missing node %s", root)
		}
		if kart.NewTemplate(as, root) == nil {
			t.Fatalf("missing template %s", root)
		}
	}
	game := as.Extra.Components["game"]
	if len(game.RefArrays["slicedDemonPrefabs"]) != 4 {
		t.Fatalf("sliced prefab refs = %d, want 4", len(game.RefArrays["slicedDemonPrefabs"]))
	}
	if len(game.Lists["hordeSpawnPositions"]) != 7 {
		t.Fatalf("horde spawn positions = %d, want 7", len(game.Lists["hordeSpawnPositions"]))
	}
	for _, curve := range []string{"spawnCurve", "missCurve", "walkCurve", "game.spawnCurve", "game.missCurve", "game.walkCurve"} {
		if len(as.Extra.Curves[curve].Points) == 0 {
			t.Fatalf("missing curve %s", curve)
		}
	}
	for _, root := range []string{"SmallDemonSlicedPrefab", "MediumDemonSliced", "BigDemonSliced", "HugeDemonSliced", "SlicedHorde"} {
		c := componentByPath(as, root)
		if c.Path == "" {
			t.Fatalf("missing sliced component for %s", root)
		}
		if c.Nums["waitTime"] != 0.08 {
			t.Fatalf("%s waitTime = %v, want 0.08", root, c.Nums["waitTime"])
		}
	}
	horde := componentByPath(as, "HordePrefab")
	if horde.Path == "" || horde.Nums["gravity"] != 30 || horde.Nums["rotationSpeed"] != -50 {
		t.Fatalf("horde prefab component not extracted correctly: %+v", horde.Nums)
	}
	for _, snd := range []string{
		"demon1_1", "demon1_2", "demon1_3", "demon4_1", "demon4_2", "demon4_3",
		"combo1", "combo2", "combo3", "combo4", "HIT1", "HIT2_A", "HIT2_B",
		"HIT6_START", "HIT6_ALT", "ROLLING_START", "ROLLING_ALL_HIT",
		"SWING1_A", "SWING2_C", "YARARE1", "YARARE2", "OSII",
		"THUNDER1", "THUNDER2", "THUNDER3", "CLOUD_PRERENDER", "slice",
	} {
		if as.Sounds[snd] == nil {
			t.Fatalf("missing sound %s", snd)
		}
	}
}

func TestControllersAndAnimationPaths(t *testing.T) {
	as := loadAuditAssets(t)
	for root, ctrl := range map[string]string{
		"SteveHolder":                         "SamuraiWii",
		"SteveHolder/WindmillHolder/Windmill": "Windmill",
		"DemonHolder/SDemon":                  "SmallDemon",
		"DemonHolder/MDemon":                  "MediumDemon",
		"DemonHolder/LDemon":                  "LargeDemon",
		"DemonHolder/XLDemon":                 "XLDemon",
		"HordePrefab":                         "HordeDemon",
		"LightningHolder/Lightning1":          "Lightning",
		"Flash":                               "SamuraiFlash",
	} {
		if got := as.Animators[root]; got != ctrl {
			t.Fatalf("animator %s = %q, want %q", root, got, ctrl)
		}
	}
	for ctrl, states := range map[string][]string{
		"SamuraiWii":   {"SamuraiIdle", "SamuraiBop", "SamuraiReady", "SamuraiSlash", "SamuraiSlash2", "SamuraiMiss", "SamuraiHordeStart", "SamuraiHordeLoop", "SamuraiHordeEnd", "SamuraiHordeMiss"},
		"SmallDemon":   {"SmallSummon", "SmallIdle", "SmallWaddle"},
		"MediumDemon":  {"MediumSummon", "MediumIdle", "MediumWaddle"},
		"LargeDemon":   {"LargeSummon", "LargeIdle", "LargeWaddle"},
		"XLDemon":      {"XLSummon", "XLIdle", "XLWaddle"},
		"HordeDemon":   {"HordeIdle", "HordeSummon", "HordeSummonFinal", "HordeRush", "HordeBite"},
		"Lightning":    {"LightningStrike1", "LightningStrike2", "LightningStrike3"},
		"SamuraiFlash": {"Flash"},
		// Unity's Windmill.controller state is named WindmillSpin2 even though
		// it binds the WindmillIdle clip; keep the state name literal.
		"Windmill": {"WindmillIdle", "WindmillSpin2"},
	} {
		c := as.Controllers[ctrl]
		if c.States == nil {
			t.Fatalf("missing controller %s", ctrl)
		}
		for _, st := range states {
			cs, ok := c.States[st]
			if !ok {
				t.Fatalf("controller %s missing state %s", ctrl, st)
			}
			if cs.Clip != "" && as.Anims[cs.Clip] == nil {
				t.Fatalf("controller %s state %s references missing clip %s", ctrl, st, cs.Clip)
			}
		}
	}
	for root, ctrl := range as.Animators {
		c := as.Controllers[ctrl]
		for state, st := range c.States {
			if st.Clip != "" {
				assertClipPaths(t, as, st.Clip, root, ctrl+"/"+state)
			}
		}
	}
}

func TestSmogAndTimingConstants(t *testing.T) {
	steps := smogSteps([2]float64{-30, -22.5})
	if len(steps) != 16 {
		t.Fatalf("smog steps = %d, want 16", len(steps))
	}
	if steps[2][1] != -22.5 || steps[8][0] != -30 {
		t.Fatalf("smog default replacement changed: step2=%v step8=%v", steps[2], steps[8])
	}
	if actionCombo != 3 || actionSlice != 0 {
		t.Fatalf("input action mapping changed")
	}
	if got := rollingEndBeat(24); got != 25 {
		t.Fatalf("rolling loop end beat = %v, want 25", got)
	}
	if rollingFadeSec != 0.1 {
		t.Fatalf("rolling fade = %v, want 0.1", rollingFadeSec)
	}
}

func assertClipPaths(t *testing.T, as *kart.Assets, clip, root, label string) {
	t.Helper()
	anim := as.Anims[clip]
	if anim == nil {
		t.Fatalf("%s missing clip %s", label, clip)
	}
	check := func(curvePath string) {
		full := root
		if curvePath != "" {
			full = path.Join(root, curvePath)
		}
		if _, ok := as.NodeIndex(full); !ok {
			t.Fatalf("%s clip %s curve path %q resolved to missing node %q", label, clip, curvePath, full)
		}
	}
	for p := range anim.Pos {
		check(p)
	}
	for p := range anim.Scale {
		check(p)
	}
	for p := range anim.Euler {
		check(p)
	}
	for p := range anim.Sprites {
		check(p)
	}
	for p := range anim.Floats {
		check(p)
	}
}
