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
