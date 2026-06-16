package somen

import (
	"testing"

	"hsdemo/games/internal/audittest"
)

func TestRhythmSomenExtractedAssetCoverage(t *testing.T) {
	as := audittest.LoadAssets(t, "rhythmSomen")
	audittest.RequireRoles(t, as, map[string]string{
		"SomenPlayer":  "SomenMan",
		"FrontArm":     "FrontArm",
		"backArm":      "BackArm",
		"FarCrane":     "FarArm",
		"CloseCrane":   "CloseArm",
		"EffectHit":    "SomenMan/Hit Effect",
		"EffectSweat":  "Sweat",
		"EffectExclam": "SomenMan/Exclam",
		"EffectShock":  "SomenMan/Shock",
	})
	audittest.RequireNodes(t, as, "Shoot", "Shoot/Flow", "SplashEffect")
	audittest.RequireSounds(t, as,
		"somen_lowerfar", "somen_lowerclose", "somen_doublealarm", "somen_drop",
		"somen_woosh", "somen_bell", "somen_catch", "somen_catch_old",
		"somen_splash", "somen_mistake",
	)
}

func TestRhythmSomenAnimationPaths(t *testing.T) {
	as := audittest.LoadAssets(t, "rhythmSomen")
	audittest.RequireClipPaths(t, as, "SomenMan", "NothingMan", "SomenMan/NothingMan")
	audittest.RequireClipPaths(t, as, "SomenMan", "HeadBob", "SomenMan/HeadBob")
	audittest.RequireClipPaths(t, as, "FrontArm", "ArmNothing", "FrontArm/ArmNothing")
	audittest.RequireClipPaths(t, as, "FrontArm", "ArmPluckOK", "FrontArm/ArmPluckOK")
	audittest.RequireClipPaths(t, as, "FrontArm", "ArmPluckNG", "FrontArm/ArmPluckNG")
	audittest.RequireClipPaths(t, as, "BackArm", "BackArmNothing", "BackArm/BackArmNothing")
	audittest.RequireClipPaths(t, as, "FarArm", "Drop", "FarArm/Drop")
	audittest.RequireClipPaths(t, as, "FarArm", "Open", "FarArm/Open")
	audittest.RequireClipPaths(t, as, "FarArm", "Lift", "FarArm/Lift")
	audittest.RequireClipPaths(t, as, "CloseArm", "DropClose", "CloseArm/DropClose")
	audittest.RequireClipPaths(t, as, "CloseArm", "OpenClose", "CloseArm/OpenClose")
	audittest.RequireClipPaths(t, as, "CloseArm", "LiftClose", "CloseArm/LiftClose")
	audittest.RequireClipPaths(t, as, "SomenMan/Hit Effect", "HitNothing", "HitNothing")
	audittest.RequireClipPaths(t, as, "Sweat", "BlobNothing", "Sweat/BlobNothing")
	audittest.RequireClipPaths(t, as, "SomenMan/Exclam", "ExclamNothing", "ExclamNothing")
	audittest.RequireClipPaths(t, as, "SomenMan/Shock", "ShockNothing", "ShockNothing")
	audittest.RequireClipPaths(t, as, "Shoot", "WaterFlow", "Shoot/WaterFlow")
}
