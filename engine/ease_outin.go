package engine

import "math"

func easeOutIn(kind int, d, v float64) float64 {
	switch kind {
	case 33: // EaseOutInQuad
		v *= 2
		if v < 1 {
			return d / 2 * (1 - (1-v)*(1-v))
		}
		v--
		return d/2 + d/2*v*v
	case 34: // EaseOutInCubic
		v *= 2
		if v < 1 {
			return d / 2 * (1 - math.Pow(1-v, 3))
		}
		v--
		return d/2 + d/2*v*v*v
	case 35: // EaseOutInQuart
		v *= 2
		if v < 1 {
			return d / 2 * (1 - math.Pow(1-v, 4))
		}
		v--
		return d/2 + d/2*v*v*v*v
	case 36: // EaseOutInQuint
		v *= 2
		if v < 1 {
			return d / 2 * (1 - math.Pow(1-v, 5))
		}
		v--
		return d/2 + d/2*v*v*v*v*v
	case 37: // EaseOutInSine
		v *= 2
		if v < 1 {
			return d / 2 * math.Sin(v*math.Pi/2)
		}
		v--
		return d / 2 * (math.Sin(v*math.Pi/2) + 1)
	case 38: // EaseOutInExpo
		v *= 2
		if v < 1 {
			return d / 2 * (1 - math.Pow(2, -10*v))
		}
		v--
		return d/2 + d/2*math.Pow(2, 10*(v-1))
	case 39: // EaseOutInCirc
		v *= 2
		if v < 1 {
			return d / 2 * math.Sqrt(1-(v-1)*(v-1))
		}
		v--
		return d/2 - d/2*(math.Sqrt(1-v*v)-1)
	case 40: // EaseOutInBounce
		if v < 0.5 {
			return bounceOut(d, v*2) / 2
		}
		return (d-bounceOut(d, 1-(v*2-1)))/2 + d/2
	case 41: // EaseOutInBack
		const s = 1.70158
		if v < 0.5 {
			w := v*2 - 1
			return d / 2 * (w*w*((s+1)*w+s) + 1)
		}
		w := v*2 - 1
		return d/2 + d/2*w*w*((s+1)*w-s)
	default: // EaseOutInElastic
		if v < 0.5 {
			return elasticOut(d, v*2) / 2
		}
		return d/2 + elasticIn(d, v*2-1)/2
	}
}
