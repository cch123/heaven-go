// Package nogame ports Heaven Studio's No Game loader.
//
// Unity's NoGame prefab is intentionally simple: a dark grey full-screen card
// with centered "No Game" text and no gameplay actions. Registering it
// explicitly prevents charts that switch through noGame from being reported as
// unported while preserving the original inert behavior.
package nogame

import (
	"github.com/hajimehoshi/ebiten/v2"

	"hsdemo/engine"
	"hsdemo/kart"
	"hsdemo/riq"
)

type Module struct {
	ctx  *engine.Ctx
	proj kart.Aff
}

func New() engine.Module { return &Module{} }

func (m *Module) ID() string { return "noGame" }
func (m *Module) Load(ctx *engine.Ctx) error {
	if err := ctx.LoadAssets("noGame"); err != nil {
		return err
	}
	if err := ctx.Assets.ApplyTexts(); err != nil {
		return err
	}
	m.ctx = ctx
	m.proj = kart.Translate(engine.ScreenW/2, engine.ScreenH/2).Mul(kart.Scale(54, -54))
	return nil
}
func (m *Module) OnEvent(*riq.Entity)     {}
func (m *Module) Ready()                  {}
func (m *Module) OnSwitch(float64)        {}
func (m *Module) Whiff(float64)           {}
func (m *Module) Update(float64, float64) {}
func (m *Module) Draw(screen *ebiten.Image, _, beat float64) {
	if m.ctx == nil || m.ctx.Scene == nil {
		return
	}
	m.ctx.SampleScene(beat)
	m.ctx.Scene.Draw(screen, m.proj)
}
