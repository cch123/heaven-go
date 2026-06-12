package marchingorders

import (
	"testing"

	"hsdemo/kart"
)

// 审计：嵌套 prefab 展开后的 cadet 路径、controller 状态、材质组、音效序列。
func TestMarchingOrdersAssets(t *testing.T) {
	as, err := kart.Load("../../assets/marchingOrders", 44100)
	if err != nil {
		t.Skipf("assets not extracted: %v", err)
	}
	if got := len(as.Extra.RefArrays["Cadets"]); got != 3 {
		t.Errorf("Cadets = %d, want 3（嵌套 prefab 展开）", got)
	}
	if as.Roles["CadetPlayer"] == "" || as.Roles["CadetHeadPlayer"] == "" {
		t.Error("玩家 cadet 角色未解析")
	}
	mats := as.Extra.RefArrays["RecolorMats"]
	if len(mats) != 3 || mats[0] != "tilesMat" || mats[1] != "pipesMat" || mats[2] != "conveyorMat" {
		t.Errorf("RecolorMats = %v", mats)
	}
	mappedCount := 0
	for _, n := range as.Rig.Nodes {
		if n.Mapped && n.Mat != "" {
			mappedCount++
		}
	}
	if mappedCount < 3 {
		t.Errorf("mapped 节点 %d 过少", mappedCount)
	}
	cad := as.Controllers["Cadets"]
	for _, st := range []string{"Idle", "Bop", "Clap", "MarchL", "MarchR", "Halt", "PointL", "PointR"} {
		if _, ok := cad.States[st]; !ok {
			t.Errorf("Cadets controller 缺状态 %q", st)
		}
	}
	head := as.Controllers["CadetHead"]
	for _, st := range []string{"FaceL", "FaceR", "Idle"} {
		if _, ok := head.States[st]; !ok {
			t.Errorf("CadetHead 缺状态 %q", st)
		}
	}
	for _, seq := range []string{"zentai", "susume", "tomare"} {
		if len(as.Extra.Sequences[seq]) == 0 {
			t.Errorf("音效序列 %s 缺失", seq)
		}
	}
	for _, snd := range []string{"stepPlayer", "stepOther", "steam", "turnAction", "turnActionPlayer",
		"leftFaceTurn1", "rightFaceTurn1", "halt1", "attention1", "march1"} {
		if _, ok := as.Sounds[snd]; !ok {
			t.Errorf("音效 %s 缺失", snd)
		}
	}
}
