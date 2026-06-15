package fruitbasket

import (
	"math"

	"hsdemo/engine"
	"hsdemo/kart"
)

type activeFruit struct {
	ev       fruitEvt
	inst     *kart.Instance
	sprite   string
	isLeft   bool
	state    int
	dead     bool
	dieBeat  float64
	hitBeat  float64
	missed   bool
	missDir  float64
	path     curvePath
	alpha    float64
	lastT    float64
	hitRotT  float64
	fadeT    float64
	hoopDone bool
}

func (m *Module) spawnFruit(ev fruitEvt) {
	t := m.fruitTemplates[ev.fruit]
	if t == nil {
		return
	}
	f := &activeFruit{
		ev:      ev,
		inst:    t.NewInstance(),
		isLeft:  ev.side == sideLeft,
		alpha:   1,
		lastT:   m.ctx.Time(),
		missDir: 1,
		hitBeat: math.Inf(1),
	}
	f.inst.Offset = [2]float64{}
	f.inst.SetActive("", true)
	f.inst.SetOrder(f.spriteRel(), 2)
	f.sprite = f.spriteRel()
	switch ev.fruit {
	case fruitApple:
		f.dieBeat = ev.beat + 5
		if ev.bothSidesRole == 0 {
			m.ctx.Sound(pick(f.isLeft, "appleL", "appleR"))
		} else if ev.bothSidesRole == 1 {
			m.ctx.Sound("apple")
		}
		m.ctx.ScheduleInput(ev.beat+2,
			func(state float64, _ engine.Judgment) { f.hitApple(m, state) },
			func() { f.missWindow(m) })
	case fruitLemon:
		f.dieBeat = ev.beat + 8
		if ev.bothSidesRole == 0 {
			m.ctx.Sound(pick(f.isLeft, "lemonL", "lemonR"))
		} else if ev.bothSidesRole == 1 {
			m.ctx.Sound("lemon")
		}
		f.path = m.paths[pick(f.isLeft, "LemonRollRight", "LemonRollLeft")]
		m.ctx.ScheduleInput(ev.beat+4,
			func(state float64, _ engine.Judgment) { f.hitLemon(m, state) },
			func() { f.missWindow(m) })
	case fruitMelon:
		f.dieBeat = ev.beat + 6
		if ev.bothSidesRole != 2 {
			m.ctx.SoundAt(ev.beat, "whistle", 1)
			m.ctx.SoundAt(ev.beat+1, "whistle", 1)
		}
		m.ctx.SoundAtOff(ev.beat+2.5, pick(f.isLeft, "melonL", "melonR"), 1, 0)
		m.ctx.ScheduleInput(ev.beat+2,
			func(state float64, _ engine.Judgment) { f.hitMelon(m, state) },
			func() { f.missWindow(m) })
	}
	m.fruits = append(m.fruits, f)
}

func (f *activeFruit) spriteRel() string {
	switch f.ev.fruit {
	case fruitApple:
		return "AppleSprite"
	case fruitLemon:
		return "LemonSprite"
	default:
		return "MelonSprite"
	}
}

func (f *activeFruit) update(m *Module, t, beat float64) {
	if f.dead || f.inst == nil {
		return
	}
	dt := t - f.lastT
	if dt < 0 || f.lastT == 0 {
		dt = 0
	}
	f.lastT = t
	if beat > f.dieBeat {
		f.dead = true
		return
	}
	if f.ev.fruit == fruitMelon {
		m.updateMelonPipe(f.ev, beat)
	}
	switch f.state {
	case 0:
		f.roll(m, beat)
	case 1:
		if f.ev.fruit == fruitApple || f.ev.fruit == fruitMelon {
			f.roll(m, beat)
		}
		f.barely(m, dt, beat)
	case 2:
		if !f.fading() && f.ev.fruit != fruitLemon {
			f.roll(m, beat)
		}
		f.hitUpdate(m, dt, beat)
	}
	if f.fading() && !f.hoopDone && !f.missed {
		f.hoopDone = true
		switch f.ev.fruit {
		case fruitApple:
			m.hoopAnimation("hoopScore", !f.isLeft, fruitApple, beat)
		case fruitLemon:
			m.hoopAnimation("hoopScore", !f.isLeft, fruitLemon, beat)
		case fruitMelon:
			m.hoopAnimation("hoopScoreMelon", f.isLeft, fruitMelon, beat)
		}
	}
}

func (m *Module) updateMelonPipe(ev fruitEvt, beat float64) {
	pipe := pick(ev.side == sideLeft, m.pipeL, m.pipeR)
	if pipe == "" {
		return
	}
	// Melon.cs calls DoScaledAnimation("pipeFlash", startBeat, 3.5f) every
	// Update, which samples the animation by normalized song-beat progress
	// rather than playing it once at real-time clip speed.
	m.ctx.Scene.PlayFrozen(pipe, "pipeFlash", clamp01((beat-ev.beat)/3.5))
}

func (f *activeFruit) fading() bool { return f.alpha < 1 }

func (f *activeFruit) roll(m *Module, beat float64) {
	switch f.ev.fruit {
	case fruitApple:
		u := clamp01((beat - f.ev.beat) / 4)
		x := lerp(pickFloat(f.isLeft, -8, 8), pickFloat(f.isLeft, 8, -8), u)
		rot := lerp(0, pickFloat(f.isLeft, -720, 720), u) * f.missDir
		f.setPos(x, -3.1)
		f.inst.SetRot(f.sprite, deg(rot))
	case fruitLemon:
		f.rollLemon(m, beat)
	case fruitMelon:
		if beat <= f.ev.beat+1.5 {
			return
		}
		u := clamp01((beat - (f.ev.beat + 1.5)) / 1)
		x := lerp(pickFloat(f.isLeft, -8, 8), pickFloat(f.isLeft, 8, -8), u)
		rot := lerp(0, pickFloat(f.isLeft, -720, 720), u)
		f.setPos(x, -2.96)
		f.inst.SetRot(f.sprite, deg(rot))
	}
}

func (f *activeFruit) rollLemon(m *Module, beat float64) {
	idx, u := f.path.segmentAt(beat, f.ev.beat)
	if idx < 0 || idx+1 >= len(f.path.points) {
		return
	}
	cur := f.path.points[idx]
	next := f.path.points[idx+1]
	eased := quadEase(u, 2)
	startY, endY, yEase := 0.0, 0.0, 2
	switch {
	case idx == 0:
		startY, endY, yEase = -0.24, 0, 0
	case idx == len(f.path.points)-2:
		startY, endY, yEase = 0, -0.24, 1
	case u < 0.5:
		startY, endY, yEase = 0, -0.45, 2
	default:
		startY, endY, yEase = -0.45, 0, 2
	}
	x := lerp(cur.pos[0], next.pos[0], eased)
	y := -2.96 + lerp(startY, endY, quadEase(u, yEase))
	rot := lerp(cur.value("Rotation"), next.value("Rotation"), eased)
	f.setPos(x, y)
	f.inst.SetRot(f.sprite, deg(rot))
}

func (f *activeFruit) barely(m *Module, dt, beat float64) {
	if f.ev.fruit != fruitApple {
		endRot := pickFloat(f.missDir < 0, 360, -360)
		f.hitRotT += dt
		spb := m.ctx.SecPerBeat(beat)
		f.inst.SetRot(f.sprite, deg(lerp(0, endRot, clamp01(f.hitRotT/(2*spb)))))
	}
	p, idx := f.path.at(math.Max(f.hitBeat, beat), f.hitBeat)
	f.setPos(p[0], p[1])
	f.alpha = math.Max(0, f.alpha-dt*m.ctx.BPMAt(beat)/120)
	f.inst.SetColor(f.sprite, [4]float64{1, 1, 1, f.alpha})
	if idx >= 0 && idx < len(f.path.points) {
		v := f.path.points[idx].value("Direction")
		if v == 0 {
			v = f.path.points[idx].value("value")
		}
		if v != 0 {
			if f.isLeft {
				f.missDir = math.Copysign(1, v)
			} else {
				f.missDir = -math.Copysign(1, v)
			}
		}
	}
}

func (f *activeFruit) hitUpdate(m *Module, dt, beat float64) {
	switch f.ev.fruit {
	case fruitApple:
		if beat > f.ev.beat+3 {
			f.dropFromPathPoint(m, 1, dt, beat)
			return
		}
		p, _ := f.path.at(math.Max(f.hitBeat, beat), f.hitBeat)
		f.setPos(p[0], p[1])
	case fruitLemon:
		if !f.fading() {
			f.hitRotT += dt
			spb := m.ctx.SecPerBeat(beat)
			f.inst.SetRot(f.sprite, deg(lerp(pickFloat(f.isLeft, -90, 90), pickFloat(f.isLeft, -360, 360), clamp01(f.hitRotT/spb))))
		}
		if beat > f.ev.beat+5 {
			f.dropFromPathPoint(m, 1, dt, beat)
			return
		}
		p, _ := f.path.at(math.Max(f.hitBeat, beat), f.hitBeat)
		f.setPos(p[0], p[1])
	case fruitMelon:
		if beat > f.ev.beat+3.5 {
			f.dropFromPathPoint(m, 2, dt, beat)
			return
		}
		p, _ := f.path.at(math.Max(f.hitBeat, beat), f.hitBeat)
		f.setPos(p[0], p[1])
	}
}

func (f *activeFruit) dropFromPathPoint(m *Module, idx int, dt, beat float64) {
	if idx >= len(f.path.points) {
		idx = len(f.path.points) - 1
	}
	if idx < 0 {
		return
	}
	p := f.path.points[idx]
	dur := p.dur * m.ctx.SecPerBeat(beat)
	if dur <= 0 {
		dur = m.ctx.SecPerBeat(beat)
	}
	u := f.fadeT / dur
	f.fadeT += dt
	f.setPos(p.pos[0], p.pos[1]+lerp(0, -7, u*u))
	f.inst.SetRot(f.sprite, 0)
	f.alpha = math.Max(0, f.alpha-dt*m.ctx.BPMAt(beat)/80)
	f.inst.SetColor(f.sprite, [4]float64{1, 1, 1, f.alpha})
}

func (f *activeFruit) hitApple(m *Module, state float64) {
	f.hitBeat = f.ev.beat + 2
	if isNG(state) {
		f.ng(m, state, f.hitBeat)
		return
	}
	m.hitFruitAnimation(f.hitBeat)
	f.queueBasket(m, f.ev.beat+3)
	f.path = m.paths[pick(f.isLeft, "ToRightBasket", "ToLeftBasket")]
	f.inst.SetOrder(f.sprite, 7)
	f.state = 2
	f.doExpression(m, expressionName(f.ev.successExpression), f.ev.beat+4)
}

func (f *activeFruit) hitLemon(m *Module, state float64) {
	f.hitBeat = f.ev.beat + 4
	if isNG(state) {
		f.ng(m, state, f.hitBeat)
		return
	}
	m.hitFruitAnimation(f.hitBeat)
	f.queueBasket(m, f.ev.beat+5)
	f.path = m.paths[pick(f.isLeft, "ToRightBasket", "ToLeftBasket")]
	f.inst.SetOrder(f.sprite, 7)
	f.state = 2
	f.doExpression(m, expressionName(f.ev.successExpression), f.ev.beat+6)
}

func (f *activeFruit) hitMelon(m *Module, state float64) {
	f.hitBeat = f.ev.beat + 2
	if isNG(state) {
		f.ng(m, state, f.hitBeat)
		return
	}
	m.hitFruitAnimation(f.hitBeat)
	melonBasket := "melonBasket"
	switch f.ev.bothSidesRole {
	case 0:
		melonBasket = "melonBasket" + pick(f.isLeft, "L", "R")
	case 1:
		melonBasket = "melonBasketCenter"
	}
	m.ctx.SoundAt(f.ev.beat+2.5, "goalHit"+pick(f.isLeft, "R", "L"), 1)
	m.ctx.SoundAt(f.ev.beat+3.5, "basket"+pick(f.isLeft, "L", "R"), 1)
	m.ctx.SoundAt(f.ev.beat+3.5, "melonImpact"+pick(f.isLeft, "L", "R"), 1)
	m.ctx.SoundAtOff(f.ev.beat+4, melonBasket, 1, 0.074)
	m.ctx.At(f.ev.beat+2.5, func() {
		hoop := pick(f.isLeft, m.hoopR, m.hoopL)
		m.ctx.Scene.Play(hoop, "Animations/hoop/hoopShake", f.ev.beat+2.5, 1)
	})
	f.path = m.paths[pick(f.isLeft, "ToRightBasketMelon", "ToLeftBasketMelon")]
	f.inst.SetOrder(f.sprite, 7)
	f.state = 2
	f.doExpression(m, expressionName(f.ev.successExpression), f.ev.beat+4)
}

func (f *activeFruit) ng(m *Module, state, beat float64) {
	f.missed = true
	m.missFruitAnimation(beat)
	if state >= 1 {
		f.path = m.paths[pick(f.isLeft, "ToRightBasketMiss", "ToLeftBasketMiss")]
		f.missDir = 1
	} else {
		f.path = m.paths[pick(f.isLeft, "ToLeftMiss", "ToRightMiss")]
		f.missDir = -1
	}
	f.inst.SetOrder(f.sprite, 7)
	f.state = 1
	f.doExpression(m, expressionName(f.ev.missExpression), expressionBeatFor(f.ev.fruit, f.ev.beat))
}

func (f *activeFruit) missWindow(m *Module) {
	f.doExpression(m, expressionName(f.ev.missExpression), expressionBeatFor(f.ev.fruit, f.ev.beat))
}

func (f *activeFruit) doExpression(m *Module, name string, atBeat float64) {
	if f.ev.bothSidesRole == 2 && f.ev.fruit != fruitMelon {
		return
	}
	if f.ev.expressionDuration == 0 {
		return
	}
	m.ctx.At(atBeat, func() { m.courtneyExpression(name, atBeat+f.ev.expressionDuration) })
}

func (f *activeFruit) queueBasket(m *Module, beat float64) {
	if f.isLeft {
		m.queueBasketSound(beat, 1)
	} else {
		m.queueBasketSound(beat, 2)
	}
}

func (f *activeFruit) setPos(x, y float64) {
	f.inst.SetPos(f.sprite, x, y)
}

func (f *activeFruit) queue(sc *kart.SceneInst, beat float64) {
	if f.dead || f.inst == nil {
		return
	}
	f.inst.Queue(sc, beat, kart.Identity(), 0)
}

func expressionBeatFor(fruit int, start float64) float64 {
	switch fruit {
	case fruitLemon:
		return start + 6
	default:
		return start + 4
	}
}

func isNG(state float64) bool { return state >= 1 || state <= -1 }
