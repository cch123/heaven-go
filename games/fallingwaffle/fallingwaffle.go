// Package fallingwaffle ports Falling Waffle's flop/reset/countdown flow,
// single hit window, and splat/tink/miss/count sound timing.
package fallingwaffle

import (
	"github.com/hajimehoshi/ebiten/v2"

	"hsdemo/engine"
	"hsdemo/kart"
	"hsdemo/riq"
)

type Module struct {
	ctx  *engine.Ctx
	proj kart.Aff

	waffle string
	square string

	hasFallen bool
}

func New() engine.Module { return &Module{} }

func (m *Module) ID() string { return "fallingWaffle" }

func (m *Module) Load(ctx *engine.Ctx) error {
	m.ctx = ctx
	if err := ctx.LoadAssets("fallingWaffle"); err != nil {
		return err
	}
	m.proj = kart.Translate(engine.ScreenW/2, engine.ScreenH/2).Mul(kart.Scale(54, -54))
	m.waffle = roleOr(ctx, "waffleAnim", "WaffleHolder")
	m.square = roleOr(ctx, "squareAnim", "Square")
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
	case "fallingWaffle/splat":
		b := e.Beat
		m.ctx.At(b, func() { m.flop(b) })
	case "fallingWaffle/unfall":
		b := e.Beat
		m.ctx.At(b, func() { m.forceStand(b) })
	case "fallingWaffle/fall":
		b := e.Beat
		m.ctx.At(b, func() { m.forceFall(b) })
	case "fallingWaffle/countdown":
		m.scheduleCountdown(e)
	case "fallingWaffle/count":
		m.scheduleCount(e.Beat, int(e.Float("numbr", 1)))
	}
}

func (m *Module) Ready() {}

func (m *Module) OnSwitch(beat float64) {
	m.reset(beat)
}

func (m *Module) Whiff(float64) {}

func (m *Module) Update(float64, float64) {}

func (m *Module) Draw(screen *ebiten.Image, _ float64, beat float64) {
	m.ctx.SampleScene(beat)
	m.ctx.Scene.Draw(screen, m.proj)
}

func (m *Module) reset(beat float64) {
	m.hasFallen = false
	sec := m.ctx.SecPerBeat(beat)
	m.ctx.Scene.PlayDefaultState(m.waffle, beat, sec)
	m.ctx.Scene.PlayDefaultState(m.square, beat, sec)
}

func (m *Module) flop(beat float64) {
	if m.hasFallen {
		return
	}
	m.hasFallen = true
	m.ctx.At(beat+0.5, func() { m.ctx.Scene.PlayState(m.waffle, "fall", beat+0.5, 0.5) })
	m.ctx.SoundAt(beat+1, "wafflesplat", 1)
	m.ctx.ScheduleInput(beat+1,
		func(state float64, _ engine.Judgment) {
			m.hasFallen = true
			if state >= 1 || state <= -1 {
				m.ctx.Sound("tink")
			}
		},
		func() {
			m.hasFallen = true
			m.ctx.Sound("miss")
		})
}

func (m *Module) forceFall(beat float64) {
	m.hasFallen = true
	m.ctx.Scene.PlayState(m.waffle, "IdleFlop", beat, 0.5)
}

func (m *Module) forceStand(beat float64) {
	m.hasFallen = false
	m.ctx.Scene.PlayState(m.square, "Fade", beat, 0.5)
	m.ctx.At(beat+1, func() { m.ctx.Scene.PlayState(m.waffle, "Idle", beat+1, 0.5) })
}

func (m *Module) scheduleCountdown(e *riq.Entity) {
	beat := e.Beat
	if !boolParam(e, "mute1") {
		m.ctx.SoundAtOff(beat, "one", 1, 0.028)
	}
	if !boolParam(e, "mute2") {
		m.ctx.SoundAtOff(beat+2, "two", 1, 0.064)
	}
	if !boolParam(e, "mute3") {
		m.ctx.SoundAtOff(beat+4, "three", 1, 0.082)
	}
	if !boolParam(e, "mute4") {
		m.ctx.SoundAt(beat+6, "four", 1)
	}
}

func (m *Module) scheduleCount(beat float64, n int) {
	switch n {
	case 1:
		m.ctx.SoundAtOff(beat, "one", 1, 0.028)
	case 2:
		m.ctx.SoundAtOff(beat, "two", 1, 0.064)
	case 3:
		m.ctx.SoundAtOff(beat, "three", 1, 0.082)
	case 4:
		m.ctx.SoundAt(beat, "four", 1)
	}
}

func boolParam(e *riq.Entity, key string) bool { return e.Float(key, 0) != 0 }
