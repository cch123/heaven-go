package sneakyspirits

import (
	"math"
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
	for _, sprite := range []string{"Raindrop", "Rainplop1", "Rain_slowmo1", "Rain_slowmo2", "Rain_slowmo3", "Rain_slowmo4"} {
		if _, ok := as.Sheet.Sprites[sprite]; !ok {
			t.Fatalf("missing rain sprite %q", sprite)
		}
	}
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

func TestSneakySpiritsSlowdownRuntimeSemantics(t *testing.T) {
	if got, want := rainSimulationScale(true), sneakySlowRainSimRate/sneakyNormalRainSimRate; math.Abs(got-want) > 1e-12 {
		t.Fatalf("slow rain simulation scale = %.12f, want %.12f", got, want)
	}
	if got := rainSimulationScale(false); got != 1 {
		t.Fatalf("normal rain simulation scale = %.12f, want 1", got)
	}
	m := &Module{slowT: 9}
	if !m.slowActiveAt(8.999) {
		t.Fatal("slowdown should stay active before its restore beat")
	}
	if m.slowActiveAt(9) {
		t.Fatal("slowdown should end exactly on its restore beat")
	}
}
