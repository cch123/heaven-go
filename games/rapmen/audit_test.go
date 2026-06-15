package rapmen

import (
	"strings"
	"testing"

	"hsdemo/kart"
	"hsdemo/kmdata"
)

func loadAssets(t *testing.T) *kart.Assets {
	t.Helper()
	as, err := kart.Load("../../assets/rapMen", 44100)
	if err != nil {
		t.Skipf("assets not extracted: %v", err)
	}
	return as
}

func TestRapMenBindings(t *testing.T) {
	as := loadAssets(t)
	for _, role := range []string{
		"rapperRed", "rapperYellow", "rapperCherry", "rapperBlue",
		"rapperRedObj", "rapperYellowObj", "rapperCherryObj", "rapperBlueObj",
		"rapText", "uhnParticle", "background",
	} {
		if as.Roles[role] == "" {
			t.Fatalf("missing role %s", role)
		}
	}
	game := as.Extra.Components["game"]
	for _, ref := range []string{"backgroundMaterial", "speakerMaterial"} {
		if game.Refs[ref] == "" {
			t.Fatalf("missing game ref %s", ref)
		}
	}
	if got := len(game.RefArrays["justParticles"]); got != 9 {
		t.Fatalf("justParticles count = %d, want 9", got)
	}
	if err := as.ApplyTexts(); err != nil {
		t.Fatalf("ApplyTexts: %v", err)
	}
	if err := as.SetText(as.Roles["rapText"], "Yo"); err != nil {
		t.Fatalf("SetText rapText: %v", err)
	}
}

func TestRapMenControllersAndSounds(t *testing.T) {
	as := loadAssets(t)
	want := map[string][]string{
		"RedRapper":    {"idle", "bop", "yo", "kamone", "saiko", "just", "justEnd"},
		"YellowRapper": {"idle", "bop", "prepare", "just", "miss"},
		"RedWoman":     {"idle", "bop", "yo", "kamone", "saiko", "just", "justEnd"},
		"BlueWoman":    {"idle", "bop", "prepare", "just", "miss"},
		"background":   {"backgroundMen", "backgroundWomen"},
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
	for _, sound := range []string{
		"yo", "yeah", "wakari1", "oyatsu1", "desuA", "kaA", "kamone1", "saiko1",
		"drum", "cymbal", "uhn", "uhnnn", "miss", "whiff",
		"rapWomen/yoW", "rapWomen/yeahW", "rapWomen/uhnW", "rapWomen/uhnnnW",
		"rapWomen/desuAW", "rapWomen/kamone1W", "rapWomen/saikoA1W",
	} {
		if len(as.Sounds[sound]) == 0 {
			t.Fatalf("missing sound %s", sound)
		}
	}
}

func TestRapMenClipPathCoverageAndProperties(t *testing.T) {
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
	return strings.HasPrefix(attr, "m_Color.") ||
		strings.HasPrefix(attr, "m_fontColor.") ||
		strings.HasPrefix(attr, "material._Color.") ||
		strings.HasPrefix(attr, "material._AddColor.") ||
		strings.HasPrefix(attr, "material._BlendColor.")
}
