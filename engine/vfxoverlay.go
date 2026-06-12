// vfxoverlay.go：vfx/filter（AmplifyColor LUT 滤镜）与 vfx/display textbox。
//
// filter：10 个 slot，事件按拍序持久覆盖（Filter.cs：beat 起永久生效，
// 直到后续事件改写同 slot）；BlendAmount = ease(1-start, 1-end)（AmplifyColor
// 语义 0=全滤镜、1=无效果），本地混合强度为其反相。LUT 为 1024×32 的 32³ 条带。
//
// textbox：事件期间显示九宫格文本框（textboxSDF 贴图 + OTF 排版），
// 富文本仅剥离标签（<align=center> 等），按 anchor 摆位。
package engine

import (
	"image"
	"image/color"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
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
	anchor       int
	w, h         float64
	x, y         float64
}

type textboxFX struct {
	evts  []textboxEvt
	box   *ebiten.Image
	font  *opentype.Font
	cache map[string]*ebiten.Image
}

var richTagRe = regexp.MustCompile(`<[^>]*>`)

func (t *textboxFX) add(e *riq.Entity) {
	t.evts = append(t.evts, textboxEvt{
		beat: e.Beat, length: e.Length,
		text:   richTagRe.ReplaceAllString(e.Str("text1", ""), ""),
		anchor: int(e.Float("type", 1)),
		w:      e.Float("valA", 1), h: e.Float("valB", 1),
		x: e.Float("x", 0), y: e.Float("y", 0),
	})
}

func (t *textboxFX) reset() { t.evts = nil }

// anchorPos 返回 anchor 枚举的屏幕坐标（TextboxAnchor：0=TopLeft..8=BottomRight 风格，
// 1=TopMiddle；Custom 用 x/y，单位与游戏世界一致 54px/unit）。
func anchorPos(anchor int, x, y float64) (float64, float64) {
	cx, cy := float64(ScreenW/2), float64(ScreenH/2)
	top, bot := 70.0, float64(ScreenH)-70
	lft, rgt := 160.0, float64(ScreenW)-160
	switch anchor {
	case 0:
		return lft, top
	case 1:
		return cx, top
	case 2:
		return rgt, top
	case 3:
		return lft, cy
	case 4:
		return cx, cy
	case 5:
		return rgt, cy
	case 6:
		return lft, bot
	case 7:
		return cx, bot
	case 8:
		return rgt, bot
	default: // Custom
		return cx + x*54, cy - y*54
	}
}

func (t *textboxFX) ensure(assetsRoot string) bool {
	if t.box != nil && t.font != nil {
		return true
	}
	raw, err := os.ReadFile(filepath.Join(assetsRoot, "common", "textbox.png"))
	if err != nil {
		return false
	}
	img, _, err := image.Decode(strings.NewReader(string(raw)))
	if err != nil {
		return false
	}
	t.box = ebiten.NewImageFromImage(img)
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

func (t *textboxFX) renderText(s string) *ebiten.Image {
	if img, ok := t.cache[s]; ok {
		return img
	}
	face, err := opentype.NewFace(t.font, &opentype.FaceOptions{Size: 40, DPI: 72})
	if err != nil {
		return nil
	}
	defer face.Close()
	adv := font.MeasureString(face, s)
	met := face.Metrics()
	w, h := adv.Ceil()+8, met.Ascent.Ceil()+met.Descent.Ceil()+8
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	d := &font.Drawer{Dst: img, Src: image.White, Face: face, Dot: fixed.P(4, 4+met.Ascent.Ceil())}
	d.DrawString(s)
	e := ebiten.NewImageFromImage(img)
	t.cache[s] = e
	return e
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
		// 框体（textboxSize 3×0.75 unit × 宽高参数 × 54px/unit，双倍留白）
		bw, bh := 3*e.w*54*2, 0.75*e.h*54*2
		op := &ebiten.DrawImageOptions{Filter: ebiten.FilterLinear}
		bs := t.box.Bounds()
		op.GeoM.Scale(bw/float64(bs.Dx()), bh/float64(bs.Dy()))
		op.GeoM.Translate(px-bw/2, py-bh/2)
		op.ColorScale.ScaleAlpha(0.92)
		dst.DrawImage(t.box, op)
		if e.text != "" {
			txt := t.renderText(e.text)
			if txt != nil {
				to := &ebiten.DrawImageOptions{Filter: ebiten.FilterLinear}
				tb := txt.Bounds()
				scale := 1.0
				if maxW := bw * 0.9; float64(tb.Dx()) > maxW {
					scale = maxW / float64(tb.Dx())
				}
				to.GeoM.Scale(scale, scale)
				to.GeoM.Translate(px-float64(tb.Dx())*scale/2, py-float64(tb.Dy())*scale/2)
				to.ColorScale.ScaleWithColor(color.Black)
				dst.DrawImage(txt, to)
			}
		}
	}
}
