package bluebear

import (
	"testing"

	"hsdemo/engine"
	"hsdemo/kart"
)

// 审计：treat 路径/渐变/controller 状态/嘴部状态机条件。
func TestBlueBearAssets(t *testing.T) {
	as, err := kart.Load("../../assets/blueBear", 44100)
	if err != nil {
		t.Skipf("assets not extracted: %v", err)
	}
	game := as.Extra.Components["game"]
	if len(game.Lists["_treatCurves"]) != 2 {
		t.Errorf("treatCurves = %d, want 2", len(game.Lists["_treatCurves"]))
	}
	if len(game.Lists["donutGradient"]) == 0 || len(game.Lists["cakeGradient"]) == 0 {
		t.Error("渐变未导出")
	}
	hb := as.Controllers["HeadAndBody"]
	for _, st := range []string{
		"Idle", "Open", "BiteL", "BiteR", "CryBiteL", "CryBiteR",
		"LongBiteL", "LongBiteR", "LongCryBiteL", "LongCryBiteR",
		"OpenEyes", "Sad", "Smile", "StopSmile", "SmileIdle", "CryIdle",
		"EyesClosed", "Sigh",
	} {
		if _, ok := hb.States[st]; !ok {
			t.Errorf("HeadAndBody 缺状态 %q", st)
		}
	}
	// 嘴部开合：Idle→Open 必须是无退出时间的条件转换（gate 0）
	for _, tr := range hb.States["Idle"].Transitions {
		if tr.Dst == "Open" && tr.ExitTime != 0 {
			t.Errorf("Idle→Open exitTime = %v, want 0（即时开合）", tr.ExitTime)
		}
	}
	for _, ctrl := range []string{"Story", "Wind", "BagHolder", "DonutBag", "CakeBag"} {
		if _, ok := as.Controllers[ctrl]; !ok {
			t.Errorf("缺 controller %s", ctrl)
		}
	}
	for _, role := range []string{"donutBase", "cakeBase", "foodHolder", "leftCrumb", "rightCrumb"} {
		if as.Roles[role] == "" {
			t.Errorf("role %s 未解析", role)
		}
	}
}

func TestBlueBearShortBiteSurvivesFirstSample(t *testing.T) {
	as, err := kart.Load("../../assets/blueBear", 44100)
	if err != nil {
		t.Skipf("assets not extracted: %v", err)
	}
	sc := kart.NewScene(as)
	root := "Bear/HeadAndBody"
	beat := 10.0

	sc.PlayState(root, "BiteR", beat, 0.5)
	sc.SetBool(root, "ShouldOpenMouth", false)
	sc.Sample(beat)
	if got, _ := sc.StateInfo(root, beat); got != "BiteR" {
		t.Fatalf("首帧状态 = %q, want BiteR", got)
	}

	next := beat + 1.0/60.0
	sc.Sample(next)
	if got, _ := sc.StateInfo(root, next); got != "Idle" {
		t.Fatalf("下一帧状态 = %q, want Idle", got)
	}
}

func TestRestoreTreatsOnSwitchCatchesMidFlightTreat(t *testing.T) {
	as, err := kart.Load("../../assets/blueBear", 44100)
	if err != nil {
		t.Skipf("assets not extracted: %v", err)
	}
	m := &Module{
		ctx:            &engine.Ctx{Assets: as, Scene: kart.NewScene(as)},
		donutT:         kart.NewTemplate(as, as.Roles["donutBase"]),
		cakeT:          kart.NewTemplate(as, as.Roles["cakeBase"]),
		emoCancelBeat:  -1,
		emoFirstFrame:  true,
		rightThreshold: 15,
		leftThreshold:  30,
		treatLog: []treatEvt{{
			beat: 10, length: 3, isCake: false, long: true, open: true,
		}},
	}

	m.restoreTreatsAt(11)
	if got := len(m.treats); got != 1 {
		t.Fatalf("treats = %d, want 1", got)
	}
	if got := m.openCount; got != 1 {
		t.Fatalf("openCount = %d, want 1", got)
	}
	if m.squashing {
		t.Fatal("切入中段的补生成不应重播包袋挤压")
	}

	m.restoreTreatsAt(11.5)
	if got := len(m.treats); got != 1 {
		t.Fatalf("restore duplicated treat, got %d", got)
	}
}
