package engine

import "math"

func easeQuad(kind int, d, v float64) float64 {
	switch kind {
	case 2: // EaseInQuad
		return d * v * v
	case 3: // EaseOutQuad
		return -d * v * (v - 2)
	default: // EaseInOutQuad
		v *= 2
		if v < 1 {
			return d / 2 * v * v
		}
		v--
		return -d / 2 * (v*(v-2) - 1)
	}
}

func easeCubic(kind int, d, v float64) float64 {
	switch kind {
	case 5: // EaseInCubic
		return d * v * v * v
	case 6: // EaseOutCubic
		v--
		return d * (v*v*v + 1)
	default: // EaseInOutCubic
		v *= 2
		if v < 1 {
			return d / 2 * v * v * v
		}
		v -= 2
		return d / 2 * (v*v*v + 2)
	}
}

func easeQuart(kind int, d, v float64) float64 {
	switch kind {
	case 8: // EaseInQuart
		return d * v * v * v * v
	case 9: // EaseOutQuart
		v--
		return -d * (v*v*v*v - 1)
	default: // EaseInOutQuart
		v *= 2
		if v < 1 {
			return d / 2 * v * v * v * v
		}
		v -= 2
		return -d / 2 * (v*v*v*v - 2)
	}
}

func easeQuint(kind int, d, v float64) float64 {
	switch kind {
	case 11: // EaseInQuint
		return d * v * v * v * v * v
	case 12: // EaseOutQuint
		v--
		return d * (v*v*v*v*v + 1)
	default: // EaseInOutQuint
		v *= 2
		if v < 1 {
			return d / 2 * v * v * v * v * v
		}
		v -= 2
		return d / 2 * (v*v*v*v*v + 2)
	}
}

func easeSine(kind int, d, v float64) float64 {
	switch kind {
	case 14: // EaseInSine
		return d - d*math.Cos(v*math.Pi/2)
	case 15: // EaseOutSine
		return d * math.Sin(v*math.Pi/2)
	default: // EaseInOutSine
		return -d / 2 * (math.Cos(math.Pi*v) - 1)
	}
}

func easeExpo(kind int, d, v float64) float64 {
	switch kind {
	case 17: // EaseInExpo
		return d * math.Pow(2, 10*(v-1))
	case 18: // EaseOutExpo
		return d * (1 - math.Pow(2, -10*v))
	default: // EaseInOutExpo
		v *= 2
		if v < 1 {
			return d / 2 * math.Pow(2, 10*(v-1))
		}
		v--
		return d / 2 * (2 - math.Pow(2, -10*v))
	}
}

func easeCirc(kind int, d, v float64) float64 {
	switch kind {
	case 20: // EaseInCirc
		return -d * (math.Sqrt(1-v*v) - 1)
	case 21: // EaseOutCirc
		v--
		return d * math.Sqrt(1-v*v)
	default: // EaseInOutCirc
		v *= 2
		if v < 1 {
			return -d / 2 * (math.Sqrt(1-v*v) - 1)
		}
		v -= 2
		return d / 2 * (math.Sqrt(1-v*v) + 1)
	}
}
