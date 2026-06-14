// Package ninjabodyguard ports Ninja Bodyguard's call/response arrows,
// camera cuts, alternating input actions, dynamic ninja rows, and hit arrows.
package ninjabodyguard

import (
	"image/color"
	"math"
	"math/rand"
	"sort"

	"github.com/hajimehoshi/ebiten/v2"

	"hsdemo/engine"
	"hsdemo/kart"
	"hsdemo/kmdata"
	"hsdemo/riq"
)

const (
	normalAction = 0
	padAction    = 1

	cameraEnemies = 0
	cameraCut     = 1
	cameraPlayer  = 2

	arrowDestroy = iota
	arrowDivertL
	arrowDivertR
	arrowHit
)

type intervalEvt struct {
	beat, length float64
	auto, flash  bool
}

type arrowEvt struct {
	beat float64
}

type prepareEvt struct {
	beat float64
}

type passEvt struct {
	beat  float64
	flash bool
}

type cameraEvt struct {
	beat  float64
	pos   int
	flash bool
}

type ninjaInst struct {
	inst *kart.Instance
}

type arrowInst struct {
	inst     *kart.Instance
	bornBeat float64
	state    int
}

type hitSpark struct {
	beat float64
}

type Module struct {
	ctx  *engine.Ctx
	proj kart.Aff

	player, guide, lord string
	firstNinja          string
	ninjaArrow          string
	leftScene           string
	blackout            string
	hitParticle         string

	ninjaT *kart.Template
	arrowT *kart.Template

	enemyStart [2]float64
	xDist      float64
	yDist      float64
	divertPos  [2]float64
	hitCurve   kmdata.Curve

	intervals []intervalEvt
	arrows    []arrowEvt
	prepares  []prepareEvt
	passes    []passEvt
	cameras   []cameraEvt

	enemies []*ninjaInst
	effects []*arrowInst
	sparks  []hitSpark

	dPad          bool
	canWhiff      bool
	cameraOnEnemy bool
	cameraX       float64

	rng *rand.Rand
}

func New() engine.Module { return &Module{xDist: -1.9, yDist: -1, rng: rand.New(rand.NewSource(26))} }

func (m *Module) ID() string { return "ninjaBodyguard" }

func (m *Module) Load(ctx *engine.Ctx) error {
	m.ctx = ctx
	if err := ctx.LoadAssets("ninjaBodyguard"); err != nil {
		return err
	}
	m.proj = kart.Translate(engine.ScreenW/2, engine.ScreenH/2).Mul(kart.Scale(54, -54))

	m.player = roleOr(ctx, "PlayerAnim", "Gameplay/Right/Player")
	m.guide = roleOr(ctx, "GuideAnim", "Gameplay/Right/Guide")
	m.lord = roleOr(ctx, "LordAnim", "Gameplay/Right/Samurai")
	m.firstNinja = roleOr(ctx, "FirstNinja", "Gameplay/Left/Ninja")
	m.ninjaArrow = roleOr(ctx, "NinjaArrow", "Gameplay/Right/Arrow")
	m.leftScene = roleOr(ctx, "LeftSceneObj", "Gameplay/Left")
	m.blackout = roleOr(ctx, "Blackout", "Blackout")
	m.hitParticle = roleOr(ctx, "HitParticle", "Gameplay/Right/ArrowSliceA")

	if game := ctx.Assets.Extra.Components["game"]; game.Nums != nil {
		if v, ok := game.Nums["xDistanceEnemy"]; ok {
			m.xDist = v
		}
		if v, ok := game.Nums["yDistanceEnemy"]; ok {
			m.yDist = v
		}
	}
	if arrow := ctx.Assets.Extra.Components["arrow"]; arrow.Nums != nil {
		m.divertPos = [2]float64{arrow.Nums["divertPosition.x"], arrow.Nums["divertPosition.y"]}
	}
	m.hitCurve = ctx.Assets.Extra.Curves["arrow.hitCurve"]
	m.enemyStart = m.nodePos(m.firstNinja)
	m.ninjaT = kart.NewTemplate(ctx.Assets, m.firstNinja)
	m.arrowT = kart.NewTemplate(ctx.Assets, m.ninjaArrow)

	// HitParticle is a Unity ParticleSystem. Scene extraction keeps its node
	// transform but not the renderer module, so runtime registers a tiny white
	// slash sprite and emits it from that authored transform on perfect cuts.
	slash := ebiten.NewImage(4, 28)
	slash.Fill(color.RGBA{255, 255, 255, 255})
	ctx.Assets.RegisterSprite("__ninja_hit_slash", slash, 100, 0.5, 0.5)

	m.reset(0)
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
	case "ninjaBodyguard/start interval":
		m.intervals = append(m.intervals, intervalEvt{
			beat: e.Beat, length: e.Length,
			auto: boolParamDefault(e, "auto", true), flash: boolParamDefault(e, "flash", true),
		})
	case "ninjaBodyguard/pass turn":
		m.passes = append(m.passes, passEvt{beat: e.Beat, flash: boolParamDefault(e, "flash", true)})
	case "ninjaBodyguard/arrow":
		m.arrows = append(m.arrows, arrowEvt{beat: e.Beat})
	case "ninjaBodyguard/prepare":
		m.prepares = append(m.prepares, prepareEvt{beat: e.Beat})
	case "ninjaBodyguard/cameraPos":
		m.cameras = append(m.cameras, cameraEvt{
			beat: e.Beat, pos: int(e.Float("pos", cameraEnemies)),
			flash: boolParamDefault(e, "flash", true),
		})
	}
}

func (m *Module) Ready() {
	sort.Slice(m.intervals, func(i, j int) bool { return m.intervals[i].beat < m.intervals[j].beat })
	sort.Slice(m.arrows, func(i, j int) bool { return m.arrows[i].beat < m.arrows[j].beat })
	sort.Slice(m.prepares, func(i, j int) bool { return m.prepares[i].beat < m.prepares[j].beat })
	sort.Slice(m.passes, func(i, j int) bool { return m.passes[i].beat < m.passes[j].beat })
	sort.Slice(m.cameras, func(i, j int) bool { return m.cameras[i].beat < m.cameras[j].beat })

	for _, iv := range m.intervals {
		iv := iv
		m.ctx.At(iv.beat, func() { m.startInterval(iv, iv.beat, true) })
	}
	for _, ev := range m.passes {
		ev := ev
		m.ctx.At(ev.beat, func() { m.passTurn(ev.beat, math.NaN(), math.NaN(), nil, ev.flash) })
	}
	for _, ev := range m.cameras {
		ev := ev
		m.ctx.At(ev.beat, func() { m.setCameraPos(ev.beat, ev.pos, ev.flash) })
	}
}

func (m *Module) OnSwitch(beat float64) {
	m.reset(beat)
	for _, ev := range m.cameras {
		if ev.beat > beat {
			break
		}
		m.setCameraPos(ev.beat, ev.pos, false)
	}
	for _, iv := range m.intervals {
		if iv.beat >= beat && iv.beat < beat+1.5 {
			m.cameraX = -50
			m.cameraOnEnemy = true
			m.canWhiff = false
			break
		}
	}
	for _, iv := range m.intervals {
		if iv.beat-2 < beat && iv.beat+iv.length >= beat {
			m.startInterval(iv, beat, false)
		}
	}
}

func (m *Module) Whiff(beat float64) { m.WhiffAction(beat, normalAction) }

func (m *Module) WhiffAction(beat float64, action int) {
	switch {
	case action == normalAction && !m.dPad && m.canWhiff:
		m.setInputs(true)
		m.ctx.Scene.PlayState(m.player, "NinjaSwingR", beat, 0.5)
		m.ctx.Scene.PlayState(m.guide, "Right", beat, 0.5)
		m.ctx.SoundPitch("SE_AGB_TONO_EN_FURU", 1, cents(m.rng.Intn(301)-200))
	case action == padAction && m.dPad && m.canWhiff:
		m.setInputs(false)
		m.ctx.Scene.PlayState(m.player, "NinjaSwingL", beat, 0.5)
		m.ctx.Scene.PlayState(m.guide, "Left", beat, 0.5)
		m.ctx.SoundPitch("SE_AGB_TONO_EN_FURU", 1, cents(m.rng.Intn(301)-200))
	}
}

func (m *Module) Update(t, beat float64) {
	m.pruneEffects(beat)
}

func (m *Module) Draw(screen *ebiten.Image, t, beat float64) {
	cam := m.ctx.CameraAt(beat)
	if m.ctx.Scene != nil {
		m.ctx.Scene.SetCamera(cam[0]+m.cameraX, cam[1], cam[2])
		m.ctx.Scene.Sample(beat)
	}
	for _, e := range m.enemies {
		e.inst.Queue(m.ctx.Scene, beat, kart.Identity(), 0)
	}
	for _, a := range m.effects {
		a.inst.Queue(m.ctx.Scene, beat, kart.Identity(), 0)
	}
	m.queueSparks(beat)
	m.ctx.Scene.Draw(screen, m.proj)
}

func (m *Module) reset(beat float64) {
	m.ctx.Scene.SetActive(m.firstNinja, false)
	m.ctx.Scene.SetActive(m.ninjaArrow, false)
	m.ctx.Scene.SetActive(m.blackout, false)
	m.ctx.Scene.PlayDefaultState(m.player, beat, m.ctx.SecPerBeat(beat))
	m.ctx.Scene.PlayDefaultState(m.guide, beat, m.ctx.SecPerBeat(beat))
	m.ctx.Scene.PlayState(m.lord, "Stay", beat, 0.5)
	m.ctx.Scene.PlayDefaultState(m.firstNinja, beat, m.ctx.SecPerBeat(beat))
	m.ctx.Scene.PlayDefaultState(m.ninjaArrow, beat, m.ctx.SecPerBeat(beat))
	m.enemies = nil
	m.effects = nil
	m.sparks = nil
	m.dPad = false
	m.canWhiff = true
	m.cameraOnEnemy = false
	m.cameraX = 0
}

func (m *Module) startInterval(iv intervalEvt, nowBeat float64, schedule bool) {
	calls := m.callsFor(iv.beat, iv.length)
	if len(calls) == 0 {
		return
	}
	m.spawnNinjas(len(calls))
	if schedule {
		if iv.flash {
			m.flash(iv.beat, true)
		}
		m.cameraX = -50
		m.cameraOnEnemy = true
		m.canWhiff = false
		for _, prep := range m.preparesFor(iv.beat, iv.length) {
			prep := prep
			m.ctx.At(prep.beat, func() { m.prepareEnemies(prep.beat) })
		}
		for i, call := range calls {
			i, call := i, call
			m.ctx.SoundAt(call.beat, "SE_AGB_TONO_EN_YUMI", 1)
			m.ctx.SoundAtPitchPan(call.beat, "SE_AGB_TONO_EN_YUMI_PITCH", 1, cents(m.rng.Intn(101)-100), 0)
			m.ctx.At(call.beat, func() { m.shootEnemy(i, call.beat) })
		}
		if iv.auto {
			end := iv.beat + iv.length
			m.ctx.At(end, func() { m.passTurn(end, iv.beat, iv.length, calls, iv.flash) })
		}
		return
	}

	if iv.flash && nowBeat < iv.beat+0.5 {
		m.ctx.Scene.SetActive(m.blackout, true)
	}
	m.cameraX = -50
	m.cameraOnEnemy = true
	m.canWhiff = false
	for _, prep := range m.preparesFor(iv.beat, iv.length) {
		if prep.beat <= nowBeat {
			m.prepareEnemies(nowBeat)
		}
	}
	for i, call := range calls {
		if call.beat <= nowBeat {
			m.shootEnemy(i, nowBeat)
		}
	}
}

func (m *Module) passTurn(beat, startBeat, length float64, calls []arrowEvt, flash bool) {
	if math.IsNaN(startBeat) || math.IsNaN(length) {
		iv, ok := m.lastFinishedInterval(beat)
		if !ok {
			return
		}
		startBeat, length = iv.beat, iv.length
	}
	if calls == nil {
		calls = m.callsFor(startBeat, length)
	}
	if len(calls) == 0 {
		return
	}
	m.ctx.Scene.PlayState(m.player, "Idle", beat, 0.5)
	m.ctx.Scene.PlayState(m.guide, "Left", beat, 0.5)
	m.ctx.Scene.PlayState(m.lord, "Stay", beat, 0.5)
	m.setInputs(false)
	m.effects = nil
	if flash {
		m.flash(beat, false)
	}
	m.cameraX = 0
	m.cameraOnEnemy = false
	m.canWhiff = true
	for _, call := range calls {
		target := beat + (call.beat - startBeat)
		m.scheduleAlternatingInput(target)
	}
}

func (m *Module) scheduleAlternatingInput(target float64) {
	targetGame := m.ctx.GameAt(target) == m.ID()
	m.ctx.ScheduleInputActionCond(target, normalAction, func() bool {
		return targetGame && m.canWhiff && !m.dPad
	}, func(state float64, j engine.Judgment) {
		m.onHit(target, state)
	}, func() { m.onMiss(target) })
	m.ctx.ScheduleInputActionCond(target, padAction, func() bool {
		return targetGame && m.canWhiff && m.dPad
	}, func(state float64, j engine.Judgment) {
		m.onHit(target, state)
	}, func() { m.onMiss(target) })
}

func (m *Module) onHit(beat, state float64) {
	left := m.dPad
	if state >= 1 || state <= -1 {
		m.ctx.Scene.PlayState(m.player, pick(left, "NinjaSwingL", "NinjaSwingR"), beat, 0.5)
		m.ctx.Scene.PlayState(m.guide, pick(left, "Left", "Right"), beat, 0.5)
		m.ctx.Sound("SE_AGB_TONO_EN_KIN")
		if left {
			m.spawnArrow(arrowDivertL, beat)
		} else {
			m.spawnArrow(arrowDivertR, beat)
		}
	} else {
		m.ctx.Scene.PlayState(m.player, pick(left, "NinjaCutL", "NinjaCutR"), beat, 0.5)
		m.ctx.Scene.PlayState(m.guide, pick(left, "Left", "Right"), beat, 0.5)
		m.ctx.Sound("SE_AGB_TONO_EN_HIT")
		m.spawnArrow(arrowDestroy, beat)
		m.sparks = append(m.sparks, hitSpark{beat: beat})
	}
	m.setInputs(!m.dPad)
}

func (m *Module) onMiss(beat float64) {
	m.ctx.Scene.PlayState(m.lord, "Shock", beat, 0.5)
	m.ctx.Sound("SE_AGB_TONO_EN_MISS")
	m.spawnArrow(arrowHit, beat)
}

func (m *Module) flash(beat float64, cameraEnemy bool) {
	if cameraEnemy == m.cameraOnEnemy {
		return
	}
	m.ctx.Scene.SetActive(m.blackout, true)
	m.ctx.At(beat+0.5, func() { m.ctx.Scene.SetActive(m.blackout, false) })
}

func (m *Module) setCameraPos(beat float64, pos int, flash bool) {
	switch pos {
	case cameraEnemies:
		if flash {
			m.flash(beat, true)
		}
		m.cameraX = -50
		m.cameraOnEnemy = true
		m.canWhiff = false
	case cameraCut:
		if flash {
			m.flash(beat, true)
		}
		m.cameraX = -25
		m.cameraOnEnemy = false
		m.canWhiff = false
	case cameraPlayer:
		if flash {
			m.flash(beat, true)
		}
		m.cameraX = 0
		m.cameraOnEnemy = false
		m.canWhiff = true
	}
}

func (m *Module) spawnNinjas(count int) {
	if m.ninjaT == nil {
		return
	}
	dx, dy := m.xDist, m.yDist
	if count > 6 {
		r := 6 / float64(count)
		dx, dy = dx*r, dy*r
	}
	first := [2]float64{
		m.enemyStart[0] + dx/2 - dx*float64(count)/2,
		m.enemyStart[1] + dy/2 - dy*float64(count)/2,
	}
	m.enemies = nil
	for i := 0; i < count; i++ {
		inst := m.ninjaT.NewInstance()
		inst.Offset = [2]float64{first[0] + dx*float64(i), first[1] + dy*float64(i)}
		inst.SetGroupOrder(i)
		inst.PlayDefaultState("", m.ctx.Beat(), m.ctx.SecPerBeat(m.ctx.Beat()))
		m.enemies = append(m.enemies, &ninjaInst{inst: inst})
	}
}

func (m *Module) prepareEnemies(beat float64) {
	for _, e := range m.enemies {
		e.inst.PlayState("", "ArrowReady", beat, 0.5)
	}
}

func (m *Module) shootEnemy(i int, beat float64) {
	if i < 0 || i >= len(m.enemies) {
		return
	}
	m.enemies[i].inst.PlayState("", "ArrowShot", beat, 0.5)
}

func (m *Module) spawnArrow(state int, beat float64) {
	if m.arrowT == nil {
		return
	}
	inst := m.arrowT.NewInstance()
	switch state {
	case arrowDestroy:
		inst.Offset = m.divertPos
		inst.PlayState("", "Destroy", beat, 0.5)
	case arrowDivertL:
		inst.Offset = m.divertPos
		inst.PlayState("", "DivertL", beat, 0.5)
	case arrowDivertR:
		inst.Offset = m.divertPos
		inst.PlayState("", "DivertR", beat, 0.5)
	case arrowHit:
		u := m.rng.Float64()
		p := kart.EvalBezier(m.hitCurve, u)
		inst.Offset = [2]float64{p[0], p[1]}
		inst.SetGroupOrder(pickInt(u <= 0.5, 5, 15))
		inst.PlayState("", "Hit", beat, 0.5)
	}
	m.effects = append(m.effects, &arrowInst{inst: inst, bornBeat: beat, state: state})
}

func (m *Module) pruneEffects(beat float64) {
	dst := m.effects[:0]
	for _, e := range m.effects {
		if e.state == arrowHit || beat-e.bornBeat <= 2 {
			dst = append(dst, e)
		}
	}
	m.effects = dst
	sparks := m.sparks[:0]
	for _, s := range m.sparks {
		if beat-s.beat <= 0.35 {
			sparks = append(sparks, s)
		}
	}
	m.sparks = sparks
}

func (m *Module) queueSparks(beat float64) {
	base, ok := m.ctx.Scene.NodeWorld(m.hitParticle)
	if !ok {
		base = kart.Translate(-1.57, 1.36)
	}
	for _, sp := range m.sparks {
		u := (beat - sp.beat) / 0.22
		if u < 0 || u > 1 {
			continue
		}
		alpha := 1 - u
		for i := 0; i < 5; i++ {
			off := float64(i-2) * 0.11
			lenScale := 1.0 + float64(i%2)*0.25
			world := base.Mul(kart.TRS(off+u*0.35, off*0.2, -0.62, 0.16, lenScale))
			m.ctx.Scene.Queue(kart.ExtraSprite{
				Sprite: "__ninja_hit_slash", World: world,
				Layer: 0, Order: 52 + i,
				Tint: [4]float64{1, 1, 1, alpha},
			})
		}
	}
}

func (m *Module) callsFor(beat, length float64) []arrowEvt {
	var out []arrowEvt
	end := beat + length
	for _, call := range m.arrows {
		if call.beat < beat {
			continue
		}
		if call.beat > end {
			break
		}
		out = append(out, call)
	}
	return out
}

func (m *Module) preparesFor(beat, length float64) []prepareEvt {
	var out []prepareEvt
	end := beat + length
	for _, prep := range m.prepares {
		if prep.beat < beat {
			continue
		}
		if prep.beat >= end {
			break
		}
		out = append(out, prep)
	}
	return out
}

func (m *Module) lastFinishedInterval(beat float64) (intervalEvt, bool) {
	for i := len(m.intervals) - 1; i >= 0; i-- {
		iv := m.intervals[i]
		if iv.beat+iv.length < beat {
			return iv, true
		}
	}
	return intervalEvt{}, false
}

func (m *Module) setInputs(dPad bool) {
	m.dPad = dPad
}

func (m *Module) nodePos(path string) [2]float64 {
	for _, n := range m.ctx.Assets.Rig.Nodes {
		if n.Path == path {
			return n.Pos
		}
	}
	return [2]float64{}
}

func boolParamDefault(e *riq.Entity, key string, def bool) bool {
	if v, ok := e.Data[key].(bool); ok {
		return v
	}
	return def
}

func cents(v int) float64 {
	return math.Pow(2, float64(v)/1200)
}

func pick[T any](cond bool, a, b T) T {
	if cond {
		return a
	}
	return b
}

func pickInt(cond bool, a, b int) int {
	if cond {
		return a
	}
	return b
}
