package lovelizards

import (
	"strings"
	"testing"

	"hsdemo/kart"
	"hsdemo/kmdata"
)

var expectedClips = []string{
	"Animations/GuideDown",
	"Animations/GuideUp",
	"FemaleLizard/FemaleLizardBopL",
	"FemaleLizard/FemaleLizardBopR",
	"FemaleLizard/FemaleLizardHold",
	"FemaleLizard/FemaleLizardMouthDown",
	"FemaleLizard/FemaleLizardMouthUp",
	"FemaleLizard/FemaleLizardRelease",
	"FemaleLizard/FemaleLizardSlideDown",
	"FemaleLizard/FemaleLizardSlideUp",
	"FemaleLizard/FemaleLizardStepL",
	"FemaleLizard/FemaleLizardStepR",
	"MaleLizard/MaleLizarYawn",
	"MaleLizard/MaleLizardBopL",
	"MaleLizard/MaleLizardBopR",
	"MaleLizard/MaleLizardPrepare",
	"MaleLizard/MaleLizardShakeL",
	"MaleLizard/MaleLizardShakeR",
	"MaleLizard/MaleLizardSmile",
	"MaleLizard/MaleLizardStepL",
	"MaleLizard/MaleLizardStepR",
}

func loadAssets(t *testing.T) *kart.Assets {
	t.Helper()
	as, err := kart.Load("../../assets/loveLizards", 44100)
	if err != nil {
		t.Skipf("assets not extracted: %v", err)
	}
	return as
}

func TestExtractedAssets(t *testing.T) {
	as := loadAssets(t)
	for _, role := range []string{"MaleLizard", "FemaleLizard", "Guide", "background1", "background2", "background3"} {
		if as.Roles[role] == "" {
			t.Errorf("missing role %s", role)
		}
	}
	for _, snd := range []string{"cowbell", "maleShake", "maleYawn", "maleHeart", "female1", "female2", "female3", "female4", "female5", "female6"} {
		if len(as.Sounds[snd]) == 0 {
			t.Errorf("missing sound %s", snd)
		}
	}
}

func TestControllersAndClips(t *testing.T) {
	as := loadAssets(t)
	for _, want := range []struct {
		ctrl   string
		states []string
	}{
		{"MaleLizardAnim", []string{"Idle", "MaleLizardBopL", "MaleLizardBopR", "MaleLizardPrepare", "MaleLizardShakeL", "MaleLizardShakeR", "MaleLizardSmile", "MaleLizardStepL", "MaleLizardStepR", "MaleLizardYawn"}},
		{"FemaleLizardAnim", []string{"Idle", "FemaleLizardBopL", "FemaleLizardBopR", "FemaleLizardHold", "FemaleLizardMouthDown", "FemaleLizardMouthUp", "FemaleLizardRelease", "FemaleLizardSlideDown", "FemaleLizardSlideUp", "FemaleLizardStepL", "FemaleLizardStepR"}},
		{"GuideAnim", []string{"GuideDown", "GuideUp"}},
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
	for _, clip := range expectedClips {
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
	for _, clip := range expectedClips {
		anim := as.Anims[clip]
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

func TestShadowMaterialAndLayeredPlayback(t *testing.T) {
	as := loadAssets(t)
	shadowNodes := 0
	for _, n := range as.Rig.Nodes {
		if n.Mat == "Shadow" && n.Mapped {
			shadowNodes++
		}
	}
	if shadowNodes == 0 {
		t.Fatalf("expected mapped Shadow material nodes")
	}

	sc := kart.NewScene(as)
	female := as.Roles["FemaleLizard"]
	sc.PlayStateLayer("female:hold", female, "FemaleLizardHold", 0, 0.5)
	sc.PlayStateLayer("female:mouth", female, "FemaleLizardMouthUp", 0, 0.5)
	sc.Sample(0)
	if got, _, _ := sc.NodeSprite("FemaleLizard/BodyParent/TailDParent/TailD"); got != "lizards_32" {
		t.Fatalf("hold layer tail sprite = %q, want lizards_32", got)
	}
	if got, _, _ := sc.NodeSprite("FemaleLizard/BodyParent/Body"); got != "lizards_36" {
		t.Fatalf("mouth layer body sprite = %q, want lizards_36", got)
	}
}

func rootForClip(as *kart.Assets, clip string) string {
	switch {
	case strings.HasPrefix(clip, "Animations/"):
		return as.Roles["Guide"]
	case strings.HasPrefix(clip, "FemaleLizard/"):
		return as.Roles["FemaleLizard"]
	default:
		return as.Roles["MaleLizard"]
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
