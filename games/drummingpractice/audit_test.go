package drummingpractice

import (
	"testing"

	"hsdemo/kart"
)

func TestMiiFaceSpriteNames(t *testing.T) {
	if got := miiFaceSprite(0, 0); got != "mii_guestA" {
		t.Fatalf("neutral face = %q", got)
	}
	if got := miiFaceSprite(6, 1); got != "mii_matt_happy" {
		t.Fatalf("happy face = %q", got)
	}
	if got := miiFaceSprite(11, 2); got != "mii_error_sad" {
		t.Fatalf("sad face = %q", got)
	}
}

func TestDrummingPracticeExtractedAssets(t *testing.T) {
	as, err := kart.Load("../../assets/drummingPractice", 44100)
	if err != nil {
		t.Skipf("assets not extracted: %v", err)
	}
	for _, role := range []string{
		"background", "backgroundGradient", "player", "leftDrummer",
		"rightDrummer", "hitPrefab", "NPCDrummers",
	} {
		if as.Roles[role] == "" {
			t.Errorf("role %s 未解析", role)
		}
	}
	for _, ctrl := range []string{"DrummerAnimator", "NPCDrummers"} {
		if _, ok := as.Controllers[ctrl]; !ok {
			t.Errorf("缺 controller %s", ctrl)
		}
	}
	for _, clip := range []string{
		"Animations/DrummerIdle", "Animations/DrummerBop",
		"Animations/DrummerPrepareLeft", "Animations/DrummerPrepareRight",
		"Animations/DrummerHitLeft", "Animations/DrummerHitRight",
		"Animations/NPCDrummersEnter", "Animations/NPCDrummersEntered",
		"Animations/NPCDrummersExit", "Animations/NPCDrummersExited",
	} {
		if _, ok := as.Anims[clip]; !ok {
			t.Errorf("缺动画剪辑 %s", clip)
		}
	}
	for _, snd := range []string{"prepare", "drum", "hit", "miss", "common_applause"} {
		if _, ok := as.Sounds[snd]; !ok {
			t.Errorf("缺音效 %s", snd)
		}
	}
	if got := len(as.Extra.Components["game"].RefArrays["streaks"]); got != 14 {
		t.Fatalf("streak refs = %d, want 14", got)
	}
	seenDrummers := 0
	for _, c := range as.Extra.Components {
		if c.Refs["face"] != "" {
			seenDrummers++
		}
	}
	if seenDrummers != 3 {
		t.Fatalf("drummer face refs = %d, want 3", seenDrummers)
	}
	for _, sprite := range []string{"mii_guestA", "mii_guestA_happy", "mii_guestA_sad", "mii_error_sad"} {
		if _, ok := as.Sheet.Sprites[sprite]; !ok {
			t.Errorf("缺 Mii sprite %s", sprite)
		}
	}
}
