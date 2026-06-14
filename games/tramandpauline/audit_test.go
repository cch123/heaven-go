package tramandpauline

import (
	"math"
	"testing"

	"hsdemo/kart"
)

func TestTramAndPaulineExtractedAssets(t *testing.T) {
	as, err := kart.Load("../../assets/tramAndPauline", 44100)
	if err != nil {
		t.Skipf("assets not extracted: %v", err)
	}
	for _, role := range []string{"tram", "pauline", "curtainAnim", "audienceAnim"} {
		if as.Roles[role] == "" {
			t.Errorf("role %s 未解析", role)
		}
	}
	if got := as.Roles["curtainAnim"]; got != "BgObjects/Curtain" {
		t.Fatalf("curtainAnim = %q", got)
	}

	pauline := as.Extra.Components["kid0"]
	tram := as.Extra.Components["kid1"]
	if pauline.Path != "PaulineObjects" || tram.Path != "TramObjects" {
		t.Fatalf("kid component order changed: pauline=%q tram=%q", pauline.Path, tram.Path)
	}
	for _, c := range []struct {
		name string
		kid  map[string]string
	}{
		{name: "pauline", kid: pauline.Refs},
		{name: "tram", kid: tram.Refs},
	} {
		for _, ref := range []string{"rootBody", "trampolineAnim", "bodyAnim", "transformParticle", "smokeParticle"} {
			if c.kid[ref] == "" {
				t.Errorf("%s 缺组件引用 %s", c.name, ref)
			}
		}
	}
	if pauline.Nums["jumpHeight"] != 5 || tram.Nums["jumpHeight"] != 5 {
		t.Fatalf("jumpHeight 未按 prefab 导出: pauline=%v tram=%v", pauline.Nums["jumpHeight"], tram.Nums["jumpHeight"])
	}

	for _, ctrl := range []string{"Curtain", "audience", "Tram", "Pauline", "Trampoline"} {
		if _, ok := as.Controllers[ctrl]; !ok {
			t.Errorf("缺 controller %s", ctrl)
		}
	}
	for _, clip := range []string{
		"Animations/Curtain", "Animations/Happy", "Animations/Idle",
		"Trampoline/Bounce", "Trampoline/Idle", "Trampoline/Jump", "Trampoline/Prepare",
		"Tram/BarelyIdle", "Tram/FoxIdle", "Tram/HumanIdle", "Tram/JumpBarely", "Tram/JumpFox",
		"Tram/JumpHuman", "Tram/Prepare", "Tram/PrepareBarely", "Tram/PrepareHuman",
		"Tram/TransformBarely", "Tram/TransformFox", "Tram/TransformHuman",
		"Pauline/BarelyIdle", "Pauline/FoxIdle", "Pauline/HumanIdle", "Pauline/JumpBarely",
		"Pauline/JumpFox", "Pauline/JumpHuman", "Pauline/Prepare", "Pauline/PrepareBarely",
		"Pauline/PrepareHuman", "Pauline/TransformBarely", "Pauline/TransformFox", "Pauline/TransformHuman",
	} {
		if _, ok := as.Anims[clip]; !ok {
			t.Errorf("缺动画剪辑 %s", clip)
		}
	}
	for _, sprite := range []string{"smoke1", "smoke2", "smoke3", "smoke4", "smoke5", "smoke6", "smoke7", "smoke8"} {
		if _, ok := as.Sheet.Sprites[sprite]; !ok {
			t.Errorf("缺烟雾切片 %s", sprite)
		}
	}
	for _, snd := range []string{"jumpL1", "jumpL2", "jumpR1", "jumpR2", "transformTram", "transformPauline", "common_miss"} {
		if _, ok := as.Sounds[snd]; !ok {
			t.Errorf("缺音效 %s", snd)
		}
	}
}

func TestJumpTargetMatchesScheduleInputTimer(t *testing.T) {
	if got := jumpTarget(12.5); got != 13.5 {
		t.Fatalf("jump target = %v, want beat+1", got)
	}
}

func TestAgbAnimalKidEaseEndpoints(t *testing.T) {
	if easeOutQuad(0, 5, 0) != 0 || easeOutQuad(0, 5, 1) != 5 {
		t.Fatalf("easeOutQuad endpoints changed")
	}
	if easeInQuad(5, 0, 0) != 5 || easeInQuad(5, 0, 1) != 0 {
		t.Fatalf("easeInQuad endpoints changed")
	}
	if got := easeOutQuad(0, 5, 0.5); math.Abs(got-3.75) > 1e-9 {
		t.Fatalf("easeOutQuad(0,5,.5) = %v, want 3.75", got)
	}
	if got := easeInQuad(5, 0, 0.5); math.Abs(got-3.75) > 1e-9 {
		t.Fatalf("easeInQuad(5,0,.5) = %v, want 3.75", got)
	}
}
