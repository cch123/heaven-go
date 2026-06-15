package rockers

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestRockersExtractedAssetCoverage(t *testing.T) {
	root := filepath.Join("..", "..", "assets", "rockers")
	for _, name := range []string{"scene.json", "sprites.json", "anims.json", "controllers.json", "animators.json", "atlas0.png", "atlas1.png", "atlas2.png"} {
		if _, err := os.Stat(filepath.Join(root, name)); err != nil {
			t.Fatalf("missing %s: %v", name, err)
		}
	}

	var animators map[string]string
	readJSON(t, filepath.Join(root, "animators.json"), &animators)
	for _, path := range []string{"JJHolder", "StudentHolder", "JJHolder/StrumEffects", "StudentHolder/StrumEffects"} {
		if animators[path] == "" {
			t.Fatalf("missing animator binding for %s", path)
		}
	}

	var controllers map[string]struct {
		Default string         `json:"default"`
		States  map[string]any `json:"states"`
	}
	readJSON(t, filepath.Join(root, "controllers.json"), &controllers)
	for ctrl, states := range map[string][]string{
		"JJHolder":      {"JJIdle", "JJStrum", "JJCrouch", "JJUnCrouch", "JJBend", "JJUnbend", "JJComeOnPrepare", "JJComeOnStrum", "JJMiss"},
		"StudentHolder": {"Idle", "Strum", "Crouch", "UnCrouch", "Bend", "Unbend", "ComeOnPrepare", "ComeOnStrum"},
		"StrumEffects":  {"StrumIdle", "StrumStart", "StrumStartLeft", "StrumStartRIght"},
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

	var sprites struct {
		Sprites map[string]any `json:"sprites"`
	}
	readJSON(t, filepath.Join(root, "sprites.json"), &sprites)
	for _, palette := range rockerLightningSprites {
		for _, sprite := range palette {
			if _, ok := sprites.Sprites[sprite]; !ok {
				t.Fatalf("missing lightning sprite %s", sprite)
			}
		}
	}

	var scene struct {
		Nodes []struct {
			Path   string `json:"path"`
			Sprite string `json:"sprite"`
		} `json:"nodes"`
	}
	readJSON(t, filepath.Join(root, "scene.json"), &scene)
	sceneSprites := map[string]string{}
	for _, n := range scene.Nodes {
		sceneSprites[n.Path] = n.Sprite
	}
	for _, holder := range []string{"StudentHolder", "JJHolder"} {
		for _, side := range []string{"LightningRight", "LightningLeft"} {
			for i, node := range rockerLightningNodes {
				path := holder + "/StrumEffects/" + side + "/" + node
				if got, ok := sceneSprites[path]; !ok {
					t.Fatalf("missing lightning node %s", path)
				} else if want := rockerLightningSprites[rockerLightningNormal][i]; got != want {
					t.Fatalf("lightning node %s sprite = %s, want %s", path, got, want)
				}
			}
		}
	}

	for _, sound := range []string{
		"mute.wav", "bendUp.wav", "bendDown.wav", "Cmon.wav", "LastOne.wav",
		"rocker/rockerChordA.wav", "rocker/rockerChordG.wav", "rocker/rockerRemix6ChordA.wav", "rocker/rockerRemix10ChordD.wav",
		"strings/normal/normal1.wav", "strings/normal/normal6.wav", "strings/gleeClub/gleeClub1.wav", "strings/gleeClub/gleeClub6.wav",
		"count/count1.wav", "count/count4.wav",
	} {
		if _, err := os.Stat(filepath.Join(root, "sounds", filepath.FromSlash(sound))); err != nil {
			t.Fatalf("missing sound %s: %v", sound, err)
		}
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
