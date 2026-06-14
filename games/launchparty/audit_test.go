package launchparty

import (
	"strings"
	"testing"

	"hsdemo/kart"
	"hsdemo/kmdata"
)

func loadAssets(t *testing.T) *kart.Assets {
	t.Helper()
	as, err := kart.Load("../../assets/launchParty", 44100)
	if err != nil {
		t.Skipf("assets not extracted: %v", err)
	}
	return as
}

func TestLaunchPartyControllersAndSounds(t *testing.T) {
	as := loadAssets(t)
	for _, want := range []struct {
		ctrl  string
		state []string
	}{
		{"Rocket", []string{"RocketHidden", "RocketRise", "RocketIdle", "RocketLaunch", "RocketMiss", "RocketBarelyLeft", "RocketBarelyRight"}},
		{"Number", []string{"CountOne", "CountTwo", "CountThree", "CountFour", "CountFive", "CountSix", "CountSeven", "CountImpact"}},
		{"LaunchPad", []string{"LaunchPadFloat"}},
		{"LaunchPadSprite", []string{"Idle", "SizeUp", "Still"}},
		{"Glare", []string{"Flashing"}},
	} {
		ctrl, ok := as.Controllers[want.ctrl]
		if !ok {
			t.Fatalf("missing controller %s", want.ctrl)
		}
		for _, st := range want.state {
			if _, ok := ctrl.States[st]; !ok {
				t.Errorf("controller %s missing state %s", want.ctrl, st)
			}
		}
	}
	for _, snd := range []string{
		"rocket_prepare", "rocket_pin_prepare", "rocket_note", "popper_note",
		"bell_note", "bell_short", "bell_blast", "pin", "flute",
		"rocket_family", "rocket_crackerblast", "rocket_bowling", "rocket_endBad", "miss",
	} {
		if len(as.Sounds[snd]) == 0 {
			t.Errorf("missing sound %s", snd)
		}
	}
}

func TestLaunchPartyClipPathCoverage(t *testing.T) {
	as := loadAssets(t)
	scenePaths := map[string]bool{}
	for _, n := range as.Rig.Nodes {
		scenePaths[n.Path] = true
	}
	dynamic := map[string]bool{
		"": true, "Rocket": true, "Rocket/RocketSprite": true,
		"Boom": true, "Smear": true, "Smoke0": true, "Smoke1": true,
	}
	for animPath, ctrlName := range as.Animators {
		ctrl := as.Controllers[ctrlName]
		for _, st := range ctrl.States {
			if st.Clip == "" || st.Clip == "None" {
				continue
			}
			if as.Anims[st.Clip] == nil {
				t.Errorf("state %s/%s clip %q missing", ctrlName, st.Clip, st.Clip)
			}
			checkAnimPaths(t, as.Anims[st.Clip], func(p string) bool {
				if p == "" {
					return scenePaths[animPath]
				}
				return scenePaths[animPath+"/"+p]
			}, st.Clip)
		}
	}
	for name, anim := range as.Anims {
		if !strings.HasPrefix(name, "Animations/Rocket") && !strings.HasPrefix(name, "Animations/Count") {
			continue
		}
		checkAnimPaths(t, anim, func(p string) bool { return dynamic[p] }, name)
	}
}

func checkAnimPaths(t *testing.T, anim *kmdata.Anim, okPath func(string) bool, clip string) {
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
			t.Errorf("clip %s path %q is not driven by scene or dynamic rocket runtime", clip, p)
		}
	}
}
