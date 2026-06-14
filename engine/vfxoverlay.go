// vfxoverlay.go：vfx/filter（AmplifyColor LUT 滤镜）与 vfx/display textbox。
//
// filter：10 个 slot，事件按拍序持久覆盖（Filter.cs：beat 起永久生效，
// 直到后续事件改写同 slot）；BlendAmount = ease(1-start, 1-end)（AmplifyColor
// 语义 0=全滤镜、1=无效果），本地混合强度为其反相。LUT 为 1024×32 的 32³ 条带。
//
// textbox：事件期间显示原版 TextboxPrefab 风格的文本框（原版尺寸圆角面板
// + OTF 排版），按 anchor 摆位。
package engine

import (
	"image"
	"image/color"
	"log"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"

	"hsdemo/riq"
)

// ---------- vfx/filter ----------

type filterEvt struct {
	beat, length float64
	filter       int
	slot         int
	start, end   float64
	ease         int
}

// filterNames 对应 Filter.FilterType 枚举顺序（按枚举值索引文件名）。
var filterNames = []string{
	"accent", "air", "atri", "bleach", "bleak", "blockbuster", "cinecold", "cinewarm",
	"colorshift", "dawn", "deepfry", "deuteranopia", "exposed", "friend", "friend_diffusion",
	"gamebob", "gamebob_2", "gameboy", "gameboy_color", "glare", "grayscale",
	"grayscale_invert", "invert", "iso_blue", "iso_cyan", "iso_green", "iso_highlights",
	"iso_magenta", "iso_mid", "iso_red", "iso_shadows", "iso_yellow", "maritime",
	"moonlight", "nightfall", "polar", "poster", "protanopia", "redder", "sanic",
	"sepia", "sepier", "sepiest", "shareware", "shift_behind", "shift_left", "shift_right",
	"tina", "tiny_palette", "toxic", "tritanopia", "vibrance", "winter", "blackwhite",
	"blackwhite_2",
}

type filterFX struct {
	evts   []filterEvt
	luts   map[string]*ebiten.Image // 已垫到 padW×padH 的 LUT
	shader *ebiten.Shader
	work   *ebiten.Image // padW×padH（DrawRectShader 要求各源图同尺寸）
}

// LUT 条带 1024×32 与屏幕 960×540 尺寸不同，统一垫到能容纳两者的画布。
const (
	fxPadW = 1024
	fxPadH = 544
)

const lutKage = `//kage:unit pixels
package main

var Blend float

func Fragment(dst vec4, src vec2, color vec4) vec4 {
	c := imageSrc0At(src).rgb
	// 32³ LUT 条带（1024×32）：x = b 切片*32 + r*31，y = g*31。
	// 像素模式下各 imageSrcNAt 共用 src0 的坐标系（同尺寸源按 src0
	// origin 对齐），LUT 局部坐标须加 imageSrc0Origin()。
	o := imageSrc0Origin()
	b := clamp(c.b, 0.0, 1.0) * 31.0
	bLo := floor(b)
	fr := b - bLo
	lo := imageSrc1At(o + vec2(bLo*32.0+clamp(c.r, 0.0, 1.0)*31.0+0.5, clamp(c.g, 0.0, 1.0)*31.0+0.5)).rgb
	hi := imageSrc1At(o + vec2(min(bLo+1.0, 31.0)*32.0+clamp(c.r, 0.0, 1.0)*31.0+0.5, clamp(c.g, 0.0, 1.0)*31.0+0.5)).rgb
	graded := mix(lo, hi, fr)
	return vec4(mix(c, graded, Blend), 1)
}
`

func (f *filterFX) add(e *riq.Entity) {
	f.evts = append(f.evts, filterEvt{
		beat: e.Beat, length: e.Length,
		filter: int(e.Float("filter", 0)), slot: int(e.Float("slot", 1)),
		start: e.Float("start", 0), end: e.Float("end", 0),
		ease: int(e.Float("ease", 0)),
	})
}

func (f *filterFX) reset() { f.evts = nil }

func (f *filterFX) lut(assetsRoot, name string) *ebiten.Image {
	if f.luts == nil {
		f.luts = map[string]*ebiten.Image{}
	}
	if img, ok := f.luts[name]; ok {
		return img
	}
	raw, err := os.ReadFile(filepath.Join(assetsRoot, "common", "filters", name+".png"))
	if err != nil {
		log.Printf("engine: filter LUT %s 缺失（运行 extract -game common）", name)
		f.luts[name] = nil
		return nil
	}
	img, _, err := image.Decode(strings.NewReader(string(raw)))
	if err != nil {
		f.luts[name] = nil
		return nil
	}
	pad := ebiten.NewImage(fxPadW, fxPadH)
	pad.DrawImage(ebiten.NewImageFromImage(img), nil)
	f.luts[name] = pad
	return f.luts[name]
}

// Apply 按 slot 持久语义叠加滤镜（dst 必须是 ScreenW×ScreenH）。
func (f *filterFX) Apply(dst *ebiten.Image, assetsRoot string, beat float64) {
	if len(f.evts) == 0 {
		return
	}
	if f.shader == nil {
		s, err := ebiten.NewShader([]byte(lutKage))
		if err != nil {
			log.Printf("engine: LUT shader: %v", err)
			f.evts = nil
			return
		}
		f.shader = s
		f.work = ebiten.NewImage(fxPadW, fxPadH)
	}
	type slotState struct {
		lut   string
		blend float64
	}
	slots := map[int]slotState{}
	for _, e := range f.evts {
		if beat < e.beat {
			continue
		}
		norm := 1.0
		if e.length > 0 {
			norm = clamp01((beat - e.beat) / e.length)
		}
		// Filter.cs：BlendAmount = ease(1-start, 1-end)，AmplifyColor 语义
		// 0=全滤镜、1=无效果；本地 blend 取反相（1=全滤镜）。
		blend := 1 - Ease(e.ease, 1-e.start, 1-e.end, norm)
		name := ""
		if e.filter >= 0 && e.filter < len(filterNames) {
			name = filterNames[e.filter]
		}
		slots[e.slot] = slotState{name, blend}
	}
	for slot := 1; slot <= 10; slot++ {
		st, ok := slots[slot]
		if !ok || st.blend <= 0 || st.lut == "" {
			continue
		}
		lut := f.lut(assetsRoot, st.lut)
		if lut == nil {
			continue
		}
		f.work.Clear()
		f.work.DrawImage(dst, nil)
		op := &ebiten.DrawRectShaderOptions{}
		op.Images[0] = f.work
		op.Images[1] = lut
		op.Uniforms = map[string]any{"Blend": float32(st.blend)}
		dst.DrawRectShader(fxPadW, fxPadH, f.shader, op)
	}
}

// ---------- vfx/display textbox ----------

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
		text:   strings.TrimSpace(richTagRe.ReplaceAllString(rawText, "")),
		align:  textboxAlign(rawText),
		anchor: int(e.Float("type", 1)),
		w:      e.Float("valA", 1), h: e.Float("valB", 1),
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

func drawTextboxPanel(dst *ebiten.Image, x, y, w, h float64) {
	r := math.Min(h*0.42, 34)
	drawRoundedRect(dst, x, y, w, h, r+7, color.RGBA{0, 0, 0, 245})
	drawRoundedRect(dst, x+6, y+6, w-12, h-12, r, color.RGBA{255, 255, 255, 245})
}

func drawRoundedRect(dst *ebiten.Image, x, y, w, h, r float64, c color.RGBA) {
	if w <= 0 || h <= 0 {
		return
	}
	if r > w/2 {
		r = w / 2
	}
	if r > h/2 {
		r = h / 2
	}
	fx, fy, fw, fh := float32(x), float32(y), float32(w), float32(h)
	fr := float32(r)
	vector.DrawFilledRect(dst, fx+fr, fy, fw-2*fr, fh, c, true)
	vector.DrawFilledRect(dst, fx, fy+fr, fw, fh-2*fr, c, true)
	vector.DrawFilledCircle(dst, fx+fr, fy+fr, fr, c, true)
	vector.DrawFilledCircle(dst, fx+fw-fr, fy+fr, fr, c, true)
	vector.DrawFilledCircle(dst, fx+fr, fy+fh-fr, fr, c, true)
	vector.DrawFilledCircle(dst, fx+fw-fr, fy+fh-fr, fr, c, true)
}
