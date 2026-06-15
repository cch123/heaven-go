package moaidoowop

import (
	"path/filepath"
	"strings"
	"testing"

	"hsdemo/engine"
	"hsdemo/kart"
	"hsdemo/kmdata"
	"hsdemo/riq"
)

func loadAuditAssets(t *testing.T) *kart.Assets {
	t.Helper()
	as, err := kart.Load(filepath.Join("..", "..", "assets", gameID), engine.SampleRate)
	if err != nil {
		t.Fatal(err)
	}
	return as
}

func TestBindingsMaterialsSoundsAndFallbacks(t *testing.T) {
	as := loadAuditAssets(t)
	nodes := nodeSet(as)
	wantRoles := map[string]string{
		"cpuMoaiAnim":        "Objects/Moais/Moaio/Moai",
		"playerMoaiAnim":     "Objects/Moais/Moaiko/Moaio",
		"cpuMoaiMoveAnim":    "Objects/Moais/Moaio",
		"playerMoaiMoveAnim": "Objects/Moais/Moaiko",
		"bgAnim":             "BG/Background",
		"GlassesM":           "Objects/Moais/Moaio/Moai/moai_Head/Moai_Glasses",
		"GlassesF":           "Objects/Moais/Moaiko/Moaio/moai_Head/Moai_Glasses",
		"FRibbon":            "Objects/Moais/Moaiko/Moaio/moai_Head/moai_Ribbon",
		"MFlower":            "Objects/Moais/Moaio/Moai/moai_Head/moai_Flower",
		"FFlower":            "Objects/Moais/Moaiko/Moaio/moai_Head/moai_Flower",
	}
	for role, want := range wantRoles {
		got := as.Roles[role]
		if got != want || !nodes[got] {
			t.Fatalf("role %s = %q, want existing %q", role, got, want)
		}
	}
	if !nodes["Objects/Moais/Moaio/Moai/moai_Head/moai_Ribbon"] {
		t.Fatal("male ribbon fallback path missing")
	}

	comp := as.Extra.Components["game"]
	if comp.Refs["MoaiColorM"] != "MoaiMaleMaterial" || comp.Refs["MoaiColorF"] != "MoaiFemaleMaterial" {
		t.Fatalf("material refs = %v", comp.Refs)
	}
	for _, p := range comp.RefArrays["birdAnims"] {
		if !nodes[p] {
			t.Fatalf("bird animator path %q missing", p)
		}
	}
	if got := len(comp.RefArrays["birdAnims"]); got != 3 {
		t.Fatalf("birdAnims len = %d, want 3", got)
	}
	if got := len(comp.RefArrays["bgBirdAnims"]); got != 10 {
		t.Fatalf("bgBirdAnims len = %d, want 10", got)
	}
	for _, p := range []string{
		"Objects/Moais/Moaio/Moai/moai_Head/BirdMoai/FallEffect",
		"Objects/FGBirds/Bird1/FallEffect",
		"Objects/FGBirds/Bird2/FallEffect",
	} {
		if !nodes[p] {
			t.Fatalf("poop fallback anchor %q missing", p)
		}
	}
	for i := 1; i <= 5; i++ {
		for _, prefix := range []string{"PoopSplash", "PoopRemain", "PoopDrip"} {
			if _, ok := as.Sheet.Sprites[prefix+itoaSmall(i)]; !ok {
				t.Fatalf("missing poop sprite %s%d", prefix, i)
			}
		}
	}
	for _, name := range []string{"leftDoo", "leftPah", "leftWop", "rightDoo", "rightPah", "rightWop", "switch"} {
		if len(as.Sounds[name]) == 0 {
			t.Fatalf("missing sound %s", name)
		}
	}
}

func TestControllersClipsAndAnimationPaths(t *testing.T) {
	as := loadAuditAssets(t)
	for ctrlName, states := range map[string][]string{
		"Background": {"BGIdle", "BGDayToNight", "BGNightHold", "BGNightToDay", "BGBirdsFlyIn"},
		"Bird":       {"Bird_Idle", "Bird_Bop", "Bird_Drop_In"},
		"BirdBG":     {"BirdBG", "BirdStatic"},
		"Moai":       {"Moai_Idle", "Moai_Bop", "Moai_Miss", "Moai_Shout", "Moai_Shout_Open", "Moai_Shout_Hold", "moai_Shout_Close"},
		"Moaio":      {"MoaioIdle", "MoaioUp", "MoaioDown"},
		"Moaiko":     {"MoaikoIdle", "MoaikoUp", "MoaikoDown"},
		"Poop":       {"PoopIdle", "PoopFall1", "PoopFall2", "PoopFall3", "PoopFall4", "PoopFall5"},
	} {
		ctrl, ok := as.Controllers[ctrlName]
		if !ok {
			t.Fatalf("missing controller %s", ctrlName)
		}
		for _, stName := range states {
			st, ok := ctrl.States[stName]
			if !ok {
				t.Fatalf("controller %s missing state %s", ctrlName, stName)
			}
			if st.Clip == "" || as.Anims[st.Clip] == nil {
				t.Fatalf("controller %s state %s clip %q missing", ctrlName, stName, st.Clip)
			}
		}
	}

	nodes := nodeSet(as)
	for path := range as.Animators {
		if !nodes[path] {
			t.Fatalf("animator path %q missing", path)
		}
	}
	for root, ctrlName := range as.Animators {
		ctrl := as.Controllers[ctrlName]
		for stName, st := range ctrl.States {
			if st.Clip == "" {
				if ctrlName == "Moai" && (stName == "Moai_Open" || stName == "moai_Shout_Open") {
					continue
				}
				t.Fatalf("controller %s state %s has no clip", ctrlName, stName)
			}
			checkAnimPaths(t, root, st.Clip, as.Anims[st.Clip], nodes)
		}
	}
}

func TestAllUnityClipsAccountedAndSpriteSwapsResolve(t *testing.T) {
	as := loadAuditAssets(t)
	ctrlClips := map[string]bool{}
	for _, ctrl := range as.Controllers {
		for _, st := range ctrl.States {
			if st.Clip != "" {
				ctrlClips[st.Clip] = true
			}
		}
	}
	for name := range as.Anims {
		switch {
		case strings.HasPrefix(name, "Background/"),
			strings.HasPrefix(name, "BGBirds/"),
			strings.HasPrefix(name, "FGBirds/"),
			strings.HasPrefix(name, "Moai/"),
			strings.HasPrefix(name, "Poop/"):
			if !ctrlClips[name] {
				t.Errorf("Unity clip %q has no controller state", name)
			}
		}
	}
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

func TestRuntimeHelpersMatchScriptSemantics(t *testing.T) {
	ev := &riq.Entity{Data: map[string]any{"auto": true}}
	if !boolDefault(ev, "auto", false) || !boolDefault(ev, "missing", true) || boolDefault(ev, "missing", false) {
		t.Fatal("bool defaults no longer match loader parameter semantics")
	}
	if poopSprite(3, 0) != "PoopSplash3" || poopSprite(3, 0.05) != "PoopRemain3" || poopSprite(3, 0.2) != "PoopDrip3" {
		t.Fatal("poop sprite phase no longer matches PoopFall keyframes")
	}
	def := defaultVisual()
	if def.mHat != 0 || def.fHat != 1 || def.mHead != defaultHead || def.fLens != defaultLens {
		t.Fatalf("default visual = %+v", def)
	}
}

func nodeSet(as *kart.Assets) map[string]bool {
	nodes := map[string]bool{}
	for _, n := range as.Rig.Nodes {
		nodes[n.Path] = true
	}
	return nodes
}

func checkAnimPaths(t *testing.T, root, clip string, a *kmdata.Anim, nodes map[string]bool) {
	t.Helper()
	check := func(rel string) {
		full := root
		if rel != "" {
			if full == "" {
				full = rel
			} else {
				full += "/" + rel
			}
		}
		if !nodes[full] {
			t.Fatalf("clip %s targets missing path %q (root %q rel %q)", clip, full, root, rel)
		}
	}
	for rel := range a.Pos {
		check(rel)
	}
	for rel := range a.Scale {
		check(rel)
	}
	for rel := range a.Euler {
		check(rel)
	}
	for rel := range a.Sprites {
		check(rel)
	}
	for rel := range a.Floats {
		check(rel)
	}
}
