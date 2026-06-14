package rhythmsheriff

import (
	"strings"
	"testing"

	"hsdemo/kart"
	"hsdemo/kmdata"
	"hsdemo/synth"
)

func loadAssets(t *testing.T) *kart.Assets {
	t.Helper()
	as, err := kart.Load("../../assets/rhythmSheriff", synth.SampleRate)
	if err != nil {
		t.Skipf("assets not extracted: %v", err)
	}
	return as
}

func TestRhythmSheriffBindingsAndComponents(t *testing.T) {
	as := loadAssets(t)
	for _, role := range []string{"dogSheriff", "targetObj", "tumbleweedBack", "tumbleweedFront", "tumbleweedOverlay"} {
		if as.Roles[role] == "" {
			t.Fatalf("missing role %s", role)
		}
	}
	game, ok := as.Extra.Components["game"]
	if !ok {
		t.Fatalf("missing game component")
	}
	for _, n := range []string{"ratPitch", "ratLowerPitch", "ratFinalPitch", "catPitch", "catLowerPitch", "catFinalPitch"} {
		if game.Nums[n] == 0 {
			t.Fatalf("missing pitch %s", n)
		}
	}
	target, ok := as.Extra.Components["target"]
	if !ok || target.Refs["target"] == "" || target.Refs["hole"] == "" {
		t.Fatalf("target component refs = %#v", target)
	}
	if kart.NewTemplate(as, as.Roles["targetObj"]) == nil {
		t.Fatalf("target template missing")
	}
}

func TestRhythmSheriffControllersAndSounds(t *testing.T) {
	as := loadAssets(t)
	want := map[string][]string{
		"DogSheriff": {"Idle", "Bop", "Ready", "Return", "ShootJust", "ShootNG"},
		"board":      {"empty", "Cat", "CatTarget", "CatTargetLeft", "CatTargetRight", "Rat", "RatTarget", "RatTargetLeft", "RatTargetRight", "WhiffSpin"},
	}
	for ctrl, states := range want {
		c, ok := as.Controllers[ctrl]
		if !ok {
			t.Fatalf("missing controller %s", ctrl)
		}
		for _, st := range states {
			cs, ok := c.States[st]
			if !ok {
				t.Errorf("controller %s missing state %s", ctrl, st)
				continue
			}
			if cs.Clip != "" && as.Anims[cs.Clip] == nil {
				t.Errorf("controller %s state %s references missing clip %s", ctrl, st, cs.Clip)
			}
		}
	}
	for _, snd := range []string{"targetCat", "destroyTarget1", "destroyTarget3", "miss", "whiff", "sink", "holster"} {
		if len(as.Sounds[snd]) == 0 {
			t.Fatalf("missing sound %s", snd)
		}
	}
}

func TestRhythmSheriffClipPathCoverageAndProperties(t *testing.T) {
	as := loadAssets(t)
	scenePaths := map[string]bool{}
	for _, n := range as.Rig.Nodes {
		scenePaths[n.Path] = true
	}
	for root, ctrlName := range as.Animators {
		ctrl := as.Controllers[ctrlName]
		for _, st := range ctrl.States {
			if st.Clip == "" || st.Clip == "None" {
				continue
			}
			checkAnimPaths(t, as.Anims[st.Clip], st.Clip, func(path string) bool {
				if path == "" {
					return scenePaths[root]
				}
				return scenePaths[root+"/"+path]
			})
		}
	}
	for clip, anim := range as.Anims {
		for _, attrs := range anim.Floats {
			for attr := range attrs {
				if !supportedFloatAttr(attr) {
					t.Errorf("clip %s uses unsupported float attr %s", clip, attr)
				}
			}
		}
	}
}

func TestRhythmSheriffMappedMaterialsPresent(t *testing.T) {
	as := loadAssets(t)
	seen := map[string]bool{}
	for _, n := range as.Rig.Nodes {
		if n.Mapped {
			seen[n.Mat] = true
		}
	}
	for _, mat := range []string{"Fur", "Clothes", "Bandana", "Gun", "Sky", "Ground", "Rocks", "Bush", "Tear"} {
		if !seen[mat] {
			t.Fatalf("mapped material %s not used by scene", mat)
		}
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
	return strings.HasPrefix(attr, "m_Color.") || strings.HasPrefix(attr, "m_fontColor.")
}
