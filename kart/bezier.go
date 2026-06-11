// bezier.go：NaughtyBezierCurves BezierCurve3D.GetPoint 的等价实现——
// 整条曲线的归一化时间按各段近似弧长加权分配，段内三次 Bezier。
package kart

import "hsdemo/kmdata"

const bezierLenSamples = 16

func cubic(t float64, p0, c0, c1, p1 [2]float64) [2]float64 {
	u := 1 - t
	a, b, c, d := u*u*u, 3*u*u*t, 3*u*t*t, t*t*t
	return [2]float64{
		a*p0[0] + b*c0[0] + c*c1[0] + d*p1[0],
		a*p0[1] + b*c0[1] + c*c1[1] + d*p1[1],
	}
}

func segLen(a, b kmdata.CurvePoint) float64 {
	prev := a.P
	total := 0.0
	for i := 1; i <= bezierLenSamples; i++ {
		t := float64(i) / bezierLenSamples
		p := cubic(t, a.P, a.RH, b.LH, b.P)
		dx, dy := p[0]-prev[0], p[1]-prev[1]
		total += dx*dx + dy*dy // 平方和近似足够做权重（单调）
		prev = p
	}
	return total
}

// EvalBezier 在归一化时间 t∈[0,1] 处求曲线上点。
func EvalBezier(pts []kmdata.CurvePoint, t float64) [2]float64 {
	switch {
	case len(pts) == 0:
		return [2]float64{}
	case len(pts) == 1:
		return pts[0].P
	case t <= 0:
		return pts[0].P
	case t >= 1:
		return pts[len(pts)-1].P
	}
	if len(pts) == 2 { // 常见情形：单段
		return cubic(t, pts[0].P, pts[0].RH, pts[1].LH, pts[1].P)
	}
	total := 0.0
	weights := make([]float64, len(pts)-1)
	for i := range weights {
		weights[i] = segLen(pts[i], pts[i+1])
		total += weights[i]
	}
	acc := 0.0
	for i, w := range weights {
		frac := w / total
		if acc+frac >= t || i == len(weights)-1 {
			return cubic((t-acc)/frac, pts[i].P, pts[i].RH, pts[i+1].LH, pts[i+1].P)
		}
		acc += frac
	}
	return pts[len(pts)-1].P
}
