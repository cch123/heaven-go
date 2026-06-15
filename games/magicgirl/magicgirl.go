// Package magicgirl ports Magic Girl (magicGirl).
//
// Unity logic reference:
// Assets/Scripts/Games/MagicGirl/MagicGirl.cs
// Assets/Scripts/Games/MagicGirl/Monster.cs
package magicgirl

import (
	"math"
	"sort"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"

	"hsdemo/engine"
	"hsdemo/kart"
	"hsdemo/kmdata"
	"hsdemo/riq"
)

const (
	actionAlt = 3

	makoLayerOutfit = "magicGirl:mako:outfit"
	makoLayerFlash  = "magicGirl:mako:flash"
	trCompLayerType = "magicGirl:transf:type"

	starSprite    = "backgroundstars"
	sparkleSprite = "MG_Spritesheet3_3"
	ringSprite    = "MG_Spritesheet3_4"
)

var (
	defaultTop    = [4]float64{1, 122.0 / 255.0, 228.0 / 255.0, 1}
	defaultBottom = [4]float64{1, 163.0 / 255.0, 237.0 / 255.0, 1}
	defaultStars  = [4]float64{1, 215.0 / 255.0, 1, 1}
)

type Module struct {
	ctx  *engine.Ctx
	proj kart.Aff

	makoObject string
	mako       string
	makoFace   string
	hand       string
	trComp     string

	monsters [4]*monster

	spawns    []spawnCall
	intervals []intervalEvent
	passes    []passEvent
	bgEvents  []bgEvent
	scrolls   []scrollEvent
	phaseOps  []phaseOp

	goBop        bool
	canBop       bool
	holding      bool
	transform    int
	jumpStart    float64
	lastPulse    int
	scrollX      float64
	scrollY      float64
	scrollMulX   float64
	scrollMulY   float64
	lastUpdateT  float64
	lastUpdateOK bool

	bgTop    colorEase
	bgBottom colorEase
	starTint colorEase

	bursts []burst
}

type monster struct {
	path       string
	effectPath string
	location   int
	normal     [2]float64
	curve      kmdata.Curve

	relativeBeat float64
	hasSpawned   bool
	isFleeing    bool
	fleeBeat     float64
}

type spawnCall struct {
	beat     float64
	location int
}

type intervalEvent struct {
	beat, length float64
	autoPass     bool
}

type passEvent struct {
	beat, length float64
}

type bgEvent struct {
	beat, length     float64
	top0, top1       [4]float64
	bottom0, bottom1 [4]float64
	stars0, stars1   [4]float64
	ease             int
}

type scrollEvent struct {
	beat float64
	x, y float64
}

type phaseOp struct {
	beat  float64
	set   bool
	phase int
}

type colorEase struct {
	beat, length float64
	from, to     [4]float64
	ease         int
}

type burst struct {
	beat float64
	x, y float64
	kind int
}

func New() engine.Module {
	return &Module{
		goBop:      true,
		canBop:     true,
		transform:  2,
		jumpStart:  math.Inf(-1),
		lastPulse:  math.MinInt,
		scrollMulX: 0.5,
		scrollMulY: 0.2,
		bgTop:      colorEase{from: defaultTop, to: defaultTop},
		bgBottom:   colorEase{from: defaultBottom, to: defaultBottom},
		starTint:   colorEase{from: defaultStars, to: defaultStars},
	}
}

func (m *Module) ID() string { return "magicGirl" }

func (m *Module) Load(ctx *engine.Ctx) error {
	m.ctx = ctx
	if err := ctx.LoadAssets("magicGirl"); err != nil {
		return err
	}
	m.proj = kart.Translate(engine.ScreenW/2, engine.ScreenH/2).Mul(kart.Scale(54, -54))
	m.makoObject = ctx.Role("MakoObject")
	m.mako = ctx.Role("Mako")
	m.makoFace = ctx.Role("MakoFace")
	m.hand = ctx.Role("MonsterHands")
	m.trComp = ctx.Role("TransfComponent")
	for i := 0; i < 4; i++ {
		c := ctx.Assets.Extra.Components["monster"+itoa(i)]
		m.monsters[i] = &monster{
			path:       c.Path,
			effectPath: c.Refs["hitEffect"],
			location:   int(num(c.Nums, "location", float64(i))),
			normal: [2]float64{
				num(c.Nums, "normalLocation.x", 0),
				num(c.Nums, "normalLocation.y", 0),
			},
			curve: ctx.Assets.Extra.Curves["monster"+itoa(i)+".fleeCurve"],
		}
	}
	m.initScene(0)
	return nil
}

func (m *Module) initScene(beat float64) {
	sec := m.ctx.SecPerBeat(math.Max(beat, 0))
	for _, p := range []string{m.mako, m.makoFace, m.hand, m.trComp} {
		m.ctx.Scene.PlayDefaultState(p, beat, sec)
	}
	for _, mo := range m.monsters {
		mo.hasSpawned = false
		mo.isFleeing = false
		m.ctx.Scene.PlayDefaultState(mo.path, beat, sec)
		m.ctx.Scene.SetPosOver(mo.path, mo.normal[0], mo.normal[1])
	}
	m.holding = false
	m.jumpStart = math.Inf(-1)
	m.setOutfit(beat, m.transform)
	m.ctx.Scene.SetPosOver(m.makoObject, 0, 0.7)
}

func (m *Module) OnEvent(e *riq.Entity) {
	b := e.Beat
	switch e.Datamodel {
	case "magicGirl/bop":
		should, auto := boolParamDefault(e, "shouldBop", true), boolParam(e, "auto")
		m.ctx.At(b, func() { m.goBop = auto })
		if should {
			for i := 0; float64(i) < e.Length; i++ {
				bb := b + float64(i)
				m.ctx.At(bb, func() { m.bop(bb) })
			}
		}
	case "magicGirl/start interval":
		m.intervals = append(m.intervals, intervalEvent{
			beat: b, length: e.Length, autoPass: boolParamDefault(e, "autoPass", true),
		})
	case "magicGirl/spawn":
		m.spawns = append(m.spawns, spawnCall{beat: b, location: int(e.Float("spawnLocation", 0))})
	case "magicGirl/pass turn":
		m.passes = append(m.passes, passEvent{beat: b, length: e.Length})
		m.ctx.SoundAt(b, "pass_turn", 1)
	case "magicGirl/monster hand":
		m.ctx.At(b, func() { m.monsterHand(b) })
		m.ctx.SoundAt(b, "hand1", 1)
		m.ctx.SoundAt(b+1, "hand2", 1)
	case "magicGirl/transfcomp":
		typ, loc := int(e.Float("type", 0)), int(e.Float("location", 0))
		length := e.Length
		m.ctx.At(b, func() { m.transfComp(b, length, typ, loc) })
	case "magicGirl/progress":
		m.phaseOps = append(m.phaseOps, phaseOp{beat: b + 1})
		m.ctx.At(b, func() { m.progressTransform(b) })
	case "magicGirl/set outfit":
		phase := int(e.Float("outfit", 2))
		m.phaseOps = append(m.phaseOps, phaseOp{beat: b, set: true, phase: phase})
		m.ctx.At(b, func() { m.setOutfit(b, phase) })
	case "magicGirl/changeBG":
		m.bgEvents = append(m.bgEvents, bgEvent{
			beat: b, length: e.Length,
			top0:    colorParam(e, "startTop", defaultTop),
			top1:    colorParam(e, "endTop", defaultTop),
			bottom0: colorParam(e, "startBottom", defaultBottom),
			bottom1: colorParam(e, "endBottom", defaultBottom),
			stars0:  colorParam(e, "startDots", defaultStars),
			stars1:  colorParam(e, "endDots", defaultStars),
			ease:    int(e.Float("ease", 0)),
		})
	case "magicGirl/scroll":
		ev := scrollEvent{beat: b, x: e.Float("x", 0.5), y: e.Float("y", 0.2)}
		m.scrolls = append(m.scrolls, ev)
		m.ctx.At(b, func() {
			m.scrollMulX = ev.x
			m.scrollMulY = ev.y
		})
	}
}

func (m *Module) Ready() {
	sort.SliceStable(m.spawns, func(i, j int) bool { return m.spawns[i].beat < m.spawns[j].beat })
	sort.SliceStable(m.intervals, func(i, j int) bool { return m.intervals[i].beat < m.intervals[j].beat })
	sort.SliceStable(m.passes, func(i, j int) bool { return m.passes[i].beat < m.passes[j].beat })
	sort.SliceStable(m.bgEvents, func(i, j int) bool { return m.bgEvents[i].beat < m.bgEvents[j].beat })
	sort.SliceStable(m.scrolls, func(i, j int) bool { return m.scrolls[i].beat < m.scrolls[j].beat })
	sort.SliceStable(m.phaseOps, func(i, j int) bool { return m.phaseOps[i].beat < m.phaseOps[j].beat })
	for _, iv := range m.intervals {
		iv := iv
		m.ctx.At(iv.beat, func() { m.startInterval(iv.beat, iv.length, iv.autoPass, iv.beat) })
	}
	for _, p := range m.passes {
		p := p
		m.ctx.At(p.beat, func() { m.passTurn(p.beat, p.length) })
	}
}

func (m *Module) OnSwitch(beat float64) {
	m.lastPulse = int(math.Floor(beat)) - 1
	m.transform = m.phaseAt(beat)
	m.applyLastScroll(beat)
	m.initScene(beat)
	m.checkMonsters(beat)
}

func (m *Module) Whiff(beat float64) { m.WhiffAction(beat, 0) }

func (m *Module) WhiffAction(beat float64, action int) {
	if action == actionAlt {
		if !m.isJumping(beat) && m.transform != 0 && !m.makoIsHurt(beat) {
			m.playMako("Hold", beat)
			m.playFace("FaceBarely", beat)
			m.ctx.PlayCommon("miss")
			m.holding = true
			m.ctx.ScoreMiss()
		}
		return
	}
	if !m.isJumping(beat) && !m.holding && m.transform != 0 && !m.makoIsHurt(beat) {
		state := "U_Left_Move"
		if int(math.Floor(beat*2))%2 == 0 {
			state = "U_Right_Move"
		}
		m.playMako(state, beat)
		m.playFace("FaceBarely", beat)
		m.ctx.PlayCommon("miss")
		m.ctx.ScoreMiss()
	}
}

func (m *Module) Update(t, beat float64) {
	m.updateAutoBop(beat)
	m.updateAltReleaseWhiff(beat)
	m.updateScroll(t)
	m.updateJump(beat)
	m.updateMonsters(beat)
	m.applyBackground(beat)
	m.ctx.SampleScene(beat)
}

func (m *Module) Draw(screen *ebiten.Image, _ float64, beat float64) {
	m.queueBackgroundStars(beat)
	m.queueBursts(beat)
	m.ctx.Scene.Draw(screen, m.proj)
}

func (m *Module) startInterval(beat, length float64, autoPass bool, startBeat float64) {
	if m.transform == 0 {
		return
	}
	calls := m.callsBetween(beat, beat+length)
	if len(calls) == 0 {
		return
	}
	for _, call := range calls {
		if call.beat < startBeat {
			continue
		}
		call := call
		m.ctx.At(call.beat, func() { m.spawnMonster(call.beat, call.beat-beat, call.location, true) })
	}
	if autoPass {
		passBeat := beat + length
		m.ctx.At(passBeat, func() { m.passTurn(passBeat, 1) })
		m.ctx.SoundAt(passBeat, "pass_turn", 1)
	}
}

func (m *Module) callsBetween(start, end float64) []spawnCall {
	var out []spawnCall
	for _, call := range m.spawns {
		if call.beat >= start && call.beat <= end {
			out = append(out, call)
		}
	}
	return out
}

func (m *Module) spawnMonster(beat, relativeBeat float64, location int, sound bool) {
	for i := 0; i < 4; i++ {
		idx := (location + i) % 4
		mo := m.monsters[idx]
		if mo == nil || mo.hasSpawned {
			continue
		}
		if sound {
			m.ctx.Sound("spawn_monster")
		}
		mo.relativeBeat = relativeBeat
		mo.hasSpawned = true
		mo.isFleeing = false
		m.ctx.Scene.SetPosOver(mo.path, mo.normal[0], mo.normal[1])
		m.ctx.Scene.PlayState(mo.path, "MonsterAppear", beat, 0.5)
		return
	}
}

func (m *Module) passTurn(beat, length float64) {
	if m.transform == 0 {
		return
	}
	m.playMako("Prepare", beat)
	for _, mo := range m.monsters {
		if mo == nil || !mo.hasSpawned {
			continue
		}
		target := beat + mo.relativeBeat + length
		mo.fleeBeat = target
		m.scheduleMonsterInput(mo, target)
	}
}

func (m *Module) scheduleMonsterInput(mo *monster, target float64) {
	m.ctx.ScheduleInputAny(target, func(state float64, _ engine.Judgment) {
		m.monsterHit(mo, target, state)
	}, func() { m.monsterMiss(mo, target) })
}

func (m *Module) monsterHit(mo *monster, beat, state float64) {
	mo.hasSpawned = false
	mo.isFleeing = true
	m.ctx.Scene.PlayState(mo.path, "MonsterScared", beat, 0.5)
	switch mo.location {
	case 0:
		m.playMako("U_Left_Move", beat)
	case 1:
		m.playMako("U_Right_Move", beat)
	case 2:
		m.playMako("D_Left_Move", beat)
	default:
		m.playMako("D_Right_Move", beat)
	}
	m.ctx.Sound("enemy")
	if math.Abs(state) >= 1 {
		m.ctx.SoundVol("common_nearMiss", 2)
		m.playFace("FaceBarely", beat)
		return
	}
	m.ctx.Sound("hit")
	m.playFace("FaceWink", beat)
	m.addBurst(beat, mo.effectPath, 1)
}

func (m *Module) monsterMiss(mo *monster, beat float64) {
	if mo.location > 1 {
		m.ctx.Scene.PlayState(mo.path, "MonsterAttackD", beat, 0.5)
	} else {
		m.ctx.Scene.PlayState(mo.path, "MonsterAttackU", beat, 0.5)
	}
	m.playFace("FaceMiss", beat)
	m.ctx.Sound("doingoing")
	if mo.location == 0 || mo.location == 2 {
		m.playMako("Hurt_L", beat)
	} else {
		m.playMako("Hurt_R", beat)
	}
	mo.hasSpawned = false
}

func (m *Module) monsterHand(beat float64) {
	if m.transform == 0 {
		return
	}
	m.ctx.Scene.PlayState(m.hand, "Appear", beat, 0.5)
	m.ctx.ScheduleInputAction(beat+2, actionAlt, func(_ float64, _ engine.Judgment) {
		m.holdHit(beat + 2)
	}, func() { m.holdMiss(beat + 2) })
	m.ctx.ScheduleInputActionRelease(beat+3, actionAlt, func(state float64, _ engine.Judgment) {
		m.jumpHit(beat+3, state)
	}, func() { m.jumpMiss(beat + 3) })
}

func (m *Module) holdHit(beat float64) {
	m.playMako("Hold", beat)
	m.playFace("FaceIdle", beat)
	m.ctx.Sound("hold")
	m.holding = true
}

func (m *Module) holdMiss(beat float64) {
	m.playMako("Hurt_L", beat)
	m.playFace("FaceBarely", beat)
	m.ctx.Sound("doingoing")
}

func (m *Module) jumpHit(beat, state float64) {
	if math.Abs(state) < 1 {
		m.addBurst(beat, m.ctx.Role("jumpEffect"), 0)
	}
	m.jump(beat)
	m.playFace("FaceSmile", beat)
	m.ctx.Sound("hit")
	m.ctx.Scene.PlayState(m.hand, "Hide", beat, 0.5)
}

func (m *Module) jumpMiss(beat float64) {
	m.playMako("Hurt_R", beat)
	m.playFace("FaceMiss", beat)
	m.ctx.Sound("doingoing")
	m.ctx.Scene.PlayState(m.hand, "Attack", beat, 0.5)
}

func (m *Module) jump(beat float64) {
	m.holding = false
	m.jumpStart = beat
	m.playMako("Jump", beat)
}

func (m *Module) progressTransform(beat float64) {
	if m.transform >= 5 {
		return
	}
	m.ctx.Scene.PlayStateLayer(makoLayerFlash, m.mako, "FullFlash", beat, 0.5)
	next := m.transform + 1
	m.ctx.At(beat+1, func() {
		m.setOutfit(beat+1, next)
		m.ctx.Scene.PlayStateLayer(makoLayerFlash, m.mako, "IdleFlash", beat+1, 0.5)
		m.ctx.Sound("sparkle")
	})
}

func (m *Module) setOutfit(beat float64, outfit int) {
	if outfit < 0 {
		outfit = 0
	}
	if outfit > 5 {
		outfit = 5
	}
	m.transform = outfit
	states := []string{"Uniform", "PhaseA", "PhaseB", "PhaseC", "PhaseD", "Final"}
	m.ctx.Scene.PlayStateLayer(makoLayerOutfit, m.mako, states[outfit], beat, 0.5)
}

func (m *Module) transfComp(beat, length float64, typ, location int) {
	pos := [][2]float64{{-5.5, 2.5}, {5.5, 2.5}, {-5.5, -2.5}, {5.5, -2.5}}
	if location < 0 || location >= len(pos) {
		location = 0
	}
	m.ctx.Scene.SetPosOver(m.trComp, pos[location][0], pos[location][1])
	m.ctx.Scene.PlayState(m.trComp, "Appear", beat, 0.5)
	states := []string{"DressA", "DressB", "Hand", "Legs"}
	if typ < 0 || typ >= len(states) {
		typ = 0
	}
	m.ctx.Scene.PlayStateLayer(trCompLayerType, m.trComp, states[typ], beat, 0.5)
	m.ctx.At(beat+length, func() { m.ctx.Scene.PlayState(m.trComp, "Hide", beat+length, 0.5) })
}

func (m *Module) updateAutoBop(beat float64) {
	cur := int(math.Floor(beat))
	for m.lastPulse < cur {
		m.lastPulse++
		if m.goBop {
			m.bop(float64(m.lastPulse))
		}
	}
}

func (m *Module) bop(beat float64) {
	if !m.canBop || m.holding {
		return
	}
	state, playing := m.ctx.Scene.StateInfo(m.mako, beat)
	if playing && state != "Idle" {
		return
	}
	m.playMako("Bop", beat)
	m.playFace("FaceIdle", beat)
}

func (m *Module) updateAltReleaseWhiff(beat float64) {
	if !altReleasedNow() || m.ctx.ExpectingReleaseNow() || m.isJumping(beat) || m.transform == 0 || m.makoIsHurt(beat) {
		return
	}
	if m.holding {
		m.jump(beat)
		m.playFace("FaceBarely", beat)
		m.ctx.PlayCommon("miss")
		m.ctx.ScoreMiss()
	}
}

func (m *Module) updateScroll(t float64) {
	if !m.lastUpdateOK {
		m.lastUpdateT = t
		m.lastUpdateOK = true
		return
	}
	dt := t - m.lastUpdateT
	if dt < 0 || dt > 1 {
		dt = 0
	}
	m.lastUpdateT = t
	m.scrollX -= m.scrollMulX * dt
	m.scrollY += m.scrollMulY * dt
}

func (m *Module) updateJump(beat float64) {
	if beat >= m.jumpStart && beat < m.jumpStart+0.75 {
		u := (beat - m.jumpStart) / 0.75
		yMul := u*2 - 1
		yWeight := -(yMul * yMul) + 1
		m.ctx.Scene.SetPosOver(m.makoObject, 0, 2*yWeight+0.95)
		return
	}
	m.jumpStart = math.Inf(-1)
	m.ctx.Scene.SetPosOver(m.makoObject, 0, 0.7)
}

func (m *Module) updateMonsters(beat float64) {
	for _, mo := range m.monsters {
		if mo == nil || !mo.isFleeing {
			continue
		}
		u := (beat - mo.fleeBeat) / 0.75
		if u > 1 {
			mo.isFleeing = false
			m.ctx.Scene.SetPosOver(mo.path, mo.normal[0], mo.normal[1])
			continue
		}
		p := kart.EvalBezier(mo.curve, clamp01(u))
		m.ctx.Scene.SetPosOver(mo.path, p[0], p[1])
	}
}

func (m *Module) applyBackground(beat float64) {
	for _, ev := range m.bgEvents {
		if beat < ev.beat {
			break
		}
		m.bgTop = colorEase{beat: ev.beat, length: ev.length, from: ev.top0, to: ev.top1, ease: ev.ease}
		m.bgBottom = colorEase{beat: ev.beat, length: ev.length, from: ev.bottom0, to: ev.bottom1, ease: ev.ease}
		m.starTint = colorEase{beat: ev.beat, length: ev.length, from: ev.stars0, to: ev.stars1, ease: ev.ease}
	}
	top, bottom := m.bgTop.at(beat), m.bgBottom.at(beat)
	m.ctx.Scene.SetPaletteFor("BG", kart.Palette{
		Alpha:     top,
		Fill:      top,
		Outline:   bottom,
		Threshold: 0,
	})
}

func (m *Module) queueBackgroundStars(beat float64) {
	tint := m.starTint.at(beat)
	const (
		scale = 0.32
		w     = 24.2 * scale
		h     = 43.2 * scale
	)
	ox := math.Mod(m.scrollX*w, w)
	oy := math.Mod(m.scrollY*h, h)
	for ix := -1; ix <= 1; ix++ {
		for iy := -1; iy <= 1; iy++ {
			m.ctx.Scene.Queue(kart.ExtraSprite{
				Sprite: starSprite,
				World:  kart.Translate(ox+float64(ix)*w, oy+float64(iy)*h).Mul(kart.Scale(scale, scale)),
				Order:  -999,
				Tint:   tint,
			})
		}
	}
}

func (m *Module) queueBursts(beat float64) {
	alive := m.bursts[:0]
	for _, b := range m.bursts {
		u := (beat - b.beat) / 0.6
		if u < 0 {
			alive = append(alive, b)
			continue
		}
		if u > 1 {
			continue
		}
		alpha := 1 - u
		if b.kind == 0 {
			m.ctx.Scene.Queue(kart.ExtraSprite{
				Sprite: ringSprite,
				World:  kart.Translate(b.x, b.y).Mul(kart.Scale(0.12+u*0.28, 0.12+u*0.28)),
				Order:  80,
				Tint:   [4]float64{1, 1, 1, alpha},
			})
		}
		for i := 0; i < 6; i++ {
			ang := float64(i)*math.Pi/3 + float64(b.kind)*0.35
			r := 0.2 + u*0.9
			scale := 0.025 + 0.015*(1-u)
			m.ctx.Scene.Queue(kart.ExtraSprite{
				Sprite: sparkleSprite,
				World: kart.Translate(b.x+math.Cos(ang)*r, b.y+math.Sin(ang)*r).
					Mul(kart.Rotate(ang)).
					Mul(kart.Scale(scale, scale)),
				Order: 81 + i,
				Tint:  [4]float64{1, 1, 1, alpha},
			})
		}
		alive = append(alive, b)
	}
	m.bursts = alive
}

func (m *Module) addBurst(beat float64, path string, kind int) {
	x, y := nodePos(m.ctx.Assets, path)
	m.bursts = append(m.bursts, burst{beat: beat, x: x, y: y, kind: kind})
}

func (m *Module) checkMonsters(beat float64) {
	for i := len(m.intervals) - 1; i >= 0; i-- {
		iv := m.intervals[i]
		if iv.autoPass || iv.beat+iv.length > beat {
			continue
		}
		for _, p := range m.passes {
			if p.beat < beat && p.beat >= iv.beat+iv.length {
				return
			}
		}
		for _, call := range m.callsBetween(iv.beat, iv.beat+iv.length) {
			m.spawnMonster(beat, call.beat-iv.beat, call.location, false)
		}
		return
	}
}

func (m *Module) phaseAt(beat float64) int {
	phase := 2
	for _, op := range m.phaseOps {
		if op.beat > beat {
			break
		}
		if op.set {
			phase = op.phase
		} else if phase < 5 {
			phase++
		}
	}
	return phase
}

func (m *Module) applyLastScroll(beat float64) {
	m.scrollMulX, m.scrollMulY = 0.5, 0.2
	for _, ev := range m.scrolls {
		if ev.beat > beat {
			break
		}
		m.scrollMulX, m.scrollMulY = ev.x, ev.y
	}
	m.lastUpdateOK = false
}

func (m *Module) playMako(state string, beat float64) {
	m.ctx.Scene.PlayState(m.mako, state, beat, 0.5)
}

func (m *Module) playFace(state string, beat float64) {
	m.ctx.Scene.PlayState(m.makoFace, state, beat, 0.5)
}

func (m *Module) makoIsHurt(beat float64) bool {
	st, playing := m.ctx.Scene.StateInfo(m.mako, beat)
	return playing && (st == "Hurt_L" || st == "Hurt_R")
}

func (m *Module) isJumping(beat float64) bool {
	return beat >= m.jumpStart && beat < m.jumpStart+0.75
}

func (c colorEase) at(beat float64) [4]float64 {
	if c.length <= 0 {
		return c.to
	}
	u := (beat - c.beat) / c.length
	out := c.from
	for i := 0; i < 4; i++ {
		out[i] = engine.Ease(c.ease, c.from[i], c.to[i], u)
	}
	return out
}

func nodePos(as *kart.Assets, path string) (float64, float64) {
	if idx, ok := as.NodeIndex(path); ok {
		n := as.Rig.Nodes[idx]
		return n.Pos[0], n.Pos[1]
	}
	return 0, 0
}

func altReleasedNow() bool {
	return inpututil.IsKeyJustReleased(ebiten.KeyL) ||
		inpututil.IsKeyJustReleased(ebiten.KeyS) ||
		inpututil.IsKeyJustReleased(ebiten.KeyDown) ||
		inpututil.IsKeyJustReleased(ebiten.KeyX)
}

func boolParam(e *riq.Entity, key string) bool { return boolParamDefault(e, key, false) }

func boolParamDefault(e *riq.Entity, key string, def bool) bool {
	if _, ok := e.Data[key]; !ok {
		return def
	}
	return e.Float(key, 0) != 0
}

func colorParam(e *riq.Entity, key string, def [4]float64) [4]float64 {
	v, ok := e.Data[key]
	if !ok {
		return def
	}
	switch c := v.(type) {
	case []any:
		out := def
		for i := 0; i < len(c) && i < 4; i++ {
			if f, ok := c[i].(float64); ok {
				out[i] = f
			}
		}
		return out
	case map[string]any:
		out := def
		for i, k := range []string{"r", "g", "b", "a"} {
			if f, ok := c[k].(float64); ok {
				out[i] = f
			}
		}
		return out
	}
	return def
}

func num(vals map[string]float64, key string, def float64) float64 {
	if v, ok := vals[key]; ok {
		return v
	}
	return def
}

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

func itoa(v int) string {
	if v == 0 {
		return "0"
	}
	if v == 1 {
		return "1"
	}
	if v == 2 {
		return "2"
	}
	if v == 3 {
		return "3"
	}
	return "4"
}
