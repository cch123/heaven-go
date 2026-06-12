package seesaw

// 交付前审计：路径/锚点/剪辑/调色板，资产未提取时跳过。

import (
	"testing"

	"hsdemo/kart"
)

func loadAssets(t *testing.T) *kart.Assets {
	t.Helper()
	as, err := kart.Load("../../assets/seeSaw", 44100)
	if err != nil {
		t.Skipf("assets not extracted: %v", err)
	}
	return as
}

// TestJumpPathsResolve：21 条 jumpPath 的双端点（Camera 除外）都在场景树。
func TestJumpPathsResolve(t *testing.T) {
	as := loadAssets(t)
	nodeSet := map[string]bool{}
	for _, n := range as.Rig.Nodes {
		nodeSet[n.Path] = true
	}
	paths := as.Extra.Components["game"].Lists["jumpPaths"]
	if len(paths) != 21 {
		t.Fatalf("jumpPaths = %d, want 21", len(paths))
	}
	for _, p := range paths {
		name := p.Strs["name"]
		if name == "Camera" {
			continue // 纯抛物线（无 target）
		}
		for i, pos := range p.Items["positions"] {
			tgt := pos.Refs["target"]
			if tgt == "" || !nodeSet[tgt] {
				t.Errorf("path %s 端点 %d 目标 %q 未解析", name, i, tgt)
			}
		}
	}
	// 落点锚点
	for _, p := range []string{
		"Game/Curves/See/OutSee", "Game/Curves/See/InSee",
		"Game/Curves/Saw/OutSaw", "Game/Curves/Saw/InSaw",
		"Game/Curves/See/SeeStartJump/Point0",
	} {
		if !nodeSet[p] {
			t.Errorf("锚点 %q 不在场景树", p)
		}
	}
}

// TestStatesAndClips：模块用到的全部状态在 Saw/Seesaw controller 中存在。
func TestStatesAndClips(t *testing.T) {
	as := loadAssets(t)
	saw := as.Controllers["Saw"]
	for _, st := range []string{
		"NeutralSee", "NeutralSaw", "Jump_OutOut_Start", "Jump_InIn_Start",
		"Jump_OutIn_Start", "Jump_OutOut_Fall", "Jump_InIn_Fall",
		"Jump_InOut_Tuck", "Jump_OutIn_Tuck", "Jump_OutOut_Transform",
		"Jump_OutIn_Transform", "BadOut_SeeReact", "BadIn_SeeReact",
		"Land_Out", "Land_In", "Land_Out_Big", "Land_In_Big",
		"Land_Out_Miss", "Land_In_Miss", "Land_Out_Barely", "Land_In_Barely",
		"GetUp_Out", "GetUp_In", "GetUp_Out_Big", "GetUp_In_Big",
		"GetUp_Out_Miss", "GetUp_In_Miss",
	} {
		if _, ok := saw.States[st]; !ok {
			t.Errorf("Saw controller 缺状态 %q", st)
		}
	}
	plank := as.Controllers["Seesaw"]
	for _, st := range []string{"Neut", "Good", "Bad", "Lightning"} {
		if _, ok := plank.States[st]; !ok {
			t.Errorf("Seesaw controller 缺状态 %q", st)
		}
	}
}

// TestMappedNodes：guy 子树是映射材质（recolor 生效面）。
func TestMappedNodes(t *testing.T) {
	as := loadAssets(t)
	mapped := 0
	for _, n := range as.Rig.Nodes {
		if n.Mapped {
			mapped++
		}
	}
	if mapped < 30 {
		t.Errorf("mapped 节点 %d 过少（guys + plank 应 30+）", mapped)
	}
	for _, s := range []string{"Seesaw1_33", "Seesaw1_34"} {
		if _, ok := as.Sheet.Sprites[s]; !ok {
			t.Errorf("轨道珠切片 %s 缺失", s)
		}
	}
}
