package powercalligraphy

import (
	"strconv"

	"hsdemo/engine"
	"hsdemo/kart"
)

const (
	soundNone = iota
	soundBrushTap
	soundBrush1
	soundBrush2
	soundBrush3
	soundReShout
	soundComma1
	soundComma2
	soundComma3
)

const (
	strokeNone = iota
	strokeTome
	strokeHane
	strokeHarai
)

const (
	fudeNone = iota
	fudeRelease
	fudeTap
	fudePrepare
)

type patternItem struct {
	beat        float64
	soundType   int
	soundVolume float64
	stroke      int
	fudeAnim    int
}

type paperDef struct {
	root     string
	typ      int
	nextBeat float64
	pattern  []patternItem
}

type paperInst struct {
	mod  *Module
	def  paperDef
	inst *kart.Instance

	startBeat   float64
	ongoingBeat float64
	onGoing     bool
	finished    bool
	dead        bool
	processNum  int
	stroke      int
	scrollSpeed [2]float64
	scroll      [2]float64
	finishWorld kart.Aff
}

func loadPaperDef(as *kart.Assets, root string, typ int) (paperDef, bool) {
	comp, ok := componentByPath(as, "writing", root)
	if !ok {
		return paperDef{}, false
	}
	items := comp.Lists["AnimPattern"]
	if len(items) == 0 {
		return paperDef{}, false
	}
	def := paperDef{root: root, typ: typ}
	for _, it := range items {
		nums := it.Nums
		item := patternItem{
			beat:        nums["beat"],
			soundType:   int(nums["soundType"]),
			soundVolume: numDefault(nums, "soundVolume", 1),
			stroke:      int(nums["stroke"]),
			fudeAnim:    int(nums["fudeAnim"]),
		}
		def.pattern = append(def.pattern, item)
		def.nextBeat = item.beat
	}
	return def, true
}

func (p *paperInst) play() {
	p.inst.SetGroupOrder(2)
	animNum := 0
	for _, item := range p.def.pattern {
		item := item
		itemBeat := p.startBeat + item.beat
		if snd := soundName(item.soundType); snd != "" {
			p.mod.ctx.SoundAt(itemBeat, snd, item.soundVolume)
		}
		switch item.fudeAnim {
		case fudeRelease:
			animNum++
			cur := animNum
			p.mod.ctx.At(itemBeat, func() {
				p.anim(cur, "", itemBeat)
				p.mod.playFude("fude-none", itemBeat, 0.5)
			})
		case fudeTap:
			animNum++
			cur := animNum
			p.mod.ctx.At(itemBeat, func() {
				p.anim(cur, "", itemBeat)
				p.mod.playFude("fude-tap", itemBeat, 0.5)
			})
		case fudePrepare:
			p.mod.ctx.At(itemBeat, func() { p.mod.playFude("fude-prepare", itemBeat, 0.5) })
		}
		switch item.stroke {
		case strokeTome:
			animNum++
			cur := animNum
			p.mod.ctx.At(itemBeat, func() {
				p.halt(itemBeat)
				p.stroke = strokeTome
				p.processNum = cur
			})
			p.mod.ctx.At(itemBeat, func() { p.beginOngoing(itemBeat) })
			p.scheduleInput(itemBeat+1, actionBasic)
		case strokeHane, strokeHarai:
			animNum++
			cur := animNum
			stroke := item.stroke
			p.mod.ctx.At(itemBeat, func() {
				p.sweep(itemBeat)
				p.stroke = stroke
				p.processNum = cur
			})
			p.mod.ctx.At(itemBeat+1, func() { p.beginOngoing(itemBeat + 1) })
			p.scheduleInput(itemBeat+2, actionFlick)
		}
	}
	finishBeat := p.startBeat + p.def.nextBeat
	p.mod.ctx.At(finishBeat, func() { p.finish(finishBeat) })
}

func (p *paperInst) scheduleInput(target float64, action int) {
	p.mod.ctx.ScheduleInputActionCond(target, action, p.canSuccess, func(state float64, _ engine.Judgment) {
		switch {
		case state >= 1:
			p.processInput("late")
			p.mod.chouninMiss()
		case state <= -1:
			p.processInput("fast")
			p.mod.chouninMiss()
		default:
			p.processInput("just")
		}
	}, func() {
		if p.onGoing {
			p.miss()
		}
	})
}

func (p *paperInst) beginOngoing(beat float64) {
	p.onGoing = true
	p.ongoingBeat = beat
}

func (p *paperInst) canSuccess() bool { return p.onGoing }

func (p *paperInst) halt(beat float64) {
	p.mod.playFude("fude-halt", beat, p.mod.ctx.SecPerBeat(beat))
	p.mod.ctx.Sound("releaseB1")
}

func (p *paperInst) sweep(beat float64) {
	p.mod.playFude("fude-sweep", beat, p.mod.ctx.SecPerBeat(beat))
	p.mod.ctx.Sound("releaseA1")
}

func (p *paperInst) finish(beat float64) {
	p.finished = true
	p.inst.SetGroupOrder(3)
	p.mod.ctx.SampleScene(beat)
	if w, ok := p.mod.ctx.Scene.NodeWorld(p.mod.shiftRoot); ok {
		p.finishWorld = w
	} else {
		p.finishWorld = p.mod.activePaperWorld()
	}
	p.mod.playFude("fude-none", beat, p.mod.ctx.SecPerBeat(beat))
}

func (p *paperInst) processInput(input string) {
	p.onGoing = false
	p.anim(p.processNum, input, p.mod.ctx.Beat())
	switch input {
	case "just":
		switch p.stroke {
		case strokeTome:
			p.mod.playFude("fude-tap", p.mod.ctx.Beat(), 0.5)
			p.mod.ctx.Sound("releaseB2")
		case strokeHane, strokeHarai:
			p.mod.playFude("fude-none", p.mod.ctx.Beat(), 0.5)
			p.mod.ctx.Sound("releaseA2")
		}
	case "late", "fast":
		p.mod.playFude("fude-none", p.mod.ctx.Beat(), 0.5)
		switch p.stroke {
		case strokeTome:
			p.mod.ctx.Sound("8")
		case strokeHane:
			p.mod.ctx.Sound("6")
		case strokeHarai:
			p.mod.ctx.Sound("9")
		}
	}
}

func (p *paperInst) miss() {
	p.onGoing = false
	p.mod.ctx.Sound("7")
	p.anim(p.processNum, "miss", p.mod.ctx.Beat())
	switch p.stroke {
	case strokeTome:
		p.mod.playFude("fude-none", p.mod.ctx.Beat(), 0.5)
	case strokeHane, strokeHarai:
		p.mod.playFude("fude-sweep-end", p.mod.ctx.Beat(), 0.5)
	}
}

func (p *paperInst) anim(num int, suffix string, beat float64) {
	state := strconv.Itoa(num) + suffix
	p.mod.playSceneVariant(p.mod.fudePos, fudePosController(p.def.typ), state, beat, 0.5)
	p.mod.playSceneVariant(p.mod.shiftAnim, shiftController(p.def.typ), state, beat, 0.5)
	p.inst.PlayState("", state, beat, 0.5)
}

func (p *paperInst) update(beat, dBeat float64) {
	if p.ongoingBeat > 0 {
		u := beat - p.ongoingBeat
		redRate := 1.5 - u
		if u <= 0.5 {
			redRate = u / 0.5
		}
		p.mod.fude.redRate = redRate
	}
	if !p.finished {
		return
	}
	p.scroll[0] += p.scrollSpeed[0] * dBeat / 2
	p.scroll[1] += p.scrollSpeed[1] * dBeat / 2
	if beat >= p.startBeat+24 {
		p.dead = true
	}
}

func (p *paperInst) queue(sc *kart.SceneInst, beat float64, activeWorld kart.Aff) {
	if p.dead {
		return
	}
	world := activeWorld
	if p.finished {
		world = kart.Translate(p.scroll[0], p.scroll[1]).Mul(p.finishWorld)
	}
	p.inst.Queue(sc, beat, world, 0)
}

func soundName(typ int) string {
	switch typ {
	case soundBrushTap:
		return "brushTap"
	case soundBrush1:
		return "brush1"
	case soundBrush2:
		return "brush2"
	case soundBrush3:
		return "brush3"
	case soundReShout:
		return "reShout"
	case soundComma1:
		return "comma1"
	case soundComma2:
		return "comma2"
	case soundComma3:
		return "comma3"
	default:
		return ""
	}
}
