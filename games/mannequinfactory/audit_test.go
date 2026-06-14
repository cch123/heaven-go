package mannequinfactory

import (
	"strings"
	"testing"

	"hsdemo/kart"
	"hsdemo/kmdata"
)

var expectedClips = []string{
	"EyeStamps/StampEmpty",
	"EyeStamps/StampJust",
	"Hand/SlapEmpty",
	"Hand/SlapJust",
	"MannequinHead/MannequinHeadGrabbed1",
	"MannequinHead/MannequinHeadGrabbed2",
	"MannequinHead/MannequinHeadMiss",
	"MannequinHead/MannequinHeadMove1",
	"MannequinHead/MannequinHeadMove2",
	"MannequinHead/MannequinHeadMove3",
	"MannequinHead/MannequinHeadSlapped",
	"MannequinHead/MannequinHeadStamp",
	"MannequinHead/MannequinHeadStampMiss",
}

func loadAssets(t *testing.T) *kart.Assets {
	t.Helper()
	as, err := kart.Load("../../assets/mannequinFactory", 44100)
	if err != nil {
		t.Skipf("assets not extracted: %v", err)
	}
	if err := as.ApplyTexts(); err != nil {
		t.Fatalf("apply texts: %v", err)
	}
	return as
}

func TestExtractedAssets(t *testing.T) {
	as := loadAssets(t)
	for _, role := range []string{"HandAnim", "StampAnim", "bg", "SignText", "MannequinHeadObject"} {
		if as.Roles[role] == "" {
			t.Errorf("missing role %s", role)
		}
	}
	for _, snd := range []string{
		"claw1", "claw2", "drum", "drumroll1", "drumroll2", "drumroll3", "drumroll4",
		"drumroll5", "drumroll6", "drumroll7", "eyes", "miss", "slap", "whoosh",
	} {
		if len(as.Sounds[snd]) == 0 {
			t.Errorf("missing sound %s", snd)
		}
	}
	if len(as.Texts) != 1 {
		t.Fatalf("texts = %d, want 1", len(as.Texts))
	}
	if got := as.Texts[0].Path; got != as.Roles["SignText"] {
		t.Fatalf("sign text path = %q, want %q", got, as.Roles["SignText"])
	}
	if _, ok := as.Fonts["FOT-KurokaneStd_Megamix_Modified-EB.otf"]; !ok {
		t.Fatalf("missing sign TMP font")
	}
}

func TestHeadComponentAndTemplate(t *testing.T) {
	as := loadAssets(t)
	head := as.Extra.Components["head"]
	if head.Path != as.Roles["MannequinHeadObject"] {
		t.Fatalf("head component path = %q, want %q", head.Path, as.Roles["MannequinHeadObject"])
	}
	for _, ref := range []string{"headAnim", "headSr", "eyesSr"} {
		if head.Refs[ref] == "" {
			t.Errorf("missing head ref %s", ref)
		}
	}
	if got := len(head.SpriteArrays["heads"]); got != 2 {
		t.Errorf("heads sprite array length = %d, want 2", got)
	}
	if got := len(head.SpriteArrays["eyes"]); got != 2 {
		t.Errorf("eyes sprite array length = %d, want 2", got)
	}
	if tmpl := kart.NewTemplate(as, as.Roles["MannequinHeadObject"]); tmpl == nil {
		t.Fatalf("missing MannequinHead template subtree")
	}
}

func TestControllersAndClips(t *testing.T) {
	as := loadAssets(t)
	for _, want := range []struct {
		ctrl    string
		def     string
		states  []string
		clipFor map[string]string
	}{
		{
			ctrl:   "HandAnim",
			def:    "Nothing",
			states: []string{"Nothing", "SlapEmpty", "SlapJust"},
			clipFor: map[string]string{
				"SlapEmpty": "Hand/SlapEmpty",
				"SlapJust":  "Hand/SlapJust",
			},
		},
		{
			ctrl:   "EyeStampAnim",
			def:    "Nothing",
			states: []string{"Nothing", "StampEmpty", "StampJust"},
			clipFor: map[string]string{
				"StampEmpty": "EyeStamps/StampEmpty",
				"StampJust":  "EyeStamps/StampJust",
			},
		},
		{
			ctrl:   "MannequinHeadAnim",
			def:    "Nothing",
			states: []string{"Nothing", "Move1", "Move2", "Move3", "Slapped", "Stamp", "StampMiss", "Miss", "Grabbed1", "Grabbed2"},
			clipFor: map[string]string{
				"Move1":     "MannequinHead/MannequinHeadMove1",
				"Move2":     "MannequinHead/MannequinHeadMove2",
				"Move3":     "MannequinHead/MannequinHeadMove3",
				"Slapped":   "MannequinHead/MannequinHeadSlapped",
				"Stamp":     "MannequinHead/MannequinHeadStamp",
				"StampMiss": "MannequinHead/MannequinHeadStampMiss",
				"Miss":      "MannequinHead/MannequinHeadMiss",
				"Grabbed1":  "MannequinHead/MannequinHeadGrabbed1",
				"Grabbed2":  "MannequinHead/MannequinHeadGrabbed2",
			},
		},
	} {
		ctrl, ok := as.Controllers[want.ctrl]
		if !ok {
			t.Fatalf("missing controller %s", want.ctrl)
		}
		if ctrl.Default != want.def {
			t.Errorf("controller %s default = %q, want %q", want.ctrl, ctrl.Default, want.def)
		}
		for _, st := range want.states {
			cs, ok := ctrl.States[st]
			if !ok {
				t.Errorf("controller %s missing state %s", want.ctrl, st)
				continue
			}
			if wantClip := want.clipFor[st]; wantClip != "" && cs.Clip != wantClip {
				t.Errorf("controller %s state %s clip = %q, want %q", want.ctrl, st, cs.Clip, wantClip)
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

func rootForClip(as *kart.Assets, clip string) string {
	switch {
	case strings.HasPrefix(clip, "Hand/"):
		return as.Roles["HandAnim"]
	case strings.HasPrefix(clip, "EyeStamps/"):
		return as.Roles["StampAnim"]
	default:
		return as.Roles["MannequinHeadObject"]
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
