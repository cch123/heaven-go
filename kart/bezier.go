// bezier.go：NaughtyBezierCurves BezierCurve3D.GetPoint 的逐式等价实现。
//
// 原版算法（BezierCurve3D.cs）：
//   - 每段子采样数 = Sampling/(KeyPointsCount-1) + 1
//   - 段弧长 = 子采样逐步累加的三维距离（GetApproximateLengthOfCubicCurve）
//   - GetCubicSegment：按"段弧长/总弧长"的累计占比选段（严格大于），
//     归一化时间落在末尾时回退到最后一段
package kart

import (
	"math"

	"hsdemo/kmdata"
)

func cubic(t float64, p0, c0, c1, p1 [3]float64) [3]float64 {
	u := 1 - t
	a, b, c, d := u*u*u, 3*u*u*t, 3*u*t*t, t*t*t
	return [3]float64{
		a*p0[0] + b*c0[0] + c*c1[0] + d*p1[0],
		a*p0[1] + b*c0[1] + c*c1[1] + d*p1[1],
		a*p0[2] + b*c0[2] + c*c1[2] + d*p1[2],
	}
}

// segLen 等价 GetApproximateLengthOfCubicCurve：从 t=0 起以 1/sampling
// 步长采样，累加相邻样点的三维距离。
func segLen(a, b kmdata.CurvePoint, sampling int) float64 {
	prev := cubic(0, a.P, a.RH, b.LH, b.P)
	total := 0.0
	for i := 0; i < sampling; i++ {
		t := float64(i+1) / float64(sampling)
		p := cubic(t, a.P, a.RH, b.LH, b.P)
		dx, dy, dz := p[0]-prev[0], p[1]-prev[1], p[2]-prev[2]
		total += math.Sqrt(dx*dx + dy*dy + dz*dz)
		prev = p
	}
	return total
}

// EvalBezier 在归一化时间 t∈[0,1] 处求曲线上点（GetPoint 等价）。
func EvalBezier(c kmdata.Curve, t float64) [3]float64 {
	pts := c.Points
	n := len(pts)
	switch {
	case n == 0:
		return [3]float64{}
	case n == 1:
		return pts[0].P
	case t <= 0:
		return pts[0].P
	}

	sampling := c.Sampling
	if sampling <= 0 {
		sampling = 25 // BezierCurve3D 序列化默认值
	}
	sub := sampling/(n-1) + 1

	segs := make([]float64, n-1)
	total := 0.0
	for i := range segs {
		segs[i] = segLen(pts[i], pts[i+1], sub)
		total += segs[i]
	}
	if total <= 0 {
		return pts[n-1].P
	}

	// GetCubicSegment：累计占比严格大于 t 时选中该段；
	// 未命中（t≈1 浮点欠和）回退最后一段
	totalPercent := 0.0
	seg := -1
	subPercent := 0.0
	for i := 0; i < n-1; i++ {
		subPercent = segs[i] / total
		if subPercent+totalPercent > t {
			seg = i
			break
		}
		totalPercent += subPercent
	}
	if seg < 0 {
		seg = n - 2
		subPercent = segs[seg] / total
		totalPercent -= subPercent
	}
	tt := (t - totalPercent) / subPercent
	return cubic(tt, pts[seg].P, pts[seg].RH, pts[seg+1].LH, pts[seg+1].P)
}
