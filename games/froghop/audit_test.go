package froghop

import (
	"testing"

	"hsdemo/kart"
)

func loadAssets(t *testing.T) *kart.Assets {
	t.Helper()
	as, err := kart.Load("../../assets/frogHop", 44100)
	if err != nil {
		t.Skipf("assets not extracted: %v", err)
	}
	return as
}

func TestFrogHopExtractedBindings(t *testing.T) {
	as := loadAssets(t)
	for _, role := range []string{
		"PlayerFrog", "LeaderFrog", "SingerFrog", "Darkness",
		"SpotlightFront", "SpotlightBack", "SpotlightFrontColor", "SpotlightBackColor",
		"Mike", "Mike2", "Stage", "StageTop", "gradient", "bgLow", "bgHigh",
	} {
		if as.Roles[role] == "" {
			t.Fatalf("missing role %s", role)
		}
	}
	game := as.Extra.Components["game"]
	if got := len(game.RefArrays["OtherFrogs"]); got != 3 {
		t.Fatalf("OtherFrogs count = %d, want 3", got)
	}
	if got := len(game.RefArrays["_FrogColors"]); got != 8 {
		t.Fatalf("_FrogColors count = %d, want 8", got)
	}
}

func TestFrogHopControllersAndSounds(t *testing.T) {
	as := loadAssets(t)
	for ctrl, states := range map[string][]string{
		"BackupFrogAnim": {"Bop", "Hop", "LongHop", "Charge", "Spin", "TalkWide", "TalkNarrow", "TalkSpecial", "Bump", "Ouch", "Sweat", "Glare"},
		"LeaderFrogAnim": {"Bop", "Hop", "LongHop", "Charge", "Spin", "TalkWide", "TalkNarrow", "TalkSpecial"},
		"SingerFrogAnim": {"Bop", "Hop", "LongHop", "Charge", "Spin", "SpinHS", "TalkWide", "TalkNarrow", "TalkSpecial"},
	} {
		c, ok := as.Controllers[ctrl]
		if !ok {
			t.Fatalf("missing controller %s", ctrl)
		}
		for _, st := range states {
			if _, ok := c.States[st]; !ok {
				t.Fatalf("controller %s missing state %s", ctrl, st)
			}
		}
	}
	for _, sound := range []string{
		"SE_NTR_FROG_EN_COUNT1", "SE_NTR_FROG_EN_COUNT4",
		"SE_NTR_FROG_EN_T_HA", "SE_NTR_FROG_EN_E_HAAI",
		"SE_NTR_FROG_EN_P_KURU_1", "SE_NTR_FROG_EN_P_LIN",
		"SE_NTR_FROG_EN_MISS", "SE_NTR_FROG_EN_MISS_BOING",
		"miss2", "sigh", "tyvm", "common_miss",
	} {
		if len(as.Sounds[sound]) == 0 {
			t.Fatalf("missing sound %s", sound)
		}
	}
}
