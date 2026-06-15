package engine

import "math"

func easeBounce(kind int, d, v float64) float64 {
	switch kind {
	case 23: // EaseInBounce
		return d - bounceOut(d, 1-v)
	case 24: // EaseOutBounce
		return bounceOut(d, v)
	default: // EaseInOutBounce
		if v < 0.5 {
			return (d - bounceOut(d, 1-v*2)) / 2
		}
		return bounceOut(d, v*2-1)/2 + d/2
	}
}

// bounceOut: EaseOutBounce 的差量形式（start=0，幅度 d）。
func bounceOut(d, v float64) float64 {
	switch {
	case v < 1/2.75:
		return d * 7.5625 * v * v
	case v < 2/2.75:
		v -= 1.5 / 2.75
		return d * (7.5625*v*v + 0.75)
	case v < 2.5/2.75:
		v -= 2.25 / 2.75
		return d * (7.5625*v*v + 0.9375)
	default:
		v -= 2.625 / 2.75
		return d * (7.5625*v*v + 0.984375)
	}
}

func easeBack(kind int, d, v float64) float64 {
	switch kind {
	case 26: // EaseInBack
		const s = 1.70158
		return d * v * v * ((s+1)*v - s)
	case 27: // EaseOutBack
		const s = 1.70158
		v--
		return d * (v*v*((s+1)*v+s) + 1)
	default: // EaseInOutBack
		s := 1.70158 * 1.525
		v *= 2
		if v < 1 {
			return d / 2 * (v * v * ((s+1)*v - s))
		}
		v -= 2
		return d / 2 * (v*v*((s+1)*v+s) + 2)
	}
}

func easeElastic(kind int, d, v float64) float64 {
	switch kind {
	case 29: // EaseInElastic
		return elasticIn(d, v)
	case 30: // EaseOutElastic
		return elasticOut(d, v)
	default: // EaseInOutElastic
		if v == 0 {
			return 0
		}
		v *= 2
		if v == 2 {
			return d
		}
		p := 0.3
		s := p / 4
		a := d
		if v < 1 {
			v--
			return -0.5 * a * math.Pow(2, 10*v) * math.Sin((v-s)*2*math.Pi/p)
		}
		v--
		return d + 0.5*a*math.Pow(2, -10*v)*math.Sin((v-s)*2*math.Pi/p)
	}
}

// elasticIn/elasticOut: EaseIn/OutElastic 的差量形式（a=d、p=0.3、s=p/4，
// 与 HS 实现中 a==0 分支的取值一致）。
func elasticIn(d, v float64) float64 {
	if v == 0 {
		return 0
	}
	if v == 1 {
		return d
	}
	p := 0.3
	s := p / 4
	v--
	return -d * math.Pow(2, 10*v) * math.Sin((v-s)*2*math.Pi/p)
}

func elasticOut(d, v float64) float64 {
	if v == 0 {
		return 0
	}
	if v == 1 {
		return d
	}
	p := 0.3
	s := p / 4
	return d*math.Pow(2, -10*v)*math.Sin((v-s)*2*math.Pi/p) + d
}
