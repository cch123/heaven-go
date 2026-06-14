package rhythmfighter

import (
	"strings"
	"testing"

	"hsdemo/kart"
	"hsdemo/kmdata"
	"hsdemo/synth"
)

func loadAssets(t *testing.T) *kart.Assets {
	t.Helper()
	as, err := kart.Load("../../assets/rhythmFighter", synth.SampleRate)
	if err != nil {
		t.Skipf("assets not extracted: %v", err)
	}
	return as
}

func TestRhythmFighterBindingsAndTemplate(t *testing.T) {
	as := loadAssets(t)
	for _, role := range []string{
		"fighterR", "fighterL", "holderR", "holderL", "displayHolderAnim",
		"displayHolder", "musicNote", "lightsL", "lightsR", "fightText", "spotLight",
	} {
		if as.Roles[role] == "" {
			t.Fatalf("missing role %s", role)
		}
	}
	if kart.NewTemplate(as, as.Roles["musicNote"]) == nil {
		t.Fatalf("missing music note template")
	}
}

func TestRhythmFighterControllersAndSounds(t *testing.T) {
	as := loadAssets(t)
	want := map[string][]string{
		"FighterAnim":   {"Idle", "Bop", "Punch", "Kick", "Duck", "Jump", "Whiff", "Punched", "Kicked", "Walk", "Retreat"},
		"HolderAnim":    {"Idle", "Advance", "Advanced", "Fall"},
		"DisplayAnim":   {"Call", "Normal", "Four", "Eight", "Twelve"},
		"NoteAnim":      {"QuarterO", "QuarterW", "EightO", "EightW"},
		"FightTextAnim": {"Hidden", "Show"},
		"SpotlightAnim": {"Idle", "Show", "Hide"},
		"LightsAnim":    {"Idle", "Flash"},
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
	for _, snd := range []string{
		"ready", "fight", "ding_first", "ding", "punch", "kick",
		"punch_dodge", "kick_dodge", "punch_hit", "kick_hit", "ring",
	} {
		if len(as.Sounds[snd]) == 0 {
			t.Fatalf("missing sound %s", snd)
		}
	}
}

func TestRhythmFighterClipPathCoverageAndProperties(t *testing.T) {
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
