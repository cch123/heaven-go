package animalacrobat

import (
	"testing"

	"hsdemo/kart"
	"hsdemo/kmdata"
)

func loadAnimalAssets(t *testing.T) *kart.Assets {
	t.Helper()
	as, err := kart.Load("../../assets/animalAcrobat", 44100)
	if err != nil {
		t.Skipf("assets not extracted: %v", err)
	}
	return as
}

func TestAnimalAcrobatBindingsAndTemplates(t *testing.T) {
	as := loadAnimalAssets(t)
	for _, role := range []string{
		"_elephant", "_giraffe", "_monkeysLong", "_monkeysShort", "_gorilla",
		"_scroll", "_playerMonkey", "_spotlightMain", "_partyPoppers",
	} {
		if as.Roles[role] == "" {
			t.Fatalf("missing role %s", role)
		}
	}
	for _, root := range []string{"Elephant", "Giraffe", "WhiteMonkeys", "WhiteMonkey", "Gorilla"} {
		if kart.NewTemplate(as, root) == nil {
			t.Fatalf("missing template %s", root)
		}
	}
}

func TestAnimalAcrobatControllersAndSounds(t *testing.T) {
	as := loadAnimalAssets(t)
	wantStates := map[string][]string{
		"Elephant":          {"ElephantIdle", "ElephantEar"},
		"GiraffeRoot":       {"GiraffeIdle", "GiraffeEar"},
		"Gorilla":           {"GorillaIdle", "GorillaMiss"},
		"WhiteMonkeysPivot": {"WhiteMonkeysIdle", "WhiteMonkeysSwing"},
		"FireHoop":          {"FireIdle", "FireClose"},
		"ConfettiPop":       {"PopIntro"},
		"PlayerMonkey":      {"PlayerIdle", "PlayerBop", "PlayerJump", "PlayerAir", "PlayerHang", "PlayerHanging", "PlayerLand"},
	}
	for ctrl, states := range wantStates {
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
		"start", "eek", "catch", "giraffeCatch", "release", "giraffeJump",
		"giraffeDrumroll", "giraffeCymbal", "turn", "land", "miss", "cracker",
		"applause", "common_nearMiss",
	} {
		if len(as.Sounds[snd]) == 0 {
			t.Fatalf("missing sound %s", snd)
		}
	}
}

func TestAnimalAcrobatObstacleComponents(t *testing.T) {
	as := loadAnimalAssets(t)
	comps := as.Extra.Components
	for _, root := range []string{"Elephant", "Giraffe", "WhiteMonkeys", "WhiteMonkey", "Gorilla"} {
		ob := obstacleComponent(comps, root)
		if ob.Path == "" {
			t.Fatalf("missing obstacle component for %s", root)
		}
		for _, key := range []string{"_rotateRoot", "_gripPoint", "_endPoint"} {
			if ob.Refs[key] == "" {
				t.Errorf("%s missing ref %s", root, key)
			}
		}
		if ob.Nums["_fullRotRange"] == 0 {
			t.Errorf("%s missing full rotation range", root)
		}
		if root != "Gorilla" {
			in := inputComponent(comps, root, root == "Giraffe")
			if in.Path == "" {
				t.Fatalf("missing input component for %s", root)
			}
			for _, key := range []string{"_monkey", "_holdParticle", "_sweatParticle", "_gripShadow", "_endShadow"} {
				if in.Refs[key] == "" {
					t.Errorf("%s missing input ref %s", root, key)
				}
			}
		}
	}
}

func TestAnimalAcrobatObstacleComponentDoesNotMatchInputFamily(t *testing.T) {
	comps := map[string]kmdata.Component{
		"obstacleInput0": {Path: "Elephant", Refs: map[string]string{"_monkey": "wrong"}},
		"obstacle0":      {Path: "Elephant", Refs: map[string]string{"_rotateRoot": "right"}},
	}
	if got := obstacleComponent(comps, "Elephant").Refs["_rotateRoot"]; got != "right" {
		t.Fatalf("obstacleComponent returned %q, want obstacle family component", got)
	}
}
