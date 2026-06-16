package ninjabodyguard

import (
	"math"
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

func TestNinjaBodyguardLegacyCutBindingsAreEmpty(t *testing.T) {
	as, err := kart.Load("../../assets/ninjaBodyguard", synth.SampleRate)
	if err != nil {
		t.Fatal(err)
	}
	for _, clip := range []string{"Player/NinjaApology", "Player/NinjaHold", "Player/NinjaStay"} {
		keys := as.Anims[clip].Sprites["NinjaCutL.001"]
		if len(keys) == 0 {
			t.Fatalf("%s missing legacy NinjaCutL.001 binding", clip)
		}
		for _, k := range keys {
			if k.Name != "" {
				t.Fatalf("%s legacy NinjaCutL.001 has non-empty sprite %q", clip, k.Name)
			}
		}
	}
}

func TestNinjaBodyguardHitParticleMatchesPrefabParams(t *testing.T) {
	if len(ninjaHitParticleEmitters) != 2 {
		t.Fatalf("hit particle emitters = %d, want 2", len(ninjaHitParticleEmitters))
	}
	wantRot := []float64{60, 110}
	for i, want := range wantRot {
		if got := ninjaHitParticleEmitters[i].shapeRotDeg; got != want {
			t.Fatalf("emitter %d rotation = %v, want %v", i, got, want)
		}
	}
	checks := map[string][2]float64{
		"lifetime":    {ninjaHitParticleLifetimeSec, 3.5},
		"simSpeed":    {ninjaHitParticleSimulationSpeed, 3},
		"startSize":   {ninjaHitParticleStartSize, 1.05},
		"speedMin":    {ninjaHitParticleSpeedMin, 6},
		"speedMax":    {ninjaHitParticleSpeedMax, 8},
		"angle":       {ninjaHitParticleShapeAngleDeg, 30},
		"arc":         {ninjaHitParticleShapeArcDeg, 10},
		"radius":      {ninjaHitParticleShapeRadius, 0.75},
		"shapeY":      {ninjaHitParticleShapeYOffset, -0.5},
		"forceY":      {ninjaHitParticleForceY, -10},
		"lengthScale": {ninjaHitParticleLengthScale, 2},
	}
	for name, vals := range checks {
		if math.Abs(vals[0]-vals[1]) > 1e-9 {
			t.Fatalf("%s = %v, want %v", name, vals[0], vals[1])
		}
	}
	if ninjaHitParticleBurstCount != 1 {
		t.Fatalf("burst count = %d, want 1", ninjaHitParticleBurstCount)
	}
	if ninjaHitParticleOrder != 50 {
		t.Fatalf("sorting order = %d, want 50", ninjaHitParticleOrder)
	}
}

func TestNinjaBodyguardHitParticleSimulationQueuesTwoStreaks(t *testing.T) {
	sp := newNinjaHitSpark(12, 4)
	if len(sp.particles) != 2 {
		t.Fatalf("particles = %d, want 2", len(sp.particles))
	}
	items := ninjaHitParticleSprites(sp, kart.Identity(), 4)
	if len(items) != 2 {
		t.Fatalf("initial queued particles = %d, want 2", len(items))
	}
	if items[0].Sprite != ninjaHitParticleSprite || items[1].Sprite != ninjaHitParticleSprite {
		t.Fatalf("queued sprites should use %q: %#v", ninjaHitParticleSprite, items)
	}
	if items[0].Order != 50 || items[1].Order != 51 {
		t.Fatalf("orders = %d,%d; want 50,51", items[0].Order, items[1].Order)
	}
	expired := ninjaHitParticleSprites(sp, kart.Identity(), 4+ninjaHitParticleVisibleSec+0.01)
	if len(expired) != 0 {
		t.Fatalf("expired particles queued = %d, want 0", len(expired))
	}
}
