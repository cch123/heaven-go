package wariodemambo

import (
	"strings"
	"testing"

	"hsdemo/engine"
	"hsdemo/kart"
	"hsdemo/kmdata"
)

func loadAssets(t *testing.T) *kart.Assets {
	t.Helper()
	as, err := kart.Load("../../assets/warioDeMambo", engine.SampleRate)
	if err != nil {
		t.Skipf("assets not extracted: %v", err)
	}
	return as
}

func TestBindingsComponentsTextAndSounds(t *testing.T) {
	as := loadAssets(t)
	for field, want := range map[string]string{
		"commandText":     "TextHolder/Command",
		"endPose":         "endPose",
		"spotlightLTrans": "Spotlights/SpotlightL",
		"spotlightRTrans": "Spotlights/SpotlightR",
		"DancerLSpotPos":  "SpotlightPoints/LeftSpotlight",
		"DancerRSpotPos":  "SpotlightPoints/RightSpotlight",
		"WarioSpotPos":    "SpotlightPoints/SpotlightPlayerLocation",
		"textAnimator":    "TextHolder",
		"dancerLeftAnim":  "DancerL/DancerJumper/DancerL",
		"dancerRightAnim": "DancerR/DancerJumper/DancerL",
		"warioBodyAnim":   "WarioJumper/WAAAAAAAA",
		"warioArmAnim":    "WarioJumper/WAAAAAAAA/HoldingArm",
		"warioFaceAnim":   "WarioJumper/WAAAAAAAA/Face",
		"warioJumpAnim":   "WarioJumper",
		"topLightAnim":    "LightHolderT",
		"leftLightAnim":   "LightHolderL",
		"rightLightAnim":  "LightHolderR",
	} {
		if got := as.Roles[field]; got != want {
			t.Fatalf("role %s = %q, want %q", field, got, want)
		}
	}
	game, ok := as.Extra.Components["game"]
	if !ok {
		t.Fatal("missing game component")
	}
	for field, want := range map[string]string{
		"mainMat":       "MamboShader",
		"lightMat":      "Lights",
		"floorLightMat": "FloorLights",
	} {
		if got := game.Refs[field]; got != want {
			t.Fatalf("game ref %s = %q, want %q", field, got, want)
		}
	}
	if len(as.Texts) != 1 || as.Texts[0].Path != "TextHolder/Command" {
		t.Fatalf("texts = %#v, want TextHolder/Command", as.Texts)
	}
	if err := as.ApplyTexts(); err != nil {
		t.Fatalf("ApplyTexts: %v", err)
	}
	if err := as.SetText("TextHolder/Command", "Your turn!"); err != nil {
		t.Fatalf("SetText: %v", err)
	}
	for _, snd := range []string{
		"left", "right", "jump", "memorize", "four", "three", "two", "one",
		"applause", "ladiesandgentlemen", "wariodemambo", "common_miss",
	} {
		if len(as.Sounds[snd]) == 0 {
			t.Fatalf("missing sound %s", snd)
		}
	}
}

func TestControllersStatesClipsAndPaths(t *testing.T) {
	as := loadAssets(t)
	want := map[string][]string{
		"Command":      {"Idle", "Text"},
		"Dancer":       {"DPreDancePose", "DancerBad", "DancerDance", "DancerFinalLoop", "DancerIdle", "DancerPose1", "DancerPose2", "ReadyDance"},
		"DancerArm":    {"DArmCtoR", "DArmDanceLoop", "DArmIdle", "DArmLtoC", "DArmLtoR", "DArmPose1", "DArmPose2", "DArmRtoC", "DArmRtoL", "DarmCtoL"},
		"DancerHead":   {"DHeadCtoL", "DHeadCtoR", "DHeadDanceLoop", "DHeadIdle", "DHeadLtoC", "DHeadLtoR", "DHeadPose1", "DHeadPose2", "DHeadRtoC", "DHeadRtoL"},
		"DancerJumper": {"DJump", "DJumpIdle"},
		"Player":       {"WDanceLoop", "WahBad", "WahBopHap", "WahBopNorm", "WahBopReady", "WahIdle", "WahPose1", "WahPose2", "WahThink"},
		"PlayerJump":   {"WJump", "WJumpIdle", "WWalkCLC", "WWalkCRC", "WWalkFinal"},
		"WFloorLights": {"WBLightsIdle", "WBLightsStage1", "WBLightsStage2", "WBLightsStage3", "WBLightsStage4", "WBLightsStageAlt", "WBLightsStageAlt2"},
		"WTopLights":   {"WTLightsIdle", "WTLightsStage1", "WTLightsStage2", "WTLightsStage3", "WTLightsStage4", "WTLightsStageAlt", "WTLightsStageAlt2"},
		"WahFace":      {"WFaceCtoL", "WFaceCtoR", "WFaceIdle", "WFaceLtoC", "WFaceLtoR", "WFaceRtoC", "WFaceRtoL"},
		"WahHand":      {"WHandCtoL", "WHandCtoR", "WHandIdle", "WHandLtoC", "WHandLtoR", "WHandRtoC", "WHandRtoL"},
	}
	for ctrl, states := range want {
		c, ok := as.Controllers[ctrl]
		if !ok {
			t.Fatalf("missing controller %s", ctrl)
		}
		for _, st := range states {
			cs, ok := c.States[st]
			if !ok {
				t.Fatalf("controller %s missing state %s", ctrl, st)
			}
			if cs.Clip != "" && as.Anims[cs.Clip] == nil {
				t.Fatalf("controller %s state %s references missing clip %s", ctrl, st, cs.Clip)
			}
		}
	}
	nodes := map[string]bool{}
	for _, n := range as.Rig.Nodes {
		nodes[n.Path] = true
	}
	covered := map[string]bool{}
	for root, ctrlName := range as.Animators {
		ctrl, ok := as.Controllers[ctrlName]
		if !ok {
			t.Fatalf("animator %s references missing controller %s", root, ctrlName)
		}
		for _, st := range ctrl.States {
			if st.Clip == "" {
				continue
			}
			anim := as.Anims[st.Clip]
			if anim == nil {
				t.Fatalf("controller %s references missing clip %s", ctrlName, st.Clip)
			}
			covered[st.Clip] = true
			checkAnimPaths(t, root, st.Clip, anim, nodes)
			checkSupportedAttrs(t, st.Clip, anim)
		}
	}
	for clip := range as.Anims {
		if strings.Contains(clip, "/") && !covered[clip] && !warioIntentionalLooseClip(clip) {
			t.Fatalf("clip %s is not driven by any controller state", clip)
		}
	}
}

func checkAnimPaths(t *testing.T, root, clip string, anim *kmdata.Anim, nodes map[string]bool) {
	t.Helper()
	for p := range animPathSet(anim) {
		full := root
		if p != "" {
			full += "/" + p
		}
		if nodes[full] || warioSharedControllerAlias(root, p, nodes) {
			continue
		}
		t.Fatalf("clip %s path %q resolves to missing node %q", clip, p, full)
	}
}

func warioSharedControllerAlias(root, path string, nodes map[string]bool) bool {
	// The right dancer prefab keeps the child name "DancerL"; both dancers use
	// the same controller and the serialized paths are valid under their roots.
	return false
}

func warioIntentionalLooseClip(clip string) bool {
	switch clip {
	case "Animations/Text/Command", "Animations/Text/Inactive", "Animations/Text/Text":
		return true
	default:
		return false
	}
}

func animPathSet(anim *kmdata.Anim) map[string]bool {
	paths := map[string]bool{}
	for p := range anim.Pos {
		paths[p] = true
	}
	for p := range anim.Scale {
		paths[p] = true
	}
	for p := range anim.Euler {
		paths[p] = true
	}
	for p := range anim.Sprites {
		paths[p] = true
	}
	for p := range anim.Floats {
		paths[p] = true
	}
	return paths
}

func checkSupportedAttrs(t *testing.T, clip string, anim *kmdata.Anim) {
	t.Helper()
	for path, attrs := range anim.Floats {
		for attr := range attrs {
			switch attr {
			case "m_IsActive", "m_Enabled", "m_FlipX", "m_FlipY", "m_SortingOrder",
				"m_Size.x", "m_Size.y", "material._Threshold":
				continue
			}
			if strings.HasPrefix(attr, "m_Color.") ||
				strings.HasPrefix(attr, "m_fontColor.") ||
				strings.HasPrefix(attr, "material._Color.") ||
				strings.HasPrefix(attr, "material._AddColor.") ||
				strings.HasPrefix(attr, "material._BlendColor.") {
				continue
			}
			t.Fatalf("clip %s path %s unsupported float attr %s", clip, path, attr)
		}
	}
}
