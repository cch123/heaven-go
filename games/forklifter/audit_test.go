package forklifter

import (
	"testing"

	"hsdemo/kart"
	"hsdemo/riq"
)

func TestForkLifterExtractedAssets(t *testing.T) {
	as, err := kart.Load("../../assets/forkLifter", 44100)
	if err != nil {
		t.Skipf("assets not extracted: %v", err)
	}
	for _, role := range []string{
		"ForkLifterHand", "handAnim", "flickedObject", "peaPreview",
		"bg", "gradientFiller", "mmLines", "viewerCircle", "viewerCircleBg",
		"playerShadow", "handShadow", "forkSR",
	} {
		if as.Roles[role] == "" {
			t.Errorf("role %s 未解析", role)
		}
	}
	if as.Roles["handAnim"] != "Hand" || as.Roles["flickedObject"] != "Object" {
		t.Fatalf("核心 roles = %#v", as.Roles)
	}
	for _, comp := range []string{"game", "hand", "player"} {
		if _, ok := as.Extra.Components[comp]; !ok {
			t.Fatalf("缺组件 dump %s", comp)
		}
	}
	game := as.Extra.Components["game"]
	if got := game.SpriteArrays["peaSprites"]; len(got) != 4 || got[flickBurger] != "burger" {
		t.Fatalf("peaSprites = %#v", got)
	}
	if got := game.SpriteArrays["peaHitSprites"]; len(got) != 4 || got[flickTopBun] != "topbun_hit" {
		t.Fatalf("peaHitSprites = %#v", got)
	}
	if as.Extra.Components["hand"].SpriteArrays["fastSprites"][1] != "fast_green1" {
		t.Fatalf("burger fast sprite alignment broken: %#v", as.Extra.Components["hand"].SpriteArrays["fastSprites"])
	}
	player := as.Extra.Components["player"]
	for _, ref := range []string{"early", "perfect", "late"} {
		if player.Refs[ref] == "" {
			t.Errorf("player 缺 fork stack 引用 %s", ref)
		}
	}
	for _, sp := range []string{"hitFX", "hitFXG", "hitFXMiss", "hitFX2"} {
		if player.Sprites[sp] == "" {
			t.Errorf("player 缺 sprite %s", sp)
		}
	}
	for _, ctrl := range []string{"Hand", "Object", "Player"} {
		if _, ok := as.Controllers[ctrl]; !ok {
			t.Errorf("缺 controller %s", ctrl)
		}
	}
	for _, clip := range []string{
		"Animations/Flicked_Object", "Animations/Hand_Flick", "Animations/Hand_Prepare",
		"Animations/Player_Stab", "Animations/Player_Eat", "Animations/Player_Eat_Comeback",
	} {
		if _, ok := as.Anims[clip]; !ok {
			t.Errorf("缺动画剪辑 %s", clip)
		}
	}
	if len(as.Anims["Animations/Flicked_Object"].Floats["fast_1"]["m_IsActive"]) == 0 {
		t.Fatal("Flicked_Object 缺 fast_1 激活曲线")
	}
	for _, snd := range []string{
		"flick", "zoomFast", "flickPrepare", "stab", "stabnohit", "disappointed",
		"gulp", "burger", "cough_1", "cough_2", "sigh", "common_miss",
	} {
		if _, ok := as.Sounds[snd]; !ok {
			t.Errorf("缺音效 %s", snd)
		}
	}
}

func TestForkLifterRuntimeSemantics(t *testing.T) {
	if perfectOrder(flickPea) != 101 || perfectOrder(flickTopBun) != 104 ||
		perfectOrder(flickBurger) != 103 || perfectOrder(flickBottomBun) != 102 {
		t.Fatalf("perfect stack sorting order changed")
	}
	if colorProgress(10, 10, 4) != 0 || colorProgress(14, 10, 4) != 1 {
		t.Fatalf("colorProgress endpoints changed")
	}
	a := coughSound(12.25)
	b := coughSound(12.25)
	if a != b || (a != "cough_1" && a != "cough_2") {
		t.Fatalf("cough randomization not deterministic: %q %q", a, b)
	}

	m := &Module{}
	old := &riq.Entity{Datamodel: "forkLifter/colorGrad", Beat: 4, Length: 2, Data: map[string]any{}}
	m.OnEvent(old)
	if len(m.grads) != 1 || m.grads[0].typ != gradClassic {
		t.Fatalf("legacy colorGrad default = %d, want Classic", m.grads[0].typ)
	}
}
