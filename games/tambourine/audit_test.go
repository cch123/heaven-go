package tambourine

import (
	"testing"

	"hsdemo/games/internal/audittest"
)

func TestTambourineExtractedAssetCoverage(t *testing.T) {
	as := audittest.LoadAssets(t, "tambourine")
	audittest.RequireRoles(t, as, map[string]string{
		"monkeyAnimator": "Monkey",
		"handsAnimator":  "Player",
		"frogAnimator":   "Monkey/Head/Frog",
		"sweatAnimator":  "Monkey/Head/Sweat",
		"happyFace":      "Monkey/Head/HappyFace",
		"sadFace":        "Monkey/Head/SadFace",
		"bg":             "Background",
	})
	audittest.RequireSounds(t, as,
		"miss", "frog", "frogJump",
		"monkey/shake/ms1", "monkey/hit/mh1", "monkey/turnPass/tp1",
		"player/shake/ps1", "player/hit/ph1", "player/turnPass/sweep",
		"player/turnPass/note1", "player/turnPass/note4",
	)
}

func TestTambourineControllersAndAnimationPaths(t *testing.T) {
	as := audittest.LoadAssets(t, "tambourine")
	for ctrl, states := range map[string][]string{
		"Monkey": {"MonkeyIdle", "MonkeyBop", "MonkeyShake", "MonkeyShakeRest", "MonkeySmack", "MonkeySmackRest", "MonkeyPassTurn"},
		"Player": {"Idle", "Bop", "Shake", "Smack"},
		"Frog":   {"FrogExited", "FrogEnter", "FrogIdle", "FrogExit"},
		"Sweat":  {"NoSweat", "Sweating"},
	} {
		audittest.RequireControllerStates(t, as, ctrl, states...)
	}
	audittest.RequireAnimatorPaths(t, as)
}
