package lovelab

import (
	"math"

	"hsdemo/kart"
)

type flaskObj struct {
	inst      *kart.Instance
	path      flaskPath
	startBeat float64
	endBeat   float64
	girl      bool
}

type heartObj struct {
	inst          *kart.Instance
	kind          int
	heartBeat     float64
	length        float64
	intervalSpeed float64
	addPos        float64
	heartCount    int
	baseY         float64
	y             float64
	stop          bool
	stopY         float64
	dead          bool
	deadBeat      float64
	dropStartY    float64
	dropEndY      float64
	dropLength    float64
	waiting       bool
}

type particleObj struct {
	x, y   float64
	beat   float64
	life   float64
	angle  float64
	speed  float64
	sprite string
	tint   [4]float64
}

func (m *Module) spawnCustomFlask(arriveBeat float64, pathName string) {
	p, ok := m.paths[pathName]
	if !ok || m.flaskTemplate == nil {
		return
	}
	in := m.flaskTemplate.NewInstance()
	in.SetPalette("", flaskPalette(m.boyLiquid))
	m.flasks = append(m.flasks, &flaskObj{
		inst: in, path: p, startBeat: arriveBeat - 1, endBeat: arriveBeat,
	})
}

func (m *Module) spawnFlaskForGirl(arriveBeat float64, speed int) {
	if len(m.flaskArcsGirl) == 0 || m.flaskTemplate == nil {
		return
	}
	idx := speed
	if idx < 0 || idx >= len(m.flaskArcsGirl) {
		idx = 0
	}
	p, ok := m.paths[m.flaskArcsGirl[idx]]
	if !ok {
		return
	}
	in := m.flaskTemplate.NewInstance()
	in.SetPalette("", flaskPalette(m.boyLiquid))
	m.flasks = append(m.flasks, &flaskObj{
		inst: in, path: p, startBeat: arriveBeat - 1, endBeat: arriveBeat + p.duration(), girl: true,
	})
}

func (m *Module) spawnFlaskForWeird(releaseBeat float64) {
	p, ok := m.paths["WeirdFlaskIn"]
	if !ok || m.girlFlaskTemplate == nil {
		return
	}
	in := m.girlFlaskTemplate.NewInstance()
	in.SetPalette("", flaskPalette(m.girlLiquid))
	start := releaseBeat
	m.flasks = append(m.flasks, &flaskObj{
		inst: in, path: p, startBeat: start, endBeat: start + p.duration() + 0.05,
	})
	m.ctx.At(releaseBeat+2, func() {
		m.play(m.labAssistantArm, "MittenGrabStart", releaseBeat+2)
	})
	m.ctx.At(releaseBeat+3, func() {
		m.play(m.labAssistantArm, "MittenGrab", releaseBeat+3)
	})
	m.ctx.At(releaseBeat+3.5, func() {
		m.play(m.labAssistantArm, "MittenLetGo", releaseBeat+3.5)
	})
}

func (m *Module) spawnGirlMissFlask(nowBeat float64) {
	p, ok := m.paths["GirlFlaskMiss"]
	if !ok || m.girlFlaskTemplate == nil {
		return
	}
	in := m.girlFlaskTemplate.NewInstance()
	in.SetPalette("", flaskPalette(m.girlLiquid))
	m.flasks = append(m.flasks, &flaskObj{
		inst: in, path: p, startBeat: nowBeat, endBeat: nowBeat + 1,
	})
	m.ctx.At(nowBeat+1, func() { m.flaskBreak(1, nowBeat+1) })
}

func (m *Module) queueFlasks(beat float64) {
	alive := m.flasks[:0]
	for _, f := range m.flasks {
		if f == nil || f.inst == nil {
			continue
		}
		if beat > f.endBeat+0.05 {
			continue
		}
		x, y, rot, _ := f.path.eval(beat - f.startBeat)
		f.inst.Offset = [2]float64{x, y}
		f.inst.Rot = rot
		f.inst.Queue(m.ctx.Scene, beat, kart.Identity(), 0)
		alive = append(alive, f)
	}
	m.flasks = alive
}

func (m *Module) destroyFirstGirlFlask() {
	for i, f := range m.flasks {
		if f.girl {
			m.flasks = append(m.flasks[:i], m.flasks[i+1:]...)
			return
		}
	}
}

func (m *Module) createHeart(kind int, beat, length float64, heartCount int, addPos float64, intervalSpeed float64) *heartObj {
	var tmpl *kart.Template
	switch kind {
	case 0:
		tmpl = m.guyHeartTemplate
	case 1:
		tmpl = m.girlHeartTemplate
	default:
		tmpl = m.completeHeartTemplate
	}
	if tmpl == nil {
		return nil
	}
	in := tmpl.NewInstance()
	baseY := in.Offset[1]
	if kind != 2 {
		if heartCount == 0 {
			baseY += 2
		} else {
			baseY += 4
		}
	}
	h := &heartObj{
		inst: in, kind: kind, heartBeat: beat, length: math.Max(length, 0.001),
		intervalSpeed: intervalSpeed, addPos: addPos, heartCount: heartCount,
		baseY: baseY, y: baseY, waiting: kind == 2,
	}
	if h.addPos == 0 {
		h.addPos = 2.5
	}
	in.PlayDefaultState("Heart/HeartHolder", beat, m.ctx.SecPerBeat(math.Max(beat, 0)))
	return h
}

func (m *Module) heartUp(hearts []*heartObj) {
	if len(hearts) < 2 {
		return
	}
	const factor = 1.3
	max := len(hearts) - 1
	step := hearts[max].heartBeat - hearts[max-1].heartBeat
	if step <= 0 {
		return
	}
	for i := 0; i < max; i++ {
		hearts[i].addPos += step * factor
	}
}

func (m *Module) stopHearts(hearts []*heartObj, beat float64) {
	for _, h := range hearts {
		if h == nil || h.stop {
			continue
		}
		h.stopY = h.posAt(beat)
		h.stop = true
	}
}

func (h *heartObj) posAt(beat float64) float64 {
	if h == nil {
		return 0
	}
	if h.dead {
		return h.y
	}
	if h.kind == 2 && !h.waiting {
		u := clamp01((beat - h.heartBeat) / math.Max(h.dropLength, 0.001))
		return h.dropStartY + (h.dropEndY-h.dropStartY)*easeInQuad(u)
	}
	if h.stop {
		return h.stopY
	}
	u := clamp01((beat - h.heartBeat) / math.Max(h.length, 0.001))
	return h.baseY + h.addPos*easeOutBack(u)
}

func (m *Module) queueHearts(beat float64) {
	queue := func(list []*heartObj) []*heartObj {
		alive := list[:0]
		for _, h := range list {
			if h == nil || h.inst == nil {
				continue
			}
			if h.dead && beat > h.deadBeat+0.55 {
				continue
			}
			h.y = h.posAt(beat)
			if h.kind == 2 && !h.waiting && beat > h.heartBeat+h.dropLength {
				continue
			}
			h.inst.Offset[1] = h.y
			h.inst.Queue(m.ctx.Scene, beat, kart.Identity(), 0)
			alive = append(alive, h)
		}
		return alive
	}
	m.guyHearts = queue(m.guyHearts)
	m.girlHearts = queue(m.girlHearts)
	m.completeHearts = queue(m.completeHearts)
}

func (m *Module) markDeadHearts(hearts []*heartObj, beat float64) {
	for _, h := range hearts {
		if h == nil {
			continue
		}
		h.y = h.posAt(beat)
		h.dead = true
		h.deadBeat = beat
		m.spawnHeartBurst(h.inst.Offset[0], h.y, beat)
	}
}

func (m *Module) spawnHeartBurst(x, y, beat float64) {
	for i := 0; i < 8; i++ {
		a := float64(i) * math.Pi / 4
		m.particles = append(m.particles, particleObj{
			x: x, y: y, beat: beat, life: 0.5, angle: a, speed: 2.2 + 0.15*float64(i%3),
			sprite: "lovelabmain_89", tint: [4]float64{1, 0.34, 0.5, 1},
		})
	}
}

func (m *Module) queueParticles(beat float64) {
	alive := m.particles[:0]
	for _, p := range m.particles {
		u := (beat - p.beat) / math.Max(p.life, 0.001)
		if u > 1 {
			continue
		}
		dist := p.speed * u
		tint := p.tint
		tint[3] = 1 - u
		world := kart.TRS(
			p.x+math.Cos(p.angle)*dist,
			p.y+math.Sin(p.angle)*dist,
			p.angle+u*math.Pi,
			0.22*(1-u*0.35),
			0.22*(1-u*0.35),
		)
		m.ctx.Scene.Queue(kart.ExtraSprite{
			Sprite: p.sprite, World: world, Order: 10020, Tint: tint,
		})
		alive = append(alive, p)
	}
	m.particles = alive
}
