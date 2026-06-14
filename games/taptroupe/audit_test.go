package taptroupe

import (
	"math"
	"strings"
	"testing"

	"hsdemo/kart"
)

func loadAssets(t *testing.T) *kart.Assets {
	t.Helper()
	as, err := kart.Load("../../assets/tapTroupe", 44100)
	if err != nil {
		t.Skipf("assets not extracted: %v", err)
	}
	return as
}

func TestExtractedAssets(t *testing.T) {
	as := loadAssets(t)
	nodeSet := map[string]bool{}
	for _, n := range as.Rig.Nodes {
		nodeSet[n.Path] = true
	}
	for _, p := range requiredScenePaths() {
		if !nodeSet[p] {
			t.Errorf("scene path %q missing", p)
		}
	}
	for _, snd := range requiredSounds() {
		if _, ok := as.Sounds[snd]; !ok {
			t.Errorf("missing sound %s", snd)
		}
	}
}

func TestControllersResolve(t *testing.T) {
	as := loadAssets(t)
	for _, ctrl := range []string{"tapTroupe", "Tapper", "CornerTapper", "CornerBody", "CornerExpression"} {
		if _, ok := as.Controllers[ctrl]; !ok {
			t.Fatalf("missing controller %s", ctrl)
		}
	}
	for _, st := range []string{
		"BamFeet", "BamReadyFeet", "BamReadyTap", "BopFeet", "FeetFadeOut",
		"HitBamFeet", "HitBamReadyFeet", "HitBamReadyTap", "HitStepFeet",
		"HitStepReadyTap", "HitTapFeet", "IdleFeet", "LastTapFeet",
		"StepFeet", "StepReadyTap", "TapFeet",
	} {
		if _, ok := as.Controllers["Tapper"].States[st]; !ok {
			t.Errorf("Tapper missing state %s", st)
		}
	}
	for name, ctrl := range as.Controllers {
		for st, s := range ctrl.States {
			if s.Clip == "" {
				t.Errorf("controller %s state %s has no clip", name, st)
				continue
			}
			if _, ok := as.Anims[s.Clip]; !ok {
				t.Errorf("controller %s state %s clip %q missing", name, st, s.Clip)
			}
		}
	}
	nodeSet := map[string]bool{}
	for _, n := range as.Rig.Nodes {
		nodeSet[n.Path] = true
	}
	for path := range as.Animators {
		if !nodeSet[path] {
			t.Errorf("animator binding path %q missing in scene", path)
		}
	}
}

func TestAllClipsAccounted(t *testing.T) {
	as := loadAssets(t)
	ctrlClips := map[string]bool{}
	for _, c := range as.Controllers {
		for _, st := range c.States {
			if st.Clip != "" {
				ctrlClips[st.Clip] = true
			}
		}
	}
	for name := range as.Anims {
		if !strings.Contains(name, "/") {
			continue
		}
		if !ctrlClips[name] {
			t.Errorf("clip %q has no controller state", name)
		}
	}
}

func TestClipPathResolution(t *testing.T) {
	as := loadAssets(t)
	nodeSet := map[string]bool{}
	for _, n := range as.Rig.Nodes {
		nodeSet[n.Path] = true
	}
	resolve := func(root, curvePath string) string {
		if curvePath == "" {
			return root
		}
		if root == "" {
			return curvePath
		}
		return root + "/" + curvePath
	}
	for animPath, ctrlName := range as.Animators {
		c := as.Controllers[ctrlName]
		for _, st := range c.States {
			a := as.Anims[st.Clip]
			if a == nil {
				continue
			}
			paths := map[string]bool{}
			for p := range a.Pos {
				paths[p] = true
			}
			for p := range a.Euler {
				paths[p] = true
			}
			for p := range a.Scale {
				paths[p] = true
			}
			for p := range a.Sprites {
				paths[p] = true
			}
			for p := range a.Floats {
				paths[p] = true
			}
			for p := range paths {
				if !nodeSet[resolve(animPath, p)] {
					t.Errorf("clip %s path %q under root %q misses scene", st.Clip, p, animPath)
				}
			}
		}
	}
}

func TestFloatAttrsSupported(t *testing.T) {
	as := loadAssets(t)
	supported := map[string]bool{
		"m_IsActive": true, "m_Enabled": true, "m_SortingOrder": true,
		"m_FlipX": true, "m_FlipY": true, "m_Size.x": true, "m_Size.y": true,
		"m_Color.r": true, "m_Color.g": true, "m_Color.b": true, "m_Color.a": true,
	}
	for name, a := range as.Anims {
		for path, attrs := range a.Floats {
			for attr := range attrs {
				if !supported[attr] {
					t.Errorf("clip %s path %q attr %q unsupported", name, path, attr)
				}
			}
		}
	}
}

func TestSpriteSwapsResolve(t *testing.T) {
	as := loadAssets(t)
	for name, a := range as.Anims {
		for path, keys := range a.Sprites {
			for _, k := range keys {
				if k.Name == "" {
					continue
				}
				if _, ok := as.Sheet.Sprites[k.Name]; !ok {
					t.Errorf("clip %s path %q sprite %q missing from atlas", name, path, k.Name)
				}
			}
		}
	}
}

func TestRuntimeRules(t *testing.T) {
	for _, tc := range []struct {
		length float64
		want   float64
	}{
		{1, 2.25},
		{3, 2.25},
		{5, 4.5},
		{7.25, 6.75},
	} {
		if got := actualTapLength(tc.length); math.Abs(got-tc.want) > 1e-9 {
			t.Fatalf("actualTapLength(%v) = %v, want %v", tc.length, got, tc.want)
		}
	}
	for i := 0; i < 20; i++ {
		if got := deterministicChoice(float64(i)*0.37, 3); got < 0 || got >= 3 {
			t.Fatalf("deterministicChoice out of range: %d", got)
		}
	}
	c := newCorner("UnderForegroundElementsHolder/CornerTappers/CornerTapperPlayer")
	if c.body != "UnderForegroundElementsHolder/CornerTappers/CornerTapperPlayer/Parts/Body" {
		t.Fatalf("corner body path changed: %q", c.body)
	}
	if c.expr != "UnderForegroundElementsHolder/CornerTappers/CornerTapperPlayer/Parts/Body/HeadHolder/Head" {
		t.Fatalf("corner expression path changed: %q", c.expr)
	}
	if c.popper != "UnderForegroundElementsHolder/CornerTappers/CornerTapperPlayer/Parts/Body/PartyPopper" {
		t.Fatalf("corner popper path changed: %q", c.popper)
	}
}

func requiredScenePaths() []string {
	return []string{
		"Legs/Tapper",
		"Legs/Tapper (1)",
		"Legs/Tapper (2)",
		"Legs/TapperPlayer",
		"ForegroundElements/Darkness",
		"SpotLights/spotlight",
		"SpotLights/spotlight (1)",
		"SpotLights/spotlight (2)",
		"SpotLights/spotlight (3)",
		"ZoomoutTroupe/LongTapper/Legs",
		"ZoomoutTroupe/LongTapper (1)/Legs",
		"ZoomoutTroupe/LongTapper (2)/Legs",
		"ZoomoutTroupe/LongTapper (3)/Legs",
		"UnderForegroundElementsHolder/CornerTappers/CornerTapper",
		"UnderForegroundElementsHolder/CornerTappers/CornerTapper (1)",
		"UnderForegroundElementsHolder/CornerTappers/CornerTapper (2)",
		"UnderForegroundElementsHolder/CornerTappers/CornerTapperPlayer",
		"UnderForegroundElementsHolder/CornerTappers/CornerTapper/Parts/Body/HeadHolder/Head",
		"UnderForegroundElementsHolder/CornerTappers/CornerTapper (1)/Parts/Body/HeadHolder/Head",
		"UnderForegroundElementsHolder/CornerTappers/CornerTapper (2)/Parts/Body/HeadHolder/Head",
		"UnderForegroundElementsHolder/CornerTappers/CornerTapperPlayer/Parts/Body/HeadHolder/Head",
		"UnderForegroundElementsHolder/CornerTappers/CornerTapper/Parts/Body/PartyPopper",
		"UnderForegroundElementsHolder/CornerTappers/CornerTapper (1)/Parts/Body/PartyPopper",
		"UnderForegroundElementsHolder/CornerTappers/CornerTapper (2)/Parts/Body/PartyPopper",
		"UnderForegroundElementsHolder/CornerTappers/CornerTapperPlayer/Parts/Body/PartyPopper",
	}
}

func requiredSounds() []string {
	return []string{
		"bamvoice1", "bamvoice2", "laughter", "miss", "okayA1", "okayA2",
		"okayB1", "okayB2", "okayC1", "okayC2", "other1", "other2",
		"other3", "player3", "popper", "spit", "startTap", "step1",
		"step2", "tap3", "tapAnd", "tapReady1", "tapReady2", "tapvoice1",
		"tapvoice2", "tink", "woo",
	}
}
