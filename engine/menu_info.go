package engine

import (
	"fmt"
	"image/color"
	"math"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

func (a *App) drawLibraryLevelInfo(screen *ebiten.Image, level menuLevel, idx int, ink, soft color.RGBA) {
	x, y := 585.0, 116.0
	w, h := 326.0, 344.0
	vector.DrawFilledRect(screen, float32(x), float32(y), float32(w), float32(h), color.RGBA{255, 252, 242, 236}, false)
	vector.DrawFilledRect(screen, float32(x), float32(y), float32(w), 5, color.RGBA{118, 88, 148, 230}, false)
	a.drawLevelThumbnail(screen, level, idx, x+24, y+32, 112)
	a.text(screen, a.fitText(level.displayName(), a.faceMid, 172), a.faceMid, x+154, y+38, ink, false)
	author := level.author
	if author == "" {
		author = "Unknown author"
	}
	a.text(screen, a.fitText(author, a.faceSmall, 150), a.faceSmall, x+156, y+78, soft, false)
	a.text(screen, fmt.Sprintf("%.0f BPM", level.bpm), a.faceSmall, x+156, y+104, color.RGBA{118, 88, 148, 255}, false)

	baseY := y + 176
	games := level.games
	if len(games) == 0 {
		games = []string{"Unknown game"}
	}
	a.text(screen, "Games", a.faceSmall, x+24, baseY, soft, false)
	a.drawGameList(screen, games, x+24, baseY+26, w-48)
	desc := strings.TrimSpace(level.desc)
	if desc != "" {
		a.text(screen, "Description", a.faceSmall, x+24, y+276, soft, false)
		a.drawWrappedTextLimit(screen, desc, a.faceSmall, x+24, y+302, w-48, 20, 2, ink)
	} else {
		a.text(screen, a.fitText(level.path, a.faceSmall, w-48), a.faceSmall, x+24, y+h-46, soft, false)
	}
}

func (a *App) drawGameList(screen *ebiten.Image, games []string, x, y, maxW float64) {
	cx := x
	for i, game := range games {
		label := a.fitText(menuGameLabel(game), a.faceSmall, 100)
		tw, _ := text.Measure(label, a.faceSmall, 0)
		pw := math.Min(tw+22, 120)
		if cx+pw > x+maxW {
			break
		}
		vector.DrawFilledRect(screen, float32(cx), float32(y), float32(pw), 24, menuAccent(i), false)
		a.text(screen, label, a.faceSmall, cx+11, y+6, color.RGBA{255, 252, 242, 255}, false)
		cx += pw + 8
	}
}

func (a *App) drawWrappedTextLimit(screen *ebiten.Image, s string, face *text.GoTextFace, x, y, maxW, lineH float64, maxLines int, c color.Color) {
	words := strings.Fields(s)
	if len(words) == 0 || maxLines <= 0 {
		return
	}
	line := words[0]
	lines := 0
	for _, word := range words[1:] {
		next := line + " " + word
		if w, _ := text.Measure(next, face, 0); w <= maxW {
			line = next
			continue
		}
		lines++
		if lines >= maxLines {
			a.text(screen, a.fitText(line+"...", face, maxW), face, x, y, c, false)
			return
		}
		a.text(screen, line, face, x, y, c, false)
		y += lineH
		line = word
	}
	a.text(screen, line, face, x, y, c, false)
}
