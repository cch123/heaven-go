package engine

// CameraAt 返回 beat 时刻的相机世界位置（默认 (0,0,-10)）。
// 事件按拍序折叠：进行中的事件从上一事件的终值缓动到自身目标。
func (a *App) CameraAt(beat float64) [3]float64 {
	pos := [3]float64{0, 0, -10}
	last := pos
	for _, e := range a.camEvts {
		prog := 0.0
		if beat >= e.beat {
			if e.length > 0 {
				prog = (beat - e.beat) / e.length
			} else {
				prog = 1
			}
		} else {
			continue
		}
		p := prog
		if p > 1 {
			p = 1
		}
		switch e.axis {
		case 1: // X
			pos[0] = Ease(e.ease, last[0], e.target[0], p)
		case 2: // Y
			pos[1] = Ease(e.ease, last[1], e.target[1], p)
		case 3: // Z
			pos[2] = Ease(e.ease, last[2], e.target[2], p)
		default:
			for i := 0; i < 3; i++ {
				pos[i] = Ease(e.ease, last[i], e.target[i], p)
			}
		}
		if prog > 1 {
			switch e.axis {
			case 1:
				last[0] = e.target[0]
			case 2:
				last[1] = e.target[1]
			case 3:
				last[2] = e.target[2]
			default:
				last = e.target
			}
		}
	}
	return pos
}
