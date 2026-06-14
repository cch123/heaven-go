// countin.go：引擎级公共音效（assets/common）与 countIn 事件
// （SoundEffects.cs 的移植：计数音、ready、go、and、cowbell）。
package engine

import (
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"hsdemo/kart"
)

// loadCommonSounds 加载 assets/common/sounds（可选；缺目录时 countIn 等事件静默跳过）。
func (a *App) loadCommonSounds() {
	dir := filepath.Join(a.assetsRoot, "common", "sounds")
	entries, err := os.ReadDir(dir)
	if err != nil {
		log.Printf("engine: 无公共音效目录 %s（countIn 计数音不可用，运行 go run ./cmd/extract -game common）", dir)
		return
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		pcm, err := kart.DecodePCM(raw, filepath.Ext(e.Name()), SampleRate)
		if err != nil {
			log.Printf("engine: 公共音效 %s 解码失败: %v", e.Name(), err)
			continue
		}
		base := strings.TrimSuffix(e.Name(), filepath.Ext(e.Name()))
		a.commonSounds[strings.ToLower(base)] = pcm
	}
}

// PlayCommon 立即播放公共音效（大小写不敏感，Unity Resources.Load 同语义）。
func (a *App) PlayCommon(name string, vol float64) {
	pcm, ok := a.commonSounds[strings.ToLower(name)]
	if !ok {
		return
	}
	p := audioCtx.NewPlayerFromBytes(pcm)
	p.SetVolume(vol)
	p.Play()
}

// commonAt 在指定拍播放公共音效。
func (a *App) commonAt(beat float64, name string) {
	a.at(beat, func() { a.PlayCommon(name, 1) })
}

// SoundEffects.cs 的计数表：8 拍数法 one,two,one,two,three,four @ 0,2,4,5,6,7。
var (
	countNames   = []string{"one", "two", "one", "two", "three", "four"}
	countTimings = []float64{0, 2, 4, 5, 6, 7}
)

// countSuffix 对应 GetCountInSound：0=Normal "1"，1=Alt "2"，2=Cowbell。
func countSuffix(typ int) string {
	switch typ {
	case 0:
		return "1"
	case 1:
		return "2"
	case 2:
		return "cowbell"
	default:
		// gba/dsmale/dsfemale 变体目录未提取——回退 Normal 并记录
		log.Printf("engine: countIn 类型 %d 的音色目录未提取，回退 Normal", typ)
		return "1"
	}
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
	suffix := countSuffix(int(num("type", 0)))
	cowbell := suffix == "cowbell"
	cname := func(i int) string {
		if cowbell {
			return "cowbell"
		}
		return countNames[i] + suffix
	}

	switch strings.TrimPrefix(datamodel, "countIn/") {
	case "count": // 单次计数：type=数字（0=One..3=Four），countType=音色
		suffix = countSuffix(int(num("countType", 0)))
		n := int(num("type", 0))
		names := []string{"one", "two", "three", "four"}
		if n >= 0 && n < len(names) {
			a.commonAt(beat, names[n]+suffix)
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
		a.scheduleCounts(beat, length/2, 4, 2, flag("go"), flag("and"), cowbell, suffix)
	case "4 beat count-in":
		a.scheduleCounts(beat, length/4, 2, 4, flag("go"), flag("and"), cowbell, suffix)
	case "8 beat count-in": // timings × (length/8)
		unit := length / 8
		last := len(countNames) - 1
		for i := range countNames {
			name := cname(i)
			if flag("go") && !cowbell && i == last {
				name = "go" + suffix
			}
			a.commonAt(beat+countTimings[i]*unit, name)
		}
		if flag("and") && !cowbell {
			a.commonAt(beat-0.5, "and")
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
		if flag("go") && !cowbell && len(names) > 0 {
			names[len(names)-1] = "go" + suffix
		}
		if flag("and") && !cowbell {
			andBeat := beat - 0.5
			if s := start + 3.5; s > andBeat {
				andBeat = s
			}
			a.commonAt(andBeat, "and")
		}
		for i := range beats {
			a.commonAt(beats[i], names[i])
		}
	}
}

// scheduleCounts：2/4 拍数法（countNames 的后 n 个）。
func (a *App) scheduleCounts(beat, unit float64, offset, n int, withGo, withAnd, cowbell bool, suffix string) {
	for i := 0; i < n; i++ {
		name := "cowbell"
		if !cowbell {
			name = countNames[offset+i] + suffix
			if withGo && i == n-1 {
				name = "go" + suffix
			}
		}
		a.commonAt(beat+float64(i)*unit, name)
	}
	if withAnd && !cowbell {
		a.commonAt(beat-0.5, "and")
	}
}

// ---------- 相机（vfx/move camera → GameCamera.UpdateCameraTranslate） ----------

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

// MusicFadeAt returns the minigame-local music volume multiplier. It is
// separate from riq__VolumeChange so games like Tunnel can duck the song
// without rewriting the chart's authored volume events.
func (a *App) MusicFadeAt(beat float64) float64 {
	vol := 1.0
	for _, e := range a.musicFades {
		if beat < e.beat {
			break
		}
		if e.length > 0 && beat < e.beat+e.length {
			u := (beat - e.beat) / e.length
			return e.from + (e.to-e.from)*u
		}
		vol = e.to
	}
	return vol
}

func (a *App) fadeMusicVolume(beat, length, target float64) {
	from := a.MusicFadeAt(beat)
	ev := musicFadeEvt{beat: beat, length: length, from: from, to: target}
	i := sort.Search(len(a.musicFades), func(i int) bool { return a.musicFades[i].beat > beat })
	a.musicFades = append(a.musicFades, musicFadeEvt{})
	copy(a.musicFades[i+1:], a.musicFades[i:])
	a.musicFades[i] = ev
}
