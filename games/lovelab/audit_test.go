package lovelab

import (
	"strings"
	"testing"

	"hsdemo/engine"
	"hsdemo/games/internal/particlefx"
	"hsdemo/kart"
	"hsdemo/kmdata"
)

func loadAssets(t *testing.T) *kart.Assets {
	t.Helper()
	as, err := kart.Load("../../assets/loveLab", engine.SampleRate)
	if err != nil {
		t.Skipf("assets not extracted: %v", err)
	}
	return as
}

func TestBindingsComponentsTemplatesAndSounds(t *testing.T) {
	as := loadAssets(t)
	for field, want := range map[string]string{
		"labGuy":              "Guy",
		"labGuyHead":          "Guy/Head/HeadHolder",
		"labGuyArm":           "Guy/ArmHolder/Arm",
		"labGirl":             "Girl",
		"labGirlHead":         "Girl/Head/HeadHolder",
		"labGirlArm":          "Girl/ArmHolder/Arm",
		"labAssistant":        "Assistant",
		"labAssistantHead":    "Assistant/Head/HeadHolder",
		"labAssistantArm":     "Assistant/ArmHolder/Arm",
		"boyFlaskBreak":       "BoyFlaskBreak/FlaskBreak",
		"girlFlaskBreak":      "GirlFlaskBreak/FlaskBreak",
		"heartBox":            "HeartBox",
		"boxPerson":           "SunsetBg/BoxPerson",
		"boxPersonDay":        "DayBg/BoxPerson",
		"spotlightShader":     "Shaders",
		"spotlightShaderCone": "Shaders (spot)",
		"spotlightCone":       "Shaders (spot)/spotlight",
		"clouds":              "SunsetBg/CloudHolder",
		"sunsetBG":            "SunsetBg",
		"dayBG":               "DayBg",
		"endPoint":            "HeartBox/EndPoint",
	} {
		if got := as.Roles[field]; got != want {
			t.Fatalf("role %s = %q, want %q", field, got, want)
		}
	}
	game, ok := as.Extra.Components["game"]
	if !ok {
		t.Fatal("missing game component")
	}
	if got := len(game.Lists["flaskBouncePath"]); got != 8 {
		t.Fatalf("flaskBouncePath = %d, want 8", got)
	}
	paths := loadFlaskPaths(game.Lists["flaskBouncePath"])
	for _, name := range []string{
		"GuyFlaskFastIn", "GuyFlaskSlowIn", "GirlFastFlaskIn",
		"GirlSlowFlaskIn", "GirlMidSlowFlaskIn", "GirlFlaskMiss", "WeirdFlaskIn",
	} {
		if len(paths[name].points) < 2 {
			t.Fatalf("missing or incomplete path %s", name)
		}
	}
	if got := as.Extra.Strings["flaskArcToBoy"]; len(got) != 2 || got[0] != "GuyFlaskFastIn" || got[1] != "GuyFlaskSlowIn" {
		t.Fatalf("flaskArcToBoy = %#v", got)
	}
	if got := as.Extra.Strings["flaskArcToGirl"]; len(got) != 3 || got[2] != "GirlMidSlowFlaskIn" {
		t.Fatalf("flaskArcToGirl = %#v", got)
	}
	for _, root := range []string{"Flask", "GirlFlask", "Hearts", "GirlHearts", "CompleteHearts"} {
		if tmpl := templateLast(as, root); tmpl == nil {
			t.Fatalf("missing template root %s", root)
		}
	}
	for _, comp := range []string{"heart0", "heart1", "heart2"} {
		if as.Extra.Components[comp].Path == "" {
			t.Fatalf("missing heart component %s", comp)
		}
	}
	for _, snd := range []string{
		"bagHeart", "bagHeartLast", "heartsCombine", "leftCatch", "leftThrow",
		"rightCatch", "rightThrow", "rightThrowNoShake", "shakeDown", "shakeUp",
		"common_miss", "common_nearMiss",
	} {
		if len(as.Sounds[snd]) == 0 {
			t.Fatalf("missing sound %s", snd)
		}
	}
}

func TestFlaskBreakParticleSystems(t *testing.T) {
	as := loadAssets(t)
	seen := map[string]bool{}
	for _, ps := range as.Particles.Systems {
		if seen[ps.Path] {
			t.Fatalf("duplicate ParticleSystem path %q", ps.Path)
		}
		seen[ps.Path] = true
	}
	roots := particlefx.Roots(as)
	for _, root := range []string{"BoyFlaskBreak/FlaskBreak", "GirlFlaskBreak/FlaskBreak"} {
		systems := roots[root]
		if len(systems) != 12 {
			t.Fatalf("%s systems = %d, want 12 official child systems", root, len(systems))
		}
		for _, want := range []string{"lovelabmain_10", "lovelabmain_49", "lovelabmain_53"} {
			if !particleRootUsesSprite(systems, want) {
				t.Fatalf("%s missing particle sprite %s", root, want)
			}
		}
	}
}

func TestGirlPassCuesMatchUnityInputSemantics(t *testing.T) {
	cues := buildGirlPassCues(12, 8, 10, 1, 0.5, []shakeEvt{
		{beat: 8, length: 1, speed: flaskMidSlow},
		{beat: 9, length: 1, speed: flaskMidSlow},
	})
	want := []girlPassCue{
		{beat: 13.5, kind: girlCueCatch},
		{beat: 14.5, kind: girlCueAnyDPadUp},
		{beat: 14.5, kind: girlCueAutoplayDown},
		{beat: 15.5, kind: girlCueRelease},
	}
	if len(cues) != len(want) {
		t.Fatalf("girl pass cues = %d, want %d: %#v", len(cues), len(want), cues)
	}
	for i := range want {
		if cues[i] != want[i] {
			t.Fatalf("cue %d = %#v, want %#v", i, cues[i], want[i])
		}
	}
}

func particleRootUsesSprite(systems []kmdata.ParticleSystem, sprite string) bool {
	for _, ps := range systems {
		for _, got := range ps.TextureSheet.Sprites {
			if got == sprite {
				return true
			}
		}
	}
	return false
}

func TestControllersStatesClipsAndPaths(t *testing.T) {
	as := loadAssets(t)
	want := map[string][]string{
		"Arm": {
			"ArmIdle", "GrabFlask", "MittenGrab", "MittenGrabStart", "MittenIdle",
			"MittenLetGo", "ShakeFlaskDown", "ShakeFlaskUp", "ThrowFlask",
			"WhiffDown", "WhiffGrab", "WhiffUp",
		},
		"Assistant":    {"AssistantBopLeft", "AssistantBopRight", "AssistantIdle"},
		"BoxPerson":    {"BoxIdle", "BoxPutBack", "BoxTakeAway", "NoBox"},
		"Girl":         {"GirlBopLeft", "GirlBopRight", "GirlIdle"},
		"Guy":          {"GuyBopLeft", "GuyBopRight", "GuyIdle"},
		"HeadHolder 1": {"GirlBlushFace", "GirlIdleFace", "GirlLeftFace", "GuyFaceIdle", "GuyRightFace", "WeirdFaceIdle"},
		"HeartBox":     {"HeartBoxIdle", "HeartBoxSquish"},
		"HeartHolder":  {"HeartGirlMerge", "HeartIdle", "HeartMerge"},
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

func checkAnimPaths(t *testing.T, root, clip string, anim *kmdata.Anim, nodes map[string]bool) {
	t.Helper()
	for p := range animPathSet(anim) {
		full := root
		if p != "" {
			full += "/" + p
		}
		if nodes[full] || loveLabSharedControllerAlias(root, p, nodes) {
			continue
		}
		t.Fatalf("clip %s path %q resolves to missing node %q", clip, p, full)
	}
}

func loveLabSharedControllerAlias(root, p string, nodes map[string]bool) bool {
	if root == "Assistant/ArmHolder/Arm" && p == "Flask" {
		return nodes["Assistant/ArmHolder/Arm/Flask"]
	}
	return false
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
