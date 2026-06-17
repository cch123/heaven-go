package dressyourbest

import (
	"path"
	"path/filepath"
	"testing"

	"hsdemo/engine"
	"hsdemo/kart"
	"hsdemo/kmdata"
)

func loadAuditAssets(t *testing.T) *kart.Assets {
	t.Helper()
	as, err := kart.Load(filepath.Join("..", "..", "assets", "dressYourBest"), engine.SampleRate)
	if err != nil {
		t.Fatal(err)
	}
	return as
}

func TestBindingsComponentsAndSounds(t *testing.T) {
	as := loadAuditAssets(t)
	wantRoles := map[string]string{
		"girlAnim":         "Girl",
		"monkeyAnim":       "Monkey",
		"sewingAnim":       "SewingMachine",
		"reactionAnim":     "Reaction",
		"cameoAnim":        "Background/Cameo",
		"newBG":            "Background",
		"bgSpriteRenderer": "Old BG Placeholder",
		"lightRenderer":    "SewingMachine/Light",
	}
	for k, want := range wantRoles {
		if got := as.Roles[k]; got != want {
			t.Fatalf("role %s = %q, want %q", k, got, want)
		}
	}
	game := as.Extra.Components["game"]
	if game.Refs["lightMaterialTemplate"] != "LightMat" {
		t.Fatalf("light material = %q", game.Refs["lightMaterialTemplate"])
	}
	if got := len(game.Lists["lightStates"]); got != 4 {
		t.Fatalf("lightStates = %d, want 4", got)
	}
	if game.Lists["lightStates"][2].Nums["inside.g"] != 1 {
		t.Fatalf("correct light state not dumped as color pair: %#v", game.Lists["lightStates"][2].Nums)
	}
	for _, snd := range []string{
		"monkey_call_1", "monkey_call_2", "pass_turn", "hit_1", "hit_2",
		"whiff_hit", "correct", "incorrect", "barely_hit",
		"machine_whir_start", "machine_whir_loop", "machine_whir_end", "machine_whir_full",
		"common_nearMiss",
	} {
		if as.Sounds[snd] == nil {
			t.Fatalf("missing sound %s", snd)
		}
	}
}

func TestLightPaletteIsRendererScoped(t *testing.T) {
	as := &kart.Assets{
		Rig: kmdata.Rig{Nodes: []kmdata.Node{
			{Name: "Root", Path: "", Parent: -1, Scale: [2]float64{1, 1}},
			{Name: "Light", Path: "SewingMachine/Light", Parent: 0, Scale: [2]float64{1, 1}, Mapped: true, Mat: "LightMat"},
			{Name: "Head", Path: "Monkey/Head", Parent: 0, Scale: [2]float64{1, 1}, Mapped: true, Mat: "LightMat"},
			{Name: "Head", Path: "Girl/HeadAndHair/Head", Parent: 0, Scale: [2]float64{1, 1}, Mapped: true, Mat: "LightMat"},
		}},
	}
	scene := kart.NewScene(as)
	m := &Module{
		ctx:       &engine.Ctx{Scene: scene},
		lightPath: "SewingMachine/Light",
		lightMat:  "LightMat",
		lightStates: []lightPair{
			{inside: [4]float64{1, 1, 1, 1}, outside: [4]float64{1, 1, 1, 1}},
			{inside: [4]float64{0.25, 0.5, 0.75, 1}, outside: [4]float64{0.9, 0.8, 0.7, 1}},
		},
	}
	m.setLight(1)

	got, ok := scene.NodePaletteForTest("SewingMachine/Light")
	if !ok {
		t.Fatal("missing SewingMachine/Light")
	}
	if got.Alpha != m.lightStates[1].inside || got.Fill != m.lightStates[1].outside {
		t.Fatalf("light palette = %#v, want renderer-scoped state %#v", got, m.lightStates[1])
	}
	def := kart.DefaultPalette()
	for _, path := range []string{"Monkey/Head", "Girl/HeadAndHair/Head"} {
		got, ok := scene.NodePaletteForTest(path)
		if !ok {
			t.Fatalf("missing %s", path)
		}
		if got != def {
			t.Fatalf("%s inherited light palette %#v; light material must stay renderer-scoped", path, got)
		}
	}
}

func TestAnimationClipsControllersAndPaths(t *testing.T) {
	as := loadAuditAssets(t)
	for _, clip := range []string{
		"BG/CameoIdle", "BG/CameoWalk1", "BG/CameoWalk2", "BG/CameoWalk3", "BG/CameoWalk4", "BG/CameoWalk5",
		"Girl/GirlBop", "Faces/GirlDefault", "Faces/GirlLooking", "Faces/GirlCorrect", "Faces/GirlIncorrect",
		"Monkey/MonkeyBop", "Monkey/MonkeyCall", "Monkey/MonkeyStartCalling",
		"Faces/MonkeyCallFace", "Faces/MonkeyDefault", "Faces/MonkeyCorrect", "Faces/MonkeyIncorrect",
		"Reaction/ReactionCorrect", "Reaction/ReactionIncorrect",
		"SewingMachine/Sew", "SewingMachine/SewNot",
	} {
		if as.Anims[clip] == nil {
			t.Fatalf("missing clip %s", clip)
		}
	}
	wantAnimators := map[string]string{
		"Background/Cameo": "Cameo",
		"Girl":             "GirlAnim",
		"Monkey":           "MonkeyAnim",
		"Reaction":         "ReactionAnim",
		"SewingMachine":    "SewingMachineAnim",
	}
	for root, ctrl := range wantAnimators {
		if got := as.Animators[root]; got != ctrl {
			t.Fatalf("animator %s = %q, want %q", root, got, ctrl)
		}
	}
	for ctrl, states := range map[string][]string{
		"Cameo":             {"CameoIdle", "CameoWalk1", "CameoWalk2", "CameoWalk3", "CameoWalk4", "CameoWalk5"},
		"GirlAnim":          {"Bop", "Idle", "Looking", "Happy", "Sad"},
		"MonkeyAnim":        {"Bop", "Call", "CallFace", "Idle", "Happy", "Sad"},
		"ReactionAnim":      {"Idle", "Correct", "Incorrect"},
		"SewingMachineAnim": {"Idle", "Hit", "Miss"},
	} {
		c := as.Controllers[ctrl]
		if c.States == nil {
			t.Fatalf("missing controller %s", ctrl)
		}
		for _, st := range states {
			if _, ok := c.States[st]; !ok {
				t.Fatalf("controller %s missing state %s", ctrl, st)
			}
		}
	}

	for clip, root := range map[string]string{
		"BG/CameoIdle": "Background/Cameo", "BG/CameoWalk1": "Background/Cameo", "BG/CameoWalk2": "Background/Cameo",
		"BG/CameoWalk3": "Background/Cameo", "BG/CameoWalk4": "Background/Cameo", "BG/CameoWalk5": "Background/Cameo",
		"Girl/GirlBop": "Girl", "Faces/GirlDefault": "Girl", "Faces/GirlLooking": "Girl",
		"Faces/GirlCorrect": "Girl", "Faces/GirlIncorrect": "Girl",
		"Monkey/MonkeyBop": "Monkey", "Monkey/MonkeyCall": "Monkey",
		"Faces/MonkeyCallFace": "Monkey", "Faces/MonkeyDefault": "Monkey",
		"Faces/MonkeyCorrect": "Monkey", "Faces/MonkeyIncorrect": "Monkey",
		"Reaction/ReactionCorrect": "Reaction", "Reaction/ReactionIncorrect": "Reaction",
		"SewingMachine/Sew": "SewingMachine", "SewingMachine/SewNot": "SewingMachine",
	} {
		assertClipPaths(t, as, clip, root)
	}
	if !emptyClip(as.Anims["Monkey/MonkeyStartCalling"]) {
		t.Fatalf("MonkeyStartCalling unexpectedly gained curves; runtime must start driving it")
	}
}

func TestRuntimeHelpersMatchScriptSemantics(t *testing.T) {
	m := &Module{
		calls: []callEvt{{beat: 4, sfx: 0}, {beat: 6, sfx: 1}, {beat: 7, sfx: 0}},
	}
	ev := intervalEvt{beat: 4, length: 3}
	got := m.callsIn(ev)
	if len(got) != 3 || got[1].sound() != "monkey_call_2" {
		t.Fatalf("callsIn = %#v", got)
	}
	if end := m.intervalEnd(ev); end != 8 {
		t.Fatalf("intervalEnd = %v, want extension to 8", end)
	}
	if girlFaceClip(faceIdle) != "Faces/GirlDefault" || monkeyFaceClip(faceSad) != "Faces/MonkeyIncorrect" {
		t.Fatalf("face clip mapping changed")
	}
	if cameoState(0) != "CameoWalk1" || cameoState(5) != "CameoWalk5" || cameoState(9) != "CameoWalk5" {
		t.Fatalf("cameo clamping changed")
	}
}

func assertClipPaths(t *testing.T, as *kart.Assets, clip, root string) {
	t.Helper()
	anim := as.Anims[clip]
	if anim == nil {
		t.Fatalf("missing clip %s", clip)
	}
	check := func(curvePath string) {
		full := root
		if curvePath != "" {
			full = path.Join(root, curvePath)
		}
		if _, ok := as.NodeIndex(full); !ok {
			t.Fatalf("%s curve path %q resolved to missing node %q", clip, curvePath, full)
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

func emptyClip(anim *kmdata.Anim) bool {
	if anim == nil {
		return false
	}
	return len(anim.Pos) == 0 && len(anim.Scale) == 0 && len(anim.Euler) == 0 &&
		len(anim.Sprites) == 0 && len(anim.Floats) == 0
}
