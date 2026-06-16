package taptrial

import (
	"testing"

	"hsdemo/kart"
)

func loadAssets(t *testing.T) *kart.Assets {
	t.Helper()
	as, err := kart.Load("../../assets/tapTrial", 44100)
	if err != nil {
		t.Skipf("assets not extracted: %v", err)
	}
	return as
}

func TestTapTrialRuntimeRolesResolve(t *testing.T) {
	as := loadAssets(t)
	nodeSet := tapTrialNodeSet(as)
	for role, want := range map[string]string{
		"player":      "Player",
		"monkeyL":     "MonkeyL",
		"monkeyR":     "MonkeyR",
		"rootPlayer":  "Player/root_body",
		"rootMonkeyL": "MonkeyL/root_body",
		"rootMonkeyR": "MonkeyR/root_body",
		"giraffe":     "Giraffe",
		"flash":       "Flash",
	} {
		if got := as.Roles[role]; got != want || !nodeSet[got] {
			t.Fatalf("role %s = %q, want existing %q", role, got, want)
		}
	}
	for root, ctrl := range map[string]string{
		"Player":  "Player",
		"MonkeyL": "MonkeyTapTrial",
		"MonkeyR": "MonkeyTapTrial",
		"Giraffe": "Giraffe",
	} {
		if got := as.Animators[root]; got != ctrl {
			t.Fatalf("animator %s = %q, want %q", root, got, ctrl)
		}
	}
}

func TestTapTrialSharedMonkeyJumpStatesArePlayerOnly(t *testing.T) {
	as := loadAssets(t)
	for _, state := range []string{"JumpPrepare", "JumpTap"} {
		monkeyClip := as.Controllers["MonkeyTapTrial"].States[state].Clip
		playerClip := as.Controllers["Player"].States[state].Clip
		if monkeyClip == "" || monkeyClip != playerClip {
			t.Fatalf("%s shared clip mismatch monkey=%q player=%q", state, monkeyClip, playerClip)
		}
		if as.Anims[monkeyClip].Sprites["root_body/girl_arm_l"] == nil {
			t.Fatalf("%s should be the girl's jump clip, got %s", state, monkeyClip)
		}
	}
	nodeSet := tapTrialNodeSet(as)
	for _, root := range []string{"MonkeyL", "MonkeyR"} {
		if nodeSet[root+"/root_body/girl_arm_l"] {
			t.Fatalf("%s unexpectedly has girl limbs", root)
		}
	}
	if !nodeSet["Player/root_body/girl_arm_l"] || !nodeSet["Player/root_body/girl_leg_0 (1)"] {
		t.Fatal("player root is missing girl jump/tap limbs")
	}
}

func TestTapTrialMissingStarChildrenAreHandWrittenParticles(t *testing.T) {
	as := loadAssets(t)
	nodeSet := tapTrialNodeSet(as)
	for _, root := range []string{"MonkeyL", "MonkeyR"} {
		for _, parent := range []string{"/tap_effect", "/tap_effect (1)"} {
			if !nodeSet[root+parent] || !nodeSet[root+parent+"/wave"] {
				t.Fatalf("%s missing tap effect wave under %s", root, parent)
			}
			for _, star := range []string{"/star_0", "/star_1"} {
				if nodeSet[root+parent+star] {
					t.Fatalf("%s unexpectedly has Unity star child %s", root, parent+star)
				}
			}
		}
	}
	if _, ok := as.Anims["tap/Jumpactualtap"].Pos["tap_effect/star_0"]; !ok {
		t.Fatal("Jumpactualtap should retain the original missing star_0 binding")
	}
	if _, ok := as.Anims["tap/Jumpactualtap"].Pos["tap_effect (1)/star_1"]; !ok {
		t.Fatal("Jumpactualtap should retain the original missing star bindings")
	}
	m := &Module{}
	m.monkeyStars()
	if len(m.stars) != 12 {
		t.Fatalf("monkeyStars spawned %d stars, want 12", len(m.stars))
	}
	m.playerStars()
	if len(m.stars) != 24 {
		t.Fatalf("playerStars appended to %d stars, want 24", len(m.stars))
	}
}

func TestTapTrialLegacyReferenceAndTongueBindings(t *testing.T) {
	as := loadAssets(t)
	for _, clip := range []string{"Animations/Tap", "Animations/TapPrepare", "Animations/DoubleTapPrepare"} {
		keys := as.Anims[clip].Sprites["root_body/ref (1)"]
		if len(keys) == 0 {
			t.Fatalf("%s missing legacy ref binding", clip)
		}
		for _, k := range keys {
			if k.Name != "" {
				t.Fatalf("%s ref binding sprite = %q, want empty", clip, k.Name)
			}
		}
	}
	nodeSet := tapTrialNodeSet(as)
	if nodeSet["MonkeyL/root_body/monkey_head/tongue bg"] {
		t.Fatal("MonkeyL unexpectedly has tongue bg helper")
	}
	if !nodeSet["MonkeyR/root_body/monkey_head/tongue bg"] {
		t.Fatal("MonkeyR should retain the tongue bg helper")
	}
}

func TestTapTrialWhiffMatchesUnityPlayerState(t *testing.T) {
	tests := []struct {
		state     playerState
		anim      string
		nearMiss  bool
		scoreMiss bool
	}{
		{playerStateTap, "Tap", true, true},
		{playerStateDoubleTap, "DoubleTap", true, true},
		{playerStateTripleTap, "", false, true},
		{playerStateJumping, "", false, false},
	}
	for _, tt := range tests {
		m := &Module{playerState: tt.state}
		anim, nearMiss, scoreMiss := m.whiffResponse()
		if anim != tt.anim || nearMiss != tt.nearMiss || scoreMiss != tt.scoreMiss {
			t.Fatalf("state %d whiff = (%q,%v,%v), want (%q,%v,%v)",
				tt.state, anim, nearMiss, scoreMiss, tt.anim, tt.nearMiss, tt.scoreMiss)
		}
	}
}

func tapTrialNodeSet(as *kart.Assets) map[string]bool {
	out := map[string]bool{}
	for _, n := range as.Rig.Nodes {
		out[n.Path] = true
	}
	return out
}
