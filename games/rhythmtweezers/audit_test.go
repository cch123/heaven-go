package rhythmtweezers

import (
	"testing"

	"hsdemo/kart"
)

func TestRhythmTweezersAssetsCoverScriptedStates(t *testing.T) {
	as, err := kart.Load("../../assets/rhythmTweezers", 44100)
	if err != nil {
		t.Skipf("assets not extracted: %v", err)
	}
	for _, root := range []string{
		"VegetableHolder/Vegetable/HairPrefabs/HairHolder",
		"VegetableHolder/Vegetable/HairPrefabs/LongHairHolder",
		"TweezerHolder",
		"noPeek_2",
	} {
		if tmpl := kart.NewTemplate(as, root); tmpl == nil {
			t.Fatalf("missing prefab template %q", root)
		}
	}
	for _, clip := range []string{
		"Hairs/SmallAppear",
		"Hairs/LongAppear",
		"Hairs/LoopPull",
		"Tweezers/Tweezers_Idle",
		"Tweezers/Tweezers_LongPluck",
		"Tweezers/Tweezers_Pluck",
		"Tweezers/Tweezers_Pluck_Fail",
		"Tweezers/Tweezers_Pluck_Success",
		"Vegetable/Idle",
		"Vegetable/Blink",
		"Vegetable/HopFinal",
		"Vegetable/HopFinalLightBulb",
		"Animations/NoPeekRise",
		"Animations/NoPeekLower",
	} {
		if as.Anims[clip] == nil {
			t.Fatalf("missing animation clip %q", clip)
		}
	}
	for _, snd := range []string{
		"shortAppear", "longAppear", "barely", "register",
		"longPull1", "longPullEnd", "click1", "shortPluck1",
	} {
		if len(as.Sounds[snd]) == 0 {
			t.Fatalf("missing sound %q", snd)
		}
	}
}

func TestHairRotationMatchesIntervalArc(t *testing.T) {
	if got := hairRotation(0, 4) * 180 / 3.141592653589793; got != -58 {
		t.Fatalf("start rotation = %v, want -58", got)
	}
	if got := hairRotation(3, 4) * 180 / 3.141592653589793; got != 58 {
		t.Fatalf("end rotation = %v, want 58", got)
	}
}

func TestHairTemplatesSplitShortAndLongStates(t *testing.T) {
	as, err := kart.Load("../../assets/rhythmTweezers", 44100)
	if err != nil {
		t.Skipf("assets not extracted: %v", err)
	}
	shortT := kart.NewTemplate(as, "VegetableHolder/Vegetable/HairPrefabs/HairHolder")
	longT := kart.NewTemplate(as, "VegetableHolder/Vegetable/HairPrefabs/LongHairHolder")
	if shortT == nil || longT == nil {
		t.Fatal("missing short or long hair template")
	}
	if shortT.NewInstance().CurrentState("") != "" || longT.NewInstance().CurrentState("") != "" {
		t.Fatal("new hair instances should not auto-play controller states")
	}
	if as.Anims["Hairs/SmallAppear"].Sprites["Hair"] == nil {
		t.Fatal("SmallAppear should target short HairHolder/Hair")
	}
	if as.Anims["Hairs/LongAppear"].Sprites["LongHairHolder/Hair"] == nil {
		t.Fatal("LongAppear should target nested long-hair prefab")
	}
	if as.Anims["Hairs/LoopPull"].Sprites["LongHairHolder/Loop"] == nil {
		t.Fatal("LoopPull should target nested long-hair loop")
	}
}
