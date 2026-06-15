// ease.go: HS Util.EasingFunction 的移植（按枚举值索引，0..43 全量）。
// 枚举值与 EasingFunctions.cs 的 Ease 枚举一一对应（手工编号，不连续：
// OutIn 变体在 33..42，InstantOut=43）。
package engine

import "log"

var easeWarned = map[int]bool{}

// Ease 按 HS Util.EasingFunction.Ease 枚举值缓动（value 钳制 [0,1]，
// 与调用方 Mathf.Clamp01(GetPositionFromBeat) 同语义；Instant 恒为终值）。
func Ease(kind int, start, end, v float64) float64 {
	v = clamp01(v)
	d := end - start
	switch kind {
	case 0: // Linear
		return start + d*v
	case 1: // Instant
		return end
	case 2, 3, 4:
		return start + easeQuad(kind, d, v)
	case 5, 6, 7:
		return start + easeCubic(kind, d, v)
	case 8, 9, 10:
		return start + easeQuart(kind, d, v)
	case 11, 12, 13:
		return start + easeQuint(kind, d, v)
	case 14, 15, 16:
		return start + easeSine(kind, d, v)
	case 17, 18, 19:
		return start + easeExpo(kind, d, v)
	case 20, 21, 22:
		return start + easeCirc(kind, d, v)
	case 23, 24, 25:
		return start + easeBounce(kind, d, v)
	case 26, 27, 28:
		return start + easeBack(kind, d, v)
	case 29, 30, 31:
		return start + easeElastic(kind, d, v)
	case 33, 34, 35, 36, 37, 38, 39, 40, 41, 42:
		return start + easeOutIn(kind, d, v)
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
