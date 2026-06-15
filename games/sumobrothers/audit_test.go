package sumobrothers

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
	as, err := kart.Load("../../assets/sumoBrothers", engine.SampleRate)
	if err != nil {
		t.Skipf("assets not extracted: %v", err)
	}
	return as
}

func TestBindingsComponentsAndSounds(t *testing.T) {
	as := loadAssets(t)
	for field, want := range map[string]string{
		"inuSensei":        "inuSensei",
		"sumoBrotherP":     "sumoBrotherP",
		"sumoBrotherG":     "sumoBrotherG",
		"sumoBrotherPHead": "sumoBrotherP/head/headdy",
		"sumoBrotherGHead": "sumoBrotherG/head/headdy",
		"impact":           "misc/impact",
		"glasses":          "misc/glasses",
		"bgMove":           "backgroundChanges/bgMove",
		"bgStatic":         "backgroundChanges/bgStatic",
		"bgTop":            "background/backgroundExtend",
		"bgBtm":            "background/backgroundExtend2",
	} {
		if got := as.Roles[field]; got != want {
			t.Fatalf("role %s = %q, want %q", field, got, want)
		}
	}
	game, ok := as.Extra.Components["game"]
	if !ok {
		t.Fatal("missing game component")
	}
	if got := game.Refs["backgroundMaterial"]; got != "BGColor" {
		t.Fatalf("backgroundMaterial = %q, want BGColor", got)
	}
	if got := game.Refs["mawashiMaterial"]; got != "Mawashis" {
		t.Fatalf("mawashiMaterial = %q, want Mawashis", got)
	}
	if math.Abs(game.Nums["stompShakeSpeed"]-0.125) > 1e-9 {
		t.Fatalf("stompShakeSpeed = %.6f, want 0.125", game.Nums["stompShakeSpeed"])
	}
	for _, snd := range []string{
		"Goofy", "miss", "pose", "poseSignal", "poseSignalEnd",
		"slap", "slapSignal", "stomp", "stompSignal", "tink", "whiff",
	} {
		if len(as.Sounds[snd]) == 0 {
			t.Fatalf("missing sound %s", snd)
		}
	}
}

func TestControllersStatesClipsAndPaths(t *testing.T) {
	as := loadAssets(t)
	want := map[string][]string{
		"inuSensei": {"InuIdle", "InuBop", "InuBeatChange", "InuAlarm", "InuCrouch", "InuFloat", "InuFloatMiss", "InuBopMiss"},
		"sumoBrotherP": {
			"SumoIdle", "SumoBop", "SumoCrouch", "SumoSlapPrepare", "SumoSlapFront",
			"SumoSlapBack", "SumoSlapToStomp", "SumoSlapMiss", "SumoStompPrepareL",
			"SumoStompPrepareR", "SumoStompL", "SumoStompR", "SumoStompMiss",
			"SumoPoseP1", "SumoPoseP2", "SumoPoseP3", "SumoPoseP4", "SumoPoseP6",
			"SumoPoseG1", "SumoPoseG2", "SumoPoseG3", "SumoPoseG4", "SumoPoseG4Alt", "SumoPoseG6",
			"SumoPosePMiss1", "SumoPoseGMiss1", "SumoPoseSwitch",
		},
		"SumoGHead": {
			"SumoPIdle", "SumoGIdle", "SumoPSlap", "SumoPSlapBarely", "SumoPSlapLook",
			"SumoPSlapLookBarely", "SumoGSlap", "SumoGSlapLook", "SumoPStomp",
			"SumoPStompBarely", "SumoGStomp", "SumoPPose1", "SumoGPose1",
			"SumoPPoseBarely1", "SumoGPoseAlt4", "SumoPMiss",
		},
		"bgGreatWave": {"empty", "GreatWave", "GreatWaveDark", "GreatWaveIdle", "OtaniOniji", "OtaniOnijiDark", "Nerd", "NerdDark"},
		"glasses":     {"glassesGone", "glassesThrow", "glassesLand"},
		"impact":      {"impact", "impactGone"},
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
		if strings.Contains(clip, "/") && !covered[clip] {
			t.Fatalf("clip %s is not driven by any controller state", clip)
		}
	}
}

func TestStompShakeMatchesUnityKeySeries(t *testing.T) {
	m := &Module{shakeSpeed: 0.125}
	m.startStompShake(12)
	if got := len(m.shakeKeys); got != 8 {
		t.Fatalf("shake keys = %d, want 8", got)
	}
	if got := m.cameraShakeAt(12); math.Abs(got-(-0.3)) > 1e-9 {
		t.Fatalf("shake at start = %.6f, want -0.3", got)
	}
	if got := m.cameraShakeAt(12 + 0.125); math.Abs(got-0.3) > 1e-9 {
		t.Fatalf("shake at key1 = %.6f, want 0.3", got)
	}
	if got := m.cameraShakeAt(12 + 0.125*7); math.Abs(got) > 1e-9 {
		t.Fatalf("shake at final key = %.6f, want 0", got)
	}
}

func checkAnimPaths(t *testing.T, root, clip string, anim *kmdata.Anim, nodes map[string]bool) {
	t.Helper()
	for p := range animPathSet(anim) {
		full := root
		if p != "" {
			full += "/" + p
		}
		if nodes[full] || sumoSharedControllerAlias(root, p, nodes) {
			continue
		}
		t.Fatalf("clip %s path %q resolves to missing node %q", clip, p, full)
	}
}

func sumoSharedControllerAlias(root, path string, nodes map[string]bool) bool {
	// Sumo uses a shared body/background controller on multiple roots. Unity
	// silently ignores curves whose child path is absent on the current root;
	// these aliases document the three intentional mismatches found in the
	// serialized prefab/controller pair.
	switch {
	case root == "backgroundChanges/bgStatic" && path == "mask":
		return nodes["backgroundChanges/bgMove/mask"]
	case (root == "sumoBrotherP" || root == "sumoBrotherG") && path == "head/head":
		return nodes[root+"/head/headdy"]
	case root == "sumoBrotherG" && path == "effects/stompEffect2":
		return nodes["sumoBrotherP/effects/stompEffect2"]
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
