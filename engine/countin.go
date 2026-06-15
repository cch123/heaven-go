// countin.go：countIn 事件调度（SoundEffects.cs 的移植：计数音、ready、
// go、and、cowbell）。assets/common 音效加载在 common_sounds.go。
package engine

import (
	"log"
	"strings"
)

// SoundEffects.cs 的计数表：8 拍数法 one,two,one,two,three,four @ 0,2,4,5,6,7。
var (
	countNames   = []string{"one", "two", "one", "two", "three", "four"}
	countTimings = []float64{0, 2, 4, 5, 6, 7}
)

type countInStyle struct {
	suffix  string
	folder  string
	cowbell bool
}

// countStyle 对应 SoundEffects.GetCountInSound/GetCountInFolder：
// 0=Normal, 1=Alt, 2=Cowbell, 3=GBA, 4=DSMale, 5=DSFemale。
func countStyle(typ int) countInStyle {
	switch typ {
	case 0:
		return countInStyle{suffix: "1"}
	case 1:
		return countInStyle{suffix: "2"}
	case 2:
		return countInStyle{suffix: "cowbell", cowbell: true}
	case 3:
		return countInStyle{folder: "gba/"}
	case 4:
		return countInStyle{folder: "dsmale/"}
	case 5:
		return countInStyle{folder: "dsfemale/"}
	default:
		log.Printf("engine: countIn 类型 %d 未知，回退 Normal", typ)
		return countInStyle{suffix: "1"}
	}
}

func (s countInStyle) count(name string) string {
	if s.cowbell {
		return "cowbell"
	}
	return s.folder + name + s.suffix
}

func (s countInStyle) and() string { return s.folder + "and" }
func (s countInStyle) goSound() string {
	if s.cowbell {
		return "cowbell"
	}
	return s.folder + "go" + s.suffix
}

// scheduleCountIn 把一个 countIn/* 实体翻译为公共音效调度（载入期调用）。
func (a *App) scheduleCountIn(datamodel string, beat, length float64, data map[string]any) {
	num := func(key string, def float64) float64 {
		if v, ok := data[key].(float64); ok {
			return v
		}
		return def
	}
	flag := func(key string) bool {
		b, _ := data[key].(bool)
		return b
	}
	style := countStyle(int(num("type", 0)))
	cname := func(i int) string { return style.count(countNames[i]) }

	switch strings.TrimPrefix(datamodel, "countIn/") {
	case "count": // 单次计数：type=数字（0=One..3=Four），countType=音色
		style = countStyle(int(num("countType", 0)))
		n := int(num("type", 0))
		names := []string{"one", "two", "three", "four"}
		if n >= 0 && n < len(names) {
			a.commonAt(beat, style.count(names[n]))
		}
	case "cowbell":
		a.commonAt(beat, "cowbell")
	case "and":
		a.commonAt(beat, "and")
	case "go!":
		g := "go1"
		if flag("toggle") {
			g = "go2"
		}
		a.commonAt(beat, g)
	case "ready!":
		a.commonAt(beat, "ready1")
		a.commonAt(beat+length/2, "ready2")
	case "2 beat count-in":
		a.scheduleCounts(beat, length/2, 4, 2, flag("go"), flag("and"), style)
	case "4 beat count-in":
		a.scheduleCounts(beat, length/4, 2, 4, flag("go"), flag("and"), style)
	case "8 beat count-in": // timings × (length/8)
		unit := length / 8
		last := len(countNames) - 1
		for i := range countNames {
			name := cname(i)
			if flag("go") && !style.cowbell && i == last {
				name = style.goSound()
			}
			a.commonAt(beat+countTimings[i]*unit, name)
		}
		if flag("and") && !style.cowbell {
			a.commonAt(beat-0.5, style.and())
		}
	case "count-in": // 拉伸版：startBeat = beat+length-8，绝对 timings
		start := beat + length - 8
		var beats []float64
		var names []string
		for i := range countNames {
			if start+countTimings[i] >= beat {
				beats = append(beats, start+countTimings[i])
				names = append(names, cname(i))
			}
		}
		if flag("go") && !style.cowbell && len(names) > 0 {
			names[len(names)-1] = style.goSound()
		}
		if flag("and") && !style.cowbell {
			andBeat := beat - 0.5
			if s := start + 3.5; s > andBeat {
				andBeat = s
			}
			a.commonAt(andBeat, style.and())
		}
		for i := range beats {
			a.commonAt(beats[i], names[i])
		}
	}
}

// scheduleCounts：2/4 拍数法（countNames 的后 n 个）。
func (a *App) scheduleCounts(beat, unit float64, offset, n int, withGo, withAnd bool, style countInStyle) {
	for i := 0; i < n; i++ {
		name := style.count(countNames[offset+i])
		if !style.cowbell {
			if withGo && i == n-1 {
				name = style.goSound()
			}
		}
		a.commonAt(beat+float64(i)*unit, name)
	}
	if withAnd && !style.cowbell {
		a.commonAt(beat-0.5, style.and())
	}
}
