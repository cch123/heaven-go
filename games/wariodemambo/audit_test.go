package wariodemambo

import (
	"math"
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

func TestSpotlightPointUsesPrefabWorldPosition(t *testing.T) {
	as := &kart.Assets{
		Rig: kmdata.Rig{Nodes: []kmdata.Node{
			{Name: "Root", Path: "Root", Parent: -1, Pos: [2]float64{2, 3}, Scale: [2]float64{1, 1}},
			{Name: "Pivot", Path: "Root/Pivot", Parent: 0, Pos: [2]float64{1, 0}, RotZ: 0.5, Scale: [2]float64{2, 2}},
			{Name: "Spot", Path: "Root/Pivot/Spot", Parent: 1, Pos: [2]float64{0.5, -1}, Scale: [2]float64{1, 1}},
		}},
	}
	m := &Module{ctx: &engine.Ctx{Assets: as}}

	got := m.point("Root/Pivot/Spot")
	wantWorld := kart.TRS(2, 3, 0, 1, 1).
		Mul(kart.TRS(1, 0, 0.5, 2, 2)).
		Mul(kart.TRS(0.5, -1, 0, 1, 1))
	want := [2]float64{wantWorld.Tx, wantWorld.Ty}
	if got != want {
		t.Fatalf("spotlight point = %#v, want prefab world position %#v", got, want)
	}
}

func TestMamboMaterialNamesPreservedForSharedColorShader(t *testing.T) {
	as := loadAssets(t)
	mats := map[string]int{}
	byPath := map[string]kmdata.Node{}
	for _, n := range as.Rig.Nodes {
		if n.Sprite == "" {
			continue
		}
		if n.Mat != "" {
			mats[n.Mat]++
		}
		byPath[n.Path] = n
	}
	for _, mat := range []string{"MamboShader", "Lights", "FloorLights"} {
		if mats[mat] == 0 {
			t.Fatalf("material %s should be preserved on SpriteRenderer nodes: %#v", mat, mats)
		}
	}
	for path, want := range map[string]string{
		"WarioJumper/WAAAAAAAA/Face/Eyes":              "MamboShader",
		"DancerL/DancerJumper/DancerL/HeadHolder/Head": "MamboShader",
		"LightHolderT/Light1":                          "FloorLights",
		"LightHolderL/Light3":                          "FloorLights",
		"Spotlights/SpotlightL":                        "Lights",
	} {
		if got := byPath[path].Mat; got != want {
			t.Fatalf("%s material = %q, want %q", path, got, want)
		}
	}
	if byPath["WarioJumper/WAAAAAAAA/Squiggly"].Mat == "MamboShader" {
		t.Fatal("Squiggly overlay should not be part of the shared MamboShader recolor material")
	}
}

func TestMamboMainMaterialIncludesFacesAndDancerHeads(t *testing.T) {
	as := loadAssets(t)
	byPath := map[string]kmdata.Node{}
	for _, n := range as.Rig.Nodes {
		byPath[n.Path] = n
	}
	for _, path := range []string{
		"WarioJumper/WAAAAAAAA/Face/Schnoz",
		"WarioJumper/WAAAAAAAA/Face/Eyes",
		"WarioJumper/WAAAAAAAA/Face/Mouth",
		"DancerL/DancerJumper/DancerL/HeadHolder/Head",
		"DancerR/DancerJumper/DancerL/HeadHolder/Head",
	} {
		if got := byPath[path].Mat; got != "MamboShader" {
			t.Fatalf("%s material = %q, want MamboShader", path, got)
		}
	}
}

func TestMamboDoodleMaterialParamsExported(t *testing.T) {
	as := loadAssets(t)
	wantNoise := map[string][2]float64{
		"MamboShader": {15, 15},
		"Lights":      {0, 0},
		"FloorLights": {0, 0},
	}
	for name, noise := range wantNoise {
		mat, ok := as.Materials[name]
		if !ok {
			t.Fatalf("missing sprite material %s in materials.json: %#v", name, as.Materials)
		}
		if mat.Shader.Name != "MamboDoodle" {
			t.Fatalf("%s shader = %q, want MamboDoodle", name, mat.Shader.Name)
		}
		if got := mat.Floats["_DoodleFrameTime"]; math.Abs(got-0.25) > 1e-9 {
			t.Fatalf("%s _DoodleFrameTime = %v, want 0.25", name, got)
		}
		if got := mat.Floats["_DoodleFrameCount"]; math.Abs(got-24) > 1e-9 {
			t.Fatalf("%s _DoodleFrameCount = %v, want 24", name, got)
		}
		max := mat.Colors["_DoodleMaxOffset"]
		if math.Abs(max[0]-0.003) > 1e-9 || math.Abs(max[1]-0.003) > 1e-9 {
			t.Fatalf("%s _DoodleMaxOffset = %#v, want 0.003/0.003", name, max)
		}
		gotNoise := mat.Colors["_DoodleNoiseScale"]
		if gotNoise[0] != noise[0] || gotNoise[1] != noise[1] {
			t.Fatalf("%s _DoodleNoiseScale = %#v, want %#v", name, gotNoise, noise)
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
