package clappytrio

import (
	"testing"

	"hsdemo/kart"
)

func TestClappyTrioExtractedAssets(t *testing.T) {
	as, err := kart.Load("../../assets/clappyTrio", 44100)
	if err != nil {
		t.Skipf("assets not extracted: %v", err)
	}
	if err := as.ApplyTexts(); err != nil {
		t.Fatalf("ApplyTexts: %v", err)
	}
	for _, role := range []string{"customText", "signAnim", "textTrioTiming", "textCustom"} {
		if as.Roles[role] == "" {
			t.Errorf("role %s 未解析", role)
		}
	}
	for _, ctrl := range []string{"Lion", "Sign"} {
		if _, ok := as.Controllers[ctrl]; !ok {
			t.Errorf("缺 controller %s", ctrl)
		}
	}
	for _, clip := range []string{
		"Animations/Idle", "Animations/Bop", "Animations/Clap", "Animations/Prepare",
		"Animations/Enter", "Animations/Exit", "Animations/SignIdle",
	} {
		if _, ok := as.Anims[clip]; !ok {
			t.Errorf("缺动画剪辑 %s", clip)
		}
	}
	game := as.Extra.Components["game"]
	if got := game.RefArrays["Lion"]; len(got) != 1 || got[0] != "Lion" {
		t.Fatalf("Lion refs = %#v, want [Lion]", got)
	}
	if got := game.SpriteArrays["faces"]; len(got) != 5 {
		t.Fatalf("faces = %#v, want 5 sprites", got)
	}
	if kart.NewTemplate(as, "Lion") == nil {
		t.Fatal("Lion template 未解析")
	}
	for _, snd := range []string{"leftClap", "middleClap", "rightClap", "ready", "sign"} {
		if _, ok := as.Sounds[snd]; !ok {
			t.Errorf("缺音效 %s", snd)
		}
	}
	if len(as.Texts) != 1 || as.Texts[0].Path != "Sign/SignContents/customText" {
		t.Fatalf("texts = %#v, want customText node", as.Texts)
	}
}
