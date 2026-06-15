package engine

import (
	"log"

	"github.com/hajimehoshi/ebiten/v2"
)

type filterSlotState struct {
	lut   string
	blend float64
}

// Apply 按 slot 持久语义叠加滤镜（dst 必须是 ScreenW×ScreenH）。
func (f *filterFX) Apply(dst *ebiten.Image, assetsRoot string, beat float64) {
	if len(f.evts) == 0 {
		return
	}
	if !f.ensureShader() {
		return
	}
	slots := f.activeSlots(beat)
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

func (f *filterFX) ensureShader() bool {
	if f.shader != nil {
		return true
	}
	s, err := ebiten.NewShader([]byte(lutKage))
	if err != nil {
		log.Printf("engine: LUT shader: %v", err)
		f.evts = nil
		return false
	}
	f.shader = s
	f.work = ebiten.NewImage(fxPadW, fxPadH)
	return true
}

func (f *filterFX) activeSlots(beat float64) map[int]filterSlotState {
	slots := map[int]filterSlotState{}
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
		slots[e.slot] = filterSlotState{name, blend}
	}
	return slots
}
