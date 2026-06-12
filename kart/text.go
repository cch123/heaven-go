// text.go：TMP（TextMeshPro）世界文本的运行时排版。
//
// 原版用 TMP SDF 字体资产渲染（meatGrinder 的 GRINDER 牌子等），但工程内
// 字体资产是动态填充模式（m_AtlasPopulationMode=1，glyph 表为空、运行时从
// 源 OTF 生成），因此这里直接用源字体文件排版：
//
//	em 世界高度 = m_fontSize × 0.1（TMP 非正交模式 fontScale = size/pointSize × 0.1）
//	对齐：水平 Center（m_HorizontalAlignment=2）+ 垂直 Middle（512）——
//	      行中线 = 基线 + (ascender+descender)/2，置于 RectTransform 中心
//
// 文本渲染为高分辨率位图后注册为动态切片，由场景树按 MeshRenderer 的
// sortingOrder 参与统一排序绘制（节点缩放/层级变换照常生效）。
package kart

import (
	"fmt"
	"image"
	"image/color"
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
			return a.renderTextNode(&a.Texts[i], content)
		}
	}
	return fmt.Errorf("文本节点 %q 不存在", path)
}

// renderTextNode 排版一个文本节点并更新场景节点的切片/排序。
func (a *Assets) renderTextNode(tn *kmdata.TextNode, content string) error {
	idx, ok := a.NodeIndex(tn.Path)
	if !ok {
		return fmt.Errorf("文本节点 path %q 不在场景树", tn.Path)
	}
	if tn.HAlign != 2 || tn.VAlign != 512 {
		// 目前只实现 Center/Middle（meatGrinder 唯一用例）；其他对齐出现时报错而非静默错位
		return fmt.Errorf("文本 %q 对齐 (%d,%d) 未实现", tn.Path, tn.HAlign, tn.VAlign)
	}

	f, err := a.font(tn.Font)
	if err != nil {
		return err
	}
	emPx := tn.Size * 0.1 * textPPU // fontSize → 世界 em 高 → 像素
	face, err := opentype.NewFace(f, &opentype.FaceOptions{
		Size: emPx, DPI: 72, Hinting: font.HintingNone,
	})
	if err != nil {
		return err
	}
	defer face.Close()

	met := face.Metrics()
	adv := font.MeasureString(face, content)
	w := adv.Ceil()
	ascent, descent := met.Ascent.Ceil(), met.Descent.Ceil()
	h := ascent + descent
	if w <= 0 || h <= 0 {
		return fmt.Errorf("文本 %q 排版尺寸为空", content)
	}
	const pad = 4
	img := image.NewRGBA(image.Rect(0, 0, w+2*pad, h+2*pad))
	col := color.RGBA{
		uint8(math.Round(tn.Color[0] * 255)), uint8(math.Round(tn.Color[1] * 255)),
		uint8(math.Round(tn.Color[2] * 255)), uint8(math.Round(tn.Color[3] * 255)),
	}
	d := &font.Drawer{
		Dst: img, Src: image.NewUniform(col), Face: face,
		Dot: fixed.P(pad, pad+ascent),
	}
	d.DrawString(content)

	// 枢轴：x 取水平中心；y 取行中线（基线 + (asc+desc)/2，desc 为负 →
	// 基线上方 (ascent-descent)/2 像素），换算为 Unity 归一化（自底边）
	midFromTop := float64(pad) + (float64(ascent)+float64(descent))/2
	H := float64(img.Bounds().Dy())
	pivotY := 1 - midFromTop/H

	a.RegisterSprite("__text_"+tn.Path, ebiten.NewImageFromImage(img), textPPU, 0.5, pivotY)
	n := &a.Rig.Nodes[idx]
	n.Sprite = "__text_" + tn.Path
	n.Order = tn.Order
	n.Layer = tn.Layer
	return nil
}
