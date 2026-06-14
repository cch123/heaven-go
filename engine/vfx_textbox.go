// vfx_textbox.go：vfx/display textbox。
//
// 事件期间显示原版 TextboxPrefab 风格的文本框（原版尺寸圆角面板 + OTF 排版），
// 按 anchor 摆位。
package engine

import (
	"regexp"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"golang.org/x/image/font/opentype"

	"hsdemo/riq"
)

type textboxEvt struct {
	beat, length float64
	text         string
	align        string
	anchor       int
	w, h         float64
	x, y         float64
}

type textboxFX struct {
	evts  []textboxEvt
	font  *opentype.Font
	cache map[string]*ebiten.Image
}

var richTagRe = regexp.MustCompile(`<[^>]*>`)
var alignTagRe = regexp.MustCompile(`(?i)<align\s*=\s*"?([a-z]+)"?>`)

func (t *textboxFX) add(e *riq.Entity) {
	rawText := e.Str("text1", "")
	t.evts = append(t.evts, textboxEvt{
		beat: e.Beat, length: e.Length,
		text:  strings.TrimSpace(richTagRe.ReplaceAllString(rawText, "")),
		align: textboxAlign(rawText), anchor: int(e.Float("type", 1)),
		w: e.Float("valA", 1), h: e.Float("valB", 1),
		x: e.Float("x", 0), y: e.Float("y", 0),
	})
}

func (t *textboxFX) reset() { t.evts = nil }

func textboxAlign(s string) string {
	if m := alignTagRe.FindStringSubmatch(s); len(m) == 2 {
		switch strings.ToLower(m[1]) {
		case "left", "right", "center":
			return strings.ToLower(m[1])
		}
	}
	return "center"
}

// Draw 绘制当前活动的 textbox（事件区间内显示）。
func (t *textboxFX) Draw(dst *ebiten.Image, assetsRoot string, beat float64) {
	if len(t.evts) == 0 || !t.ensure(assetsRoot) {
		return
	}
	for _, e := range t.evts {
		if beat < e.beat || beat >= e.beat+e.length {
			continue
		}
		px, py := anchorPos(e.anchor, e.x, e.y)
		// TextboxPrefab 的四个 sliced SDF SpriteRenderer 合成约 12×3 world-unit
		// 的白底黑边圆角框；这里按同一几何尺寸直接绘制圆角面板。
		bw, bh := 6*e.w*54*2, 1.5*e.h*54*2
		drawTextboxPanel(dst, px-bw/2, py-bh/2, bw, bh)
		if e.text != "" {
			txt := t.renderText(e.text, e.align, 11.2*e.w*54, 2.2*e.h*54)
			if txt != nil {
				to := &ebiten.DrawImageOptions{Filter: ebiten.FilterLinear}
				tb := txt.Bounds()
				to.GeoM.Translate(px-float64(tb.Dx())/2, py-float64(tb.Dy())/2)
				dst.DrawImage(txt, to)
			}
		}
	}
}
