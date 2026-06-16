package samuraislicentr

import (
	"path/filepath"
	"strings"
	"testing"

	"hsdemo/engine"
	"hsdemo/kart"
)

func loadAuditAssets(t *testing.T) *kart.Assets {
	t.Helper()
	as, err := kart.Load(filepath.Join("..", "..", "assets", "samuraiSliceNtr"), engine.SampleRate)
	if err != nil {
		t.Fatal(err)
	}
	return as
}

func TestBindingsComponentsCurvesTextAndSounds(t *testing.T) {
	as := loadAuditAssets(t)
	for _, role := range []string{"player", "launcher", "objectPrefab", "childParent", "objectHolder", "background", "fasterWarning", "darknessOverlay", "theMoon", "moonText"} {
		if as.Roles[role] == "" {
			t.Fatalf("missing role %s", role)
		}
	}
	for _, comp := range []string{"game", "object", "child"} {
		if _, ok := as.Extra.Components[comp]; !ok {
			t.Fatalf("missing component %s", comp)
		}
	}
	for _, key := range []string{"InCurve", "LaunchCurve", "LaunchHighCurve", "NgLaunchCurve", "DebrisLeftCurve", "DebrisRightCurve", "NgDebrisCurve"} {
		if c := as.Extra.Curves[key]; len(c.Points) != 2 {
			t.Fatalf("curve %s points = %d, want 2", key, len(c.Points))
		}
	}
	if len(as.Extra.RefArrays["Effects"]) != 4 {
		t.Fatalf("Effects refs = %d, want 4", len(as.Extra.RefArrays["Effects"]))
	}
	if err := as.ApplyTexts(); err != nil {
		t.Fatalf("ApplyTexts: %v", err)
	}
	if len(as.Texts) != 1 || as.Texts[0].Path != as.Roles["moonText"] {
		t.Fatalf("texts = %#v", as.Texts)
	}
	for _, snd := range []string{
		"ntrSamurai_in00", "ntrSamurai_in01", "ntrSamurai_launchThrough",
		"ntrSamurai_launchImpact", "ntrSamurai_through", "ntrSamurai_just00",
		"ntrSamurai_just01", "ntrSamurai_catch", "ntrSamurai_ng",
		"ntrSamurai_scoreMany", "item_splat", "melon_dig",
		"holy_mackerel1", "holy_mackerel2", "holy_mackerel3",
		"faster_normal", "faster_question", "faster_weird",
	} {
		if _, ok := as.Sounds[snd]; !ok {
			t.Fatalf("missing sound %s", snd)
		}
	}
}

func TestControllersResolve(t *testing.T) {
	as := loadAuditAssets(t)
	for ctrl, states := range map[string][]string{
		"Samurai":         {"Beat", "NoPose", "Slash", "Step", "Unstep"},
		"SamuraiLauncher": {"Launch", "NoPose", "UnStep"},
		"Child":           {"ChildBeat", "ChildWalk", "NoPose"},
		"Faster":          {"Enter", "Exit", "NoPose"},
		"Moon":            {"Enter", "Exit", "NoPose"},
		"Object":          {"ObjMelon", "ObjFish", "ObjDemon", "ObjMelonPickel", "ObjMelonDebris", "ObjFishDebris", "ObjDemonDebris01", "ObjDemonDebris02", "ObjMelonPickelDebris01", "ObjMelonPickelDebris02", "ObjMelonSplat", "ObjFishSplat", "ObjDemonSplat", "ObjMelonPickelSplat"},
	} {
		c, ok := as.Controllers[ctrl]
		if !ok {
			t.Fatalf("missing controller %s", ctrl)
		}
		for _, st := range states {
			s, ok := c.States[st]
			if !ok {
				t.Fatalf("controller %s missing state %s", ctrl, st)
			}
			if s.Clip == "" {
				t.Fatalf("controller %s state %s has no clip", ctrl, st)
			}
			if as.Anims[s.Clip] == nil {
				t.Fatalf("controller %s state %s clip %q missing", ctrl, st, s.Clip)
			}
		}
	}
	if st := as.Controllers["Object"].States["ObjFishGold"]; st.Clip != "" {
		t.Fatalf("ObjFishGold unexpectedly resolved to %q; C# never plays this orphan state", st.Clip)
	}
}

func TestClipPathResolutionAndAttrs(t *testing.T) {
	as := loadAuditAssets(t)
	nodes := map[string]bool{}
	for _, n := range as.Rig.Nodes {
		nodes[n.Path] = true
	}
	resolve := func(root, path string) string {
		if path == "" {
			return root
		}
		if root == "" {
			return path
		}
		return root + "/" + path
	}
	supported := map[string]bool{
		"m_IsActive": true, "m_Enabled": true, "m_SortingOrder": true,
		"m_FlipX": true, "m_FlipY": true, "m_Size.x": true, "m_Size.y": true,
		"m_Color.r": true, "m_Color.g": true, "m_Color.b": true, "m_Color.a": true,
		"m_fontColor.r": true, "m_fontColor.g": true, "m_fontColor.b": true, "m_fontColor.a": true,
	}
	for root, ctrlName := range as.Animators {
		ctrl := as.Controllers[ctrlName]
		for stName, st := range ctrl.States {
			if st.Clip == "" {
				continue
			}
			a := as.Anims[st.Clip]
			if a == nil {
				t.Fatalf("state %s.%s clip %q missing", ctrlName, stName, st.Clip)
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
			for p, attrs := range a.Floats {
				paths[p] = true
				for attr := range attrs {
					if !supported[attr] {
						t.Fatalf("clip %s path %q attr %q unsupported", st.Clip, p, attr)
					}
				}
			}
			for p := range paths {
				if !nodes[resolve(root, p)] {
					t.Fatalf("clip %s state %s.%s path %q root %q unresolved", st.Clip, ctrlName, stName, p, root)
				}
			}
		}
	}
	for name, a := range as.Anims {
		for path, keys := range a.Sprites {
			for _, k := range keys {
				if k.Name != "" {
					if _, ok := as.Sheet.Sprites[k.Name]; !ok {
						t.Fatalf("clip %s path %q sprite %q missing", name, path, k.Name)
					}
				}
			}
		}
	}
}

func TestRuntimeMappings(t *testing.T) {
	if actionStep != 3 || actionSlice != 0 {
		t.Fatalf("input channel mapping changed")
	}
	for typ, want := range map[int]string{
		objMelon:     "ObjMelon",
		objFish:      "ObjFish",
		objDemon:     "ObjDemon",
		objMelon2B2T: "ObjMelonPickel",
	} {
		if got := objectIdleState(typ); got != want {
			t.Fatalf("idle state %d = %s, want %s", typ, got, want)
		}
	}
	if !strings.Contains(debrisState(objDemon, true), "01") || !strings.Contains(debrisState(objDemon, false), "02") {
		t.Fatalf("demon debris halves must keep C# 01/02 split")
	}
	if splatState(objMelon2B2T) != "ObjMelonPickelSplat" {
		t.Fatalf("pickel splat mapping changed")
	}
}
