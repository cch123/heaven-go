package engine

import (
	"image"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

func (t *textboxFX) ensure(assetsRoot string) bool {
	if t.font != nil {
		return true
	}
	fraw, err := os.ReadFile(filepath.Join(assetsRoot, "common", "textbox_font.otf"))
	if err != nil {
		return false
	}
	f, err := opentype.Parse(fraw)
	if err != nil {
		return false
	}
	t.font = f
	t.cache = map[string]*ebiten.Image{}
	return true
}

func (t *textboxFX) renderText(s, align string, maxW, maxH float64) *ebiten.Image {
	key := s + "\x00" + align + "\x00" + fmtSize(maxW) + "\x00" + fmtSize(maxH)
	if img, ok := t.cache[key]; ok {
		return img
	}
	face, err := opentype.NewFace(t.font, &opentype.FaceOptions{Size: 28, DPI: 72})
	if err != nil {
		return nil
	}
	defer face.Close()
	met := face.Metrics()
	lineH := (met.Ascent + met.Descent).Ceil()
	lines := wrapTextboxLines(face, s, int(maxW)-36)
	w, h := int(maxW), int(maxH)
	if w < 1 || h < 1 {
		return nil
	}
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	totalH := lineH * len(lines)
	y := (h-totalH)/2 + met.Ascent.Ceil()
	for _, line := range lines {
		adv := font.MeasureString(face, line).Ceil()
		x := 18
		switch align {
		case "right":
			x = w - 18 - adv
		case "center":
			x = (w - adv) / 2
		}
		d := &font.Drawer{Dst: img, Src: image.Black, Face: face, Dot: fixed.P(x, y)}
		d.DrawString(line)
		y += lineH
	}
	e := ebiten.NewImageFromImage(img)
	t.cache[key] = e
	return e
}

func fmtSize(v float64) string { return strconv.Itoa(int(math.Round(v))) }

func wrapTextboxLines(face font.Face, s string, maxW int) []string {
	words := strings.Fields(s)
	if len(words) == 0 {
		return []string{""}
	}
	lines := make([]string, 0, 2)
	line := words[0]
	for _, word := range words[1:] {
		next := line + " " + word
		if font.MeasureString(face, next).Ceil() <= maxW {
			line = next
			continue
		}
		lines = append(lines, line)
		line = word
	}
	lines = append(lines, line)
	return lines
}
