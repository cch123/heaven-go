package bluebirds

import (
	"path/filepath"
	"testing"

	"hsdemo/engine"
	"hsdemo/kart"
)

func loadAuditAssets(t *testing.T) *kart.Assets {
	t.Helper()
	as, err := kart.Load(filepath.Join("..", "..", "assets", "blueBirds"), engine.SampleRate)
	if err != nil {
		t.Fatal(err)
	}
	if err := as.ApplyTexts(); err != nil {
		t.Fatalf("ApplyTexts: %v", err)
	}
	return as
}

func TestBindingsComponentsTextAndSounds(t *testing.T) {
	as := loadAuditAssets(t)
	wantRoles := map[string]string{
		"captainAnim":       "CaptainTransform/Captain/BirdHolder/Bird",
		"captainHolderAnim": "CaptainTransform/Captain/BirdHolder",
		"bird1Anim":         "Birds/CPUBirdLeft/BlueBird",
		"bird2Anim":         "Birds/CPUBirdMiddle/BlueBird",
		"bird3Anim":         "Birds/PlayerBird/BlueBird",
		"effect1Anim":       "Birds/CPUBirdLeft/BlueBird/Effect",
		"effect2Anim":       "Birds/CPUBirdMiddle/BlueBird/Effect",
		"effect3Anim":       "Birds/PlayerBird/BlueBird/Effect",
		"memoryAnim":        "Story/image",
		"memorySprite":      "Story/image",
		"finText":           "text",
		"CaptainTransform":  "CaptainTransform",
		"BirdTransform":     "Birds",
	}
	for k, want := range wantRoles {
		if got := as.Roles[k]; got != want {
			t.Fatalf("role %s = %q, want %q", k, got, want)
		}
	}

	game := as.Extra.Components["game"]
	if game.Refs["gradientMat"] != "gradient" {
		t.Fatalf("gradient material ref = %q", game.Refs["gradientMat"])
	}
	if got := game.SpriteArrays["memoryImage"]; len(got) != 6 || got[0] != "story_0" || got[5] != "story_5" {
		t.Fatalf("memoryImage = %#v", got)
	}
	if len(as.Texts) != 1 || as.Texts[0].Path != "text" || as.Texts[0].Text != "Fin." {
		t.Fatalf("texts = %#v", as.Texts)
	}
	for _, snd := range []string{
		"peck", "ur", "beak", "peck1", "stretch", "out", "your", "neck",
		"flap", "tap", "hold", "release", "miss", "common_miss",
	} {
		if as.Sounds[snd] == nil {
			t.Fatalf("missing sound %s", snd)
		}
	}
}

func TestAllAnimationClipsControllersAndAnimators(t *testing.T) {
	as := loadAuditAssets(t)
	for _, clip := range []string{
		"BlueBird/Beat", "BlueBird/Miss", "BlueBird/Pick", "BlueBird/PickRelease",
		"BlueBird/PickSuccess", "BlueBird/PickSuccessAlt", "BlueBird/PickSuccessRelease",
		"BlueBird/PrePick01", "BlueBird/PrePick02", "BlueBird/PreShout01",
		"BlueBird/PreShout02", "BlueBird/Shout", "BlueBird/Tame", "BlueBird/TameLoop",
		"BlueBird/TameSuccess", "BlueBird/idle",
		"Captain/Angry", "Captain/Beat", "Captain/Smile", "Captain/Talk", "Captain/idle",
		"Captain2/MoveIn", "Captain2/MoveOut", "Captain2/MoveOutInstant", "Captain2/idle",
		"Effects/Attack", "Effects/Miss", "Effects/idle",
		"Memory/fadeIn", "Memory/fadeOut", "Memory/idle",
	} {
		if as.Anims[clip] == nil {
			t.Fatalf("missing clip %s", clip)
		}
	}
	wantDefaults := map[string]string{
		"Blue Bird":         "Beat",
		"Captain Blue Bird": "idle",
		"Captain Holder":    "idle",
		"Effect":            "idle",
		"image":             "idle",
	}
	for ctrl, want := range wantDefaults {
		if as.Controllers[ctrl].Default != want {
			t.Fatalf("controller %s default = %q, want %q", ctrl, as.Controllers[ctrl].Default, want)
		}
	}
	for _, st := range []string{
		"Beat", "Miss", "Pick", "PickRelease", "PickSuccess", "PickSuccessAlt",
		"PickSuccessRelease", "PrePick01", "PrePick02", "PreShout01", "PreShout02",
		"Shout", "Tame", "TameLoop", "TameSuccess", "idle",
	} {
		if _, ok := as.Controllers["Blue Bird"].States[st]; !ok {
			t.Fatalf("Blue Bird controller missing state %s", st)
		}
	}
	for _, st := range []string{"Attack", "Miss"} {
		trans := as.Controllers["Effect"].States[st].Transitions
		if len(trans) != 1 || trans[0].Dst != "" {
			t.Fatalf("Effect %s transition = %#v, want Unity Exit transition", st, trans)
		}
	}
	wantAnimators := map[string]string{
		"Birds/CPUBirdLeft/BlueBird":               "Blue Bird",
		"Birds/CPUBirdMiddle/BlueBird":             "Blue Bird",
		"Birds/PlayerBird/BlueBird":                "Blue Bird",
		"Birds/CPUBirdLeft/BlueBird/Effect":        "Effect",
		"Birds/CPUBirdMiddle/BlueBird/Effect":      "Effect",
		"Birds/PlayerBird/BlueBird/Effect":         "Effect",
		"CaptainTransform/Captain/BirdHolder":      "Captain Holder",
		"CaptainTransform/Captain/BirdHolder/Bird": "Captain Blue Bird",
		"Story/image": "image",
	}
	for path, want := range wantAnimators {
		if got := as.Animators[path]; got != want {
			t.Fatalf("animator %s = %q, want %q", path, got, want)
		}
	}
}

func TestRuntimeHelpersMatchScriptSemantics(t *testing.T) {
	m := &Module{bops: []bopEvt{{beat: 4, auto: true}, {beat: 8, auto: false}}}
	if m.autoBopAt(3.99) || !m.autoBopAt(4) || m.autoBopAt(8) {
		t.Fatalf("autoBopAt did not follow SetupBopRegion semantics")
	}
	mv := moveEvt{beat: 10, length: 4, startX: -2, endX: 2, ease: 0}
	if got := engine.Ease(mv.ease, 0, 1, 0.5); got != 0.5 {
		t.Fatalf("linear move midpoint = %v", got)
	}
}
