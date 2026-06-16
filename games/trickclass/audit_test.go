package trickclass

import (
	"testing"

	"hsdemo/games/internal/audittest"
)

func TestTrickClassExtractedAssetCoverage(t *testing.T) {
	as := audittest.LoadAssets(t, "trickClass")
	audittest.RequireRoles(t, as, map[string]string{
		"girlAnim":   "girl",
		"playerAnim": "player",
		"warnAnim":   "objWarn",
		"objHolder":  "objHolder",
	})
	audittest.RequireNodes(t, as,
		"objHolder/objBall", "objHolder/objChair", "objHolder/objShock",
		"objHolder/objPhone", "objHolder/objNx", "objHolder/objPlane",
	)
	if got := len(as.Extra.RefArrays["objPrefab"]); got != 5 {
		t.Fatalf("objPrefab refs = %d, want 5", got)
	}
	if got := len(as.Extra.RefArrays["objPrefabVariant"]); got != 5 {
		t.Fatalf("objPrefabVariant refs = %d, want 5", got)
	}
	for _, curve := range []string{"ballTossCurve", "ballMissCurve", "planeTossCurve", "planeMissCurve", "shockTossCurve"} {
		if len(as.Extra.Curves[curve].Points) == 0 {
			t.Fatalf("missing curve %s", curve)
		}
	}
	audittest.RequireSequences(t, as,
		"ballThrow", "chairThrow", "shockThrow", "phoneThrow", "planeThrow", "girlCharge",
	)
	audittest.RequireSounds(t, as,
		"player_dodge", "player_dodge_success", "player_dodge_success_alt",
		"ball_impact", "chair_impact", "shock_impact", "blast_dodge", "blast_miss",
	)
}

func TestTrickClassAnimationPaths(t *testing.T) {
	as := audittest.LoadAssets(t, "trickClass")
	for _, clip := range []string{
		"Girl/NoPose", "Girl/Bop", "Girl/Throw", "Girl/ThrowAlt",
		"Girl/Charge0", "Girl/Charge1", "Girl/BlastDodged", "Girl/BlastNg",
	} {
		audittest.RequireClipPaths(t, as, "girl", clip, "girl/"+clip)
	}
	for _, clip := range []string{
		"Player/NoPose", "Player/Bop", "Player/Dodge", "Player/DodgeAlt",
		"Player/DodgeBlast0", "Player/DodgeBlast1", "Player/DodgeNg", "Player/DodgeNgShock",
	} {
		audittest.RequireClipPaths(t, as, "player", clip, "player/"+clip)
	}
	for _, clip := range []string{
		"WarnBubble/NoPose", "WarnBall", "WarnChair", "WarnShock", "WarnPhone", "WarnNx", "WarnPlane",
	} {
		audittest.RequireClipPaths(t, as, "objWarn", clip, "objWarn/"+clip)
	}
}
