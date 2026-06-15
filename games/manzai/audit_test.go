package manzai

import (
	"strings"
	"testing"

	"hsdemo/kart"
	"hsdemo/kmdata"
)

func loadAssets(t *testing.T) *kart.Assets {
	t.Helper()
	as, err := kart.Load("../../assets/manzai", 44100)
	if err != nil {
		t.Skipf("assets not extracted: %v", err)
	}
	return as
}

func TestManzaiControllersClipsAndPaths(t *testing.T) {
	as := loadAssets(t)
	for ctrl, states := range map[string][]string{
		"BothBirdsAnim":    {"Idle", "SlideIn", "SlideOut"},
		"CrowdAnim":        {"Idle", "Bop", "Cheer", "Uproar", "Angry"},
		"DonaiyanenBubble": {"Idle", "Donaiyanen", "DonaiyanenL", "DonaiyanenR"},
		"HaiBubbleL":       {"Idle", "HaiL"},
		"HaiBubbleR":       {"Idle", "HaiR"},
		"RavenAnim":        {"Idle", "Bop", "Talk", "Move", "MoveM", "Ready", "Attack", "Damage", "Spin", "Miss"},
		"StageAnim":        {"Idle", "LightsOn", "LightsOff"},
		"VultureAnim":      {"Idle", "Bop", "Talk", "Move", "Boing", "Damage", "Dodge", "Attack"},
	} {
		c, ok := as.Controllers[ctrl]
		if !ok {
			t.Fatalf("missing controller %s", ctrl)
		}
		for _, st := range states {
			if _, ok := c.States[st]; !ok {
				t.Errorf("controller %s missing state %s", ctrl, st)
			}
		}
	}
	for path, ctrl := range map[string]string{
		"BG":                          "StageAnim",
		"Birds":                       "BothBirdsAnim",
		"Birds/RavenHolder/Raven":     "RavenAnim",
		"Birds/VultureHolder/Vulture": "VultureAnim",
		"Birds/RavenHolder/Bubbles/PivotL/SpeechL": "HaiBubbleL",
		"Birds/RavenHolder/Bubbles/PivotR/SpeechR": "HaiBubbleR",
		"Birds/RavenHolder/Bubbles/PivotD/SpeechD": "DonaiyanenBubble",
		"Crowd": "CrowdAnim",
	} {
		if got := as.Animators[path]; got != ctrl {
			t.Errorf("animator %s = %q, want %q", path, got, ctrl)
		}
	}

	nodes := nodeSet(as)
	covered := map[string]bool{}
	looseClips := map[string]string{
		// CrowdCheerLoop.anim is exported next to the live crowd clips, but
		// CrowdAnim.controller has no state for it and Manzai.cs only dispatches
		// the CrowdAnimationList enum names Idle/Bop/Cheer/Uproar/Angry/Jump.
		// Keep its curves path-checked so a future extraction change cannot
		// silently drop the unused loop asset.
		"Animations/Crowd/CrowdCheerLoop": "Crowd",
	}
	for root, ctrlName := range as.Animators {
		for stName, st := range as.Controllers[ctrlName].States {
			if st.Clip == "" {
				continue
			}
			covered[st.Clip] = true
			a := as.Anims[st.Clip]
			if a == nil {
				t.Errorf("%s/%s missing clip %s", ctrlName, stName, st.Clip)
				continue
			}
			checkAnimPaths(t, a, st.Clip, root, nodes)
			checkSupportedAttrs(t, a, st.Clip)
		}
	}
	for clip, root := range looseClips {
		covered[clip] = true
		a := as.Anims[clip]
		if a == nil {
			t.Errorf("loose clip %q missing", clip)
			continue
		}
		checkAnimPaths(t, a, clip, root, nodes)
		checkSupportedAttrs(t, a, clip)
	}
	for name := range as.Anims {
		if strings.Contains(name, "/") && !covered[name] {
			t.Errorf("clip %q has no controller state", name)
		}
	}
}

func TestManzaiSoundsAndRuntimeEnums(t *testing.T) {
	as := loadAssets(t)
	for _, snd := range []string{
		"hai", "haiAccent", "haiClap", "disappointed", "miss1", "miss2", "missClick", "missWrongButton",
		"boing", "comedy", "donaiyanen1", "donaiyanen2", "donaiyanen3", "donaiyanen4",
		"donaiyanenAccent", "donaiyanenLaugh", "crowdHai1", "crowdHai2",
		"crowdDon1", "crowdDon2", "crowdDon3", "crowdDon4",
	} {
		if _, ok := as.Sounds[snd]; !ok {
			t.Errorf("missing sound %s", snd)
		}
	}
	for _, val := range randomPunValues {
		def, ok := punDefs[val]
		if !ok || def.name == "" {
			t.Fatalf("random pun value %d is not mapped", val)
		}
		if def.name == "SarugaSaru" && (!def.short || def.boingSyllables != 3) {
			t.Fatalf("SarugaSaru short/boing metadata drifted: %#v", def)
		}
	}
	if actionAlt != 3 {
		t.Fatalf("actionAlt = %d, want engine South channel 3", actionAlt)
	}
}

func TestManzaiParticleAndSceneAnchors(t *testing.T) {
	as := loadAssets(t)
	nodes := nodeSet(as)
	for _, path := range []string{
		"Birds/VultureHolder/Vulture/Feathers",
		"Birds/RavenHolder/Raven/POW/Smear",
		"Birds/RavenHolder/Raven/POW/Starburst",
		"Birds/VultureHolder/Vulture/Starburst",
		"Crowd/CrowdHolder",
	} {
		if !nodes[path] {
			t.Errorf("missing anchor %s", path)
		}
	}
	if _, ok := as.Sheet.Sprites["Comedians_8"]; !ok {
		t.Fatalf("manual feather particle sprite Comedians_8 missing")
	}
	pos := nodeWorldPos(as, "Birds/VultureHolder/Vulture/Feathers")
	if pos == ([2]float64{}) {
		t.Fatalf("Feathers world anchor resolved to zero")
	}
}

func nodeSet(as *kart.Assets) map[string]bool {
	out := map[string]bool{}
	for _, n := range as.Rig.Nodes {
		out[n.Path] = true
	}
	return out
}

func checkAnimPaths(t *testing.T, anim *kmdata.Anim, clip, root string, nodes map[string]bool) {
	t.Helper()
	for p := range animPaths(anim) {
		full := root
		if p != "" {
			full = root + "/" + p
		}
		if !nodes[full] {
			t.Errorf("clip %s path %q resolves to missing node %q", clip, p, full)
		}
	}
}

func animPaths(anim *kmdata.Anim) map[string]bool {
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
	return paths
}

func checkSupportedAttrs(t *testing.T, anim *kmdata.Anim, clip string) {
	t.Helper()
	for _, attrs := range anim.Floats {
		for attr := range attrs {
			if !supportedFloatAttr(attr) {
				t.Errorf("clip %s uses unsupported attr %s", clip, attr)
			}
		}
	}
}

func supportedFloatAttr(attr string) bool {
	switch attr {
	case "m_FlipX", "m_FlipY", "m_SortingOrder", "m_IsActive", "m_Enabled", "m_Size.x", "m_Size.y":
		return true
	default:
		return strings.HasPrefix(attr, "m_Color.") || strings.HasPrefix(attr, "m_fontColor.")
	}
}
