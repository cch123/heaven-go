package firstcontact

import (
	"path"
	"testing"

	"hsdemo/engine"
	"hsdemo/kart"
)

func loadAuditAssets(t *testing.T) *kart.Assets {
	t.Helper()
	as, err := kart.Load("../../assets/firstContact", engine.SampleRate)
	if err != nil {
		t.Skipf("assets not extracted: %v", err)
	}
	return as
}

func TestExtractedAssets(t *testing.T) {
	as := loadAuditAssets(t)
	for _, node := range []string{
		"Alien", "Translator", "MissionControl", "Live", "Background/CrowdOfAliens",
		"Textboxes/AlienTextbox", "Textboxes/TranslateTextbox", "Textboxes/TranslateTextboxFail",
		"Alien/Body/Torso/Head/Mouth", "Translator/Head/Mouth",
	} {
		if _, ok := as.NodeIndex(node); !ok {
			t.Fatalf("missing scene node %s", node)
		}
	}
	for _, sprite := range []string{
		"interpreterTextboxes_0", "interpreterTextboxes_1", "interpreterTextboxes_2",
		"textIcnSDF_0", "textIcnSDF_1",
	} {
		if _, ok := as.Sheet.Sprites[sprite]; !ok {
			t.Fatalf("missing sprite %s", sprite)
		}
	}
	for _, snd := range []string{
		"ALIEN_PLAYER_A", "ALIEN_PLAYER_B", "ALIEN_PLAYER_MISS2_A",
		"Bob1", "Bob2", "Bob3", "Bob4", "Bob5", "Bob6", "Bob7", "Bob8", "Bob9", "Bob10", "BobB",
		"alienNoHit", "fail", "failContact", "nod", "shakeHead",
		"successCrowd", "successExtra1", "successExtra2", "turnover", "whistle",
	} {
		if as.Sounds[snd] == nil {
			t.Fatalf("missing sound %s", snd)
		}
	}
	if as.Sounds["slightlyFail"] != nil {
		t.Fatalf("slightlyFail unexpectedly exists; remove README/code known-gap handling")
	}
}

func TestControllersAndAnimationPaths(t *testing.T) {
	as := loadAuditAssets(t)
	for root, ctrl := range map[string]string{
		"Alien":                    "Alien",
		"Translator":               "Translator",
		"Live":                     "Live",
		"MissionControl":           "MissionControl",
		"Background/CrowdOfAliens": "CrowdOfAliens",
	} {
		if got := as.Animators[root]; got != ctrl {
			t.Fatalf("animator %s = %q, want %q", root, got, ctrl)
		}
	}
	for ctrl, states := range map[string][]string{
		"Alien": {
			"alien_idle", "alien_lookAt", "alien_talk", "alien_point",
			"alien_success", "alien_fail", "alien_noHit", "alienNoHit",
			"alien_fail2", "alien_lookoAt",
		},
		"Translator": {
			"translator_idle", "translator_speakidle", "translator_lookAtAlien",
			"translator_lookAtAlien_nod", "translator_speak", "translator_eh",
		},
		"MissionControl": {"missionControl_success", "missionControl_fail"},
		"Live":           {"New State", "liveBar"},
		"CrowdOfAliens":  {"crowdIdle"},
	} {
		c := as.Controllers[ctrl]
		if c.States == nil {
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
	if as.Controllers["Alien"].States["alien_fail2"].Clip != "" || as.Controllers["Alien"].States["alien_lookoAt"].Clip != "" {
		t.Fatal("unused Alien typo/fallback states should remain no-motion states")
	}

	for clip, root := range map[string]string{
		"alien_idle/alien_idle":                       "Alien",
		"alien_lookAt/alien_lookAt":                   "Alien",
		"alien_talk/alien_talk":                       "Alien",
		"alien_point/alien_point":                     "Alien",
		"alien_good/alien_success":                    "Alien",
		"alien_bad/alien_fail":                        "Alien",
		"alien_bad/alien_noHit":                       "Alien",
		"interpret_look/translator_lookAtAlien":       "Translator",
		"interpret_nod/translator_lookAtAlien_nod":    "Translator",
		"interpret_talk/translator_speak":             "Translator",
		"interpret_talk_alt/translator_eh":            "Translator",
		"translator_anim/translator_idle":             "Translator",
		"translator_anim/translator_speakidle":        "Translator",
		"mission_control_anim/missionControl_fail":    "MissionControl",
		"mission_control_anim/missionControl_success": "MissionControl",
		"Animations/liveBar":                          "Live",
		"Animations/crowdIdle":                        "Background/CrowdOfAliens",
	} {
		assertClipPaths(t, as, clip, root)
	}
}

func TestDialogueHelpers(t *testing.T) {
	m := &Module{
		intervals: []intervalEvt{{beat: 4, length: 3}, {beat: 12, length: 2}},
		speaks: []speakEvt{
			{beat: 4.5, dialogue: "one"},
			{beat: 6.9, dialogue: "two"},
			{beat: 7.0, dialogue: "out"},
		},
	}
	if iv, ok := m.previousInterval(13); !ok || iv.beat != 12 {
		t.Fatalf("previousInterval = %#v, %v", iv, ok)
	}
	got := m.speaksBetween(4, 7)
	if len(got) != 2 || got[0].dialogue != "one" || got[1].dialogue != "two" {
		t.Fatalf("speaksBetween = %#v", got)
	}
	if defaultLength(0, 1) != 1 || defaultLength(2, 1) != 2 {
		t.Fatalf("defaultLength changed")
	}
}

func assertClipPaths(t *testing.T, as *kart.Assets, clip, root string) {
	t.Helper()
	anim := as.Anims[clip]
	if anim == nil {
		t.Fatalf("missing clip %s", clip)
	}
	check := func(curvePath string) {
		full := root
		if curvePath != "" {
			full = path.Join(root, curvePath)
		}
		if _, ok := as.NodeIndex(full); !ok {
			t.Fatalf("%s curve path %q resolved to missing node %q", clip, curvePath, full)
		}
	}
	for p := range anim.Pos {
		check(p)
	}
	for p := range anim.Scale {
		check(p)
	}
	for p := range anim.Euler {
		check(p)
	}
	for p := range anim.Sprites {
		check(p)
	}
	for p := range anim.Floats {
		check(p)
	}
}
