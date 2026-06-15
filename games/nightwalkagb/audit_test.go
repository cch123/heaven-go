package nightwalkagb

import (
	"strings"
	"testing"

	"hsdemo/kart"
	"hsdemo/kmdata"
)

func loadAssets(t *testing.T) *kart.Assets {
	t.Helper()
	as, err := kart.Load("../../assets/nightWalkAgb", 44100)
	if err != nil {
		t.Skipf("assets not extracted: %v", err)
	}
	return as
}

func TestNightWalkBindingsComponentsAndTemplates(t *testing.T) {
	as := loadAssets(t)
	for field, want := range map[string]string{
		"playYan":          "Player",
		"platformHandler":  "JumpPlatformHolder",
		"starHandler":      "StarsHolder",
		"Text":             "Textbox/textbox/Text (TMP)",
		"TextboxTransform": "Textbox/textbox",
		"TextboxGO":        "Textbox",
		"TextboxSprite":    "Textbox/textbox",
	} {
		if got := as.Roles[field]; got != want {
			t.Fatalf("role %s = %q, want %q", field, got, want)
		}
	}
	comp := as.Extra.Components
	for _, name := range []string{"game", "platformHandler", "platform", "starHandler"} {
		if _, ok := comp[name]; !ok {
			t.Fatalf("missing component %s", name)
		}
	}
	if got := len(comp["game"].Lists["jumpPaths"]); got != 3 {
		t.Fatalf("jumpPaths = %d, want 3", got)
	}
	if got := comp["platformHandler"].Refs["platformRef"]; got != "JumpPlatform" {
		t.Fatalf("platformRef = %q", got)
	}
	if got := comp["starHandler"].Refs["starRef"]; got != "Star" {
		t.Fatalf("starRef = %q", got)
	}
	for _, root := range []string{"JumpPlatform", "Star"} {
		if tmpl := kart.NewTemplate(as, root); tmpl == nil {
			t.Fatalf("template %s missing", root)
		}
	}
	if err := as.ApplyTexts(); err != nil {
		t.Fatalf("ApplyTexts: %v", err)
	}
	if err := as.SetText(as.Roles["Text"], "Night Walk"); err != nil {
		t.Fatalf("SetText: %v", err)
	}
}

func TestNightWalkControllersSoundsAndClips(t *testing.T) {
	as := loadAssets(t)
	want := map[string][]string{
		"Player":                             {"Jump", "Walk", "HighJump", "Roll", "RollShock", "Shock"},
		"playYanFall":                        {"FallIdle", "FallSmear"},
		"BallonNW":                           {"Idle", "Pop"},
		"Animations/PlayYan/Star.controller": {"Blink"},
		"JumpPlatform": {
			"Idle", "Kick", "Note", "NoteIdle", "Flower", "FlowerBarely",
			"Lollipop", "LollipopBarely", "Umbrella", "UmbrellaBarely",
			"EndIdle", "EndGlow", "EndPop", "Destroy",
		},
		"Fish": {"FishIdle", "Shock"},
		"Animations/Star/Star.controller": {
			"Small", "Blink1", "Blink2", "Blink3", "Blink4", "Blink5",
			"Evolve1", "Evolve2", "Evolve3", "Evolve4",
			"Devolve1", "Devolve2", "Devolve3", "Devolve4", "Devolve5",
		},
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
				t.Fatalf("controller %s state %s missing clip %s", ctrl, st, cs.Clip)
			}
		}
	}
	for _, snd := range []string{
		"boxKick", "disappear", "fall", "fill1A", "fill1B", "fill1C", "fill1D",
		"fill2A", "fill2B", "fill2C", "fill3A", "fill3B", "fillStart",
		"fish1", "fish2", "fish3", "jump1", "jump2", "jump3", "ng",
		"open1", "open2", "open3", "shock", "whiff", "wot",
		"common_count-ins_cowbell",
		"common_games_nightWalkRvl_highJump1", "common_games_nightWalkRvl_highJump2",
		"common_games_nightWalkRvl_highJump3", "common_games_nightWalkRvl_highJump4",
		"common_games_nightWalkRvl_highJump5", "common_games_nightWalkRvl_highJump6",
		"common_games_nightWalkRvl_highJump7",
	} {
		if len(as.Sounds[snd]) == 0 {
			t.Fatalf("missing sound %s", snd)
		}
	}
}

func TestNightWalkAnimationCoverageAndProperties(t *testing.T) {
	as := loadAssets(t)
	nodes := map[string]bool{}
	for _, n := range as.Rig.Nodes {
		nodes[n.Path] = true
	}
	covered := map[string]bool{}
	for root, ctrlName := range as.Animators {
		ctrl := as.Controllers[ctrlName]
		for _, st := range ctrl.States {
			if st.Clip == "" {
				continue
			}
			anim := as.Anims[st.Clip]
			if anim == nil {
				t.Fatalf("controller %s references missing clip %s", ctrlName, st.Clip)
			}
			covered[st.Clip] = true
			checkNWAnimPaths(t, root, st.Clip, anim, nodes)
			checkNWAttrs(t, st.Clip, anim)
		}
	}
	for clip := range as.Anims {
		if strings.Contains(clip, "/") && !covered[clip] {
			t.Fatalf("clip %s is not driven by any controller state", clip)
		}
	}
}

func checkNWAnimPaths(t *testing.T, root, clip string, anim *kmdata.Anim, nodes map[string]bool) {
	t.Helper()
	for p := range animPathSet(anim) {
		full := root
		if p != "" {
			full += "/" + p
		}
		if !nodes[full] && strings.HasPrefix(root, "JumpPlatform/rollPlatform/RodHolder") {
			alt := "JumpPlatform/" + p
			if nodes[alt] {
				full = alt
			}
		}
		if !nodes[full] {
			t.Fatalf("clip %s path %q resolves to missing node %q", clip, p, full)
		}
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

func checkNWAttrs(t *testing.T, clip string, anim *kmdata.Anim) {
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
