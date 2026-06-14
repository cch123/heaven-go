package cointoss

import (
	"testing"

	"hsdemo/kart"
)

func TestImageAlphaTiming(t *testing.T) {
	m := &Module{images: []imageEvt{{beat: 10, length: 4, instantShow: false, instantHide: false}}}
	if got := m.imageAlphaAt(9.9); got != 0 {
		t.Fatalf("before image alpha = %v, want 0", got)
	}
	if got := m.imageAlphaAt(12); got != 1 {
		t.Fatalf("after fade-in alpha = %v, want 1", got)
	}
	if got := m.imageAlphaAt(15); got != 0.5 {
		t.Fatalf("fade-out alpha = %v, want 0.5", got)
	}
	if got := m.imageAlphaAt(16.1); got != 0 {
		t.Fatalf("after fade-out alpha = %v, want 0", got)
	}
}

func TestCoinTossExtractedAssets(t *testing.T) {
	as, err := kart.Load("../../assets/coinToss", 44100)
	if err != nil {
		t.Skipf("assets not extracted: %v", err)
	}
	for _, role := range []string{
		"fg", "bg", "imageBG", "handAnimator", "manHand",
		"handHolder", "manHolder", "imageAnim",
	} {
		if as.Roles[role] == "" {
			t.Errorf("role %s 未解析", role)
		}
	}
	for _, ctrl := range []string{
		"CoinTossController", "CoinTossManController", "CTHolder", "CTManHolder", "CTImageAnim",
	} {
		if _, ok := as.Controllers[ctrl]; !ok {
			t.Errorf("缺 controller %s", ctrl)
		}
	}
	for _, clip := range []string{
		"Animations/Throw", "Animations/Throw_empty", "Animations/Catch_success",
		"Animations/Catch_empty", "Animations/Pickup", "Animations/ManThrow",
		"Animations/ManShow", "Animations/ManExit", "Animations/ImageShow",
		"Animations/ImageFadeIn", "Animations/ImageFadeOut",
	} {
		if _, ok := as.Anims[clip]; !ok {
			t.Errorf("缺动画剪辑 %s", clip)
		}
	}
	for _, snd := range []string{
		"throw", "catch", "miss", "cowbell1", "cowbell2",
		"women_thank", "women_you", "common_applause", "common_audienceSad",
	} {
		if _, ok := as.Sounds[snd]; !ok {
			t.Errorf("缺音效 %s", snd)
		}
	}
}
