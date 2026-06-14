package loverap

import (
	"strings"
	"testing"

	"hsdemo/kart"
	"hsdemo/kmdata"
	"hsdemo/synth"
)

func loadAssets(t *testing.T) *kart.Assets {
	t.Helper()
	as, err := kart.Load("../../assets/loveRap", synth.SampleRate)
	if err != nil {
		t.Skipf("assets not extracted: %v", err)
	}
	return as
}

func TestLoveRapBindingsAndTexts(t *testing.T) {
	as := loadAssets(t)
	for _, role := range []string{
		"playerRapper", "playerBody", "playerLegs", "playerFace", "playerMouth", "playerFlash", "playerBubble", "playerText",
		"cpuRapper", "cpuBody", "cpuLegs", "cpuFace", "cpuMouth", "cpuFlash", "cpuBubble", "cpuHeadTrans", "cpuText",
		"mcMouth", "mcBody", "mcLegs", "car", "car_lights", "mcBubble", "mcText",
		"bgSR", "bgWetSR",
	} {
		if as.Roles[role] == "" {
			t.Fatalf("missing role %s", role)
		}
	}
	if err := as.ApplyTexts(); err != nil {
		t.Fatalf("ApplyTexts: %v", err)
	}
	wantTexts := map[string]bool{
		as.Roles["playerText"]: true,
		as.Roles["cpuText"]:    true,
		as.Roles["mcText"]:     true,
	}
	for _, tn := range as.Texts {
		delete(wantTexts, tn.Path)
	}
	for p := range wantTexts {
		t.Fatalf("missing TMP text node %s", p)
	}
}

func TestLoveRapControllersAndSounds(t *testing.T) {
	as := loadAssets(t)
	want := map[string][]string{
		"Rapper":      {"idle", "Beat"},
		"rap_body":    {"idle", "D", "DB", "H", "HB", "MD", "MDB", "S", "SB", "cough"},
		"rap_foot":    {"A", "B"},
		"rap_eye":     {"A", "B", "C", "D", "E", "F", "G"},
		"rap_mouth":   {"idle", "D", "H", "HB", "H_ura", "MD", "MD_ura", "Miss", "S", "SB", "Cough"},
		"rappercolor": {"idle", "FadeOut"},
		"Body":        {"idle", "D", "DB", "H", "HB", "MD", "MDB", "S", "SB", "blowLoop", "blowLoopEnd"},
		"Legs":        {"A", "B", "Beat"},
		"Mouth":       {"idle", "D", "H", "H_ura", "MD", "MD_ura", "S"},
		"bubble":      {"idle", "Appear", "Hide", "AppearMiss", "HideMiss"},
		"CarHolder":   {"idle", "beat"},
		"Lights":      {"idle", "light"},
		"Rain":        {"idle", "drop"},
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
	for _, snd := range loveRapSounds {
		if len(as.Sounds[snd]) == 0 {
			t.Fatalf("missing sound %s", snd)
		}
	}
}

func TestLoveRapClipPathCoverageAndProperties(t *testing.T) {
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

var loveRapSounds = []string{
	"common_miss",
	"Woman/SE_RAP_EN_WOMAN_DAISUKI_1", "Woman/SE_RAP_EN_WOMAN_DAISUKI_2", "Woman/SE_RAP_EN_WOMAN_DAISUKI_3",
	"Woman/SE_RAP_EN_WOMAN_MAJI_1", "Woman/SE_RAP_EN_WOMAN_MAJI_2", "Woman/SE_RAP_EN_WOMAN_MAJI_3", "Woman/SE_RAP_EN_WOMAN_MAJI_4", "Woman/SE_RAP_EN_WOMAN_MAJI_5",
	"Woman/SE_RAP_EN_WOMAN_MAJI_OFFBEAT_1", "Woman/SE_RAP_EN_WOMAN_MAJI_OFFBEAT_2", "Woman/SE_RAP_EN_WOMAN_MAJI_OFFBEAT_3", "Woman/SE_RAP_EN_WOMAN_MAJI_OFFBEAT_4", "Woman/SE_RAP_EN_WOMAN_MAJI_OFFBEAT_5",
	"Woman/SE_RAP_EN_WOMAN_HONTO_1", "Woman/SE_RAP_EN_WOMAN_HONTO_2",
	"Woman/SE_RAP_EN_WOMAN_HONTO_OFFBEAT_1", "Woman/SE_RAP_EN_WOMAN_HONTO_OFFBEAT_2", "Woman/SE_RAP_EN_WOMAN_HONTO_OFFBEAT_3",
	"Woman/SE_RAP_EN_WOMAN_SUKINANDA_OFFBEAT_1", "Woman/SE_RAP_EN_WOMAN_SUKINANDA_OFFBEAT_2", "Woman/SE_RAP_EN_WOMAN_SUKINANDA_OFFBEAT_3", "Woman/SE_RAP_EN_WOMAN_SUKINANDA_OFFBEAT_4",
	"Left/SE_RAP_EN_MAN_LEFT_DAISUKI_1", "Left/SE_RAP_EN_MAN_LEFT_DAISUKI_2", "Left/SE_RAP_EN_MAN_LEFT_DAISUKI_3",
	"Left/SE_RAP_EN_MAN_LEFT_MAJI_1", "Left/SE_RAP_EN_MAN_LEFT_MAJI_2", "Left/SE_RAP_EN_MAN_LEFT_MAJI_3", "Left/SE_RAP_EN_MAN_LEFT_MAJI_4", "Left/SE_RAP_EN_MAN_LEFT_MAJI_5",
	"Left/SE_RAP_EN_MAN_LEFT_MAJI_OFFBEAT_1", "Left/SE_RAP_EN_MAN_LEFT_MAJI_OFFBEAT_2", "Left/SE_RAP_EN_MAN_LEFT_MAJI_OFFBEAT_3", "Left/SE_RAP_EN_MAN_LEFT_MAJI_OFFBEAT_4", "Left/SE_RAP_EN_MAN_LEFT_MAJI_OFFBEAT_5",
	"Left/SE_RAP_EN_MAN_LEFT_HONTO_1", "Left/SE_RAP_EN_MAN_LEFT_HONTO_2",
	"Left/SE_RAP_EN_MAN_LEFT_HONTO_OFFBEAT_1", "Left/SE_RAP_EN_MAN_LEFT_HONTO_OFFBEAT_2", "Left/SE_RAP_EN_MAN_LEFT_HONTO_OFFBEAT_3",
	"Left/SE_RAP_EN_MAN_LEFT_SUKINANDA_1", "Left/SE_RAP_EN_MAN_LEFT_SUKINANDA_2", "Left/SE_RAP_EN_MAN_LEFT_SUKINANDA_3", "Left/SE_RAP_EN_MAN_LEFT_SUKINANDA_4",
	"Right/SE_RAP_EN_MAN_RIGHT_DAISUKI_1", "Right/SE_RAP_EN_MAN_RIGHT_DAISUKI_2", "Right/SE_RAP_EN_MAN_RIGHT_DAISUKI_3",
	"Right/SE_RAP_EN_MAN_RIGHT_MAJI_1", "Right/SE_RAP_EN_MAN_RIGHT_MAJI_2", "Right/SE_RAP_EN_MAN_RIGHT_MAJI_3", "Right/SE_RAP_EN_MAN_RIGHT_MAJI_4", "Right/SE_RAP_EN_MAN_RIGHT_MAJI_5",
	"Right/SE_RAP_EN_MAN_RIGHT_MAJI_OFFBEAT_1", "Right/SE_RAP_EN_MAN_RIGHT_MAJI_OFFBEAT_2", "Right/SE_RAP_EN_MAN_RIGHT_MAJI_OFFBEAT_3", "Right/SE_RAP_EN_MAN_RIGHT_MAJI_OFFBEAT_4", "Right/SE_RAP_EN_MAN_RIGHT_MAJI_OFFBEAT_5",
	"Right/SE_RAP_EN_MAN_RIGHT_HONTO_1", "Right/SE_RAP_EN_MAN_RIGHT_HONTO_2",
	"Right/SE_RAP_EN_MAN_RIGHT_HONTO_OFFBEAT_1", "Right/SE_RAP_EN_MAN_RIGHT_HONTO_OFFBEAT_2", "Right/SE_RAP_EN_MAN_RIGHT_HONTO_OFFBEAT_3",
	"Right/SE_RAP_EN_MAN_RIGHT_SUKINANDA_1", "Right/SE_RAP_EN_MAN_RIGHT_SUKINANDA_2", "Right/SE_RAP_EN_MAN_RIGHT_SUKINANDA_3", "Right/SE_RAP_EN_MAN_RIGHT_SUKINANDA_4",
}
