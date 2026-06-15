package airboarder

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestAirboarderExtractedAssetCoverage(t *testing.T) {
	root := filepath.Join("..", "..", "assets", "airboarder")
	for _, name := range []string{"scene.json", "sprites.json", "roles.json", "extra.json", "anims.json", "controllers.json", "animators.json", "atlas0.png"} {
		if _, err := os.Stat(filepath.Join(root, name)); err != nil {
			t.Fatalf("missing %s: %v", name, err)
		}
	}

	var roles map[string]string
	readJSON(t, filepath.Join(root, "roles.json"), &roles)
	for field, want := range map[string]string{
		"CPU1":      "MovingObjects/airboarders/CPU1/neo_airboy",
		"CPU2":      "MovingObjects/airboarders/CPU2/neo_airboy",
		"Player":    "MovingObjects/airboarders/Player/neo_airboy",
		"Dog":       "MovingObjects/dog",
		"Floor":     "Environment/floor_model",
		"archBasic": "Environment/ArchFinePosition/Arch",
		"wallBasic": "Environment/WallFinePosition/Wall",
	} {
		if got := roles[field]; got != want {
			t.Fatalf("role %s = %q, want %q", field, got, want)
		}
	}

	var controllers map[string]struct {
		Default string         `json:"default"`
		States  map[string]any `json:"states"`
	}
	readJSON(t, filepath.Join(root, "controllers.json"), &controllers)
	for ctrl, states := range map[string][]string{
		"airboy":      {"hover", "bop", "duck", "charge", "hold", "jump", "hit1", "hit2", "letsgo"},
		"arch":        {"idle", "move", "shake", "break"},
		"Wall":        {"idle", "move", "shake", "break"},
		"floor_model": {"idle", "move"},
		"dog":         {"run", "wag"},
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
		"MovingObjects/airboarders/CPU1/neo_airboy":   "airboy",
		"MovingObjects/airboarders/CPU2/neo_airboy":   "airboy",
		"MovingObjects/airboarders/Player/neo_airboy": "airboy",
		"Environment/ArchFinePosition/Arch":           "arch",
		"Environment/WallFinePosition/Wall":           "Wall",
	} {
		if got := animators[path]; got != ctrl {
			t.Fatalf("animator %s = %q, want %q", path, got, ctrl)
		}
	}

	for _, sound := range []string{
		"ready.wav", "crouch.wav", "crouchCharge.wav", "crouchvox.wav",
		"jump.wav", "jumpvox.wav", "barely.wav", "barelyvox.wav",
		"start1.wav", "start2.wav", "start3.wav", "miss1.wav", "miss15.wav", "missvox.wav",
	} {
		if _, err := os.Stat(filepath.Join(root, "sounds", sound)); err != nil {
			t.Fatalf("missing sound %s: %v", sound, err)
		}
	}
}

func TestAirboarderBopRegionSemantics(t *testing.T) {
	m := &Module{bops: []bopEvt{{beat: 4, auto: true}, {beat: 8, auto: false}}}
	if m.autoBopAt(3.99) || !m.autoBopAt(4) || m.autoBopAt(7.99) != true || m.autoBopAt(8) {
		t.Fatalf("auto bop did not match SetupBopRegion semantics")
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
