package kart

import (
	"fmt"
	"testing"
)

func TestBlastGirlVisibility(t *testing.T) {
	as, err := Load("../assets/trickClass", 44100)
	if err != nil {
		t.Skip(err)
	}
	s := NewScene(as)
	s.Play("girl", "Girl/BlastNg", 0, 0.5)
	for _, beat := range []float64{0.05, 0.4, 1.0, 1.9} {
		s.Sample(beat)
		clipT := beat * 0.5
		visible := 0
		fmt.Printf("--- clipT=%.2fs\n", clipT)
		for i, n := range as.Rig.Nodes {
			if n.Path != "girl" && !hasPrefix(n.Path, "girl/") {
				continue
			}
			st := &s.state[i]
			mark := ""
			sizeCollapsed := n.DrawMode != 0 && (st.size[0] <= 0 || st.size[1] <= 0)
			if s.actives[i] && st.renderOn && st.sprite != "" && st.color[3] > 0 && !sizeCollapsed {
				if _, ok := as.Sheet.Sprites[st.sprite]; ok {
					visible++
					mark = "可见"
				} else {
					mark = "!! sprite 未解析: " + st.sprite
				}
			}
			if mark != "" || st.sprite != "" {
				fmt.Printf("  %-26s sprite=%-24s renderOn=%v active=%v %s\n",
					n.Path, st.sprite, st.renderOn, s.actives[i], mark)
			}
		}
		fmt.Printf("  可见节点数=%d\n", visible)
		// 收束后（m_Size.y→0）光束必须消失，不能以原始尺寸残留
		if clipT >= 0.5 {
			ei := s.byPath["girl/head/effect2"]
			if st := &s.state[ei]; !(st.size[0] <= 0 || st.size[1] <= 0) {
				t.Errorf("clipT=%.2fs 光束 size=%v 未收束", clipT, st.size)
			}
		}
		if visible < 4 {
			t.Errorf("clipT=%.2fs 女孩子树可见节点仅 %d（应 >=4：发射期间她换爆发造型但不消失）", clipT, visible)
		}
	}
}

func hasPrefix(s, p string) bool { return len(s) >= len(p) && s[:len(p)] == p }
