package frogprincess

import (
	"testing"

	"hsdemo/kart"
)

func TestFrogPrincessExtractedAssets(t *testing.T) {
	as, err := kart.Load("../../assets/frogPrincess", 44100)
	if err != nil {
		t.Skipf("assets not extracted: %v", err)
	}
	for _, role := range []string{"frogAnim", "princessAnim", "Leaves", "Lotuses", "splashEffect", "BGPlane"} {
		if as.Roles[role] == "" {
			t.Errorf("role %s 未解析", role)
		}
	}
	for _, ctrl := range []string{"frog", "lotus", "princess"} {
		if _, ok := as.Controllers[ctrl]; !ok {
			t.Errorf("缺 controller %s", ctrl)
		}
	}
	for _, path := range []string{"Lotuses/lotus (1)", "Lotuses/lotus (2)", "Lotuses/lotus (3)", "Lotuses/lotus (4)"} {
		if _, ok := as.Animators[path]; !ok {
			t.Errorf("lotus animator %s 未绑定", path)
		}
	}
	for _, clip := range []string{
		"Animations/frogReady", "Animations/frogHold", "Animations/frogRelease",
		"Animations/frogJump", "Animations/frogJumpBarely", "Animations/frogJumpFast",
		"Animations/frogFall", "Animations/frogAppear", "Animations/lotusHold",
		"Animations/lotusRelease", "Animations/lotusJump", "Animations/lotusJumpBarely",
		"Animations/lotusFall", "Animations/princessReady", "Animations/princessWary",
		"Animations/princessHold", "Animations/princessHoldBarely",
		"Animations/princessSurpriseHoldBarely", "Animations/princessJump",
		"Animations/princessHappy", "Animations/princessJumpBarely",
		"Animations/princessSurpriseJumpBarely", "Animations/princessJumpFast",
		"Animations/princessFallForward", "Animations/princessFallBackward",
		"Animations/princessSurpriseFall", "Animations/princessAppear",
	} {
		if _, ok := as.Anims[clip]; !ok {
			t.Errorf("缺动画剪辑 %s", clip)
		}
	}
	game := as.Extra.Components["game"]
	if game.Nums["moveDistance"] != 7.7 || game.Nums["moveTime"] != 0.3 {
		t.Fatalf("move params = %#v, want distance 7.7 time 0.3", game.Nums)
	}
	for _, snd := range []string{"ready", "lean", "jump", "A", "7"} {
		if _, ok := as.Sounds[snd]; !ok {
			t.Errorf("缺音效 %s", snd)
		}
	}
}

func TestScrollerUpdatePosMatchesUnityFormula(t *testing.T) {
	if got := wrapLeavesX(0); got != 0 {
		t.Fatalf("wrapLeavesX(0) = %v, want 0", got)
	}
	if got, want := wrapLeavesX(7.7), 7.7; got != want {
		t.Fatalf("wrapLeavesX(7.7) = %v, want %v", got, want)
	}
}

func TestSplashParticlesAreStable(t *testing.T) {
	a := newSplash(12, 1)
	b := newSplash(12, 1)
	if len(a.particles) != 18 || len(b.particles) != 18 {
		t.Fatalf("particle count %d/%d, want 18", len(a.particles), len(b.particles))
	}
	if a.particles[7] != b.particles[7] {
		t.Fatalf("same seed produced different particle: %#v vs %#v", a.particles[7], b.particles[7])
	}
}
