package engine

import (
	"fmt"
	"image/color"
	"math"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
)

func drawImageFit(dst, src *ebiten.Image, x, y, w, h float64, alpha float32) {
	if src == nil {
		return
	}
	sw, sh := src.Bounds().Dx(), src.Bounds().Dy()
	if sw == 0 || sh == 0 {
		return
	}
	s := math.Min(w/float64(sw), h/float64(sh))
	dw, dh := float64(sw)*s, float64(sh)*s
	op := &ebiten.DrawImageOptions{Filter: ebiten.FilterLinear}
	op.GeoM.Scale(s, s)
	op.GeoM.Translate(x+(w-dw)/2, y+(h-dh)/2)
	op.ColorScale.ScaleAlpha(alpha)
	dst.DrawImage(src, op)
}

func drawImageCover(dst, src *ebiten.Image, x, y, w, h float64, alpha float32) {
	if src == nil {
		return
	}
	sw, sh := src.Bounds().Dx(), src.Bounds().Dy()
	if sw == 0 || sh == 0 {
		return
	}
	s := math.Max(w/float64(sw), h/float64(sh))
	dw, dh := float64(sw)*s, float64(sh)*s
	op := &ebiten.DrawImageOptions{Filter: ebiten.FilterLinear}
	op.GeoM.Scale(s, s)
	op.GeoM.Translate(x+(w-dw)/2, y+(h-dh)/2)
	op.ColorScale.ScaleAlpha(alpha)
	dst.DrawImage(src, op)
}

func (a *App) drawWrappedText(screen *ebiten.Image, s string, face *text.GoTextFace, x, y, maxW, lineH float64, c color.Color) {
	words := strings.Fields(s)
	if len(words) == 0 {
		return
	}
	line := words[0]
	for _, word := range words[1:] {
		next := line + " " + word
		if w, _ := text.Measure(next, face, 0); w <= maxW {
			line = next
			continue
		}
		a.text(screen, line, face, x, y, c, false)
		y += lineH
		line = word
	}
	a.text(screen, line, face, x, y, c, false)
}

func (a *App) drawPlaceholder(screen *ebiten.Image, id string) {
	screen.Fill(color.RGBA{40, 40, 52, 255})
	a.text(screen, id, a.faceBig, ScreenW/2, ScreenH/2-40, color.RGBA{210, 210, 225, 255}, true)
	a.text(screen, "This minigame is not ported yet; the song continues.", a.faceMid, ScreenW/2, ScreenH/2+20, color.RGBA{160, 160, 175, 255}, true)
}

func (a *App) drawDebug(screen *ebiten.Image, t, beat float64) {
	white := color.RGBA{235, 235, 245, 255}
	lines := []string{
		fmt.Sprintf("songPos %8.3fs  beat %7.3f", t, beat),
		fmt.Sprintf("tps %.0f fps %.0f", ebiten.ActualTPS(), ebiten.ActualFPS()),
	}
	if a.cond != nil {
		lines = append(lines, fmt.Sprintf("drift %+6.1fms", a.cond.Drift()*1000))
	}
	n := 0
	for _, in := range a.inputs {
		if !in.judged {
			n++
		}
	}
	lines = append(lines, fmt.Sprintf("actions %d/%d  inputs left %d", a.actIdx, len(a.actions), n))
	for i, s := range lines {
		a.text(screen, s, a.faceSmall, 20, 40+float64(i)*18, white, false)
	}
}

func (a *App) text(screen *ebiten.Image, s string, face *text.GoTextFace, x, y float64, c color.Color, center bool) {
	if center {
		w, _ := text.Measure(s, face, 0)
		x -= w / 2
	}
	op := &text.DrawOptions{}
	op.GeoM.Translate(x, y)
	op.ColorScale.ScaleWithColor(c)
	text.Draw(screen, s, face, op)
}

func (a *App) Layout(_, _ int) (int, int) { return ScreenW, ScreenH }
