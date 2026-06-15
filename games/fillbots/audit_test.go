package fillbots

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestFillbotsExtractedAssetCoverage(t *testing.T) {
	root := filepath.Join("..", "..", "assets", "fillbots")
	for _, name := range []string{"scene.json", "sprites.json", "roles.json", "extra.json", "anims.json", "controllers.json", "animators.json", "atlas0.png", "atlas1.png"} {
		if _, err := os.Stat(filepath.Join(root, name)); err != nil {
			t.Fatalf("missing %s: %v", name, err)
		}
	}

	var controllers map[string]struct {
		Default string         `json:"default"`
		States  map[string]any `json:"states"`
	}
	readJSON(t, filepath.Join(root, "controllers.json"), &controllers)
	for ctrl, states := range map[string][]string{
		"Filler":      {"FillerIdle", "FillerPrepare", "HoldSmall", "HoldMedium", "HoldLarge", "ReleaseSmall", "ReleaseMedium", "ReleaseLarge", "ReleaseWhiffSmall", "ReleaseWhiffMedium", "ReleaseWhiffLarge"},
		"FallingLimb": {"Idle", "Impact"},
		"SmallBot":    {"IdleBody", "Hold", "HoldBarely", "HoldBeat", "Release", "ReleaseEarly", "ReleaseLate", "Dead", "Beyond", "Fly", "Success"},
		"MediumBot":   {"IdleBody", "Hold", "HoldBarely", "HoldBeat", "Release", "ReleaseEarly", "ReleaseLate", "Dead", "Beyond", "Fly", "Success"},
		"LargeBot":    {"IdleBody", "Hold", "HoldBarely", "HoldBeat", "Release", "ReleaseEarly", "ReleaseLate", "Dead", "Beyond", "Fly", "Success"},
		"Meter":       {"Idle", "Up", "Down"},
		"Conveyer":    {"ConveyerIdle", "Move"},
	} {
		c, ok := controllers[ctrl]
		if !ok {
			t.Fatalf("missing controller %s", ctrl)
		}
		if c.Default == "" {
			t.Fatalf("controller %s has no default state", ctrl)
		}
		for _, state := range states {
			if _, ok := c.States[state]; !ok {
				t.Fatalf("controller %s missing state %s", ctrl, state)
			}
		}
	}

	var animators map[string]string
	readJSON(t, filepath.Join(root, "animators.json"), &animators)
	for path, ctrl := range map[string]string{
		"Filler":                  "Filler",
		"BgObjects/Conveyer":      "Conveyer",
		"BotSmall/FullBody":       "SmallBot",
		"BotMedium/FullBody":      "MediumBot",
		"BotLarge/FullBody":       "LargeBot",
		"BotSmall/FullBody/Fill":  "Animations/Small/Fill.controller",
		"BotMedium/FullBody/Fill": "Animations/Medium/Fill.controller",
		"BotLarge/FullBody/Fill":  "Animations/Large/Fill.controller",
	} {
		if got := animators[path]; got != ctrl {
			t.Fatalf("animator %s = %q, want %q", path, got, ctrl)
		}
	}

	var extra struct {
		Components map[string]struct {
			Path string             `json:"path"`
			Nums map[string]float64 `json:"nums"`
		} `json:"components"`
	}
	readJSON(t, filepath.Join(root, "extra.json"), &extra)
	for _, path := range []string{"BotSmall", "BotMedium", "BotLarge"} {
		found := false
		for _, comp := range extra.Components {
			if comp.Path == path && comp.Nums["limbFallHeight"] == 15 && comp.Nums["stackDistanceRate"] > 0 {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("missing bot component spec for %s", path)
		}
	}

	for _, sound := range []string{
		"armExtension.ogg", "armRetraction.ogg", "armRetractionPop.wav", "armRetractionWhiff.wav",
		"beep.ogg", "water.ogg", "explosion.wav", "miss.wav",
		"smallFall.ogg", "mediumFall.ogg", "bigFall.ogg",
		"smallMove.ogg", "mediumMove.ogg", "bigMove.ogg",
		"smallOK1.ogg", "mediumOK1.ogg", "bigOK1.ogg",
		"smallOK2.ogg", "mediumOK2.ogg", "bigOK2.ogg",
		"fillErUp1.ogg", "fillErUp2.ogg", "fillErUp3.ogg",
	} {
		if _, err := os.Stat(filepath.Join(root, "sounds", sound)); err != nil {
			t.Fatalf("missing sound %s: %v", sound, err)
		}
	}
}

func TestFillbotsBopRegionSemantics(t *testing.T) {
	m := &Module{bops: []bopEvt{{beat: 4, auto: true}, {beat: 8, auto: false}}}
	if m.autoBopAt(3.99) || !m.autoBopAt(4) || !m.autoBopAt(7.99) || m.autoBopAt(8) {
		t.Fatalf("auto bop did not match Fillbots SetupBopRegion semantics")
	}
}

func readJSON(t *testing.T, path string, out any) {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(b, out); err != nil {
		t.Fatalf("decode %s: %v", path, err)
	}
}
