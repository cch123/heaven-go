package engine

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"sort"
	"strings"

	"github.com/hajimehoshi/ebiten/v2/audio/mp3"
	"github.com/hajimehoshi/ebiten/v2/audio/vorbis"
	"github.com/hajimehoshi/ebiten/v2/audio/wav"

	"hsdemo/conductor"
	"hsdemo/riq"
)

// ---------- 谱面装载 ----------

func (a *App) loadRiq(r *riq.Riq) error {
	// 解码音乐
	var (
		stream io.Reader
		err    error
	)
	br := bytes.NewReader(r.Audio)
	switch r.AudioFormat {
	case riq.AudioWAV:
		stream, err = wav.DecodeWithSampleRate(SampleRate, br)
	case riq.AudioOGG:
		stream, err = vorbis.DecodeWithSampleRate(SampleRate, br)
	case riq.AudioMP3:
		stream, err = mp3.DecodeWithSampleRate(SampleRate, br)
	default:
		return fmt.Errorf("unsupported audio format (%s)", r.AudioName)
	}
	if err != nil {
		return fmt.Errorf("decode music: %w", err)
	}
	player, err := audioCtx.NewPlayer(stream)
	if err != nil {
		return err
	}
	player.SetVolume(0.85)

	if a.player != nil {
		a.player.Close()
	}

	a.r, a.bm = r, r.Beatmap
	a.player = player
	a.cond = conductor.New(r.Beatmap, player)
	a.modules = map[string]Module{}
	a.active = nil
	a.switches = nil
	a.actions = nil
	a.inputs = nil
	a.scores = nil
	a.flashes = nil
	a.camEvts = nil
	a.musicFades = nil
	a.viewScales = nil
	a.fx.reset()
	a.flt.reset()
	a.tbx.reset()
	a.unported = nil
	a.starBeat, a.endBeat = -1, 0
	a.resetRunState()

	// 找出谱面用到的游戏并实例化模块
	used := map[string]bool{}
	for i := range a.bm.Entities {
		e := &a.bm.Entities[i]
		if g, ok := strings.CutPrefix(e.Datamodel, "gameManager/switchGame/"); ok {
			a.switches = append(a.switches, gameSwitch{e.Beat, g})
			used[g] = true
			continue
		}
		switch e.Game() {
		case "gameManager", "vfx", "countIn", "global", "ppe":
			continue // ppe（屏幕后处理）由引擎处理，不占模块位
		}
		used[e.Game()] = true
	}
	for id := range used {
		var m Module
		if f, ok := registry[id]; ok {
			m = f()
		} else {
			m = newPlaceholder(id)
			a.unported = append(a.unported, id)
		}
		ctx := &Ctx{App: a, module: m}
		if err := m.Load(ctx); err != nil {
			return fmt.Errorf("load %s assets: %w (run go run ./cmd/extract -game %s first)", id, err, id)
		}
		a.modules[id] = m
	}
	sort.Strings(a.unported)
	sort.Slice(a.switches, func(i, j int) bool { return a.switches[i].beat < a.switches[j].beat })
	sort.Slice(a.flashes, func(i, j int) bool { return a.flashes[i].beat < a.flashes[j].beat })

	// 分发实体
	for i := range a.bm.Entities {
		e := &a.bm.Entities[i]
		switch {
		case strings.HasPrefix(e.Datamodel, "gameManager/switchGame/"):
			// 已在上面收集
		case e.Datamodel == "gameManager/end":
			a.endBeat = e.Beat
		case e.Datamodel == "gameManager/skill star":
			a.starBeat = e.Beat + e.Length
		case e.Datamodel == "gameManager/toggle inputs":
			on := boolParam(e, "toggle")
			b := e.Beat
			a.at(b, func() { a.inputOn = on })
		case e.Datamodel == "vfx/flash":
			a.flashes = append(a.flashes, flashEvt{
				beat: e.Beat, length: e.Length,
				c0: colorParam(e, "colorA"), c1: colorParam(e, "colorB"),
			})
		case e.Datamodel == "vfx/scale view":
			a.viewScales = append(a.viewScales, viewScaleEvt{
				beat: e.Beat, length: e.Length,
				x: e.Float("valA", 1), y: e.Float("valB", 1),
				ease: int(e.Float("ease", 0)),
				axis: int(e.Float("axis", 0)),
			})
		case e.Datamodel == "vfx/move camera" || e.Datamodel == "gameManager/move camera":
			a.camEvts = append(a.camEvts, camEvt{
				beat: e.Beat, length: e.Length,
				target: [3]float64{e.Float("valA", 0), e.Float("valB", 0), -e.Float("valC", 10)},
				ease:   int(e.Float("ease", 0)),
				axis:   int(e.Float("axis", 0)),
			})
		case e.Game() == "countIn":
			a.scheduleCountIn(e.Datamodel, e.Beat, e.Length, e.Data)
		case e.Datamodel == "vfx/filter":
			a.flt.add(e)
		case e.Datamodel == "vfx/display textbox":
			a.tbx.add(e)
		case e.Game() == "ppe":
			a.fx.add(e)
		case e.Game() == "gameManager" || e.Game() == "vfx" || e.Game() == "global":
			// 其余全局事件暂不支持
		default:
			if m, ok := a.modules[e.Game()]; ok {
				m.OnEvent(e)
			}
		}
	}
	for _, m := range a.modules {
		m.Ready()
	}
	if a.endBeat == 0 && len(a.bm.Entities) > 0 {
		last := a.bm.Entities[len(a.bm.Entities)-1]
		a.endBeat = last.Beat + last.Length + 4
	}
	sortActions(a.actions)
	sort.Slice(a.inputs, func(i, j int) bool { return a.inputs[i].Beat < a.inputs[j].Beat })
	a.fx.sortAll()
	if a.fx.active() {
		if err := a.fx.ensure(); err != nil {
			return fmt.Errorf("compile ppe shader: %w", err)
		}
	}

	// 初始活动游戏
	if len(a.switches) > 0 {
		a.setActive(a.switches[0].id, 0)
		a.swIdx = 1
	}
	log.Printf("riq loaded: %q by %q, %d entities, games=%v unported=%v",
		a.bm.Prop("remixtitle"), a.bm.Prop("remixauthor"), len(a.bm.Entities), keys(a.modules), a.unported)
	return nil
}

func keys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func sortActions(as []beatAction) {
	sort.SliceStable(as, func(i, j int) bool { return as[i].beat < as[j].beat })
}

func (a *App) setActive(id string, beat float64) {
	if m, ok := a.modules[id]; ok {
		a.active = m
		m.OnSwitch(beat)
	}
}

func (a *App) resetRunState() {
	a.swIdx = 0
	a.inputOn = true
	a.starGot = false
	a.aces, a.justs, a.ngs, a.misses, a.whiffs = 0, 0, 0, 0, 0
	a.lastMsg, a.msgT = "", 0
	a.tdArrow, a.tdTarget, a.tdHits = 0, 0, nil
	a.scores = nil
	a.result, a.resultT, a.resultEpilogue = resultSummary{}, 0, false
	a.stopResultAudio()
}

// restart 重置一轮游玩（不重载资产）。
func (a *App) restart() error {
	if err := a.cond.Reset(); err != nil {
		return err
	}
	for _, in := range a.inputs {
		in.judged = false
		in.Result = JudgeNone
	}
	a.actIdx = 0
	a.resetRunState()
	if len(a.switches) > 0 {
		a.setActive(a.switches[0].id, 0)
		a.swIdx = 1
	}
	a.state = stateTitle
	return nil
}

// returnToLevelSelect unloads the active chart so stateTitle falls through to
// the Library selector instead of the current chart's title card.
func (a *App) returnToLevelSelect() {
	if a.player != nil {
		a.player.Close()
		a.player = nil
	}
	a.r, a.bm, a.cond = nil, nil, nil
	a.modules = nil
	a.active = nil
	a.switches = nil
	a.actions = nil
	a.inputs = nil
	a.flashes = nil
	a.camEvts = nil
	a.musicFades = nil
	a.viewScales = nil
	a.viewBuf = nil
	a.unported = nil
	a.actIdx = 0
	a.starBeat, a.endBeat = -1, 0
	a.fx.reset()
	a.flt.reset()
	a.tbx.reset()
	a.resetRunState()
	a.loadErr = ""
	a.state = stateTitle
	a.keepMenuSelectionVisible()
}
