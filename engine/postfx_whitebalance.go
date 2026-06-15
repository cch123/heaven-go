package engine

// whiteBalance 计算 PPv2 的 LMS 白平衡系数（temperature/tint ∈ [-100,100]）。
func whiteBalance(temp, tint float64) (float64, float64, float64) {
	t1, t2 := temp/60, tint/60 // PPv2: range scaled /60
	x := 0.31271 - t1*b2f(t1 < 0, 0.1, 0.05)
	y := standardIlluminantY(x) + t2*0.05
	// CIExyToLMS
	Y := 1.0
	X := Y * x / y
	Z := Y * (1 - x - y) / y
	L := 0.7328*X + 0.4296*Y - 0.1624*Z
	M := -0.7036*X + 1.6975*Y + 0.0061*Z
	S := 0.0030*X + 0.0136*Y + 0.9834*Z
	// D65 白点的 LMS
	const w1L, w1M, w1S = 0.949237, 1.03542, 1.08728
	return w1L / L, w1M / M, w1S / S
}

func standardIlluminantY(x float64) float64 {
	return 2.87*x - 3*x*x - 0.27509507
}

func b2f(c bool, t, f float64) float64 {
	if c {
		return t
	}
	return f
}
