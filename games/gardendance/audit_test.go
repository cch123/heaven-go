package gardendance

import (
	"strings"
	"testing"

	"hsdemo/kart"
	"hsdemo/kmdata"
	"hsdemo/synth"
)

func loadAssets(t *testing.T) *kart.Assets {
	t.Helper()
	as, err := kart.Load("../../assets/gardenDance", synth.SampleRate)
	if err != nil {
		t.Skipf("assets not extracted: %v", err)
	}
	return as
}

func TestGardenDanceBindingsFlowersAndMaterials(t *testing.T) {
	as := loadAssets(t)
	for _, role := range []string{"flowerPlayer", "sunAnim", "birdAnim"} {
		if as.Roles[role] == "" {
			t.Fatalf("missing role %s", role)
		}
	}
	flowers := as.Extra.RefArrays["flowers"]
	if len(flowers) != 3 {
		t.Fatalf("flowers ref array len = %d, want 3", len(flowers))
	}
	seenFlower := map[string]bool{}
	for _, comp := range as.Extra.Components {
		if comp.Refs["anim"] == "" {
			t.Fatalf("flower component %s missing anim ref", comp.Path)
		}
		seenFlower[comp.Path] = true
	}
	for _, p := range append([]string{as.Roles["flowerPlayer"]}, flowers...) {
		if !seenFlower[p] {
			t.Fatalf("missing flower component for %s", p)
		}
	}
	seenMat := map[string]bool{}
	for _, n := range as.Rig.Nodes {
		if n.Mapped {
			seenMat[n.Mat] = true
		}
	}
	for _, mat := range []string{"SkyMat", "GrassMat", "DirtMat", "SunMat", "SunFaceMat", "FlowerPurpleMat", "FlowerGreenMat", "FlowerRedMat", "FlowerSunMat"} {
		if !seenMat[mat] {
			t.Fatalf("mapped material %s not used by scene", mat)
		}
	}
}

func TestGardenDanceControllersAndSounds(t *testing.T) {
	as := loadAssets(t)
	want := map[string][]string{
		"FlowerAnim": {"Idle", "Bop", "DanceL", "DanceR", "PDanceL", "PDanceR", "PoseL", "PoseR", "TripletL", "TripletR", "IdleFace", "MissFace", "Glare", "Blink", "Barely", "PoseFace", "TripletFace", "Stare"},
		"SunAnim":    {"Idle", "Bop", "Enter", "Leave", "Hide"},
		"BirdAnim":   {"Stationary", "Idle", "Shut", "FlyLeft", "FlyRight", "Flap", "Whistle"},
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
	for _, snd := range []string{"dance", "nearMiss", "sparkle", "sway", "tripletWhistle", "whistle1", "whistle2", "whistle3", "miss", "pose"} {
		if len(as.Sounds[snd]) == 0 {
			t.Fatalf("missing sound %s", snd)
		}
	}
}

func TestGardenDanceClipPathCoverageAndProperties(t *testing.T) {
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
	return strings.HasPrefix(attr, "m_Color.") || strings.HasPrefix(attr, "m_fontColor.")
}
