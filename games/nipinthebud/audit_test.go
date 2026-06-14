package nipinthebud

import (
	"strings"
	"testing"

	"hsdemo/kart"
	"hsdemo/kmdata"
)

func loadAssets(t *testing.T) *kart.Assets {
	t.Helper()
	as, err := kart.Load("../../assets/nipInTheBud", 44100)
	if err != nil {
		t.Skipf("assets not extracted: %v", err)
	}
	return as
}

func TestExtractedAssets(t *testing.T) {
	as := loadAssets(t)
	for _, role := range []string{"Leilani", "Bubble", "Mosquito", "Mayfly", "mosquitoStart", "mayflyStart", "bg"} {
		if as.Roles[role] == "" {
			t.Errorf("missing role %s", role)
		}
	}
	if kart.NewTemplate(as, as.Roles["Mosquito"]) == nil {
		t.Fatalf("mosquito template %q not extractable", as.Roles["Mosquito"])
	}
	if kart.NewTemplate(as, as.Roles["Mayfly"]) == nil {
		t.Fatalf("mayfly template %q not extractable", as.Roles["Mayfly"])
	}
	for _, key := range []string{
		"mosquito.startCurve", "mosquito.approachCurve", "mosquito.fleeCurve",
		"mayfly.startCurve", "mayfly.approachCurve", "mayfly.fleeCurve", "mayfly.exitCurve",
	} {
		if c := as.Extra.Curves[key]; len(c.Points) < 2 {
			t.Errorf("curve %s has %d points", key, len(c.Points))
		}
	}
	for _, snd := range []string{"mosquito1", "mosquito2", "mayfly1", "mayfly2", "blink1", "blink2", "catch", "barely", "whiff"} {
		if len(as.Sounds[snd]) == 0 {
			t.Errorf("missing sound %s", snd)
		}
	}
}

func TestControllersAndRequiredClips(t *testing.T) {
	as := loadAssets(t)
	for _, want := range []struct {
		ctrl   string
		states []string
	}{
		{"Leilani", []string{"Idle", "Bop", "Prepare", "PrepFace", "Snap", "SnapMiss", "SnapWhiff", "Unprepare", "Neutral", "Happy", "Sad"}},
		{"bubble", []string{"disable", "alert1", "alert2"}},
		{"Mosquito", []string{"Buzz"}},
		{"mayfly", []string{"BuzzMayfly"}},
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
	for _, clip := range []string{
		"Leilani/Bop", "Leilani/Prepare", "Leilani/PrepFace", "Leilani/Snap",
		"Leilani/SnapMiss", "Leilani/SnapWhiff", "Leilani/Unprepare",
		"Leilani/Neutral", "Leilani/Happy", "Leilani/Sad",
		"Bubble/alert1", "Bubble/alert2", "Bubble/disable",
		"Mosquito/Buzz", "Mayfly/BuzzMayfly",
	} {
		if as.Anims[clip] == nil {
			t.Errorf("missing required clip %s", clip)
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
		if root == "" {
			t.Errorf("clip %s has no runtime root", clip)
			continue
		}
		checkAnimPaths(t, anim, clip, func(path string) bool {
			if path == "" {
				return scenePaths[root]
			}
			return scenePaths[root+"/"+path]
		})
		for path, attrs := range anim.Floats {
			_ = path
			for attr := range attrs {
				if !supportedFloatAttr(attr) {
					t.Errorf("clip %s uses unsupported float attr %s", clip, attr)
				}
			}
		}
	}
	// The upstream prefab has a dangling controller guid on this flower
	// Animator. It is intentionally not exported; Leilani root clips drive this
	// node directly, so gameplay does not depend on that missing controller.
	if _, ok := as.Animators["Scene/Leilani/her head/flower"]; ok {
		t.Fatalf("dangling flower controller should not be exported as animator binding")
	}
}

func rootForClip(as *kart.Assets, clip string) string {
	switch {
	case strings.HasPrefix(clip, "Bubble/") || clip == "alert1" || clip == "alert2" || clip == "disable":
		return as.Roles["Bubble"]
	case strings.HasPrefix(clip, "Mosquito/") || clip == "Buzz":
		return as.Roles["Mosquito"]
	case strings.HasPrefix(clip, "Mayfly/") || clip == "BuzzMayfly":
		return as.Roles["Mayfly"]
	default:
		return as.Roles["Leilani"]
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
	case "m_FlipX", "m_FlipY", "m_SortingOrder", "m_IsActive", "m_Enabled", "m_Size.x", "m_Size.y":
		return true
	}
	return strings.HasPrefix(attr, "m_Color.")
}
