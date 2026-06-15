// Package shootemup ports Shoot-'Em-Up's call-and-response enemy spawning,
// arbitrary-button shooting, gate/monitor animation cues, and per-enemy
// mapped-material colors.
package shootemup

import (
	"math"
	"sort"

	"github.com/hajimehoshi/ebiten/v2"

	"hsdemo/engine"
	"hsdemo/kart"
	"hsdemo/riq"
)

const (
	gameID    = "shootEmUp"
	animScale = 0.5

	placementManual = 3

	monitorEnter = iota
	monitorExit
	monitorTalk
	monitorIdle
	monitorBop
)

type intervalEvt struct {
	beat, length float64
	placement    int
	autoPass     bool
	processed    bool
}

type spawnEvt struct {
	beat           float64
	pos            vec2
	manual         bool
	enemyType      int
	colorA, colorB [4]float64
}

type passEvt struct {
	beat      float64
	processed bool
}

type bopEvt struct {
	beat, length float64
	bop, auto    bool
}

type gateEvt struct {
	beat, length float64
	mute         bool
}

type monitorEvt struct {
	beat, length float64
	typ          int
	mute         bool
}

type intervalSession struct {
	start, length float64
	enemies       []*enemy
}

type Module struct {
	ctx  *engine.Ctx
	proj kart.Aff

	enemyT, trajectoryT, originT, impactT, missImpactT *kart.Template

	intervals []intervalEvt
	spawns    []spawnEvt
	passes    []passEvt
	bops      []bopEvt
	gates     []gateEvt
	monitors  []monitorEvt

	sessions  []*intervalSession
	enemies   []*enemy
	effects   []*effectInst
	particles []particle

	canBop   bool
	autoBop  bool
	lastBeat int

	shipDamageUntil float64
}

func New() engine.Module { return &Module{lastBeat: math.MinInt} }

func (m *Module) ID() string { return gameID }

func (m *Module) Load(ctx *engine.Ctx) error {
	m.ctx = ctx
	if err := ctx.LoadAssets(gameID); err != nil {
		return err
	}
	m.proj = kart.Translate(engine.ScreenW/2, engine.ScreenH/2).Mul(kart.Scale(54, -54))
	m.enemyT = kart.NewTemplate(ctx.Assets, "prefabs/enemy")
	m.trajectoryT = kart.NewTemplate(ctx.Assets, "prefabs/trajectory")
	m.originT = kart.NewTemplate(ctx.Assets, "prefabs/origin")
	m.impactT = kart.NewTemplate(ctx.Assets, "prefabs/impact")
	m.missImpactT = kart.NewTemplate(ctx.Assets, "prefabs/missimpact")
	m.resetScene(0)
	return nil
}

func (m *Module) OnEvent(e *riq.Entity) {
	switch e.Datamodel {
	case "shootEmUp/bop":
		ev := bopEvt{
			beat: e.Beat, length: e.Length,
			bop:  boolDefault(e, "toggle", true),
			auto: boolParam(e, "toggle2"),
		}
		m.bops = append(m.bops, ev)
		m.ctx.At(ev.beat, func() {
			if m.ctx.GameAt(ev.beat) == gameID {
				m.autoBop = ev.auto
			}
		})
		if ev.bop {
			for i := 0; float64(i) < ev.length; i++ {
				b := ev.beat + float64(i)
				m.ctx.At(b, func() {
					if m.ctx.GameAt(b) == gameID {
						m.bop(b)
					}
				})
			}
		}
	case "shootEmUp/simple interval", "shootEmUp/start interval":
		idx := len(m.intervals)
		length := e.Length
		if length <= 0 {
			length = 4
		}
		m.intervals = append(m.intervals, intervalEvt{
			beat: e.Beat, length: length,
			placement: int(e.Float("placement", 0)),
			autoPass:  boolDefault(e, "auto", true),
		})
		m.ctx.At(e.Beat, func() {
			if m.ctx.GameAt(e.Beat) == gameID {
				m.setIntervalStart(idx, e.Beat)
			}
		})
	case "shootEmUp/spawn enemy":
		pos := vec2{e.Float("x_int", 0), e.Float("y_int", 0)}
		manual := boolParam(e, "fine")
		if manual {
			pos = vec2{e.Float("x_float", 0), e.Float("y_float", 0)}
		}
		m.spawns = append(m.spawns, spawnEvt{
			beat: e.Beat, pos: pos, manual: manual,
			enemyType: int(e.Float("type", enemyBasic)),
			colorA:    colorParam(e, "colorA", white),
			colorB:    colorParam(e, "colorB", white),
		})
	case "shootEmUp/passTurn":
		idx := len(m.passes)
		m.passes = append(m.passes, passEvt{beat: e.Beat})
		m.ctx.At(e.Beat, func() {
			if m.ctx.GameAt(e.Beat) == gameID {
				m.passTurnStandalone(idx)
			}
		})
	case "shootEmUp/gate events":
		ev := gateEvt{beat: e.Beat, length: e.Length, mute: boolParam(e, "mute")}
		if ev.length <= 0 {
			ev.length = 1
		}
		m.gates = append(m.gates, ev)
		m.ctx.At(ev.beat, func() {
			if m.ctx.GameAt(ev.beat) == gameID {
				m.gateAnims(ev)
			}
		})
	case "shootEmUp/monitor events":
		ev := monitorEvt{
			beat: e.Beat, length: e.Length,
			typ:  int(e.Float("toggle", monitorEnter)),
			mute: boolParam(e, "mute"),
		}
		if ev.length <= 0 {
			ev.length = 1
		}
		m.monitors = append(m.monitors, ev)
		m.ctx.At(ev.beat, func() {
			if m.ctx.GameAt(ev.beat) == gameID {
				m.monitorAnims(ev)
			}
		})
	}
}

func (m *Module) Ready() {
	sort.Slice(m.spawns, func(i, j int) bool { return m.spawns[i].beat < m.spawns[j].beat })
	sort.Slice(m.bops, func(i, j int) bool { return m.bops[i].beat < m.bops[j].beat })
	sort.Slice(m.gates, func(i, j int) bool { return m.gates[i].beat < m.gates[j].beat })
}

func (m *Module) OnSwitch(beat float64) {
	m.resetScene(beat)
	m.sessions = nil
	m.enemies = nil
	m.effects = nil
	m.particles = nil
	m.shipDamageUntil = 0
	m.lastBeat = int(math.Floor(beat)) - 1
	m.autoBop = false
	for _, ev := range m.bops {
		if ev.beat > beat {
			break
		}
		m.autoBop = ev.auto
	}
	for i := range m.intervals {
		if !m.intervals[i].processed && m.intervals[i].beat <= beat {
			m.setIntervalStart(i, beat)
		}
	}
	for i := range m.passes {
		if !m.passes[i].processed && m.passes[i].beat <= beat {
			m.passTurnStandalone(i)
		}
	}
	m.gateClose(beat)
}

func (m *Module) Whiff(beat float64) { m.shootIfAble(beat) }

func (m *Module) WhiffAction(beat float64, _ int) { m.shootIfAble(beat) }

func (m *Module) Update(_ float64, beat float64) {
	whole := int(math.Floor(beat))
	if whole > m.lastBeat {
		for b := m.lastBeat + 1; b <= whole; b++ {
			if m.autoBop {
				m.bop(float64(b))
			}
		}
		m.lastBeat = whole
	}
	if m.shipDamageUntil > 0 && beat >= m.shipDamageUntil {
		m.shipDamageUntil = 0
	}
}

func (m *Module) Draw(screen *ebiten.Image, _ float64, beat float64) {
	baseCam := m.ctx.SampleScene(beat)
	if w, ok := m.ctx.Scene.NodeWorld("DamageEffect/shootCam"); ok {
		m.ctx.Scene.SetCamera(baseCam[0]+w.Tx, baseCam[1]+w.Ty, baseCam[2])
	}
	aliveEnemies := m.enemies[:0]
	for _, e := range m.enemies {
		if e.queue(beat) {
			aliveEnemies = append(aliveEnemies, e)
		}
	}
	m.enemies = aliveEnemies
	aliveEffects := m.effects[:0]
	for _, fx := range m.effects {
		if fx.queue(m.ctx.Scene, beat) {
			aliveEffects = append(aliveEffects, fx)
		}
	}
	m.effects = aliveEffects
	m.drawParticles(beat)
	m.ctx.Scene.Draw(screen, m.proj)
}

func (m *Module) setIntervalStart(idx int, gameSwitchBeat float64) {
	if idx < 0 || idx >= len(m.intervals) || m.intervals[idx].processed {
		return
	}
	iv := &m.intervals[idx]
	iv.processed = true
	session := &intervalSession{start: iv.beat, length: iv.length}
	relevant := m.relevantSpawns(iv.beat, iv.beat+iv.length)
	for i := range relevant {
		ev := relevant[i]
		pos := ev.pos
		if iv.placement >= 0 && iv.placement < placementManual {
			pos = placementFor(iv.placement, len(relevant), i)
		}
		en := m.spawnEnemy(ev, pos, iv.length, ev.beat >= gameSwitchBeat)
		session.enemies = append(session.enemies, en)
	}
	m.sessions = append(m.sessions, session)
	if iv.autoPass {
		m.passTurn(iv.beat+iv.length, session)
	}
}

func (m *Module) relevantSpawns(start, end float64) []spawnEvt {
	var out []spawnEvt
	for _, ev := range m.spawns {
		if ev.beat < start {
			continue
		}
		if ev.beat >= end {
			break
		}
		out = append(out, ev)
	}
	return out
}

func (m *Module) spawnEnemy(ev spawnEvt, pos vec2, interval float64, active bool) *enemy {
	en := newEnemy(m, ev, pos, interval)
	m.enemies = append(m.enemies, en)
	if active {
		m.atOrNow(ev.beat, func() {
			m.ctx.Sound("spawn")
			en.activate(ev.beat, true)
			m.spawnTrajectory(en, ev.beat)
		})
	} else {
		en.activate(m.ctx.Beat(), false)
	}
	return en
}

func (m *Module) passTurnStandalone(idx int) {
	if idx < 0 || idx >= len(m.passes) || m.passes[idx].processed {
		return
	}
	m.passes[idx].processed = true
	if len(m.sessions) == 0 {
		return
	}
	m.passTurn(m.passes[idx].beat, m.sessions[len(m.sessions)-1])
}

func (m *Module) passTurn(passBeat float64, session *intervalSession) {
	if session == nil {
		return
	}
	m.atOrNow(passBeat-0.25, func() {
		for _, en := range session.enemies {
			rel := en.createBeat - session.start
			en.startInput(passBeat, rel)
		}
	})
}

func (m *Module) gateAnims(ev gateEvt) {
	if !ev.mute {
		m.ctx.Sound("gate1")
		m.ctx.SoundAt(ev.beat+ev.length, "gate2", 1)
		m.ctx.SoundAt(ev.beat+2*ev.length, "gate3", 1)
	}
	m.ctx.Scene.PlayState("IntroGate", "gateOpen1", ev.beat, animScale)
	m.ctx.At(ev.beat+ev.length, func() {
		m.ctx.Scene.PlayState("IntroGate", "gateOpen2", ev.beat+ev.length, animScale)
	})
	m.ctx.At(ev.beat+2*ev.length, func() {
		m.ctx.Scene.PlayState("IntroGate", "gateOpen3", ev.beat+2*ev.length, animScale)
	})
}

func (m *Module) gateClose(beat float64) {
	next := m.ctx.NextSwitchBeat(beat)
	for _, ev := range m.gates {
		if ev.beat >= beat && ev.beat <= next {
			m.ctx.Scene.PlayState("IntroGate", "gateShow", beat, animScale)
			return
		}
	}
}

func (m *Module) monitorAnims(ev monitorEvt) {
	m.canBop = false
	switch ev.typ {
	case monitorEnter:
		m.ctx.Scene.PlayState("Monitor/monitor/Captain", "capHidden", ev.beat, animScale)
		m.ctx.Scene.PlayState("Monitor", "monitorIn", ev.beat, animScale)
		m.ctx.At(ev.beat+ev.length, func() {
			m.ctx.Scene.PlayState("Monitor/monitor/Captain", "capShow", ev.beat+ev.length, animScale)
		})
		if !ev.mute {
			m.ctx.SoundAt(ev.beat+ev.length, "commStart", 1)
		}
	case monitorExit:
		m.ctx.Scene.PlayState("Monitor/monitor/Captain", "capHide", ev.beat, animScale)
		m.ctx.At(ev.beat+ev.length, func() {
			m.ctx.Scene.PlayState("Monitor", "monitorOut", ev.beat+ev.length, animScale)
		})
		if !ev.mute {
			m.ctx.Sound("commEnd")
		}
	case monitorTalk:
		m.ctx.Scene.SetBool("Monitor/monitor/Captain", "isTalk", true)
		m.ctx.Scene.PlayState("Monitor/monitor/Captain", "capTalk", ev.beat, animScale)
		m.ctx.At(ev.beat+ev.length, func() {
			m.ctx.Scene.SetBool("Monitor/monitor/Captain", "isTalk", false)
		})
	case monitorIdle:
		m.ctx.Scene.PlayState("Monitor", "monitorIdle", ev.beat, animScale)
		m.ctx.Scene.PlayState("Monitor/monitor/Captain", "capIdle", ev.beat, animScale)
	case monitorBop:
		m.canBop = true
		m.ctx.Scene.PlayState("Monitor/monitor/Captain", "capBop", ev.beat, animScale)
	}
}

func (m *Module) bop(beat float64) {
	if m.canBop {
		m.ctx.Scene.PlayState("Monitor/monitor/Captain", "capBop", beat, animScale)
	}
}

func (m *Module) shootIfAble(beat float64) {
	if m.shipDamageUntil > beat {
		return
	}
	m.ctx.Sound("16")
	m.shoot(beat)
}

func (m *Module) shoot(beat float64) {
	m.ctx.Scene.PlayState("ship", "shipShoot", beat, animScale)
	m.ctx.Scene.PlayState("laser", "laser", beat, animScale)
}

func (m *Module) damageShip(beat float64) {
	m.ctx.Sound("15")
	m.shipDamageUntil = beat + animDuration(m.ctx, "Animations/shipDamage")/animScale
	m.ctx.Scene.PlayState("ship", "shipDamage", beat, animScale)
	m.ctx.Scene.PlayState("DamageEffect", "damage", beat, animScale)
}

func (m *Module) resetScene(beat float64) {
	sec := m.ctx.SecPerBeat(beat)
	for _, root := range []string{"ship", "laser", "DamageEffect", "IntroGate", "Monitor", "Monitor/monitor/Captain"} {
		m.ctx.Scene.PlayDefaultState(root, beat, sec)
	}
	m.canBop = false
	m.autoBop = false
}

func (m *Module) atOrNow(beat float64, fn func()) {
	if beat <= m.ctx.Beat()+1e-6 {
		fn()
		return
	}
	m.ctx.At(beat, fn)
}

func boolParam(e *riq.Entity, key string) bool {
	if v, ok := e.Data[key]; ok {
		if b, ok := v.(bool); ok {
			return b
		}
	}
	return e.Float(key, 0) != 0
}

func boolDefault(e *riq.Entity, key string, def bool) bool {
	if _, ok := e.Data[key]; !ok {
		return def
	}
	return boolParam(e, key)
}

func colorParam(e *riq.Entity, key string, def [4]float64) [4]float64 {
	v, ok := e.Data[key]
	if !ok {
		return def
	}
	mm, ok := v.(map[string]any)
	if !ok {
		return def
	}
	out := def
	for i, k := range []string{"r", "g", "b", "a"} {
		if f, ok := mm[k].(float64); ok {
			out[i] = f
		}
	}
	return out
}

func animDuration(ctx *engine.Ctx, clip string) float64 {
	if a := ctx.Assets.Anims[clip]; a != nil {
		return a.Duration
	}
	return 0.25
}
