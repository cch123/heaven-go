package sneakyspirits

import (
	"testing"

	"hsdemo/games/internal/audittest"
)

func TestSneakySpiritsExtractedAssetCoverage(t *testing.T) {
	as := audittest.LoadAssets(t, "sneakySpirits")
	audittest.RequireRoles(t, as, map[string]string{
		"bowHolderAnim":    "BowHolder",
		"bowAnim":          "BowHolder/Bow",
		"doorAnim":         "House/Door",
		"deathGhostPrefab": "GhostDeath",
		"ghostMissPrefab":  "GhostMiss",
		"arrowMissPrefab":  "ArrowMiss",
		"normalTree":       "House/Tree",
		"slowTree":         "House/Tree Stationary",
		"normalRain":       "Rain",
		"slowRain":         "Rain Slow",
	})
	if got := len(as.Extra.RefArrays["ghostPositions"]); got != 7 {
		t.Fatalf("ghostPositions = %d, want 7", got)
	}
	for _, p := range as.Extra.RefArrays["ghostPositions"] {
		audittest.RequireNodes(t, as, p)
	}
	audittest.RequireSounds(t, as,
		"moving", "arrowMiss", "hit", "ghostScared", "ghostEscape", "laugh", "rainLoop",
	)
}

func TestSneakySpiritsControllersAndAnimationPaths(t *testing.T) {
	as := audittest.LoadAssets(t, "sneakySpirits")
	for ctrl, states := range map[string][]string{
		"BowHolder":       {"Entered", "Enter", "Exit"},
		"Bow":             {"BowIdle", "BowDraw", "BowRecoil", "BowRelease"},
		"Door":            {"DoorClosed", "DoorOpen", "DoorClose"},
		"GhostDeath":      {"GhostDieNose", "GhostDieMouth", "GhostDieBody", "GhostDieCheek"},
		"GhostMiss":       {"GhostBarely", "GhostMiss", "GhostLaugh"},
		"MovingGhost":     {"Gone", "Move", "MoveDown"},
		"ArrowMiss":       {"ArrowRecoil"},
		"Tree_1":          {"Tree"},
		"Tree Stationary": {"TreeSlow"},
	} {
		audittest.RequireControllerStates(t, as, ctrl, states...)
	}
	audittest.RequireAnimatorPaths(t, as)
	audittest.RequireClips(t, as,
		"Animations/Move", "Animations/MoveDown", "Animations/BowDraw",
		"Animations/BowRecoil", "Animations/GhostBarely", "Animations/GhostDieNose",
	)
}
