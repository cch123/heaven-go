package lockstep

// 交付前审计：controller/绑定/roles/音效/棋盘格参数来源。

import (
	"testing"

	"hsdemo/kart"
)

func loadAssets(t *testing.T) *kart.Assets {
	t.Helper()
	as, err := kart.Load("../../assets/lockstep", 44100)
	if err != nil {
		t.Skipf("assets not extracted: %v", err)
	}
	return as
}

func TestControllersResolve(t *testing.T) {
	as := loadAssets(t)
	for name, ctrl := range as.Controllers {
		for st, s := range ctrl.States {
			if s.Clip == "" {
				t.Errorf("controller %s 状态 %s 无剪辑", name, st)
				continue
			}
			if _, ok := as.Anims[s.Clip]; !ok {
				t.Errorf("controller %s 状态 %s 剪辑 %q 缺失", name, st, s.Clip)
			}
		}
	}
	nodeSet := map[string]bool{}
	for _, n := range as.Rig.Nodes {
		nodeSet[n.Path] = true
	}
	for path := range as.Animators {
		if !nodeSet[path] {
			t.Errorf("animator 绑定路径 %q 不在场景树", path)
		}
	}
	// 行进状态机：march/miss/bop 全套状态存在
	ss := as.Controllers["stepswitcher"]
	for _, st := range []string{"OnbeatMarch", "OffbeatMarch", "OnbeatMiss", "OffbeatMiss", "Bop", "Idle"} {
		if _, ok := ss.States[st]; !ok {
			t.Errorf("stepswitcher 缺状态 %s", st)
		}
	}
}

func TestRolesAndSounds(t *testing.T) {
	as := loadAssets(t)
	nodeSet := map[string]bool{}
	for _, n := range as.Rig.Nodes {
		nodeSet[n.Path] = true
	}
	for _, r := range []string{"stepswitcherPlayer", "masterStepperAnim", "bach", "background"} {
		if p := as.Roles[r]; p == "" || !nodeSet[p] {
			t.Errorf("role %s = %q 未解析", r, p)
		}
	}
	for _, k := range []string{"hai", "ho", "nha1", "nha2", "hahai1", "hahai2",
		"foot1", "foot2", "drumOn", "drumOff", "wayOff"} {
		if _, ok := as.Sounds[k]; !ok {
			t.Errorf("音效 %q 缺失", k)
		}
	}
}

// TestGridPhase：棋盘格常量与 prefab 序列化一致（stepswitchers 根 (6.39,3.72)、
// 行距 3.48、列距 2.55、玩家行 y=0.24、玩家 scale 1.1）。
func TestGridPhase(t *testing.T) {
	as := loadAssets(t)
	var root, player *struct {
		pos   [2]float64
		scale [2]float64
	}
	for i := range as.Rig.Nodes {
		n := &as.Rig.Nodes[i]
		if n.Path == "stepswitchers" {
			root = &struct {
				pos   [2]float64
				scale [2]float64
			}{n.Pos, n.Scale}
		}
		if n.Path == "stepswitcherP" {
			player = &struct {
				pos   [2]float64
				scale [2]float64
			}{n.Pos, n.Scale}
		}
	}
	if root == nil || player == nil {
		t.Fatal("stepswitchers/stepswitcherP 不在场景树")
	}
	if root.pos != [2]float64{6.39, 3.72} {
		t.Errorf("stepswitchers 根 = %v, want (6.39,3.72)", root.pos)
	}
	if player.pos[1] != gridYBase {
		t.Errorf("玩家行 y = %v, want %v", player.pos[1], gridYBase)
	}
	if player.scale[0] != stepScale {
		t.Errorf("玩家 scale = %v, want %v", player.scale[0], stepScale)
	}
}
