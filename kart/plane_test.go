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
	// 判定时刻（progress 0.5×0.95=0.475）：原版飞机仍在画面中央偏左、
	// 保持距离（z≈2.5、缩放 0.8），按键后才加速掠过镜头——
	// 对照 python 全量转写参考值 (1.73, 0.21)
	p := EvalBezier(c, 0.475)
	ps := CamDist / (CamDist + p[2])
	x := p[0] * ps
	if x < 0.5 || x > 3.5 || ps > 1.0 {
		t.Errorf("判定时刻飞机投影 x=%.2f 缩放=%.2f，应在画面中央且未贴近镜头", x, ps)
	}
}
