package fallingwaffle

import (
	"strings"
	"testing"

	"hsdemo/kart"
	"hsdemo/kmdata"
)

func loadAssets(t *testing.T) *kart.Assets {
	t.Helper()
	as, err := kart.Load("../../assets/fallingWaffle", 44100)
	if err != nil {
		t.Skipf("assets not extracted: %v", err)
	}
	return as
}

func TestExtractedAssets(t *testing.T) {
	as := loadAssets(t)
	for _, role := range []string{"waffleAnim", "squareAnim"} {
		if as.Roles[role] == "" {
			t.Errorf("missing role %s", role)
		}
	}
	for _, snd := range []string{"one", "two", "three", "four", "miss", "tink", "wafflesplat"} {
		if len(as.Sounds[snd]) == 0 {
			t.Errorf("missing sound %s", snd)
		}
	}
	// Unity calls fallingWaffle/waffleSplat, while the bundled file is
	// wafflesplat.wav; runtime keys follow the actual asset filename.
	if _, ok := as.Sounds["waffleSplat"]; ok {
		t.Fatalf("unexpected camel-case sound alias; module should use asset key wafflesplat")
	}
}

func TestControllersAndClips(t *testing.T) {
	as := loadAssets(t)
	for _, want := range []struct {
		ctrl   string
		states []string
	}{
		{"waffle", []string{"Idle", "fall", "IdleFlop"}},
		{"Square", []string{"idleSquare", "Fade"}},
	} {
		ctrl, ok := as.Controllers[want.ctrl]
		if !ok {
			t.Fatalf("missing controller %s", want.ctrl)
		}
		for _, st := range want.states {
			cs, ok := ctrl.States[st]
			if !ok {
				t.Errorf("controller %s missing state %s", want.ctrl, st)
				continue
			}
			if cs.Clip != "" && as.Anims[cs.Clip] == nil {
				t.Errorf("controller %s state %s references missing clip %s", want.ctrl, st, cs.Clip)
			}
		}
	}
	for _, clip := range []string{"Animation/Idle", "Animation/Flop", "Animation/IdleFlop", "Animation/Fade", "Animation/idleSquare"} {
		if as.Anims[clip] == nil {
			t.Errorf("missing clip %s", clip)
		}
	}
}

func TestClipPathCoverageAndSupportedProperties(t *testing.T) {
	as := loadAssets(t)
	scenePaths := map[string]bool{}
	for _, n := range as.Rig.Nodes {
		scenePaths[n.Path] = true
	}
	for clip, anim := range as.Anims {
		root := rootForClip(as, clip)
		checkAnimPaths(t, anim, clip, func(path string) bool {
			if path == "" {
				return scenePaths[root]
			}
			return scenePaths[root+"/"+path]
		})
		for _, attrs := range anim.Floats {
			for attr := range attrs {
				if !supportedFloatAttr(attr) {
					t.Errorf("clip %s uses unsupported float attr %s", clip, attr)
				}
			}
		}
	}
}

func rootForClip(as *kart.Assets, clip string) string {
	switch {
	case strings.Contains(clip, "Fade") || strings.Contains(clip, "idleSquare"):
		return as.Roles["squareAnim"]
	default:
		return as.Roles["waffleAnim"]
	}
}

func checkAnimPaths(t *testing.T, anim *kmdata.Anim, clip string, okPath func(string) bool) {
	t.Helper()
	if anim == nil {
		t.Errorf("clip %s missing", clip)
		return
	}
	paths := map[string]bool{}
	for p := range anim.Pos {
		paths[p] = true
	}
	for p := range anim.Euler {
		paths[p] = true
	}
	for p := range anim.Scale {
		paths[p] = true
	}
	for p := range anim.Sprites {
		paths[p] = true
	}
	for p := range anim.Floats {
		paths[p] = true
	}
	for p := range paths {
		if !okPath(p) {
			t.Errorf("clip %s path %q not found under runtime root", clip, p)
		}
	}
}

func supportedFloatAttr(attr string) bool {
	switch attr {
	case "m_IsActive", "m_Enabled", "m_FlipX", "m_FlipY", "m_SortingOrder", "m_Size.x", "m_Size.y":
		return true
	}
	return strings.HasPrefix(attr, "m_Color.")
}
