package builttoscalervl

import (
	"testing"

	"hsdemo/kart"
)

func loadAssets(t *testing.T) *kart.Assets {
	t.Helper()
	as, err := kart.Load("../../assets/builtToScaleRvl", 44100)
	if err != nil {
		t.Skipf("assets not extracted: %v", err)
	}
	return as
}

func TestAssetsContainBuiltToScaleRuntimePieces(t *testing.T) {
	as := loadAssets(t)
	for _, role := range []string{"baseRod", "baseLeftSquare", "baseRightSquare", "baseAssembled", "widgetHolder"} {
		if as.Roles[role] == "" {
			t.Fatalf("missing role %s", role)
		}
	}
	for _, ctrl := range []string{"whiteblock", "rod", "square", "assembled"} {
		if _, ok := as.Controllers[ctrl]; !ok {
			t.Fatalf("missing controller %s", ctrl)
		}
	}
	for _, state := range []string{"bounce", "idle", "miss", "miss_open", "open", "prepare B", "shoot", "shoot miss B"} {
		if _, ok := as.Controllers["whiteblock"].States[state]; !ok {
			t.Fatalf("missing whiteblock state %s", state)
		}
	}
	for i := 0; i < 32; i++ {
		if c := as.Extra.Curves["game.curve"+itoa(i)]; len(c.Points) == 0 {
			t.Fatalf("missing game.curve%d", i)
		}
	}
	for i := 0; i < 2; i++ {
		if c := as.Extra.Curves["game.missCurve"+itoa(i)]; len(c.Points) == 0 {
			t.Fatalf("missing game.missCurve%d", i)
		}
	}
	for _, snd := range []string{"left", "middleLeft", "middleRight", "right", "tink", "barely", "playerRetract", "preparestart", "prepareend", "shoot"} {
		if len(as.Sounds[snd]) == 0 {
			t.Fatalf("missing sound %s", snd)
		}
	}
}

func TestFollowingPosMatchesUnityRules(t *testing.T) {
	items := []customBounceItem{{time: 3, pos: -1}}
	if got := followingPos(1, 2, 3, items); got != -1 {
		t.Fatalf("custom following = %d, want -1", got)
	}
	if got := followingPos(-1, 0, 1, nil); got != 1 {
		t.Fatalf("first bounce following = %d, want 1", got)
	}
	if got := followingPos(4, 3, 1, nil); got != 2 {
		t.Fatalf("fourth bounce following = %d, want 2", got)
	}
	if got := followingPos(0, 2, 1, nil); got != 3 {
		t.Fatalf("ascending following = %d, want 3", got)
	}
	if got := followingPos(3, 1, 1, nil); got != 0 {
		t.Fatalf("descending following = %d, want 0", got)
	}
}

func TestRodPlanningWaitsUntilShootableMiddleRightBlock(t *testing.T) {
	m := &Module{
		shoots: []shootEvent{{beat: 5, id: 1}},
	}
	bounces := []customBounceItem{}
	end, shoot, _ := m.calcRodEndTime(0, 1, -1, 0, 1, &bounces)
	if !shoot {
		t.Fatal("rod should be planned as shoot")
	}
	if end != 6 {
		t.Fatalf("shoot end time = %d, want 6 after rod returns to player block", end)
	}
}

func TestOutSidesOverridesShoot(t *testing.T) {
	m := &Module{
		outs:   []outEvent{{beat: 4, id: 1}},
		shoots: []shootEvent{{beat: 8, id: 1}},
	}
	bounces := []customBounceItem{}
	m.addBounceOutSides(0, 1, -1, 0, 1, &bounces)
	end, shoot, _ := m.calcRodEndTime(0, 1, -1, 0, 1, &bounces)
	if shoot {
		t.Fatal("out sides should cancel shoot planning")
	}
	if end != 4 {
		t.Fatalf("out side end time = %d, want 4", end)
	}
}

func itoa(v int) string {
	if v == 0 {
		return "0"
	}
	var buf [12]byte
	i := len(buf)
	for v > 0 {
		i--
		buf[i] = byte('0' + v%10)
		v /= 10
	}
	return string(buf[i:])
}
