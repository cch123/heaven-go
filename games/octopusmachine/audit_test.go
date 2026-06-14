package octopusmachine

import (
	"path/filepath"
	"testing"

	"hsdemo/engine"
	"hsdemo/kart"
)

func loadAuditAssets(t *testing.T) *kart.Assets {
	t.Helper()
	as, err := kart.Load(filepath.Join("..", "..", "assets", "octopusMachine"), engine.SampleRate)
	if err != nil {
		t.Fatal(err)
	}
	if err := as.ApplyTexts(); err != nil {
		t.Fatal(err)
	}
	return as
}

func TestBindingsComponentsTextAndSounds(t *testing.T) {
	as := loadAuditAssets(t)
	if as.Roles["bg"] != "Background" || as.Roles["Text"] != "Text" {
		t.Fatalf("roles = %#v", as.Roles)
	}
	if got := len(as.Extra.RefArrays["Bubbles"]); got != 2 {
		t.Fatalf("Bubbles refs = %d, want 2", got)
	}
	if got := len(as.Extra.RefArrays["octopodes"]); got != 3 {
		t.Fatalf("octopodes refs = %d, want 3", got)
	}
	for i, key := range []string{"octopus0", "octopus1", "octopus2"} {
		c := as.Extra.Components[key]
		if c.Path == "" {
			t.Fatalf("missing component %s", key)
		}
		if got := int(c.Nums["octoNum"]); got != i {
			t.Fatalf("%s octoNum = %d, want %d", key, got, i)
		}
		if got := len(c.RefArrays["sr"]); got != 6 {
			t.Fatalf("%s sr = %d, want 6", key, got)
		}
		if got := len(c.RefArrays["srAll"]); got != 13 {
			t.Fatalf("%s srAll = %d, want 13", key, got)
		}
	}
	if as.Extra.Components["octopus2"].Nums["player"] != 1 {
		t.Fatalf("third octopus is not marked as player")
	}
	if len(as.Texts) != 1 || as.Texts[0].Path != "Text" {
		t.Fatalf("texts = %#v", as.Texts)
	}
	for _, snd := range []string{"squeeze", "release", "pop", "common_nearMiss"} {
		if as.Sounds[snd] == nil {
			t.Fatalf("missing sound %s", snd)
		}
	}
}

func TestAllAnimationClipsAndControllerStates(t *testing.T) {
	as := loadAuditAssets(t)
	for _, clip := range []string{
		"Animations/Angry", "Animations/Bop", "Animations/ForceSqueeze",
		"Animations/Happy", "Animations/Idle", "Animations/Oops",
		"Animations/Pop", "Animations/Prepare", "Animations/PrepareIdle",
		"Animations/Release", "Animations/Squeeze",
	} {
		if as.Anims[clip] == nil {
			t.Fatalf("missing clip %s", clip)
		}
	}
	c, ok := as.Controllers["Octopus"]
	if !ok {
		t.Fatalf("missing Octopus controller")
	}
	if c.Default != "Idle" {
		t.Fatalf("default state = %s, want Idle", c.Default)
	}
	for _, st := range []string{"Angry", "Bop", "ForceSqueeze", "Happy", "Idle", "Oops", "Pop", "Prepare", "Release", "Squeeze"} {
		if _, ok := c.States[st]; !ok {
			t.Fatalf("controller missing state %s", st)
		}
	}
	for _, path := range []string{"Octopodes/Octopus1", "Octopodes/Octopus2", "Octopodes/Octopus3_Player"} {
		if as.Animators[path] != "Octopus" {
			t.Fatalf("animator %s = %q, want Octopus", path, as.Animators[path])
		}
	}
}
