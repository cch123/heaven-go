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

func (a *App) drawLevelCard(screen *ebiten.Image, level menuLevel, idx int, x, y float64, selected bool) {
	if selected {
		vector.DrawFilledRect(screen, float32(x-5), float32(y-5), menuCardW+10, menuCardH+10, color.RGBA{255, 227, 95, 235}, false)
	}
	vector.DrawFilledRect(screen, float32(x), float32(y), menuCardW, menuCardH, color.RGBA{255, 252, 242, 236}, false)
	vector.DrawFilledRect(screen, float32(x), float32(y+menuCardH-37), menuCardW, 37, color.RGBA{246, 239, 229, 245}, false)
	if selected {
		vector.DrawFilledRect(screen, float32(x), float32(y+menuCardH-4), menuCardW, 4, color.RGBA{118, 88, 148, 255}, false)
	}
	a.drawLevelThumbnail(screen, level, idx, x+11, y+9, 126)
	title := a.fitText(level.displayName(), a.faceSmall, menuCardW-20)
	a.text(screen, title, a.faceSmall, x+10, y+menuCardH-29, color.RGBA{68, 54, 82, 255}, false)
	meta := "RIQ"
	if len(level.games) > 0 {
		meta = fmt.Sprintf("%d games", len(level.games))
	}
	a.text(screen, meta, a.faceSmall, x+10, y+menuCardH-12, color.RGBA{120, 106, 133, 255}, false)
}

func (a *App) drawLevelThumbnail(screen *ebiten.Image, level menuLevel, idx int, x, y, size float64) {
	inner := size * 0.78
	innerX := x + (size-inner)/2
	innerY := y + (size-inner)/2
	vector.DrawFilledRect(screen, float32(innerX), float32(innerY), float32(inner), float32(inner), color.RGBA{240, 232, 220, 255}, false)
	if level.customIcon != nil {
		drawImageFit(screen, level.customIcon, innerX, innerY, inner, inner, 1)
	} else {
		a.drawFallbackLevelIcon(screen, level, idx, innerX, innerY, inner)
	}
	if a.libraryAssets.border != nil {
		drawImageFit(screen, a.libraryAssets.border, x, y, size, size, 1)
	} else {
		vector.DrawFilledRect(screen, float32(x), float32(y), float32(size), 5, color.RGBA{116, 94, 128, 255}, false)
		vector.DrawFilledRect(screen, float32(x), float32(y+size-5), float32(size), 5, color.RGBA{116, 94, 128, 255}, false)
		vector.DrawFilledRect(screen, float32(x), float32(y), 5, float32(size), color.RGBA{116, 94, 128, 255}, false)
		vector.DrawFilledRect(screen, float32(x+size-5), float32(y), 5, float32(size), color.RGBA{116, 94, 128, 255}, false)
	}
}

func (a *App) drawFallbackLevelIcon(screen *ebiten.Image, level menuLevel, idx int, x, y, size float64) {
	games := level.games
	if len(games) == 0 {
		games = []string{"RIQ"}
	}
	if len(games) == 1 {
		vector.DrawFilledRect(screen, float32(x), float32(y), float32(size), float32(size), menuAccent(idx), false)
		a.text(screen, a.fitText(menuGameLabel(games[0]), a.faceMid, size-18), a.faceMid, x+size/2, y+size/2-12, color.RGBA{255, 252, 242, 255}, true)
		return
	}
	tile := (size - 5) / 2
	for i := 0; i < 4; i++ {
		tx := x + float64(i%2)*(tile+5)
		ty := y + float64(i/2)*(tile+5)
		c := menuAccent(idx + i)
		vector.DrawFilledRect(screen, float32(tx), float32(ty), float32(tile), float32(tile), c, false)
		if i < len(games) {
			label := a.fitText(menuGameLabel(games[i]), a.faceSmall, tile-8)
			a.text(screen, label, a.faceSmall, tx+tile/2, ty+tile/2-8, color.RGBA{255, 252, 242, 255}, true)
		}
	}
}

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

func menuGameLabel(game string) string {
	names := map[string]string{
		"blueBear":       "Blue Bear",
		"marchingOrders": "Marching Orders",
		"munchyMonk":     "Munchy Monk",
		"seeSaw":         "See-Saw",
		"somen":          "Somen",
		"totemClimb":     "Totem Climb",
		"trickClass":     "Trick Class",
	}
	if name, ok := names[game]; ok {
		return name
	}
	return humanizeGameID(game)
}

func humanizeGameID(id string) string {
	if id == "" {
		return "Unknown"
	}
	var b strings.Builder
	for i, r := range id {
		if i > 0 && r >= 'A' && r <= 'Z' {
			b.WriteByte(' ')
		}
		if i == 0 && r >= 'a' && r <= 'z' {
			r -= 'a' - 'A'
		}
		b.WriteRune(r)
	}
	return b.String()
}

func menuAccent(i int) color.RGBA {
	palette := []color.RGBA{
		{232, 184, 74, 255},
		{83, 189, 179, 255},
		{235, 111, 94, 255},
		{147, 194, 86, 255},
		{157, 139, 214, 255},
		{76, 151, 218, 255},
	}
	return palette[i%len(palette)]
}

func (a *App) fitText(s string, face *text.GoTextFace, maxW float64) string {
	if w, _ := text.Measure(s, face, 0); w <= maxW {
		return s
	}
	rs := []rune(s)
	for len(rs) > 0 {
		rs = rs[:len(rs)-1]
		candidate := string(rs) + "..."
		if w, _ := text.Measure(candidate, face, 0); w <= maxW {
			return candidate
		}
	}
	return "..."
}
