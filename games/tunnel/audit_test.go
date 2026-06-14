package tunnel

import (
	"math"
	"testing"

	"hsdemo/kart"
)

func TestTunnelExtractedAssets(t *testing.T) {
	as, err := kart.Load("../../assets/tunnel", 44100)
	if err != nil {
		t.Skipf("assets not extracted: %v", err)
	}
	for _, role := range []string{"tunnelWall", "tunnelWallRenderer", "frontHand", "cowbellAnimator", "driverAnimator"} {
		if as.Roles[role] == "" {
			t.Errorf("role %s 未解析", role)
		}
	}
	if got := as.Roles["frontHand"]; got != "Player/Hand_Front" {
		t.Fatalf("frontHand = %q", got)
	}
	if got := as.Extra.RefArrays["bg"]; len(got) != 8 {
		t.Fatalf("bg refs = %d, want 8", len(got))
	}
	if got := len(as.Extra.Curves["handCurve"].Points); got != 2 {
		t.Fatalf("handCurve points = %d, want 2", got)
	}

	game := as.Extra.Components["game"]
	if game.Nums["tunnelChunksPerSec"] != 4 || game.Nums["tunnelWallChunkSize"] != 13.7 {
		t.Fatalf("tunnel chunk params = %#v", game.Nums)
	}
	if game.Nums["tunnelTint.g"] == 0 || game.Nums["tunnelScreen.r"] == 0 {
		t.Fatalf("tunnel material colors missing: %#v", game.Nums)
	}

	for _, ctrl := range []string{"BeachBG", "Cowbell", "Driver", "Driver_Arms"} {
		if _, ok := as.Controllers[ctrl]; !ok {
			t.Errorf("缺 controller %s", ctrl)
		}
	}
	for _, clip := range []string{
		"Animations/Shake", "Animations/Idle", "Animations/Disturbed", "Animations/Angry1", "Animations/Arms_Idle",
		"Animation/Beach", "Animation/BeachFar", "Animation/Desert", "Animation/Field", "Animation/FieldFar",
		"Animation/City", "Animation/CityFar", "Animation/Night", "Animation/NightFar",
		"Animation/Moai", "Animation/CropStomp", "Animation/CropStompFar", "Animation/Quiz", "Animation/QuizFar",
	} {
		if _, ok := as.Anims[clip]; !ok {
			t.Errorf("缺动画剪辑 %s", clip)
		}
	}
	for _, snd := range []string{
		"common_count-ins_cowbell", "common_miss", "en/one", "en/two",
		"tunnelLeft", "tunnelMiddle", "tunnelRight",
	} {
		if _, ok := as.Sounds[snd]; !ok {
			t.Errorf("缺音效 %s", snd)
		}
	}
}

func TestTunnelWallWidthMatchesUnityQuarterChunkCeiling(t *testing.T) {
	got := tunnelWallWidth(1, 3.01, 4, 13.7)
	want := math.Ceil((3.01-1)*4*4) * 0.25 / 4 * 13.7 * 4
	if got != want {
		t.Fatalf("wall width = %v, want %v", got, want)
	}
}

func TestTunnelEaseOutQuadMatchesUnityEndpoints(t *testing.T) {
	if easeOutQuad(0) != 0 || easeOutQuad(1) != 1 {
		t.Fatal("ease endpoints changed")
	}
	if got := easeOutQuad(0.5); math.Abs(got-0.75) > 1e-9 {
		t.Fatalf("easeOutQuad(0.5) = %v, want .75", got)
	}
}
