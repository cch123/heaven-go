package showtime

import (
	"strings"
	"testing"

	"hsdemo/kart"
	"hsdemo/kmdata"
	"hsdemo/synth"
)

func loadAssets(t *testing.T) *kart.Assets {
	t.Helper()
	as, err := kart.Load("../../assets/showtime", synth.SampleRate)
	if err != nil {
		t.Skipf("assets not extracted: %v", err)
	}
	return as
}

func TestShowtimeBindingsCurvesAndTemplates(t *testing.T) {
	as := loadAssets(t)
	for _, role := range []string{
		"MonkeyAnim", "ButtonAnim", "LauncherAnim", "blockOneAnim", "blockTwoAnim",
		"penguinStart", "ballStart", "leapStart", "fallStart", "destroyerPoint", "slideStart",
	} {
		if as.Roles[role] == "" {
			t.Fatalf("missing role %s", role)
		}
	}
	for _, key := range []string{
		"entryCurve", "hopCurve", "leapCurve", "fallCurve", "exitCurve", "chuteCurve",
		"ballUpCurve", "ballDownCurve",
	} {
		c := as.Extra.Curves[key]
		if len(c.Points) < 2 {
			t.Fatalf("curve %s has %d points", key, len(c.Points))
		}
	}
	for _, root := range []string{"penguinGray", "penguinWhite", "penguinBig", "showtimeBall"} {
		if kart.NewTemplate(as, root) == nil {
			t.Fatalf("missing template %s", root)
		}
	}
}

func TestShowtimeControllersSoundsAndRawClips(t *testing.T) {
	as := loadAssets(t)
	wantStates := map[string][]string{
		"ButtonBase":  {"Idle", "Press"},
		"LauncherNew": {"Idle", "Launch"},
		"Block":       {"Idle", "Squish"},
		"Water":       {"Waves"},
		"WaterHolder": {"Move"},
	}
	for ctrl, states := range wantStates {
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
	for _, clip := range []string{
		"MonkeyNew/Idle", "MonkeyNew/Bop", "MonkeyNew/Hit", "MonkeyNew/Miss",
		"Gray/Idle", "Gray/Land", "Gray/Leap", "Gray/Prepare", "Gray/Catch", "Gray/Slide", "Gray/SlideBall",
		"White/Idle", "White/Land", "White/Leap", "White/Prepare", "White/Catch", "White/Slide", "White/SlideBall",
		"Big/Idle", "Big/Land", "Big/Leap", "Big/Prepare", "Big/Catch", "Big/Slide", "Big/SlideBall",
	} {
		if as.Anims[clip] == nil {
			t.Fatalf("missing raw clip %s", clip)
		}
	}
	for _, snd := range []string{"moving", "hit", "small4", "medium4", "large4"} {
		if len(as.Sounds[snd]) == 0 {
			t.Fatalf("missing sound %s", snd)
		}
	}
}

func TestShowtimeMappedMaterialsPresent(t *testing.T) {
	as := loadAssets(t)
	seen := map[string]bool{}
	for _, n := range as.Rig.Nodes {
		if n.Mapped {
			seen[n.Mat] = true
		}
	}
	for _, mat := range []string{"backgroundRecolorable", "Gray", "White", "Black"} {
		if !seen[mat] {
			t.Fatalf("mapped material %s not used by scene", mat)
		}
	}
}

func TestShowtimeClipPathCoverageAndProperties(t *testing.T) {
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
