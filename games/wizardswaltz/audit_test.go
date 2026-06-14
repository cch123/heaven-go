package wizardswaltz

import (
	"math"
	"testing"

	"hsdemo/kart"
)

func TestFlowerPositionMatchesUnityFormula(t *testing.T) {
	m := New().(*Module)
	x, y, z := m.flowerPos(0, 0)
	if math.Abs(x) > 1e-9 || math.Abs(y-(-1.25)) > 1e-9 || math.Abs(z-3.5) > 1e-9 {
		t.Fatalf("front interval start pos = (%.3f, %.3f, %.3f), want (0, -1.25, 3.5)", x, y, z)
	}
	x, y, z = m.flowerPos(1.5, 0)
	if math.Abs(x-6) > 1e-9 || math.Abs(y-(-2)) > 1e-9 || math.Abs(z) > 1e-9 {
		t.Fatalf("quarter interval pos = (%.3f, %.3f, %.3f), want (6, -2, 0)", x, y, z)
	}
}

func TestWizardsWaltzExtractedAssets(t *testing.T) {
	as, err := kart.Load("../../assets/wizardsWaltz", 44100)
	if err != nil {
		t.Skipf("assets not extracted: %v", err)
	}
	for _, role := range []string{"wizard", "girl", "plantHolder", "plantBase"} {
		if as.Roles[role] == "" {
			t.Errorf("role %s 未解析", role)
		}
	}
	for _, ctrl := range []string{"WizardAnimator", "WandAnimator", "GirlAnimator", "GirlFlowerAnimator", "PlantAnimator"} {
		if _, ok := as.Controllers[ctrl]; !ok {
			t.Errorf("缺 controller %s", ctrl)
		}
	}
	for _, clip := range []string{
		"Animations/WizardIdle", "Animations/WizardMagic", "Animations/WandIdle",
		"Animations/GirlIdle", "Animations/GirlHappy", "Animations/GirlSad", "Animations/GirlFlower",
		"Animations/PlantAppear", "Animations/PlantIdlePlant", "Animations/PlantHit",
		"Animations/PlantIdleFlower", "Animations/PlantEat", "Animations/PlantEatLoop",
	} {
		if _, ok := as.Anims[clip]; !ok {
			t.Errorf("缺动画剪辑 %s", clip)
		}
	}
	for _, snd := range []string{"plant", "grow", "wand", "common_miss"} {
		if _, ok := as.Sounds[snd]; !ok {
			t.Errorf("缺音效 %s", snd)
		}
	}
	comp := as.Extra.Components
	if comp["game"].Nums["xRange"] != 6 || comp["game"].Nums["zRange"] != 3.5 {
		t.Fatalf("game component ranges = %#v", comp["game"].Nums)
	}
	if got := len(comp["girl"].RefArrays["flowers"]); got != 6 {
		t.Fatalf("girl flowers = %d, want 6", got)
	}
	if comp["wizard"].Refs["shadow"] != "WizardShadow" {
		t.Fatalf("wizard shadow = %q", comp["wizard"].Refs["shadow"])
	}
	if comp["plant"].Refs["spriteRenderer"] != "Prefabs/Plant/SpriteHolder" {
		t.Fatalf("plant spriteRenderer = %q", comp["plant"].Refs["spriteRenderer"])
	}
}
