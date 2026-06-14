package bigrockfinish

import (
	"math"
	"path"
	"path/filepath"
	"testing"

	"hsdemo/engine"
	"hsdemo/kart"
)

func loadAuditAssets(t *testing.T) *kart.Assets {
	t.Helper()
	as, err := kart.Load(filepath.Join("..", "..", "assets", "bigRockFinish"), engine.SampleRate)
	if err != nil {
		t.Fatal(err)
	}
	return as
}

func TestBindingsAndSounds(t *testing.T) {
	as := loadAuditAssets(t)
	wantRoles := map[string]string{
		"playerGhost":   "PlayerGhostHolder",
		"greenGhost":    "GreenGhostHolder",
		"drummerGhost":  "RedGhostHolder",
		"ghostHandL":    "RedGhostHolder/Ghost/ArmLeft",
		"ghostHandR":    "RedGhostHolder/Ghost/ArmRight",
		"audience":      "AudienceHolder",
		"spotlightMask": "BackgroundHolder/SpotlightMask",
		"flash":         "Flash",
		"Bass":          "DrumHolder/Bass",
		"Cymbal":        "DrumHolder/Cymbal",
		"TomL":          "DrumHolder/TomLeft",
		"TomR":          "DrumHolder/TomRight",
		"Snare":         "DrumHolder/Snare",
		"Hihat":         "DrumHolder/Hi-Hat",
		"UnlitArea":     "BackgroundHolder/Black",
	}
	for k, want := range wantRoles {
		if got := as.Roles[k]; got != want {
			t.Fatalf("role %s = %q, want %q", k, got, want)
		}
	}

	for _, snd := range []string{
		"boo", "cheering", "comeOn", "cough", "countinFour", "countinOne",
		"countinOneFast", "countinThree", "countinThreeFast", "countinTwo",
		"countinTwoFast", "cymbal", "grunt", "guitar", "thankYouA", "thankYouB",
		"yeahA", "yeahB",
	} {
		if as.Sounds[snd] == nil {
			t.Fatalf("missing sound %s", snd)
		}
	}
}

func TestAnimationControllersAndPaths(t *testing.T) {
	as := loadAuditAssets(t)
	for root, ctrl := range map[string]string{
		"PlayerGhostHolder":              "PlayerGhostHolder",
		"GreenGhostHolder":               "GreenGhostHolder",
		"RedGhostHolder":                 "RedGhostHolder",
		"RedGhostHolder/Ghost/ArmLeft":   "ArmLeft",
		"RedGhostHolder/Ghost/ArmRight":  "ArmRight",
		"AudienceHolder":                 "AudienceHolder",
		"BackgroundHolder/SpotlightMask": "SpotlightMask",
		"DrumHolder/Bass":                "Bass",
		"DrumHolder/Cymbal":              "Cymbal",
		"DrumHolder/TomLeft":             "TomLeft",
		"DrumHolder/TomRight":            "TomRight",
		"DrumHolder/Snare":               "Snare",
		"DrumHolder/Hi-Hat":              "Hi-Hat",
	} {
		if got := as.Animators[root]; got != ctrl {
			t.Fatalf("animator %s = %q, want %q", root, got, ctrl)
		}
	}

	for ctrl, states := range map[string][]string{
		"PlayerGhostHolder": {"Idle", "Beat", "Prepare", "Release", "Strum", "Jump"},
		"GreenGhostHolder":  {"Idle", "Beat", "Prepare", "Release", "Strum", "Jump"},
		"RedGhostHolder":    {"Idle", "Beat"},
		"ArmLeft":           {"Idle", "Idle2", "Hit01", "Hit02"},
		"ArmRight":          {"Idle", "Idle2", "Hit01", "Hit02"},
		"AudienceHolder":    {"Idle", "Beat", "Cheer", "Miss"},
		"SpotlightMask":     {"None", "Red", "Green", "Blue"},
		"Bass":              {"Idle", "Hit"},
		"Cymbal":            {"Idle", "Hit"},
		"TomLeft":           {"Idle", "Hit"},
		"TomRight":          {"Idle", "Hit"},
		"Snare":             {"I", "Hit"},
		"Hi-Hat":            {"Idle", "Hit"},
	} {
		c := as.Controllers[ctrl]
		if c.States == nil {
			t.Fatalf("missing controller %s", ctrl)
		}
		for _, st := range states {
			if _, ok := c.States[st]; !ok {
				t.Fatalf("controller %s missing state %s", ctrl, st)
			}
		}
	}

	for clip, root := range map[string]string{
		"Animations/Beat":    "PlayerGhostHolder",
		"Animations/Idle":    "PlayerGhostHolder",
		"Animations/Jump":    "PlayerGhostHolder",
		"Animations/Prepare": "PlayerGhostHolder",
		"Animations/Release": "PlayerGhostHolder",
		"Animations/Strum":   "PlayerGhostHolder",
		"GreenGhost/Beat":    "GreenGhostHolder",
		"GreenGhost/Idle":    "GreenGhostHolder",
		"GreenGhost/Jump":    "GreenGhostHolder",
		"GreenGhost/Prepare": "GreenGhostHolder",
		"GreenGhost/Release": "GreenGhostHolder",
		"GreenGhost/Strum":   "GreenGhostHolder",
		"RedGhost/Beat":      "RedGhostHolder",
		"RedGhost/Idle":      "RedGhostHolder",
		"ArmLeft/Hit01":      "RedGhostHolder/Ghost/ArmLeft",
		"ArmLeft/Hit02":      "RedGhostHolder/Ghost/ArmLeft",
		"ArmLeft/Idle":       "RedGhostHolder/Ghost/ArmLeft",
		"ArmLeft/Idle2":      "RedGhostHolder/Ghost/ArmLeft",
		"ArmRight/Hit01":     "RedGhostHolder/Ghost/ArmRight",
		"ArmRight/Hit02":     "RedGhostHolder/Ghost/ArmRight",
		"ArmRight/Idle":      "RedGhostHolder/Ghost/ArmRight",
		"ArmRight/Idle2":     "RedGhostHolder/Ghost/ArmRight",
		"Audience/Beat":      "AudienceHolder",
		"Audience/Cheer":     "AudienceHolder",
		"Audience/Idle":      "AudienceHolder",
		"Audience/Miss":      "AudienceHolder",
		"Spotlight/Blue":     "BackgroundHolder/SpotlightMask",
		"Spotlight/Green":    "BackgroundHolder/SpotlightMask",
		"Spotlight/None":     "BackgroundHolder/SpotlightMask",
		"Spotlight/Red":      "BackgroundHolder/SpotlightMask",
		"Flash/Flash":        "Flash",
		"Flash/Idle":         "Flash",
		"Bass/Hit":           "DrumHolder/Bass",
		"Bass/Idle":          "DrumHolder/Bass",
		"Cymbal/Hit":         "DrumHolder/Cymbal",
		"Cymbal/Idle":        "DrumHolder/Cymbal",
		"Hi-Hat/Hit":         "DrumHolder/Hi-Hat",
		"Hi-Hat/Idle":        "DrumHolder/Hi-Hat",
		"Snare/Hit":          "DrumHolder/Snare",
		"Snare/Idle":         "DrumHolder/Snare",
		"TomLeft/Hit":        "DrumHolder/TomLeft",
		"TomLeft/Idle":       "DrumHolder/TomLeft",
		"TomRight/Hit":       "DrumHolder/TomRight",
		"TomRight/Idle":      "DrumHolder/TomRight",
	} {
		assertClipPaths(t, as, clip, root)
	}
}

func TestCountinTimingAndPitch(t *testing.T) {
	targets := countinTargets(8, 4)
	want := []float64{12, 12.75, 13.5, 14}
	for i := range want {
		if targets[i] != want[i] {
			t.Fatalf("target %d = %v, want %v", i, targets[i], want[i])
		}
	}
	if got := semitonePitch(12); math.Abs(got-2) > 1e-9 {
		t.Fatalf("semitonePitch(12) = %v, want 2", got)
	}
	if actionFlick != 1 {
		t.Fatalf("flick action channel = %d, want 1", actionFlick)
	}
}

func countinTargets(beat, length float64) []float64 {
	return []float64{
		beat + length,
		beat + length + length*0.1875,
		beat + length + length*0.375,
		beat + length + length*0.5,
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
