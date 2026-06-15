package lumbearjack

import (
	"image/color"
	"math"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"hsdemo/engine"
	"hsdemo/kart"
)

type cutObject struct {
	mod    *Module
	inst   *kart.Instance
	ev     objectEvt
	beat   float64
	length float64
	unit   float64
	right  bool
	dead   bool
	stage  int
	sprite string
}

type babyEffect struct {
	inst  *kart.Instance
	start float64
	index int
}

type bombEffect struct {
	inst  *kart.Instance
	start float64
	endX  float64
}

type missEffect struct {
	start, duration float64
	x, y            float64
	vx, vy          float64
	rot             float64
	sprite          string
}

func (m *Module) spawnObject(ev objectEvt, startUpBeat float64) {
	var tmpl *kart.Template
	switch ev.kind {
	case objSmall:
		tmpl = m.smallT
	case objBig:
		tmpl = m.bigT
	default:
		tmpl = m.hugeT
	}
	if tmpl == nil {
		return
	}
	o := &cutObject{mod: m, inst: tmpl.NewInstance(), ev: ev, beat: ev.beat, length: ev.length, unit: m.unit(ev), right: m.shouldBeRight(ev.beat, ev.cat)}
	o.initVisuals(startUpBeat)
	o.scheduleInputs(startUpBeat)
	m.activeObjects = append(m.activeObjects, o)
}

func (o *cutObject) initVisuals(startUpBeat float64) {
	switch o.ev.kind {
	case objSmall:
		for _, p := range []string{"log", "can", "bat", "broom", "barrel", "book"} {
			o.inst.SetActive(p, false)
		}
		p := []string{"log", "can", "bat", "broom", "barrel", "book"}[o.ev.small]
		o.sprite = p
		o.inst.SetActive(p, true)
	case objBig:
		o.inst.SetActive("log", false)
		o.inst.SetActive("dough", false)
		if o.ev.big == bigBall {
			o.sprite = "dough"
			o.inst.SetActive("dough", true)
		} else {
			o.sprite = "log"
			o.inst.SetActive("log", true)
		}
		if startUpBeat > o.beat+2*o.unit {
			o.setBigCutSprite()
		}
	case objHuge:
		for _, p := range []string{"log", "freezer", "peach"} {
			o.inst.SetActive(p, false)
		}
		o.sprite = []string{"log", "freezer", "peach"}[o.ev.huge]
		o.inst.SetActive(o.sprite, true)
		switch {
		case startUpBeat > o.beat+4*o.unit:
			o.setHugeCutSprite(3)
		case startUpBeat > o.beat+3*o.unit:
			o.setHugeCutSprite(2)
		case startUpBeat > o.beat+2*o.unit:
			o.setHugeCutSprite(1)
		}
	}
}

func (o *cutObject) scheduleInputs(startUpBeat float64) {
	canHit := func() bool { return !o.dead }
	switch o.ev.kind {
	case objSmall:
		target := o.beat + 2*o.unit
		if target >= startUpBeat {
			o.mod.ctx.ScheduleInputCond(target, canHit, func(state float64, _ engine.Judgment) { o.hitSmall(state) }, func() { o.miss() })
		}
	case objBig:
		hit := o.beat + 2*o.unit
		cut := o.beat + 3*o.unit
		if hit >= startUpBeat {
			o.mod.ctx.ScheduleInputCond(hit, canHit, func(state float64, _ engine.Judgment) { o.hitBig(state) }, func() { o.miss() })
		}
		if cut >= startUpBeat {
			o.mod.ctx.ScheduleInputCond(cut, canHit, func(state float64, _ engine.Judgment) { o.cutBig(state) }, func() { o.miss() })
		}
	case objHuge:
		for step := 1; step <= 3; step++ {
			target := o.beat + float64(step+1)*o.unit
			step := step
			if target >= startUpBeat {
				o.mod.ctx.ScheduleInputCond(target, canHit, func(state float64, _ engine.Judgment) { o.hitHuge(step, state) }, func() { o.miss() })
			}
		}
		cut := o.beat + 5*o.unit
		if cut >= startUpBeat {
			o.mod.ctx.ScheduleInputCond(cut, canHit, func(state float64, _ engine.Judgment) { o.cutHuge(state) }, func() { o.miss() })
		}
	}
}

func (o *cutObject) update(beat float64) {
	switch o.ev.kind {
	case objSmall:
		o.applyRotation(beat, o.beat+2*o.unit, o.unit, o.ev.small == smallBat)
	case objBig:
		rotBeat := o.beat + 2*o.unit
		if o.stage > 0 {
			rotBeat = o.beat + 3*o.unit
		}
		o.applyRotation(beat, rotBeat, o.unit, o.ev.big == bigBall)
	case objHuge:
		rotBeat := o.beat + 2*o.unit
		if o.stage > 0 {
			rotBeat = o.beat + float64(o.stage+2)*o.unit
		}
		o.applyRotation(beat, rotBeat, o.unit, false)
	}
}

func (o *cutObject) applyRotation(beat, target, length float64, single bool) {
	n := (beat - (target - length)) / (length * 2)
	n = clamp01(n)
	if !o.right {
		n = 1 - n
	}
	left, right := 22.0, -22.0
	if comp, ok := componentByPath(o.mod.ctx.Assets.Extra.Components, o.inst.T.RootPath); ok {
		if comp.Nums["_rotationLeft"] != 0 {
			left = comp.Nums["_rotationLeft"]
		}
		if comp.Nums["_rotationRight"] != 0 {
			right = comp.Nums["_rotationRight"]
		}
	}
	var deg float64
	if single {
		deg = easeInOutQuad(right, left, n)
	} else if n <= 0.5 {
		deg = math.Min(easeInOutQuad(right, -right, n), 0)
	} else {
		deg = math.Max(easeInOutQuad(-left, left, n), 0)
	}
	o.inst.SetRot(o.sprite, radians(deg))
}

func (o *cutObject) hitSmall(state float64) {
	if math.Abs(state) >= 1 {
		o.mod.swingWhiff(o.mod.ctx.Beat(), false)
		o.miss()
		return
	}
	o.mod.randomSound("cutVoiceA", "cutVoiceB")
	switch o.ev.small {
	case smallCan:
		o.mod.ctx.Sound("canCut")
	case smallBat:
		o.mod.ctx.Sound("batCut")
	case smallBroom:
		o.mod.ctx.Sound("broomCut")
	case smallBarrel:
		o.mod.ctx.Sound("barrelCut")
	case smallBook:
		o.mod.ctx.Sound("bookCut")
		o.mod.ctx.Sound("bookBoom")
	default:
		o.mod.ctx.Sound("smallLogCut")
	}
	target := o.beat + 2*o.unit
	o.mod.doSmallEffect(o.ev.small, o.ev.bomb, target)
	huh := false
	huhLeft := !o.right
	if o.ev.huh == huhOn || (o.ev.huh == huhObjectSpecific && o.ev.small != smallLog) {
		huh = true
		o.mod.ctx.SoundAt(target+1, "huh", 1)
	}
	o.mod.bearCut(target, huh, huhLeft, false)
	o.dead = true
}

func (o *cutObject) hitBig(state float64) {
	if math.Abs(state) >= 1 {
		o.mod.swingWhiff(o.mod.ctx.Beat(), false)
		o.miss()
		return
	}
	o.mod.randomSound("hitVoiceA", "hitVoiceB")
	o.mod.ctx.Sound("baseHit")
	if o.ev.big == bigBall {
		o.mod.ctx.Sound("bigBallHit")
	} else {
		o.mod.ctx.Sound("bigLogHit")
	}
	o.mod.doBigEffect(o.ev.big, true)
	o.mod.bearCutMid(o.mod.ctx.Beat(), false)
	o.stage = 1
	o.setBigCutSprite()
}

func (o *cutObject) cutBig(state float64) {
	if math.Abs(state) >= 1 {
		o.mod.swingWhiff(o.mod.ctx.Beat(), false)
		o.miss()
		return
	}
	o.mod.ctx.Sound("bigLogCutVoice")
	o.mod.doBigEffect(o.ev.big, false)
	if o.ev.big == bigBall {
		o.mod.ctx.SoundVol("bigBallCut", 2)
		o.mod.ctx.SoundVol("bigBallHit", 1.5)
	} else {
		o.mod.ctx.Sound("bigLogCut")
	}
	o.mod.bearCut(o.beat+3*o.unit, false, false, false)
	o.dead = true
}

func (o *cutObject) hitHuge(step int, state float64) {
	if math.Abs(state) >= 1 {
		o.mod.swingWhiff(o.mod.ctx.Beat(), false)
		o.miss()
		return
	}
	if o.ev.huge == hugePeach {
		o.mod.randomSound("peachVoiceA", "peachVoiceB")
	} else {
		o.mod.randomSound("hitVoiceA", "hitVoiceB")
	}
	o.mod.ctx.Sound("baseHit")
	o.mod.doHugeEffect(o.ev.huge, true)
	switch o.ev.huge {
	case hugeFreezer:
		o.mod.ctx.Sound("freezerHit" + itoa(step))
	case hugePeach:
		o.mod.ctx.Sound("peachHit" + itoa(step))
	default:
		o.mod.ctx.Sound("hugeLogHit" + itoa(step))
	}
	o.mod.bearCutMid(o.mod.ctx.Beat(), o.ev.huge == hugeFreezer)
	o.stage = step
	o.setHugeCutSprite(step)
}

func (o *cutObject) cutHuge(state float64) {
	if math.Abs(state) >= 1 {
		o.mod.swingWhiff(o.mod.ctx.Beat(), false)
		o.miss()
		return
	}
	if o.ev.huge == hugePeach {
		o.mod.ctx.Sound("peachCutVoice")
	} else {
		o.mod.ctx.Sound("hugeLogCutVoice")
	}
	o.mod.doHugeEffect(o.ev.huge, false)
	switch o.ev.huge {
	case hugeFreezer:
		o.mod.ctx.Sound("freezerCut")
	case hugePeach:
		o.mod.ctx.Sound("peachCut")
		if o.ev.baby {
			o.mod.activateBaby(o.beat+5*o.unit, o.unit)
		}
	default:
		o.mod.ctx.Sound("hugeLogCut")
	}
	o.mod.bearCut(o.beat+5*o.unit, false, false, o.ev.zoom)
	o.dead = true
}

func (o *cutObject) setBigCutSprite() {
	comp := o.mod.ctx.Assets.Extra.Components["bigObject"]
	if o.ev.big == bigBall {
		o.inst.SetSprite("dough/dough", comp.Sprites["_ballCutSprite"])
		o.inst.SetSprite("dough", comp.Sprites["_ballCutSprite"])
	} else {
		o.inst.SetSprite("log", comp.Sprites["_logCutSprite"])
	}
}

func (o *cutObject) setHugeCutSprite(step int) {
	comp := o.mod.ctx.Assets.Extra.Components["hugeObject"]
	idx := step - 1
	switch o.ev.huge {
	case hugeFreezer:
		if idx < len(comp.SpriteArrays["_freezerCutSprites"]) {
			o.inst.SetSprite("freezer", comp.SpriteArrays["_freezerCutSprites"][idx])
		}
	case hugePeach:
		if idx < len(comp.SpriteArrays["_peachCutSprites"]) {
			o.inst.SetSprite("peach", comp.SpriteArrays["_peachCutSprites"][idx])
		}
	default:
		if idx < len(comp.SpriteArrays["_logCutSprites"]) {
			o.inst.SetSprite("log", comp.SpriteArrays["_logCutSprites"][idx])
		}
	}
}

func (o *cutObject) miss() {
	if o.dead {
		return
	}
	o.mod.ctx.PlayCommon("miss")
	o.mod.activateMiss(o)
	o.dead = true
}

func (o *cutObject) queue(sc *kart.SceneInst, beat float64) {
	o.inst.Queue(sc, beat, kart.Identity(), 0)
}

func (m *Module) spawnPersistedObjects(beat float64) {
	for _, ev := range m.objects {
		if ev.beat < beat && ev.beat+ev.length > beat {
			if ev.sound {
				m.scheduleObjectSounds(ev, beat)
			}
			m.spawnObject(ev, beat)
		}
	}
}

func (m *Module) randomSound(a, b string) {
	if signedSeed(m.ctx.Beat(), 3) < 0.5 {
		m.ctx.Sound(a)
	} else {
		m.ctx.Sound(b)
	}
}

func (m *Module) doSmallEffect(t smallType, bomb bool, beat float64) {
	ref := ""
	switch t {
	case smallCan:
		ref = m.gameComp["_canCutParticle"]
	case smallBat:
		ref = m.gameComp["_batCutParticle"]
	case smallBroom:
		ref = m.gameComp["_broomCutParticle"]
	case smallBarrel:
		ref = m.gameComp["_barrelCutParticle"]
	case smallBook:
		ref = m.gameComp["_bookCutParticle"]
	default:
		ref = m.gameComp["_smallLogCutParticle"]
	}
	m.flashEffect(ref, beat)
	if t == smallBarrel && bomb {
		m.activateBomb(beat)
		m.ctx.SoundAt(beat, "bombCut", 1)
		m.ctx.SoundAtOff(beat+4, "bombBreak", 1, 0.2)
	}
}

func (m *Module) doBigEffect(t bigType, hit bool) {
	if t == bigBall {
		if !hit {
			m.flashEffect(m.gameComp["_bigBallCutParticle"], m.ctx.Beat())
		}
		return
	}
	if hit {
		m.flashEffect(m.gameComp["_bigLogHitParticle"], m.ctx.Beat())
	} else {
		m.flashEffect(m.gameComp["_bigLogCutParticle"], m.ctx.Beat())
	}
}

func (m *Module) doHugeEffect(t hugeType, hit bool) {
	switch t {
	case hugeFreezer:
		if hit {
			m.flashEffect(m.gameComp["_freezerChipParticle"], m.ctx.Beat())
		} else {
			m.flashEffect(m.gameComp["_freezerBreakParticle"], m.ctx.Beat())
		}
	case hugePeach:
		if hit {
			m.flashEffect(m.gameComp["_peachHitParticle"], m.ctx.Beat())
		} else {
			m.flashEffect(m.gameComp["_peachCutParticle"], m.ctx.Beat())
		}
	default:
		if hit {
			m.flashEffect(m.gameComp["_hugeLogHitParticle"], m.ctx.Beat())
		} else {
			m.flashEffect(m.gameComp["_hugeLogCutParticle"], m.ctx.Beat())
		}
	}
}

func (m *Module) flashEffect(path string, beat float64) {
	if path == "" {
		return
	}
	m.ctx.Scene.SetActive(path, true)
	m.ctx.At(beat+0.5, func() { m.ctx.Scene.SetActive(path, false) })
}

func (m *Module) activateBaby(beat, duration float64) {
	if m.babyT == nil {
		return
	}
	b := &babyEffect{inst: m.babyT.NewInstance(), start: beat, index: m.babyIndex}
	m.babyIndex++
	comp := m.ctx.Assets.Extra.Components["baby"]
	if sp := comp.Sprites["_flySprite"]; sp != "" {
		b.inst.SetSprite("", sp)
	}
	b.inst.SetOrder("", 90+b.index)
	m.babies = append(m.babies, b)
	m.ctx.At(beat+duration, func() {
		if sp := comp.Sprites["_standSprite"]; sp != "" {
			b.inst.SetSprite("", sp)
		}
	})
}

func (m *Module) persistBabies(beat float64) {
	for _, ev := range m.objects {
		if ev.kind != objHuge || ev.huge != hugePeach || !ev.baby {
			continue
		}
		cut := ev.beat + ev.length - m.unit(ev)
		if cut < beat && ev.pBaby {
			m.activateBaby(cut, m.unit(ev))
		}
	}
}

func (b *babyEffect) queue(sc *kart.SceneInst, beat float64) {
	t := clamp01((beat - b.start) / 1.5)
	x := 1.2 + 1.5*t
	y := -2.1 + float64(b.index)*0.35 + 3.2*math.Sin(t*math.Pi/2)
	b.inst.Offset = [2]float64{x, y}
	b.inst.Rot = radians(360 * t)
	b.inst.Queue(sc, beat, kart.Identity(), 0)
}

func (m *Module) activateBomb(beat float64) {
	if m.bombT == nil {
		return
	}
	end := -3 + 6*signedSeed(beat, 11)
	m.bombs = append(m.bombs, &bombEffect{inst: m.bombT.NewInstance(), start: beat, endX: end})
}

func (b *bombEffect) queue(sc *kart.SceneInst, beat float64) {
	t := clamp01((beat - b.start) / 4)
	x := b.endX * t
	y := -2 + 3.5*math.Sin(math.Pi*t) - 2*t
	b.inst.Offset = [2]float64{x, y}
	b.inst.Rot = radians(1080 * t)
	b.inst.Queue(sc, beat, kart.Identity(), 0)
}

func (m *Module) activateMiss(o *cutObject) {
	m.missFx = append(m.missFx, &missEffect{
		start: m.ctx.Beat(), duration: 1,
		x: 0, y: -0.5,
		vx: -2.5, vy: -1.4,
		rot: 135, sprite: o.currentSpriteName(),
	})
}

func (o *cutObject) currentSpriteName() string {
	root := o.inst.T.RootPath
	path := root
	if o.sprite != "" {
		path += "/" + o.sprite
	}
	for _, n := range o.mod.ctx.Assets.Rig.Nodes {
		if n.Path == path {
			return n.Sprite
		}
	}
	return ""
}

func (fx *missEffect) draw(screen *ebiten.Image, proj kart.Aff, beat float64) {
	if fx.sprite == "" {
		return
	}
	u := clamp01((beat - fx.start) / fx.duration)
	x := fx.x + fx.vx*u
	y := fx.y + fx.vy*u + 3*u*(1-u)
	px, py := proj.Apply(x, y)
	col := color.RGBA{255, 255, 255, uint8(255 * (1 - u))}
	vector.DrawFilledCircle(screen, float32(px), float32(py), 18, col, false)
}

func itoa(v int) string {
	switch v {
	case 1:
		return "1"
	case 2:
		return "2"
	case 3:
		return "3"
	default:
		return "0"
	}
}
