// ease.go：HS Util.EasingFunction 的移植（按枚举值索引，0..43 全量）。
// 枚举值与 EasingFunctions.cs 的 Ease 枚举一一对应（手工编号，不连续：
// OutIn 变体在 33..42，InstantOut=43）。
package engine

import (
	"log"
	"math"
)

var easeWarned = map[int]bool{}

// Ease 按 HS Util.EasingFunction.Ease 枚举值缓动（value 钳制 [0,1]，
// 与调用方 Mathf.Clamp01(GetPositionFromBeat) 同语义；Instant 恒为终值）。
func Ease(kind int, start, end, v float64) float64 {
	if v < 0 {
		v = 0
	}
	if v > 1 {
		v = 1
	}
	d := end - start
	switch kind {
	case 0: // Linear
		return start + d*v
	case 1: // Instant
		return end
	case 2: // EaseInQuad
		return start + d*v*v
	case 3: // EaseOutQuad
		return start - d*v*(v-2)
	case 4: // EaseInOutQuad
		v *= 2
		if v < 1 {
			return start + d/2*v*v
		}
		v--
		return start - d/2*(v*(v-2)-1)
	case 5: // EaseInCubic
		return start + d*v*v*v
	case 6: // EaseOutCubic
		v--
		return start + d*(v*v*v+1)
	case 7: // EaseInOutCubic
		v *= 2
		if v < 1 {
			return start + d/2*v*v*v
		}
		v -= 2
		return start + d/2*(v*v*v+2)
	case 8: // EaseInQuart
		return start + d*v*v*v*v
	case 9: // EaseOutQuart
		v--
		return start - d*(v*v*v*v-1)
	case 10: // EaseInOutQuart
		v *= 2
		if v < 1 {
			return start + d/2*v*v*v*v
		}
		v -= 2
		return start - d/2*(v*v*v*v-2)
	case 11: // EaseInQuint
		return start + d*v*v*v*v*v
	case 12: // EaseOutQuint
		v--
		return start + d*(v*v*v*v*v+1)
	case 13: // EaseInOutQuint
		v *= 2
		if v < 1 {
			return start + d/2*v*v*v*v*v
		}
		v -= 2
		return start + d/2*(v*v*v*v*v+2)
	case 14: // EaseInSine
		return start + d - d*math.Cos(v*math.Pi/2)
	case 15: // EaseOutSine
		return start + d*math.Sin(v*math.Pi/2)
	case 16: // EaseInOutSine
		return start - d/2*(math.Cos(math.Pi*v)-1)
	case 17: // EaseInExpo
		return start + d*math.Pow(2, 10*(v-1))
	case 18: // EaseOutExpo
		return start + d*(1-math.Pow(2, -10*v))
	case 19: // EaseInOutExpo
		v *= 2
		if v < 1 {
			return start + d/2*math.Pow(2, 10*(v-1))
		}
		v--
		return start + d/2*(2-math.Pow(2, -10*v))
	case 20: // EaseInCirc
		return start - d*(math.Sqrt(1-v*v)-1)
	case 21: // EaseOutCirc
		v--
		return start + d*math.Sqrt(1-v*v)
	case 22: // EaseInOutCirc
		v *= 2
		if v < 1 {
			return start - d/2*(math.Sqrt(1-v*v)-1)
		}
		v -= 2
		return start + d/2*(math.Sqrt(1-v*v)+1)
	case 23: // EaseInBounce
		return start + d - bounceOut(d, 1-v)
	case 24: // EaseOutBounce
		return start + bounceOut(d, v)
	case 25: // EaseInOutBounce
		if v < 0.5 {
			return start + (d-bounceOut(d, 1-v*2))/2
		}
		return start + bounceOut(d, v*2-1)/2 + d/2
	case 26: // EaseInBack
		const s = 1.70158
		return start + d*v*v*((s+1)*v-s)
	case 27: // EaseOutBack
		const s = 1.70158
		v--
		return start + d*(v*v*((s+1)*v+s)+1)
	case 28: // EaseInOutBack
		s := 1.70158 * 1.525
		v *= 2
		if v < 1 {
			return start + d/2*(v*v*((s+1)*v-s))
		}
		v -= 2
		return start + d/2*(v*v*((s+1)*v+s)+2)
	case 29: // EaseInElastic
		return start + elasticIn(d, v)
	case 30: // EaseOutElastic
		return start + elasticOut(d, v)
	case 31: // EaseInOutElastic
		if v == 0 {
			return start
		}
		v *= 2
		if v == 2 {
			return start + d
		}
		p := 0.3
		s := p / 4
		a := d
		if v < 1 {
			v--
			return start - 0.5*a*math.Pow(2, 10*v)*math.Sin((v-s)*2*math.Pi/p)
		}
		v--
		return start + d + 0.5*a*math.Pow(2, -10*v)*math.Sin((v-s)*2*math.Pi/p)
	case 33: // EaseOutInQuad
		v *= 2
		if v < 1 {
			return start + d/2*(1-(1-v)*(1-v))
		}
		v--
		return start + d/2 + d/2*v*v
	case 34: // EaseOutInCubic
		v *= 2
		if v < 1 {
			return start + d/2*(1-math.Pow(1-v, 3))
		}
		v--
		return start + d/2 + d/2*v*v*v
	case 35: // EaseOutInQuart
		v *= 2
		if v < 1 {
			return start + d/2*(1-math.Pow(1-v, 4))
		}
		v--
		return start + d/2 + d/2*v*v*v*v
	case 36: // EaseOutInQuint
		v *= 2
		if v < 1 {
			return start + d/2*(1-math.Pow(1-v, 5))
		}
		v--
		return start + d/2 + d/2*v*v*v*v*v
	case 37: // EaseOutInSine
		v *= 2
		if v < 1 {
			return start + d/2*math.Sin(v*math.Pi/2)
		}
		v--
		return start + d/2*(math.Sin(v*math.Pi/2)+1)
	case 38: // EaseOutInExpo
		v *= 2
		if v < 1 {
			return start + d/2*(1-math.Pow(2, -10*v))
		}
		v--
		return start + d/2 + d/2*math.Pow(2, 10*(v-1))
	case 39: // EaseOutInCirc
		v *= 2
		if v < 1 {
			return start + d/2*math.Sqrt(1-(v-1)*(v-1))
		}
		v--
		return start + d/2 - d/2*(math.Sqrt(1-v*v)-1)
	case 40: // EaseOutInBounce
		if v < 0.5 {
			return start + bounceOut(d, v*2)/2
		}
		return start + (d-bounceOut(d, 1-(v*2-1)))/2 + d/2
	case 41: // EaseOutInBack
		const s = 1.70158
		if v < 0.5 {
			w := v*2 - 1
			return start + d/2*(w*w*((s+1)*w+s)+1)
		}
		w := v*2 - 1
		return start + d/2 + d/2*w*w*((s+1)*w-s)
	case 42: // EaseOutInElastic
		if v < 0.5 {
			return start + elasticOut(d, v*2)/2
		}
		return start + d/2 + elasticIn(d, v*2-1)/2
	case 43: // InstantOut
		if v >= 1 {
			return end
		}
		return start
	default:
		if !easeWarned[kind] {
			easeWarned[kind] = true
			log.Printf("engine: 缓动 %d 未实现，回退线性（需要时补 ease.go）", kind)
		}
		return start + d*v
	}
}

// bounceOut：EaseOutBounce 的差量形式（start=0，幅度 d）。
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

// elasticIn/elasticOut：EaseIn/OutElastic 的差量形式（a=d、p=0.3、s=p/4，
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
