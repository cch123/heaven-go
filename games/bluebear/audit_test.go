package bluebear

import (
	"testing"

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
