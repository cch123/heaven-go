// Package forklifter ports Fork Lifter's flicked food, fork stack, swallow
// animation events, recolors, background gradient controls, and cue sounds.
package forklifter

import (
	"image/color"
	"math"
	"math/rand"
	"sort"

	"github.com/hajimehoshi/ebiten/v2"

	"hsdemo/engine"
	"hsdemo/kart"
	"hsdemo/riq"
)

const (
	flickPea = iota
	flickTopBun
	flickBurger
	flickBottomBun
)

const (
	gradGame = iota
	gradRemix
	gradClassic
)

const (
	eatDefault = iota
	eatNormal
	eatBurger
)

var (
	stageFill        = color.NRGBA{R: 0xf1, G: 0xf1, B: 0xf1, A: 0xff}
	whiteColor       = [4]float64{1, 1, 1, 1}
	defaultGradTop   = [4]float64{224.0 / 255, 224.0 / 255, 224.0 / 255, 1}
	defaultLines     = [4]float64{243.0 / 255, 243.0 / 255, 243.0 / 255, 1}
	defaultSkin      = [4]float64{1, 0xde / 255.0, 0x94 / 255.0, 1}
	defaultShade     = [4]float64{0xc1 / 255.0, 0x9c / 255.0, 0x49 / 255.0, 1}
	defaultForkColor = [4]float64{0xc6 / 255.0, 0xc6 / 255.0, 0xc6 / 255.0, 1}
)

type flickEvt struct {
	beat float64
	typ  int
	idx  int
}

type prepareEvt struct {
	beat float64
	mute bool
}

type gulpEvt struct {
	beat float64
	sfx  int
}

type colorEvt struct {
	beat, length float64
	start, end   [4]float64
	ease         int
}

type gradEvt struct {
	beat, length float64
	typ          int
	viewCircle   bool
	lines        bool
	top0, top1   [4]float64
	bot0, bot1   [4]float64
	line0, line1 [4]float64
	ease         int
}

type handEvt struct {
	beat              float64
	skin, shade, fork [4]float64
}

type flyingFood struct {
	inst      *kart.Instance
	eventIdx  int
	startBeat float64
	typ       int
	dead      bool
}

type forkFood struct {
	typ int
}

type flashFX struct {
	sprite string
	born   float64
	dur    float64
	pos    [2]float64
	scale  [2]float64
	rot    float64
	order  int
	layer  int
	mapped bool
	mat    string
}

type Module struct {
	ctx  *engine.Ctx
	proj kart.Aff

	flicks  []flickEvt
	preps   []prepareEvt
	gulps   []gulpEvt
	sighs   []float64
	colors  []colorEvt
	grads   []gradEvt
	hands   []handEvt
	spawned map[int]bool

	objectT *kart.Template
	foods   []*flyingFood
	fx      []flashFX

	peaSprites     []string
	peaHitSprites  []string
	fastSprites    []string
	gradients      []string
	forkEffects    []string
	handPath       string
	playerPath     string
	objectPath     string
	peaPreviewPath string
	fastPath       string
	bgPath         string
	gradientFiller string
	mmLines        string
	viewerCircle   string
	viewerCircleBg string
	playerShadow   string
	handShadow     string
	forkPath       string
	earlyPath      string
	perfectPath    string
	latePath       string
	handMat        string

	hitFX     string
	hitFXG    string
	hitFXMiss string
	hitFX2    string

	currentFlickIndex int
	earlyCount        int
	perfectCount      int
	lateCount         int
	earlyStack        []forkFood
	perfectStack      []forkFood
	lateStack         []forkFood
	isEating          bool
	eatType           int
	topbun            bool
	middleburger      bool
	bottombun         bool
}

func New() engine.Module {
	return &Module{spawned: map[int]bool{}}
}

func (m *Module) ID() string { return "forkLifter" }

func (m *Module) Load(ctx *engine.Ctx) error {
	m.ctx = ctx
	if err := ctx.LoadAssets("forkLifter"); err != nil {
		return err
	}
	m.proj = kart.Translate(engine.ScreenW/2, engine.ScreenH/2).Mul(kart.Scale(54, -54))

	m.handPath = roleOr(ctx, "handAnim", "Hand")
	m.playerPath = "Player"
	m.objectPath = roleOr(ctx, "flickedObject", "Object")
	m.peaPreviewPath = roleOr(ctx, "peaPreview", "Hand/PeaVisual")
	m.bgPath = roleOr(ctx, "bg", "Background/SolidBG")
	m.gradientFiller = roleOr(ctx, "gradientFiller", "Background/Filler")
	m.mmLines = roleOr(ctx, "mmLines", "Background/megamixfloor")
	m.viewerCircle = roleOr(ctx, "viewerCircle", "Hand/ViewerCircle")
	m.viewerCircleBg = roleOr(ctx, "viewerCircleBg", "Hand/ViewerCircle/viewcirclebg")
	m.playerShadow = roleOr(ctx, "playerShadow", "Player/shadow")
	m.handShadow = roleOr(ctx, "handShadow", "Hand/HandVisual/hand_shadow1")
	m.forkPath = roleOr(ctx, "forkSR", "Player/fork")

	extra := ctx.Assets.Extra
	if extra.RefArrays != nil {
		m.gradients = append(m.gradients, extra.RefArrays["Gradients"]...)
		m.forkEffects = append(m.forkEffects, extra.RefArrays["forkEffects"]...)
	}
	if game := extra.Components["game"]; game.SpriteArrays != nil {
		m.peaSprites = append(m.peaSprites, game.SpriteArrays["peaSprites"]...)
		m.peaHitSprites = append(m.peaHitSprites, game.SpriteArrays["peaHitSprites"]...)
		m.handMat = game.Refs["handMaterial"]
	}
	if hand := extra.Components["hand"]; hand.SpriteArrays != nil {
		m.fastSprites = append(m.fastSprites, hand.SpriteArrays["fastSprites"]...)
		m.fastPath = hand.Refs["fastSprite"]
	}
	if player := extra.Components["player"]; player.Sprites != nil {
		m.hitFX = player.Sprites["hitFX"]
		m.hitFXG = player.Sprites["hitFXG"]
		m.hitFXMiss = player.Sprites["hitFXMiss"]
		m.hitFX2 = player.Sprites["hitFX2"]
		m.earlyPath = player.Refs["early"]
		m.perfectPath = player.Refs["perfect"]
		m.latePath = player.Refs["late"]
	}
	m.objectT = kart.NewTemplate(ctx.Assets, m.objectPath)
	ctx.Scene.SetActive(m.objectPath, false)
	ctx.Scene.PlayDefaultState(m.handPath, 0, ctx.SecPerBeat(0))
	ctx.Scene.PlayDefaultState(m.playerPath, 0, ctx.SecPerBeat(0))
	m.checkNextFlick()
	return nil
}

func roleOr(ctx *engine.Ctx, key, fallback string) string {
	if p := ctx.Role(key); p != "" {
		return p
	}
	return fallback
}

func (m *Module) OnEvent(e *riq.Entity) {
	switch e.Datamodel {
	case "forkLifter/flick":
		m.flicks = append(m.flicks, flickEvt{beat: e.Beat, typ: int(e.Float("type", flickPea)), idx: len(m.flicks)})
	case "forkLifter/prepare":
		m.preps = append(m.preps, prepareEvt{beat: e.Beat, mute: boolParam(e, "mute")})
	case "forkLifter/gulp":
		m.gulps = append(m.gulps, gulpEvt{beat: e.Beat, sfx: int(e.Float("sfx", eatDefault))})
	case "forkLifter/sigh":
		m.sighs = append(m.sighs, e.Beat)
	case "forkLifter/color":
		m.colors = append(m.colors, colorEvt{
			beat: e.Beat, length: e.Length,
			start: colorParam(e, "start", whiteColor),
			end:   colorParam(e, "end", whiteColor),
			ease:  int(e.Float("ease", 0)),
		})
	case "forkLifter/colorGrad":
		typ := gradGame
		if _, ok := e.Data["type"]; !ok {
			// Heaven Studio upgrades v0 gradient events by setting type=Classic.
			typ = gradClassic
		} else {
			typ = int(e.Float("type", gradGame))
		}
		m.grads = append(m.grads, gradEvt{
			beat: e.Beat, length: e.Length, typ: typ,
			viewCircle: boolParam(e, "toggleVC"), lines: boolParam(e, "toggleLines"),
			top0:  colorParam(e, "start", defaultGradTop),
			top1:  colorParam(e, "end", defaultGradTop),
			bot0:  colorParam(e, "startBG", whiteColor),
			bot1:  colorParam(e, "endBG", whiteColor),
			line0: colorParam(e, "startLines", defaultLines),
			line1: colorParam(e, "endLines", defaultLines),
			ease:  int(e.Float("ease", 0)),
		})
	case "forkLifter/colorHand":
		m.hands = append(m.hands, handEvt{
			beat:  e.Beat,
			skin:  colorParam(e, "color1", defaultSkin),
			shade: colorParam(e, "color2", defaultShade),
			fork:  colorParam(e, "color3", defaultForkColor),
		})
	}
}

func (m *Module) Ready() {
	sort.SliceStable(m.flicks, func(i, j int) bool { return m.flicks[i].beat < m.flicks[j].beat })
	for i := range m.flicks {
		m.flicks[i].idx = i
	}
	sort.SliceStable(m.preps, func(i, j int) bool { return m.preps[i].beat < m.preps[j].beat })
	sort.SliceStable(m.gulps, func(i, j int) bool { return m.gulps[i].beat < m.gulps[j].beat })
	sort.Float64s(m.sighs)
	sort.SliceStable(m.colors, func(i, j int) bool { return m.colors[i].beat < m.colors[j].beat })
	sort.SliceStable(m.grads, func(i, j int) bool { return m.grads[i].beat < m.grads[j].beat })
	sort.SliceStable(m.hands, func(i, j int) bool { return m.hands[i].beat < m.hands[j].beat })

	for i, ev := range m.flicks {
		i, ev := i, ev
		m.scheduleFlick(i, ev)
	}
	for _, ev := range m.preps {
		ev := ev
		m.ctx.At(ev.beat, func() { m.prepare(ev.beat, ev.mute) })
	}
	for _, ev := range m.gulps {
		ev := ev
		m.ctx.At(ev.beat, func() { m.eat(ev.beat, ev.sfx) })
	}
	for _, b := range m.sighs {
		beat := b
		m.ctx.SoundAt(beat, "sigh", 1)
	}
}

func (m *Module) scheduleFlick(idx int, ev flickEvt) {
	m.ctx.SoundAt(ev.beat, "flick", 1)
	if off := m.zoomFastOffset(); off > 0 {
		m.ctx.SoundAtOff(ev.beat+2, "zoomFast", 1, off)
	} else {
		m.ctx.SoundAt(ev.beat+2, "zoomFast", 1)
	}
	m.ctx.At(ev.beat, func() {
		if m.ctx.GameAt(ev.beat) == m.ID() {
			m.spawnFlick(idx, ev, ev.beat)
		}
	})
	m.ctx.ScheduleInput(ev.beat+2,
		func(state float64, _ engine.Judgment) { m.hitFlick(idx, state) },
		func() { m.missFlick(idx) })
	m.ctx.At(ev.beat+0.5, func() { m.checkNextFlick() })
}

func (m *Module) zoomFastOffset() float64 {
	pcm, ok := m.ctx.Assets.Sounds["zoomFast"]
	if !ok {
		return 0
	}
	sec := float64(len(pcm)) / (float64(engine.SampleRate) * 4)
	if sec <= 0.03 {
		return 0
	}
	return sec - 0.03
}

func (m *Module) OnSwitch(beat float64) {
	if beat <= 0 {
		m.resetRuntime()
	}
	m.ctx.Scene.PlayDefaultState(m.handPath, beat, m.ctx.SecPerBeat(beat))
	m.ctx.Scene.PlayDefaultState(m.playerPath, beat, m.ctx.SecPerBeat(beat))
	m.currentFlickIndex = 0
	for i, ev := range m.flicks {
		if ev.beat < beat {
			m.currentFlickIndex = i + 1
		}
		if ev.beat < beat && beat < ev.beat+2 {
			m.spawnFlick(i, ev, ev.beat)
		}
	}
	m.checkNextFlick()
}

func (m *Module) resetRuntime() {
	m.foods = nil
	m.fx = nil
	m.spawned = map[int]bool{}
	m.currentFlickIndex = 0
	m.earlyCount, m.perfectCount, m.lateCount = 0, 0, 0
	m.earlyStack, m.perfectStack, m.lateStack = nil, nil, nil
	m.isEating = false
	m.eatType = eatDefault
	m.topbun, m.middleburger, m.bottombun = false, false, false
}

func (m *Module) Whiff(beat float64) {
	m.stabNoHit(beat)
}

func (m *Module) Update(t, beat float64) {
	for _, f := range m.foods {
		if !f.dead && beat >= f.startBeat+2.45 {
			f.dead = true
		}
	}
	m.foods = filterFoods(m.foods)
	m.fx = filterFX(m.fx, t)
}

func (m *Module) Draw(screen *ebiten.Image, t float64, beat float64) {
	screen.Fill(stageFill)
	m.applyColors(beat)
	m.ctx.SampleScene(beat)

	for _, f := range m.foods {
		if f.dead || f.inst == nil {
			continue
		}
		norm := clamp01((beat - f.startBeat) / 2.45)
		f.inst.PlayNormalized("", "Flicked_Object", norm)
		f.inst.Queue(m.ctx.Scene, beat, kart.Identity(), 0)
	}
	m.queueForkStack()
	m.queueFX(t)
	m.ctx.Scene.Draw(screen, m.proj)
}

func (m *Module) spawnFlick(idx int, ev flickEvt, animBeat float64) {
	if m.spawned[idx] || m.objectT == nil {
		return
	}
	m.spawned[idx] = true
	if idx >= m.currentFlickIndex {
		m.currentFlickIndex = idx + 1
	}
	m.ctx.Scene.PlayState(m.handPath, "Hand_Flick", ev.beat, 0.5)
	inst := m.objectT.NewInstance()
	sprite := spriteAt(m.peaSprites, ev.typ)
	inst.SetSprite("Sprite", sprite)
	inst.SetSprite("Sprite/follow", sprite)
	inst.SetSprite("Sprite/follow (1)", sprite)
	m.foods = append(m.foods, &flyingFood{inst: inst, eventIdx: idx, startBeat: animBeat, typ: ev.typ})
}

func (m *Module) prepare(beat float64, mute bool) {
	if !mute {
		m.ctx.Sound("flickPrepare")
	}
	m.ctx.Scene.PlayState(m.handPath, "Hand_Prepare", beat, m.ctx.SecPerBeat(beat))
}

func (m *Module) checkNextFlick() {
	if m.currentFlickIndex >= 0 && m.currentFlickIndex < len(m.flicks) {
		typ := m.flicks[m.currentFlickIndex].typ
		m.ctx.Scene.SetSpriteOver(m.peaPreviewPath, spriteAt(m.peaSprites, typ))
		if m.fastPath != "" {
			fast := spriteAt(m.fastSprites, 0)
			if typ == flickBurger {
				fast = spriteAt(m.fastSprites, 1)
			}
			m.ctx.Scene.SetSpriteOver(m.fastPath, fast)
		}
		return
	}
	m.ctx.Scene.SetSpriteOver(m.peaPreviewPath, "")
}

func (m *Module) hitFlick(idx int, state float64) {
	f := m.foodByEvent(idx)
	if f == nil || f.dead {
		return
	}
	if state >= 1 {
		m.late(f)
		return
	}
	if state <= -1 {
		m.early(f)
		return
	}
	m.hit(f)
}

func (m *Module) missFlick(idx int) {
	if f := m.foodByEvent(idx); f != nil && !f.dead {
		m.ctx.Sound("disappointed")
	}
}

func (m *Module) foodByEvent(idx int) *flyingFood {
	for _, f := range m.foods {
		if f.eventIdx == idx {
			return f
		}
	}
	return nil
}

func (m *Module) hit(f *flyingFood) {
	beat := m.ctx.Beat()
	m.stab(beat)
	if len(m.perfectStack) < 4 {
		m.perfectStack = append(m.perfectStack, forkFood{typ: f.typ})
	}
	m.addFX(m.hitFX, beat, 0.05, [2]float64{1.9969, -3.7026}, [2]float64{3.142196, 3.142196}, 0, 100)
	m.fastEffect(f.typ, beat)
	m.ctx.Sound("stab")
	m.perfectCount++
	switch f.typ {
	case flickTopBun:
		m.topbun = true
	case flickBurger:
		m.middleburger = true
	case flickBottomBun:
		m.bottombun = true
	}
	f.dead = true
}

func (m *Module) early(f *flyingFood) {
	beat := m.ctx.Beat()
	m.stabNoHit(beat)
	m.earlyStack = append(m.earlyStack, forkFood{typ: f.typ})
	m.spawnMissFX(beat)
	m.fastEffect(f.typ, beat)
	m.ctx.PlayCommon("miss")
	m.earlyCount++
	f.dead = true
}

func (m *Module) late(f *flyingFood) {
	beat := m.ctx.Beat()
	m.stabNoHit(beat)
	m.lateStack = append(m.lateStack, forkFood{typ: f.typ})
	m.spawnMissFX(beat)
	m.fastEffect(f.typ, beat)
	m.ctx.PlayCommon("miss")
	m.lateCount++
	f.dead = true
}

func (m *Module) stab(beat float64) {
	if m.isEating {
		return
	}
	m.ctx.Scene.PlayState(m.playerPath, "Player_Stab", beat, m.ctx.SecPerBeat(beat))
}

func (m *Module) stabNoHit(beat float64) {
	if m.isEating {
		return
	}
	m.ctx.Sound("stabnohit")
	m.ctx.Scene.PlayState(m.playerPath, "Player_Stab", beat, m.ctx.SecPerBeat(beat))
}

func (m *Module) spawnMissFX(beat float64) {
	for _, spec := range []struct {
		pos   [2]float64
		scale [2]float64
	}{
		{[2]float64{1.0424, -4.032}, [2]float64{1.129612, 1.129612}},
		{[2]float64{0.771, -3.016}, [2]float64{1.71701, 1.71701}},
		{[2]float64{2.598, -2.956}, [2]float64{1.576043, 1.576043}},
		{[2]float64{2.551, -3.609}, [2]float64{1.200788, 1.200788}},
	} {
		m.addFX(m.hitFXMiss, beat, 0.05, spec.pos, spec.scale, 0, 100)
	}
}

func (m *Module) fastEffect(typ int, beat float64) {
	sprite := m.hitFX2
	if typ == flickBurger {
		sprite = m.hitFXG
	}
	m.addFX(sprite, beat, 0.07, [2]float64{0.11, -2.15}, [2]float64{5.401058, 1.742697}, -38.402*math.Pi/180, -5)
}

func (m *Module) addFX(sprite string, beat, dur float64, pos, scale [2]float64, rot float64, order int) {
	if sprite == "" {
		return
	}
	m.fx = append(m.fx, flashFX{
		sprite: sprite, born: m.ctx.Time(), dur: dur,
		pos: pos, scale: scale, rot: rot, order: order,
	})
}

func (m *Module) eat(beat float64, eatType int) {
	if m.earlyCount == 0 && m.perfectCount == 0 && m.lateCount == 0 {
		return
	}
	m.eatType = eatType
	m.isEating = true
	m.ctx.Scene.PlayState(m.playerPath, "Player_Eat", beat, m.ctx.SecPerBeat(beat))
	confirmBeat := m.beatAfterSeconds(beat, 0.18333334)
	m.ctx.At(confirmBeat, func() { m.eatConfirm(confirmBeat) })
}

func (m *Module) eatConfirm(beat float64) {
	if !m.isEating {
		return
	}
	if m.eatType != eatNormal && ((m.topbun && m.middleburger && m.bottombun) || m.eatType == eatBurger) {
		m.ctx.Sound("burger")
	} else if m.earlyCount > 0 || m.lateCount > 0 {
		m.ctx.Sound(coughSound(beat))
	} else {
		m.ctx.Sound("gulp")
	}
	m.clearFork()
}

func (m *Module) clearFork() {
	m.earlyCount, m.perfectCount, m.lateCount = 0, 0, 0
	m.earlyStack, m.perfectStack, m.lateStack = nil, nil, nil
	m.isEating = false
	m.topbun, m.middleburger, m.bottombun = false, false, false
}

func coughSound(beat float64) string {
	r := rand.New(rand.NewSource(int64(beat*1000) + 17))
	if r.Intn(2) == 0 {
		return "cough_1"
	}
	return "cough_2"
}

func (m *Module) beatAfterSeconds(beat, sec float64) float64 {
	return m.ctx.TimeToBeat(m.ctx.BeatToTime(beat) + sec)
}

func (m *Module) queueForkStack() {
	m.queueStack(m.earlyPath, m.earlyStack, true, 20)
	m.queueStack(m.perfectPath, m.perfectStack, false, 0)
	m.queueStack(m.latePath, m.lateStack, true, 20)
}

func (m *Module) queueStack(path string, stack []forkFood, sideways bool, fixedOrder int) {
	if len(stack) == 0 || path == "" {
		return
	}
	root, ok := m.ctx.Scene.NodeWorld(path)
	if !ok {
		return
	}
	total := len(stack)
	beforeCount := total - 1
	extraOffset := 0.0
	if !sideways && beforeCount == 3 {
		extraOffset = -0.15724
	}
	for i, item := range stack {
		y := (-1.67 - 0.15724*float64(i)) + 0.15724*float64(beforeCount) + extraOffset
		order := fixedOrder
		if !sideways {
			order = perfectOrder(item.typ)
		}
		rot := 0.0
		if sideways {
			rot = math.Pi / 2
		}
		m.ctx.Scene.Queue(kart.ExtraSprite{
			Sprite: spriteAt(m.peaHitSprites, item.typ),
			World:  root.Mul(kart.TRS(0, y, rot, 1, 1)),
			Order:  order,
		})
	}
}

func perfectOrder(typ int) int {
	switch typ {
	case flickPea:
		return 101
	case flickTopBun:
		return 104
	case flickBurger:
		return 103
	case flickBottomBun:
		return 102
	default:
		return 20
	}
}

func (m *Module) queueFX(t float64) {
	for _, f := range m.fx {
		if f.dur <= 0 {
			continue
		}
		u := clamp01((t - f.born) / f.dur)
		if u >= 1 {
			continue
		}
		m.ctx.Scene.Queue(kart.ExtraSprite{
			Sprite: f.sprite,
			World:  kart.Translate(f.pos[0], f.pos[1]).Mul(kart.Rotate(f.rot)).Mul(kart.Scale(f.scale[0], f.scale[1])),
			Order:  f.order,
			Layer:  f.layer,
			Tint:   [4]float64{1, 1, 1, 1 - u},
			Mapped: f.mapped,
			Mat:    f.mat,
		})
	}
}

func (m *Module) applyColors(beat float64) {
	bg := m.bgAt(beat)
	gr := m.gradAt(beat)
	top := easeColor(gr.ease, gr.top0, gr.top1, colorProgress(beat, gr.beat, gr.length))
	bottom := easeColor(gr.ease, gr.bot0, gr.bot1, colorProgress(beat, gr.beat, gr.length))
	line := easeColor(gr.ease, gr.line0, gr.line1, colorProgress(beat, gr.beat, gr.length))

	sc := m.ctx.Scene
	sc.SetColorOver(m.bgPath, bg)
	sc.SetColorOver(m.viewerCircle, bg)
	sc.SetColorOver(m.mmLines, line)
	for i, p := range m.gradients {
		sc.SetActive(p, i == gr.typ)
		sc.SetColorOver(p, top)
	}
	sc.SetActive(m.mmLines, gr.typ != gradClassic && gr.lines)

	shadow := bottom
	if gr.typ == gradClassic {
		shadow = top
	}
	sc.SetColorOver(m.gradientFiller, shadow)
	sc.SetColorOver(m.playerShadow, shadow)
	for _, p := range m.forkEffects {
		sc.SetColorOver(p, shadow)
	}
	vc := bg
	if gr.viewCircle {
		vc = top
	}
	sc.SetColorOver(m.viewerCircleBg, vc)
	sc.SetColorOver(m.handShadow, vc)

	hand := m.handAt(beat)
	if m.handMat != "" {
		sc.SetPaletteFor(m.handMat, kart.Palette{
			Alpha: hand.skin, Fill: hand.shade, Outline: whiteColor,
		})
	}
	sc.SetColorOver(m.forkPath, hand.fork)
}

func (m *Module) bgAt(beat float64) [4]float64 {
	out := whiteColor
	for _, ev := range m.colors {
		if ev.beat > beat {
			break
		}
		out = easeColor(ev.ease, ev.start, ev.end, colorProgress(beat, ev.beat, ev.length))
	}
	return out
}

func (m *Module) gradAt(beat float64) gradEvt {
	out := gradEvt{
		typ: gradGame, top0: defaultGradTop, top1: defaultGradTop,
		bot0: whiteColor, bot1: whiteColor, line0: defaultLines, line1: defaultLines,
	}
	for _, ev := range m.grads {
		if ev.beat > beat {
			break
		}
		out = ev
	}
	return out
}

func (m *Module) handAt(beat float64) handEvt {
	out := handEvt{skin: defaultSkin, shade: defaultShade, fork: defaultForkColor}
	for _, ev := range m.hands {
		if ev.beat > beat {
			break
		}
		out = ev
	}
	return out
}

func colorProgress(beat, start, length float64) float64 {
	if length <= 0 || beat >= start+length {
		return 1
	}
	return clamp01((beat - start) / length)
}

func easeColor(ease int, a, b [4]float64, u float64) [4]float64 {
	return [4]float64{
		engine.Ease(ease, a[0], b[0], u),
		engine.Ease(ease, a[1], b[1], u),
		engine.Ease(ease, a[2], b[2], u),
		engine.Ease(ease, a[3], b[3], u),
	}
}

func spriteAt(s []string, idx int) string {
	if idx >= 0 && idx < len(s) {
		return s[idx]
	}
	return ""
}

func filterFoods(in []*flyingFood) []*flyingFood {
	out := in[:0]
	for _, f := range in {
		if !f.dead {
			out = append(out, f)
		}
	}
	return out
}

func filterFX(in []flashFX, t float64) []flashFX {
	out := in[:0]
	for _, f := range in {
		if f.dur <= 0 || t-f.born < f.dur {
			out = append(out, f)
		}
	}
	return out
}

func boolParam(e *riq.Entity, key string) bool { return e.Float(key, 0) != 0 }

func colorParam(e *riq.Entity, key string, def [4]float64) [4]float64 {
	v, ok := e.Data[key]
	if !ok {
		return def
	}
	m, ok := v.(map[string]any)
	if !ok {
		return def
	}
	return [4]float64{
		num(m["r"], def[0]),
		num(m["g"], def[1]),
		num(m["b"], def[2]),
		num(m["a"], def[3]),
	}
}

func num(v any, def float64) float64 {
	if f, ok := v.(float64); ok {
		return f
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
