package engine

import (
	"image"
	"image/color"
	"math"
	"strconv"

	"github.com/hajimehoshi/ebiten/v2"
)

// anchorPos 返回原版 VFXObject.TextboxAnchor 坐标：XAnchor=3, YAnchor=3.5；
// Custom 用 x/y，单位与游戏世界一致 54px/unit。
func anchorPos(anchor int, x, y float64) (float64, float64) {
	cx, cy := float64(ScreenW/2), float64(ScreenH/2)
	xAnchor, yAnchor := 3*54.0, 3.5*54.0
	top, mid, bot := cy-yAnchor, cy, cy+yAnchor
	lft, ctr, rgt := cx-xAnchor, cx, cx+xAnchor
	switch anchor {
	case 0:
		return lft, top
	case 1:
		return ctr, top
	case 2:
		return rgt, top
	case 3:
		return lft, mid
	case 4:
		return ctr, mid
	case 5:
		return rgt, mid
	case 6:
		return lft, bot
	case 7:
		return ctr, bot
	case 8:
		return rgt, bot
	default: // Custom
		return cx + x*54, cy - y*54
	}
}

func (t *textboxFX) renderPanel(w, h float64) *ebiten.Image {
	if t.panelSDF == nil || w <= 0 || h <= 0 {
		return nil
	}
	pw, ph := int(math.Round(w)), int(math.Round(h))
	if pw <= 0 || ph <= 0 {
		return nil
	}
	key := strconv.Itoa(pw) + "x" + strconv.Itoa(ph)
	if img, ok := t.panelCache[key]; ok {
		return img
	}
	rgba := t.renderPanelRGBA(pw, ph)
	img := ebiten.NewImageFromImage(rgba)
	t.panelCache[key] = img
	return img
}

func (t *textboxFX) renderPanelRGBA(pw, ph int) *image.RGBA {
	rgba := image.NewRGBA(image.Rect(0, 0, pw, ph))
	drawTextboxQuadrant(rgba, t.panelSDF, 0, 0, pw/2, ph/2, false, false)
	drawTextboxQuadrant(rgba, t.panelSDF, pw/2, 0, pw-pw/2, ph/2, true, false)
	drawTextboxQuadrant(rgba, t.panelSDF, 0, ph/2, pw/2, ph-ph/2, false, true)
	drawTextboxQuadrant(rgba, t.panelSDF, pw/2, ph/2, pw-pw/2, ph-ph/2, true, true)
	return rgba
}

func drawTextboxQuadrant(dst *image.RGBA, src image.Image, ox, oy, w, h int, flipX, flipY bool) {
	if w <= 0 || h <= 0 {
		return
	}
	sb := src.Bounds()
	sw, sh := sb.Dx(), sb.Dy()
	// TextboxPrefab uses one sliced SpriteRenderer per quadrant. The source
	// sprite has border left/top = 57px and right/bottom = 0px, so the 57px
	// corner stays stable while the 7px inner strip stretches toward the center.
	const border = 57
	for y := 0; y < h; y++ {
		ty := y
		if flipY {
			ty = h - 1 - y
		}
		sy := textboxSlicedCoord(ty, h, sh, border)
		for x := 0; x < w; x++ {
			tx := x
			if flipX {
				tx = w - 1 - x
			}
			sx := textboxSlicedCoord(tx, w, sw, border)
			dst.SetRGBA(ox+x, oy+y, textboxSDFColor(src.At(sb.Min.X+sx, sb.Min.Y+sy)))
		}
	}
}

func textboxSlicedCoord(pos, size, srcSize, border int) int {
	if size <= 1 || srcSize <= 1 {
		return 0
	}
	if border > srcSize-1 {
		border = srcSize - 1
	}
	keep := border
	if keep > size {
		keep = size
	}
	if pos < keep {
		return clampInt(pos, 0, srcSize-1)
	}
	innerDst := size - keep
	innerSrc := srcSize - border
	if innerDst <= 0 || innerSrc <= 0 {
		return clampInt(border, 0, srcSize-1)
	}
	u := float64(pos-keep) / float64(innerDst)
	return clampInt(border+int(math.Round(u*float64(innerSrc-1))), 0, srcSize-1)
}

func textboxSDFColor(c color.Color) color.RGBA {
	r, _, _, _ := c.RGBA()
	a := float64(r) / 65535.0
	outline := smoothstep(0.85-0.025, 0.85+0.025, a)
	alpha := smoothstep(1.0-0.45-0.025, 1.0-0.45+0.025, a)
	v := uint8(math.Round(outline * 255))
	return color.RGBA{R: v, G: v, B: v, A: uint8(math.Round(alpha * 255))}
}

func smoothstep(edge0, edge1, x float64) float64 {
	if edge0 == edge1 {
		if x < edge0 {
			return 0
		}
		return 1
	}
	t := (x - edge0) / (edge1 - edge0)
	if t < 0 {
		t = 0
	} else if t > 1 {
		t = 1
	}
	return t * t * (3 - 2*t)
}

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
