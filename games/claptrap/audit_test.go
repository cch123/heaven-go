package claptrap

import (
	"testing"

	"hsdemo/kart"
)

func TestClapTrapExtractedAssets(t *testing.T) {
	as, err := kart.Load("../../assets/clapTrap", 44100)
	if err != nil {
		t.Skipf("assets not extracted: %v", err)
	}
	for _, role := range []string{
		"Background", "bg", "stageLeft", "stageRight", "stageLeftRim", "stageRightRim",
		"spotlight", "doll", "dollHead", "dollArms", "dollBody", "clapEffect",
		"swordObj", "shadowHead", "shadowLeftArm", "shadowLeftGlove",
		"shadowLeftGloveRim", "shadowRightArm", "shadowRightGlove", "shadowRightGloveRim",
	} {
		if as.Roles[role] == "" {
			t.Errorf("role %s 未解析", role)
		}
	}
	for _, ctrl := range []string{"clapTrapDoll", "head", "arms", "body", "clapEffect", "sword"} {
		if _, ok := as.Controllers[ctrl]; !ok {
			t.Errorf("缺 controller %s", ctrl)
		}
	}
	for _, clip := range []string{
		"Animations/ArmsHit", "Animations/ArmsMiss", "Animations/ArmsWhiff",
		"Animations/BodyIdle", "Animations/BodyIdleLit", "Animations/ClapEffect",
		"Animations/DollHit", "Animations/DollMiss", "Animations/HeadBarely",
		"Animations/HeadBreatheIn", "Animations/HeadBreatheOut", "Animations/HeadHit",
		"Animations/HeadIdle", "Animations/HeadMiss", "Animations/HeadTalk",
		"Animations/swordHandHit", "Animations/swordHandMiss",
	} {
		if _, ok := as.Anims[clip]; !ok {
			t.Errorf("缺动画剪辑 %s", clip)
		}
	}
	game := as.Extra.Components["game"]
	if game.Refs["spotlightMaterial"] != "SpotlightMaterial" {
		t.Fatalf("spotlight material = %q", game.Refs["spotlightMaterial"])
	}
	if kart.NewTemplate(as, as.Roles["swordObj"]) == nil {
		t.Fatal("swordObj template 未解析")
	}
	for _, snd := range []string{
		"donk", "whiff", "clap", "barely1", "barely2",
		"goodClap1", "goodClap2", "goodClap3", "goodClap4",
		"clapAce", "clapGood", "miss", "deepInhale", "deepExhale1", "deepExhale2",
	} {
		if _, ok := as.Sounds[snd]; !ok {
			t.Errorf("缺音效 %s", snd)
		}
	}
}

func TestClapTargetMatchesUnityCueLength(t *testing.T) {
	if got, want := clapTarget(8, 1.5), 14.0; got != want {
		t.Fatalf("clapTarget(8, 1.5) = %v, want %v", got, want)
	}
}

func TestClapTypeNamesIncludeUnusedExtractedSwordClips(t *testing.T) {
	if got := clapTypeName(clapTypeHand); got != "Hand" {
		t.Fatalf("hand type = %q", got)
	}
	if got := clapTypeName(clapTypePaw); got != "Paw" {
		t.Fatalf("paw type = %q", got)
	}
	if got := clapTypeName(clapTypeGreenOnion); got != "GreenOnion" {
		t.Fatalf("green onion type = %q", got)
	}
}
