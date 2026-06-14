package dogninja

import (
	"testing"

	"hsdemo/kart"
)

func TestDogNinjaExtractedAssets(t *testing.T) {
	as, err := kart.Load("../../assets/dogNinja", 44100)
	if err != nil {
		t.Skipf("assets not extracted: %v", err)
	}
	if err := as.ApplyTexts(); err != nil {
		t.Fatalf("ApplyTexts: %v", err)
	}
	for _, role := range []string{"DogAnim", "BirdAnim", "ObjectBase", "CutEverythingText"} {
		if as.Roles[role] == "" {
			t.Errorf("role %s 未解析", role)
		}
	}
	if as.Roles["DogAnim"] != "Dog" || as.Roles["BirdAnim"] != "BirdFull" {
		t.Fatalf("anim roles = %#v", as.Roles)
	}
	for _, comp := range []string{"game", "throwObject", "halves0", "halves1"} {
		if _, ok := as.Extra.Components[comp]; !ok {
			t.Errorf("缺组件 dump %s", comp)
		}
	}
	for _, comp := range []string{"throwObject", "halves0", "halves1"} {
		if as.Extra.Components[comp].Path == "" {
			t.Errorf("组件 %s path 为空", comp)
		}
	}
	for _, key := range []string{
		"throwObject.LeftCurve", "throwObject.RightCurve", "throwObject.BarelyLeftCurve", "throwObject.BarelyRightCurve",
		"halves0.fallLeftCurve", "halves0.fallRightCurve", "halves1.fallLeftCurve", "halves1.fallRightCurve",
	} {
		if got := len(as.Extra.Curves[key].Points); got != 3 {
			t.Errorf("%s points = %d, want 3", key, got)
		}
	}
	game := as.Extra.Components["game"]
	full := appendObjectTypePlaceholder(game.RefArrays["ObjectTypes"], game.SpriteArrays["ObjectTypes"])
	if len(full) < typeTacoBell+1 || full[typeApple] != "Apple_Full" || full[typeTacoBell] != "TacoBell_Full" {
		t.Fatalf("ObjectTypes alignment broken: %#v", full)
	}
	throw := as.Extra.Components["throwObject"]
	if len(throw.SpriteArrays["objectLeftHalves"]) < typeTacoBell || throw.SpriteArrays["objectLeftHalves"][typeApple-1] != "Apple_Left" {
		t.Fatalf("left halves alignment broken: %#v", throw.SpriteArrays["objectLeftHalves"])
	}
	for _, ctrl := range []string{"DogAnim", "BirdAnim", "LeftHalfAnim", "RightHalfAnim"} {
		if _, ok := as.Controllers[ctrl]; !ok {
			t.Errorf("缺 controller %s", ctrl)
		}
	}
	for _, clip := range []string{
		"Dog/Idle", "Dog/Bop", "Dog/Prepare", "Dog/UnPrepare",
		"Dog/SliceLeft", "Dog/SliceRight", "Dog/SliceBoth",
		"Dog/BarelyLeft", "Dog/BarelyRight", "Dog/BarelyGlobal",
		"Dog/WhiffLeft", "Dog/WhiffRight", "Bird/FlyIn", "Bird/FlyOut",
		"Halves/LeftHalfFallLeft", "Halves/LeftHalfFallRight", "Halves/RightHalfFallLeft", "Halves/RightHalfFallRight",
	} {
		if _, ok := as.Anims[clip]; !ok {
			t.Errorf("缺动画剪辑 %s", clip)
		}
	}
	for _, snd := range []string{"fruit1", "fruit2", "bone1", "bone2", "pan1", "pan2", "tire1", "tire2", "barely", "bird_flap", "here", "we", "go", "whiff", "common_miss"} {
		if _, ok := as.Sounds[snd]; !ok {
			t.Errorf("缺音效 %s", snd)
		}
	}
	if len(as.Texts) != 1 || as.Texts[0].Path != "BirdFull/CutEverythingSign/textstuffs" {
		t.Fatalf("texts = %#v", as.Texts)
	}
}

func TestDogNinjaSfxAndRandomObjectSemantics(t *testing.T) {
	for typ := typeApple; typ <= typePotato; typ++ {
		if got := sfxFromType(typ); got != "fruit" {
			t.Fatalf("fruit type %d sfx = %q", typ, got)
		}
	}
	if sfxFromType(typeBone) != "bone" || sfxFromType(typeAirBatter) != "AirBatter" || sfxFromType(typeTacoBell) != "tacobell" {
		t.Fatalf("named object sfx mapping changed")
	}
	a := randomObjectType(10.2, 7, false)
	b := randomObjectType(10.2, 7, false)
	c := randomObjectType(10.2, 7, true)
	if a != b {
		t.Fatalf("randomObjectType not deterministic: %d vs %d", a, b)
	}
	if a < typeApple || a > typePotato || c < typeApple || c > typePotato {
		t.Fatalf("random object outside fruit range: %d %d", a, c)
	}
}
