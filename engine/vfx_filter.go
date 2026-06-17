// vfx_filter.go：vfx/filter（AmplifyColor LUT 滤镜）。
//
// 10 个 slot，事件按拍序持久覆盖（Filter.cs：beat 起永久生效，直到后续事件
// 改写同 slot）；BlendAmount = ease(1-start, 1-end)。AmplifyColor 会把目标
// LUT 与 default_lut 预混，BlendAmount=0 是目标 LUT，BlendAmount=1 是原图。
// 本地 shader 直接混屏幕图，因此使用 1-BlendAmount。LUT 为 1024×32 的
// 32³ 条带。
package engine

import (
	"github.com/hajimehoshi/ebiten/v2"

	"hsdemo/riq"
)

type filterEvt struct {
	beat, length float64
	filter       int
	slot         int
	start, end   float64
	ease         int
}

type filterFX struct {
	evts   []filterEvt
	luts   map[string]*ebiten.Image // 已垫到 padW×padH 的 LUT
	shader *ebiten.Shader
	work   *ebiten.Image // padW×padH（DrawRectShader 要求各源图同尺寸）
}

func (f *filterFX) add(e *riq.Entity) {
	f.evts = append(f.evts, filterEvt{
		beat: e.Beat, length: e.Length,
		filter: int(e.Float("filter", 0)), slot: int(e.Float("slot", 1)),
		start: e.Float("start", 0), end: e.Float("end", 0),
		ease: int(e.Float("ease", 0)),
	})
}

func (f *filterFX) reset() { f.evts = nil }
