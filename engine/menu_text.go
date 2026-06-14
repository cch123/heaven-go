package engine

import (
	"image/color"
	"strings"

	"github.com/hajimehoshi/ebiten/v2/text/v2"
)

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
