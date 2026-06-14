package rhythmtestgba

import (
	"strings"
	"testing"

	"hsdemo/kart"
	"hsdemo/kmdata"
	"hsdemo/synth"
)

func loadAssets(t *testing.T) *kart.Assets {
	t.Helper()
	as, err := kart.Load("../../assets/rhythmTestGBA", synth.SampleRate)
	if err != nil {
		t.Skipf("assets not extracted: %v", err)
	}
	return as
}

func TestRhythmTestGBAExtractedBindings(t *testing.T) {
	as := loadAssets(t)
	for _, role := range []string{
		"noteFlash", "screenText", "buttonAnimator", "flashAnimator",
		"numberBGAnimator", "numberAnimator", "textAnimator",
	} {
		if as.Roles[role] == "" {
			t.Fatalf("missing role %s", role)
		}
	}
	for _, snd := range []string{"blip", "blip2", "blip3", "end_ding", "press"} {
		if len(as.Sounds[snd]) == 0 {
			t.Fatalf("missing sound %s", snd)
		}
	}
}

func TestRhythmTestGBATextExtraction(t *testing.T) {
	as := loadAssets(t)
	if len(as.Texts) != 1 || as.Texts[0].Path != "Text" {
		t.Fatalf("texts = %#v, want Text node", as.Texts)
	}
	if _, ok := as.Fonts["FOT-KurokaneStd_Megamix_Modified-EB.otf"]; !ok {
		t.Fatalf("missing TMP source font")
	}
	if err := as.ApplyTexts(); err != nil {
		t.Fatalf("apply texts: %v", err)
	}
	if err := as.SetText("Text", "Get ready..."); err != nil {
		t.Fatalf("set text: %v", err)
	}
	if err := as.SetText("Text", ""); err != nil {
		t.Fatalf("clear text: %v", err)
	}
}

func TestRhythmTestGBAControllersCoverScriptedStates(t *testing.T) {
	as := loadAssets(t)
	want := map[string][]string{
		"Button":     {"Idle", "Press"},
		"Flash":      {"Idle", "KTBPulse"},
		"BG":         {"Idle", "Static", "FlashBG", "FlashHit"},
		"Number":     {"Idle", "One", "Two", "Three", "Four", "Five", "Six", "Seven", "Eight", "Nine", "Zero"},
		"Text (TMP)": {"TextIdle", "TextFlash"},
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
	if anim := as.Anims["Text/TextGone"]; anim == nil {
		t.Fatalf("missing TextGone clip")
	} else if len(anim.Pos)+len(anim.Euler)+len(anim.Scale)+len(anim.Sprites)+len(anim.Floats) != 0 {
		t.Fatalf("TextGone should remain an empty no-op clip, got %#v", anim)
	}
}

func TestRhythmTestGBAClipPathCoverageAndProperties(t *testing.T) {
	as := loadAssets(t)
	scenePaths := map[string]bool{}
	for _, n := range as.Rig.Nodes {
		scenePaths[n.Path] = true
	}
	for root, ctrlName := range as.Animators {
		ctrl := as.Controllers[ctrlName]
		for state, st := range ctrl.States {
			if st.Clip == "" || st.Clip == "None" {
				continue
			}
			checkAnimPaths(t, as.Anims[st.Clip], st.Clip, func(path string) bool {
				if path == "" {
					return scenePaths[root]
				}
				return scenePaths[root+"/"+path]
			})
			if as.Anims[st.Clip] == nil {
				t.Errorf("controller %s state %s missing clip %s", ctrlName, state, st.Clip)
			}
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
	return strings.HasPrefix(attr, "m_Color.") || strings.HasPrefix(attr, "m_fontColor.")
}
