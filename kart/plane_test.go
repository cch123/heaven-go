package kart

import (
	"fmt"
	"testing"
)

func TestPlaneTrajectoryTable(t *testing.T) {
	as, err := Load("../assets/trickClass", 44100)
	if err != nil {
		t.Skip(err)
	}
	c := as.Extra.Curves["planeTossCurve"]
	fmt.Printf("planeTossCurve: %d 点, sampling=%d\n", len(c.Points), c.Sampling)
	for _, ft := range []float64{0, 0.1, 0.2, 0.3, 0.4, 0.475, 0.55, 0.7, 0.85, 0.95} {
		p := EvalBezier(c, ft)
		ps := CamDist / (CamDist + p[2])
		fmt.Printf("  flyPos=%5.3f 投影=(%7.2f,%7.2f) 缩放=%.2f\n", ft, p[0]*ps, p[1]*ps, ps)
	}
	// 判定时刻（dodgeBeats=2 / flyBeats=4 → progress 0.5×0.95=0.475）应抵达男孩附近 x≈5.4
	p := EvalBezier(c, 0.475)
	ps := CamDist / (CamDist + p[2])
	if x := p[0] * ps; x < 3.5 || x > 8.5 {
		t.Errorf("判定时刻飞机投影 x=%.2f，未在男孩附近", x)
	}
}
