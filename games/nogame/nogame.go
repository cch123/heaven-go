// Package nogame ports Heaven Studio's No Game loader.
//
// Unity's NoGame prefab is intentionally simple: a dark grey full-screen card
// with centered "No Game" text and no gameplay actions. Registering it
// explicitly prevents charts that switch through noGame from being reported as
// unported while preserving the original inert behavior.
package nogame

import (
	"bytes"
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
	"golang.org/x/image/font/gofont/goregular"

	"hsdemo/engine"
	"hsdemo/riq"
)

type Module struct {
	face *text.GoTextFace
}

func New() engine.Module { return &Module{} }

func (m *Module) ID() string { return "noGame" }
func (m *Module) Load(*engine.Ctx) error {
	src, err := text.NewGoTextFaceSource(bytes.NewReader(goregular.TTF))
	if err != nil {
		return err
	}
	m.face = &text.GoTextFace{Source: src, Size: 64}
	return nil
}
func (m *Module) OnEvent(*riq.Entity)     {}
func (m *Module) Ready()                  {}
func (m *Module) OnSwitch(float64)        {}
func (m *Module) Whiff(float64)           {}
func (m *Module) Update(float64, float64) {}
func (m *Module) Draw(screen *ebiten.Image, _, _ float64) {
	screen.Fill(color.RGBA{0x27, 0x27, 0x27, 0xff})
	if m.face == nil {
		return
	}
	const label = "No Game"
	w, h := text.Measure(label, m.face, 0)
	op := &text.DrawOptions{}
	op.GeoM.Translate((engine.ScreenW-w)/2, (engine.ScreenH-h)/2)
	op.ColorScale.ScaleWithColor(color.White)
	text.Draw(screen, label, m.face, op)
}
