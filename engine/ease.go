// ease.go：HS Util.EasingFunction 的移植（按枚举值索引）。
// 仅实现官方关卡用到的曲线族；未实现的枚举值回退线性并打日志。
package engine

import (
	"log"
	"math"
)

var easeWarned = map[int]bool{}

// Ease 按 HS Util.EasingFunction.Ease 枚举值缓动（value 自动钳制 [0,1]，
// Mathf.Lerp 同语义；Instant 恒为终值）。
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
	case 14: // EaseInSine
		return start + d - d*math.Cos(v*math.Pi/2)
	case 15: // EaseOutSine
		return start + d*math.Sin(v*math.Pi/2)
	case 16: // EaseInOutSine
		return start - d/2*(math.Cos(math.Pi*v)-1)
	default:
		if !easeWarned[kind] {
			easeWarned[kind] = true
			log.Printf("engine: 缓动 %d 未实现，回退线性（需要时补 ease.go）", kind)
		}
		return start + d*v
	}
}
