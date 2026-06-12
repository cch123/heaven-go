package kart

import (
	"fmt"
	"testing"
)

// TestTrickClassLayout 打印关键节点的投影屏幕几何（960x540，54px/unit，中心 480,270）。
func TestTrickClassLayout(t *testing.T) {
	as, err := Load("../assets/trickClass", 44100)
	if err != nil {
		t.Skip(err)
	}
	s := NewScene(as)
	s.Sample(0)
	proj := Translate(480, 270).Mul(Scale(54, -54))

	dump := func(path string) {
		i, ok := s.byPath[path]
		if !ok {
			fmt.Printf("%-28s 不存在\n", path)
			return
		}
		w := s.world[i]
		z := s.worldZ[i]
		ps := CamDist / (CamDist + z)
		sx, sy := proj.Apply(w.Tx*ps, w.Ty*ps)
		sp, hasSpr := as.Sheet.Sprites[s.state[i].sprite]
		hw := 0.0
		if hasSpr {
			hw = float64(sp.H) / sp.PPU * ps * 54 // 屏幕像素高
		}
		fmt.Printf("%-28s z=%5.1f 投影屏幕=(%6.1f,%6.1f) 透视=%.3f 精灵高=%4.0fpx sprite=%s\n",
			path, z, sx, sy, ps, hw, s.state[i].sprite)
	}
	for _, p := range []string{
		"girl", "girl/body", "girl/head",
		"bg/desks/mobTrick_bgDesk1 (1)", "bg/desks/mobTrick_bgDesk2 (1)", "bg/desks/mobTrick_bgDesk3 (1)",
		"bg/mobTrick_bg00_0", "player/body", "objWarn",
	} {
		dump(p)
	}

	// 光束：播放 Girl/BlastNg 后 effect2 的两个端点
	s.Play("girl", "Girl/BlastNg", 0, 0.5)
	s.Sample(0.3) // 剪辑 0.15s 处（timeScale 0.5）
	i := s.byPath["girl/head/effect2"]
	w := s.world[i]
	z := s.worldZ[i]
	ps := CamDist / (CamDist + z)
	st := s.state[i]
	x0, y0 := proj.Apply(w.Tx*ps, w.Ty*ps)
	// 端点 = 局部 +x 方向 m_Size.x
	ex, ey := w.Apply(st.size[0], 0)
	x1, y1 := proj.Apply(ex*ps, ey*ps)
	fmt.Printf("BlastNg t=0.15s 光束: 根=(%6.1f,%6.1f) 端=(%6.1f,%6.1f) size=%v sprite=%q order=%d active=%v\n",
		x0, y0, x1, y1, st.size, st.sprite, st.order, s.actives[i])
}

// TestSortingGroupOcclusion 验证 SortingGroup：girl（组 order=-5）整体
// 画在课桌排（order=-4）之前，即被其遮挡。
func TestSortingGroupOcclusion(t *testing.T) {
	as, err := Load("../assets/trickClass", 44100)
	if err != nil {
		t.Skip(err)
	}
	s := NewScene(as)
	s.Sample(0)

	gi := s.byPath["girl/body"]
	di := s.byPath["bg/desks/mobTrick_bgDesk3 (1)"]
	gg := s.groupOf[gi]
	if gg < 0 || len(as.Rig.Nodes[gg].SortGroup) != 2 || as.Rig.Nodes[gg].SortGroup[1] != -5 {
		t.Fatalf("girl/body 未归属 order=-5 的 SortingGroup（groupOf=%d）", gg)
	}
	if dg := s.groupOf[di]; dg >= 0 {
		t.Fatalf("课桌不应归属任何 SortingGroup（groupOf=%d）", dg)
	}
	// 组级比较：girl 组 order -5 < 课桌 order -4 → 女孩先画（被遮挡）
	if !(as.Rig.Nodes[gg].SortGroup[1] < as.Rig.Nodes[di].Order) {
		t.Fatalf("排序键未满足遮挡关系: girl 组 %d vs 课桌 %d",
			as.Rig.Nodes[gg].SortGroup[1], as.Rig.Nodes[di].Order)
	}
}
