// Package nogame ports Heaven Studio's No Game loader.
//
// Unity's NoGame minigame has no prefab, actions, or runtime logic. Registering
// it explicitly prevents charts that switch through noGame from being reported
// as unported while preserving the original inert behavior.
package nogame

import (
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"

	"hsdemo/engine"
	"hsdemo/riq"
)

type Module struct{}

func New() engine.Module { return &Module{} }

func (m *Module) ID() string              { return "noGame" }
func (m *Module) Load(*engine.Ctx) error  { return nil }
func (m *Module) OnEvent(*riq.Entity)     {}
func (m *Module) Ready()                  {}
func (m *Module) OnSwitch(float64)        {}
func (m *Module) Whiff(float64)           {}
func (m *Module) Update(float64, float64) {}
func (m *Module) Draw(screen *ebiten.Image, _, _ float64) {
	screen.Fill(color.RGBA{0x27, 0x27, 0x27, 0xff})
}
