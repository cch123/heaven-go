package munchymonk

import (
	"testing"

	"hsdemo/games/internal/audittest"
)

func TestMunchyMonkExtractedAssetCoverage(t *testing.T) {
	as := audittest.LoadAssets(t, "munchyMonk")
	audittest.RequireRoles(t, as, map[string]string{
		"MonkAnim":       "MonkHolder/Monk",
		"MonkArmsAnim":   "MonkHolder/ArmsHolder/Arms",
		"MonkHolderAnim": "MonkHolder",
		"OneGiverAnim":   "AssistantHolder1",
		"TwoGiverAnim":   "AssistantHolder2",
		"ThreeGiverAnim": "AssistantHolder3",
		"DumplingObj":    "DumplingStuff/DumplingHolder/Dumpling",
		"CloudMonkey":    "CloudMonkey",
		"StacheHolder":   "MonkHolder/Monk/Head/StacheHolder",
		"BrowHolder":     "MonkHolder/Monk/Head/BrowHolder",
		"Baby":           "MonkHolder/Monk/Torso/Baby",
	})
	audittest.RequireSounds(t, as,
		"gulp", "slap", "miss", "barely", "gong",
		"one_1", "one_2", "two_1", "two_4", "three_1", "three_4",
	)
	audittest.RequireSequences(t, as, "one_go", "two_go", "three_go")
}

func TestMunchyMonkLoadsFanClubVineBoom(t *testing.T) {
	pcm, err := loadVineBoomPCM("../../assets")
	if err != nil {
		t.Skipf("fanClub vine boom asset not extracted: %v", err)
	}
	if len(pcm) == 0 {
		t.Fatal("decoded fanClub/arisa_dab is empty")
	}
}

func TestMunchyMonkDefaultColorsPersistBeforeSwitchBeat(t *testing.T) {
	m := New().(*Module)
	red := [4]float64{1, 0, 0, 1}
	green := [4]float64{0, 1, 0, 1}
	blue := [4]float64{0, 0, 1, 1}
	yellow := [4]float64{1, 1, 0, 1}
	m.defaultColorEvts = []defaultColorEvt{
		{beat: 4, one: red, two: green, three: blue},
		{beat: 8, one: yellow, two: yellow, three: yellow},
	}

	m.restoreDefaultColors(8)
	if m.oneColor != red || m.twoColor != green || m.threeColor != blue {
		t.Fatalf("restore at exact event beat applied wrong colors: one=%v two=%v three=%v", m.oneColor, m.twoColor, m.threeColor)
	}
	m.restoreDefaultColors(8.01)
	if m.oneColor != yellow || m.twoColor != yellow || m.threeColor != yellow {
		t.Fatalf("restore after event beat did not apply latest colors: one=%v two=%v three=%v", m.oneColor, m.twoColor, m.threeColor)
	}
	m.restoreDefaultColors(1)
	if m.oneColor != defaultOneColor || m.twoColor != defaultTwoColor || m.threeColor != defaultThreeColor {
		t.Fatalf("restore before color events should reset defaults: one=%v two=%v three=%v", m.oneColor, m.twoColor, m.threeColor)
	}
}

func TestMunchyMonkControllersAndAnimationPaths(t *testing.T) {
	as := audittest.LoadAssets(t, "munchyMonk")
	for ctrl, states := range map[string][]string{
		"MonkAnim":        {"Idle", "Bop", "Eat", "Miss", "Barely", "Stare", "Blush", "NoseRed"},
		"MonkArm":         {"ArmIdle", "WristSlap", "WristSlapWhiff"},
		"DumplingsAnim":   {"Idle", "FollowHand", "IdleOnTop", "Squish", "HitHead", "FallOff"},
		"OneGiverAnim":    {"Idle", "GiveIn", "GiveOut"},
		"TwoGiverAnim":    {"Idle", "GiveIn", "GiveOut"},
		"ThreeGiverAnim":  {"Idle", "GiveIn", "GiveOut"},
		"HolderAnim":      {"IdleLeft", "IdleRight", "GoLeft", "GoRight"},
		"CloudMonkeyAnim": {"Idle", "Bop"},
		"StacheAnim":      {"Idle1", "Idle2", "Idle3", "Idle4", "Bop1", "Bop2", "Bop3", "Bop4"},
		"BrowAnim":        {"Idle", "Bop"},
	} {
		audittest.RequireControllerStates(t, as, ctrl, states...)
	}
	audittest.RequireAnimatorPaths(t, as)
}
