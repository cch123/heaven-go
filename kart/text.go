// text.go：TMP（TextMeshPro）世界文本的运行时排版。
//
// 原版用 TMP SDF 字体资产渲染（meatGrinder 的 GRINDER 牌子等），但工程内
// 字体资产是动态填充模式（m_AtlasPopulationMode=1，glyph 表为空、运行时从
// 源 OTF 生成），因此这里直接用源字体文件排版：
//
//	em 世界高度 = m_fontSize × 0.1（TMP 非正交模式 fontScale = size/pointSize × 0.1）
//	对齐：水平 Center（m_HorizontalAlignment=2）+ 垂直 Middle/Top（512/256）；
//	      行中线 = 基线 + (ascender+descender)/2，用 sprite 枢轴复刻 TMP 锚点
//
// 文本渲染为高分辨率位图后注册为动态切片，由场景树按 MeshRenderer 的
// sortingOrder 参与统一排序绘制（节点缩放/层级变换照常生效）。
package kart

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"math"

	"github.com/hajimehoshi/ebiten/v2"
	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"

	"hsdemo/kmdata"
)

// textPPU 是文本位图的像素密度（px/unit）。屏幕投影 54 px/unit，
// 4 倍超采样保证缩放后边缘平滑。
const textPPU = 216.0

// parsedFonts 按字体文件名缓存解析结果（多个文本节点共用）。
var parsedFonts = map[string]*opentype.Font{}

// TextRun is a single TMP-style inline text span. Color is baked into the
// generated glyph bitmap; Scale supports Bon Odori's <size=0.9375> marker.
type TextRun struct {
	Text  string
	Color [4]float64
	Scale float64
}

func (a *Assets) font(name string) (*opentype.Font, error) {
	if f, ok := parsedFonts[name]; ok {
		return f, nil
	}
	raw, ok := a.Fonts[name]
	if !ok {
		return nil, fmt.Errorf("字体 %q 未提取（fonts/ 缺文件）", name)
	}
	f, err := opentype.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("解析字体 %q: %w", name, err)
	}
	parsedFonts[name] = f
	return f, nil
}

// ApplyTexts 渲染 texts.json 的全部文本节点并挂到场景树。
// 在 LoadAssets 后调用一次；之后可用 SetText 换字（changeText 事件）。
func (a *Assets) ApplyTexts() error {
	for i := range a.Texts {
		if err := a.renderTextNode(&a.Texts[i], a.Texts[i].Text); err != nil {
			return err
		}
	}
	return nil
}

// SetText 更新指定节点的文本内容（meatGrinder/changeText 等事件）。
func (a *Assets) SetText(path, content string) error {
	for i := range a.Texts {
		if a.Texts[i].Path == path {
			return a.renderTextRuns(&a.Texts[i], []TextRun{{Text: content}}, -1)
		}
	}
	return fmt.Errorf("文本节点 %q 不存在", path)
}

// SetTextRuns renders a text node from pre-parsed rich text runs.
func (a *Assets) SetTextRuns(path string, runs []TextRun) error {
	return a.SetTextRunsClipped(path, runs, -1)
}

// SetTextRunsClipped renders rich text and clips glyph pixels after clipUnits
// from the line's left text edge. clipUnits < 0 disables clipping. This mirrors
// TMP SpriteMask usage in Bon Odori's blue lyric overlay without changing the
// generic scene mask pipeline.
func (a *Assets) SetTextRunsClipped(path string, runs []TextRun, clipUnits float64) error {
	for i := range a.Texts {
		if a.Texts[i].Path == path {
			return a.renderTextRuns(&a.Texts[i], runs, clipUnits)
		}
	}
	return fmt.Errorf("文本节点 %q 不存在", path)
}

// MeasureTextRuns reports the preferred line width and each rune's left edge in
// text units. It lets minigames reproduce TMP characterInfo positions for masks.
func (a *Assets) MeasureTextRuns(path string, runs []TextRun) (float64, []float64, error) {
	for i := range a.Texts {
		if a.Texts[i].Path == path {
			layout, err := a.layoutTextRuns(&a.Texts[i], runs)
			if err != nil {
				return 0, nil, err
			}
			defer layout.close()
			return float64(layout.textW) / textPPU, layout.charX, nil
		}
	}
	return 0, nil, fmt.Errorf("文本节点 %q 不存在", path)
}

// renderTextNode 排版一个文本节点并更新场景节点的切片/排序。
func (a *Assets) renderTextNode(tn *kmdata.TextNode, content string) error {
	return a.renderTextRuns(tn, []TextRun{{Text: content}}, -1)
}

type textSpan struct {
	text    string
	face    font.Face
	color   [4]float64
	startPx int
}

type textLayout struct {
	spans      []textSpan
	textW      int
	ascent     int
	descent    int
	charX      []float64
	closeFaces []font.Face
	contentW   int
	dotX       int
	pivotX     float64
	pivotY     float64
	content    bool
}

func (a *Assets) renderTextRuns(tn *kmdata.TextNode, runs []TextRun, clipUnits float64) error {
	idx, ok := a.NodeIndex(tn.Path)
	if !ok {
		return fmt.Errorf("文本节点 path %q 不在场景树", tn.Path)
	}
	if (tn.HAlign != 1 && tn.HAlign != 2) || (tn.VAlign != 512 && tn.VAlign != 256) {
		// 官方游戏目前只需要 TMP Left/Center + Middle/Top。遇到其它对齐时报错，
		// 避免悄悄把歌词、标牌或 UI 文本排到错误位置。
		return fmt.Errorf("文本 %q 对齐 (%d,%d) 未实现", tn.Path, tn.HAlign, tn.VAlign)
	}

	n := &a.Rig.Nodes[idx]
	n.Order = tn.Order
	n.Layer = tn.Layer
	n.Color = tn.Color
	layout, err := a.layoutTextRuns(tn, runs)
	if err != nil {
		return err
	}
	defer layout.close()
	if !layout.content {
		n.Sprite = ""
		return nil
	}

	const pad = 4
	img := image.NewRGBA(image.Rect(0, 0, layout.contentW+2*pad, layout.ascent+layout.descent+2*pad))
	for _, sp := range layout.spans {
		d := &font.Drawer{
			Dst: img, Src: image.NewUniform(runColor(sp.color)), Face: sp.face,
			Dot: fixed.P(layout.dotX+sp.startPx, pad+layout.ascent),
		}
		d.DrawString(sp.text)
	}
	if clipUnits >= 0 {
		clipPx := layout.dotX + int(math.Round(clipUnits*textPPU))
		if clipPx < 0 {
			clipPx = 0
		}
		if clipPx < img.Bounds().Dx() {
			draw.Draw(img, image.Rect(clipPx, 0, img.Bounds().Dx(), img.Bounds().Dy()), image.Transparent, image.Point{}, draw.Src)
		}
	}

	a.RegisterSprite("__text_"+tn.Path, ebiten.NewImageFromImage(img), textPPU, layout.pivotX, layout.pivotY)
	n.Sprite = "__text_" + tn.Path
	return nil
}

func (a *Assets) layoutTextRuns(tn *kmdata.TextNode, runs []TextRun) (*textLayout, error) {
	f, err := a.font(tn.Font)
	if err != nil {
		return nil, err
	}
	emPx := tn.Size * 0.1 * textPPU // fontSize → 世界 em 高 → 像素

	layout := &textLayout{}
	faces := map[float64]font.Face{}
	faceFor := func(scale float64) (font.Face, error) {
		if scale <= 0 {
			scale = 1
		}
		if fc, ok := faces[scale]; ok {
			return fc, nil
		}
		fc, err := opentype.NewFace(f, &opentype.FaceOptions{
			Size: emPx * scale, DPI: 72, Hinting: font.HintingNone,
		})
		if err != nil {
			return nil, err
		}
		faces[scale] = fc
		layout.closeFaces = append(layout.closeFaces, fc)
		return fc, nil
	}
	baseFace, err := faceFor(1)
	if err != nil {
		return nil, err
	}
	met := baseFace.Metrics()
	layout.ascent, layout.descent = met.Ascent.Ceil(), met.Descent.Ceil()
	x := 0
	for _, run := range runs {
		if run.Text == "" {
			continue
		}
		layout.content = true
		fc, err := faceFor(run.Scale)
		if err != nil {
			layout.close()
			return nil, err
		}
		layout.spans = append(layout.spans, textSpan{
			text:    run.Text,
			face:    fc,
			color:   run.Color,
			startPx: x,
		})
		for _, r := range run.Text {
			layout.charX = append(layout.charX, float64(x)/textPPU)
			x += font.MeasureString(fc, string(r)).Ceil()
		}
	}
	layout.textW = x
	if !layout.content {
		return layout, nil
	}
	h := layout.ascent + layout.descent
	if layout.textW <= 0 || h <= 0 {
		layout.close()
		return nil, fmt.Errorf("文本排版尺寸为空")
	}
	layout.contentW = layout.textW
	if tn.Rect[0] > 0 {
		layout.contentW = max(layout.contentW, int(tn.Rect[0]*textPPU+0.5))
	}
	const pad = 4
	layout.dotX = pad
	layout.pivotX = 0
	if tn.HAlign == 2 {
		layout.dotX += (layout.contentW - layout.textW) / 2
		layout.pivotX = 0.5
	}

	// 枢轴：x 取水平对齐点；y 按 TMP 垂直对齐取行中线或顶边，
	// 换算为 Unity 归一化（自底边）。
	H := float64(h + 2*pad)
	midFromTop := float64(pad) + (float64(layout.ascent)+float64(layout.descent))/2
	layout.pivotY = 1 - midFromTop/H
	if tn.VAlign == 256 {
		layout.pivotY = 1 - float64(pad)/H
	}
	return layout, nil
}

func (l *textLayout) close() {
	for _, f := range l.closeFaces {
		f.Close()
	}
}

func runColor(c [4]float64) color.Color {
	if c == [4]float64{} {
		c = [4]float64{1, 1, 1, 1}
	}
	return color.NRGBA{
		R: uint8(clamp01(c[0])*255 + 0.5),
		G: uint8(clamp01(c[1])*255 + 0.5),
		B: uint8(clamp01(c[2])*255 + 0.5),
		A: uint8(clamp01(c[3])*255 + 0.5),
	}
}

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}
