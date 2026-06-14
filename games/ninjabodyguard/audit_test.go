package ninjabodyguard

import (
	"testing"

	"hsdemo/kart"
	"hsdemo/synth"
)

func TestNinjaBodyguardAssetsExposeRuntimeBindings(t *testing.T) {
	as, err := kart.Load("../../assets/ninjaBodyguard", synth.SampleRate)
	if err != nil {
		t.Fatal(err)
	}
	for _, role := range []string{
		"PlayerAnim", "GuideAnim", "LordAnim", "FirstNinja", "NinjaArrow",
		"LeftSceneObj", "Blackout", "HitParticle",
	} {
		if as.Roles[role] == "" {
			t.Fatalf("role %s missing", role)
		}
	}
	for _, key := range []string{"game", "enemy", "arrow"} {
		if _, ok := as.Extra.Components[key]; !ok {
			t.Fatalf("component %s missing", key)
		}
	}
	if got := len(as.Extra.Curves["arrow.hitCurve"].Points); got != 2 {
		t.Fatalf("arrow.hitCurve points = %d, want 2", got)
	}
	if kart.NewTemplate(as, as.Roles["FirstNinja"]) == nil {
		t.Fatalf("FirstNinja template missing")
	}
	if kart.NewTemplate(as, as.Roles["NinjaArrow"]) == nil {
		t.Fatalf("NinjaArrow template missing")
	}
}

func TestNinjaBodyguardControllersCoverScriptedStates(t *testing.T) {
	as, err := kart.Load("../../assets/ninjaBodyguard", synth.SampleRate)
	if err != nil {
		t.Fatal(err)
	}
	states := map[string][]string{
		"Player":  {"Idle", "NinjaSwingR", "NinjaSwingL", "NinjaCutR", "NinjaCutL"},
		"Guide":   {"Left", "Right"},
		"Samurai": {"Stay", "Shock"},
		"Ninja":   {"ArrowReady", "ArrowShot"},
		"Arrow":   {"Destroy", "DivertL", "DivertR", "Hit"},
	}
	for ctrl, names := range states {
		c, ok := as.Controllers[ctrl]
		if !ok {
			t.Fatalf("controller %s missing", ctrl)
		}
		for _, name := range names {
			if _, ok := c.States[name]; !ok {
				t.Fatalf("controller %s missing state %s", ctrl, name)
			}
		}
	}
}
