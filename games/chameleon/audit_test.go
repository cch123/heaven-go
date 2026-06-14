package chameleon

import (
	"math/rand"
	"testing"

	"hsdemo/kart"
)

func TestChameleonExtractedAssets(t *testing.T) {
	as, err := kart.Load("../../assets/chameleon", 44100)
	if err != nil {
		t.Skipf("assets not extracted: %v", err)
	}
	for _, role := range []string{"baseFly", "chameleonAnim", "chameleonEye", "Crown", "gradient", "bgHigh", "bgLow"} {
		if as.Roles[role] == "" {
			t.Errorf("role %s 未解析", role)
		}
	}
	for _, ctrl := range []string{"Chameleon", "Fly", "Wing"} {
		if _, ok := as.Controllers[ctrl]; !ok {
			t.Errorf("缺 controller %s", ctrl)
		}
	}
	for _, clip := range []string{
		"Animations/chameleonIdle", "Animations/chameleonClose", "Animations/chameleonFar",
		"Animations/chameleonCloseIdle", "Animations/chameleonGurp",
		"Animations/flyCloseCatch", "Animations/flyCloseFall", "Animations/flyCloseGone",
		"Animations/flyCloseMoveEnd", "Animations/flyFarCatch", "Animations/flyFarFall",
		"Animations/flyFarGone", "Animations/flyFarMoveEnd", "Animations/wing",
	} {
		if _, ok := as.Anims[clip]; !ok {
			t.Errorf("缺动画剪辑 %s", clip)
		}
	}
	fly := as.Extra.Components["fly"]
	if fly.Refs["flyAnim"] != "Fly" || fly.Refs["wingAnim"] != "Fly/wing" {
		t.Fatalf("fly component refs = %#v", fly.Refs)
	}
	if kart.NewTemplate(as, as.Roles["baseFly"]) == nil {
		t.Fatal("baseFly template 未解析")
	}
	for _, snd := range []string{
		"blankClose", "blankFar", "eatCatch", "eatGulp",
		"flyClose1", "flyClose2", "flyClose3", "flyCloseLoop",
		"flyFar1", "flyFar2", "flyFar3", "flyFarLoop",
	} {
		if _, ok := as.Sounds[snd]; !ok {
			t.Errorf("缺音效 %s", snd)
		}
	}
}

func TestFlyMovementMilestones(t *testing.T) {
	f := &fly{
		startBeat:   10,
		lengthBeat:  4,
		currentBeat: 10,
		typ:         flyFar,
		moveCur:     [2]float64{-4.5, 5.4},
		moveNext:    [2]float64{6.15, 1.6},
		moveEnd:     [2]float64{5.15, 1.6},
		rng:         rand.New(rand.NewSource(1)),
	}
	f.update(10)
	if f.pos != f.moveCur {
		t.Fatalf("spawn pos = %#v, want %#v", f.pos, f.moveCur)
	}
	f.update(11)
	if f.pos != f.moveCur {
		t.Fatalf("first beat should stay at start, got %#v", f.pos)
	}
	f.update(12)
	if f.pos != f.moveNext {
		t.Fatalf("two-beat intro pos = %#v, want %#v", f.pos, f.moveNext)
	}
}
