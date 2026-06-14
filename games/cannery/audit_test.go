package cannery

import (
	"strings"
	"testing"

	"hsdemo/kart"
	"hsdemo/kmdata"
)

var expectedClips = []string{
	"Alarm/AlarmBop",
	"AlarmFlash/AlarmFlash",
	"BG/ClawConveyorScroll",
	"BG/ConveyorScroll",
	"Can/CanCan",
	"Can/CanFlip",
	"Can/CanMove",
	"Can/CanReopen",
	"Canner/CannerCan",
	"Canner/CannerCanBarely",
	"Canner/CannerCanEmpty",
	"ConveyorBelt/ConveyorBeltMove",
}

func loadAssets(t *testing.T) *kart.Assets {
	t.Helper()
	as, err := kart.Load("../../assets/cannery", 44100)
	if err != nil {
		t.Skipf("assets not extracted: %v", err)
	}
	return as
}

func TestExtractedAssets(t *testing.T) {
	as := loadAssets(t)
	for _, role := range []string{"can", "blackout", "conveyorBeltAnim", "alarmAnim", "dingAnim", "cannerAnim"} {
		if as.Roles[role] == "" {
			t.Errorf("missing role %s", role)
		}
	}
	for _, snd := range []string{"can", "ding"} {
		if len(as.Sounds[snd]) == 0 {
			t.Errorf("missing sound %s", snd)
		}
	}
	if got := len(as.Extra.RefArrays["bgAnims"]); got != 3 {
		t.Fatalf("bgAnims length = %d, want 3", got)
	}
	if tmpl := kart.NewTemplate(as, as.Roles["can"]); tmpl == nil {
		t.Fatalf("missing can template")
	}
}

func TestCanComponentAndMappedAlarm(t *testing.T) {
	as := loadAssets(t)
	can := as.Extra.Components["can"]
	if can.Path != as.Roles["can"] {
		t.Fatalf("can component path = %q, want %q", can.Path, as.Roles["can"])
	}
	if can.Refs["anim"] != as.Roles["can"] {
		t.Fatalf("can anim ref = %q, want %q", can.Refs["anim"], as.Roles["can"])
	}
	mapped := map[string]bool{}
	for _, n := range as.Rig.Nodes {
		if n.Mapped {
			mapped[n.Path] = true
			if n.Mat != "AlarmMat" {
				t.Fatalf("mapped node %s uses material %q, want AlarmMat", n.Path, n.Mat)
			}
		}
	}
	for _, path := range []string{as.Roles["alarmAnim"], as.Roles["dingAnim"]} {
		if !mapped[path] {
			t.Fatalf("expected mapped AlarmMat node %s", path)
		}
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
			ctrl:   "AlarmAnim",
			def:    "Start",
			states: []string{"Start", "Bop"},
			clipFor: map[string]string{
				"Bop": "Alarm/AlarmBop",
			},
		},
		{
			ctrl:   "AlarmFlashAnim",
			def:    "Start",
			states: []string{"Start", "Ding"},
			clipFor: map[string]string{
				"Ding": "AlarmFlash/AlarmFlash",
			},
		},
		{
			ctrl:   "CanAnim",
			def:    "Start",
			states: []string{"Start", "Move", "Flip", "Can", "Reopen"},
			clipFor: map[string]string{
				"Move":   "Can/CanMove",
				"Flip":   "Can/CanFlip",
				"Can":    "Can/CanCan",
				"Reopen": "Can/CanReopen",
			},
		},
		{
			ctrl:   "CannerAnim",
			def:    "Start",
			states: []string{"Start", "Can", "CanBarely", "CanEmpty"},
			clipFor: map[string]string{
				"Can":       "Canner/CannerCan",
				"CanBarely": "Canner/CannerCanBarely",
				"CanEmpty":  "Canner/CannerCanEmpty",
			},
		},
		{
			ctrl:   "Conveyor1",
			def:    "Scroll",
			states: []string{"Start", "Scroll", "ConveyorScroll"},
			clipFor: map[string]string{
				"Scroll":         "BG/ClawConveyorScroll",
				"ConveyorScroll": "BG/ConveyorScroll",
			},
		},
		{
			ctrl:   "ConveyorBeltAnim",
			def:    "Start",
			states: []string{"Start", "Move"},
			clipFor: map[string]string{
				"Move": "ConveyorBelt/ConveyorBeltMove",
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
		roots := rootsForClip(as, clip)
		for _, root := range roots {
			checkAnimPaths(t, anim, clip, func(path string) bool {
				if path == "" {
					return scenePaths[root]
				}
				return scenePaths[root+"/"+path]
			})
		}
		for _, attrs := range anim.Floats {
			for attr := range attrs {
				if !supportedFloatAttr(attr) {
					t.Errorf("clip %s uses unsupported float attr %s", clip, attr)
				}
			}
		}
	}
}

func rootsForClip(as *kart.Assets, clip string) []string {
	switch {
	case strings.HasPrefix(clip, "Alarm/"):
		return []string{as.Roles["alarmAnim"]}
	case strings.HasPrefix(clip, "AlarmFlash/"):
		return []string{as.Roles["dingAnim"]}
	case strings.HasPrefix(clip, "BG/"):
		return as.Extra.RefArrays["bgAnims"]
	case strings.HasPrefix(clip, "Can/"):
		return []string{as.Roles["can"]}
	case strings.HasPrefix(clip, "Canner/"):
		return []string{as.Roles["cannerAnim"]}
	default:
		return []string{as.Roles["conveyorBeltAnim"]}
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
