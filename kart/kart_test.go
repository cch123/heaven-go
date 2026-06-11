package kart

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"testing"

	"hsdemo/kmdata"
)

// loadDataOnly 只读 JSON（不解码图集/音频），用于无窗口环境下验证采样数学。
func loadDataOnly(t *testing.T) *Assets {
	t.Helper()
	dir := filepath.Join("..", "assets", "karateman")
	as := &Assets{Anims: map[string]*kmdata.Anim{}}
	for name, dst := range map[string]any{
		"sprites.json": &as.Sheet,
		"rig.json":     &as.Rig,
		"anims.json":   &as.Anims,
	} {
		b, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Skipf("assets not extracted: %v", err)
		}
		if err := json.Unmarshal(b, dst); err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
	}
	return as
}

func TestRigSampleSanity(t *testing.T) {
	as := loadDataOnly(t)
	rig := NewRig(as)

	for _, anim := range []string{"Beat", "Jab"} {
		rig.Play(anim, 0)
		var poses [][]Aff
		for _, at := range []float64{0, 0.1, 0.2, 0.28} {
			rig.Sample(at)
			cp := make([]Aff, len(rig.world))
			copy(cp, rig.world)
			poses = append(poses, cp)
			for i, w := range rig.world {
				for _, v := range []float64{w.A, w.B, w.C, w.D, w.Tx, w.Ty} {
					if math.IsNaN(v) || math.IsInf(v, 0) {
						t.Fatalf("%s: node %d (%s) produced non-finite transform at t=%v",
							anim, i, as.Rig.Nodes[i].Path, at)
					}
				}
			}
		}
		// 动画应当让至少一个节点的世界变换随时间变化
		changed := false
		for i := range poses[0] {
			if poses[0][i] != poses[1][i] {
				changed = true
				break
			}
		}
		if !changed {
			t.Errorf("%s: no node transform changed between t=0 and t=0.1", anim)
		}
	}
}

func TestJabSpriteSwap(t *testing.T) {
	as := loadDataOnly(t)
	rig := NewRig(as)
	rig.Play("Jab", 0)

	arm := rig.byPath["LeftArm"]
	rig.Sample(0.0)
	first := rig.state[arm].sprite
	rig.Sample(0.27)
	last := rig.state[arm].sprite
	if first == last {
		t.Errorf("Jab: LeftArm sprite did not swap (%q -> %q)", first, last)
	}
	if first == "" || last == "" {
		t.Errorf("Jab: empty sprite name (%q -> %q)", first, last)
	}
}

func TestBBoxFinite(t *testing.T) {
	as := loadDataOnly(t)
	rig := NewRig(as)
	minX, minY, maxX, maxY := rig.BBox()
	if !(minX < maxX && minY < maxY) {
		t.Fatalf("degenerate bbox: [%v %v %v %v]", minX, minY, maxX, maxY)
	}
	if maxY-minY < 1 || maxY-minY > 30 {
		t.Errorf("suspicious rig height: %.2f units", maxY-minY)
	}
}
