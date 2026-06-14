package engine

import (
	"image/color"
	"math"

	"github.com/hajimehoshi/ebiten/v2"
)

func (fx *postFX) ensure() error {
	if fx.uber != nil {
		return nil
	}
	var err error
	if fx.uber, err = ebiten.NewShader([]byte(uberKage)); err != nil {
		return err
	}
	if fx.preShader, err = ebiten.NewShader([]byte(bloomPreKage)); err != nil {
		return err
	}
	if fx.blur, err = ebiten.NewShader([]byte(blurKage)); err != nil {
		return err
	}
	fx.frame = ebiten.NewImage(ScreenW, ScreenH)
	fx.bloomFull = ebiten.NewImage(ScreenW, ScreenH)
	fx.q1 = ebiten.NewImage(ScreenW/4, ScreenH/4)
	fx.q2 = ebiten.NewImage(ScreenW/4, ScreenH/4)
	return nil
}

// Target 返回游戏画面的渲染目标（需先 ensure）。
func (fx *postFX) Target() *ebiten.Image {
	fx.frame.Clear()
	return fx.frame
}

// Apply 把离屏帧经后处理链画到 dst。
func (fx *postFX) Apply(dst *ebiten.Image, beat, t float64) {
	p := fx.eval(beat)

	// bloom 前置链：阈值预滤 → 两轮可分离高斯（1/4 分辨率）→ 升采样
	fx.bloomFull.Fill(color.Black)
	if p.bloomOn {
		knee := p.bloomThr*p.bloomKnee + 1e-5
		fx.q1.Clear()
		op := &ebiten.DrawRectShaderOptions{}
		op.GeoM.Scale(0.25, 0.25)
		op.Images[0] = fx.frame
		op.Uniforms = map[string]any{
			"Threshold": float32(p.bloomThr),
			"Curve": []float32{float32(p.bloomThr - knee), float32(knee * 2),
				float32(0.25 / knee)},
		}
		fx.q1.DrawRectShader(ScreenW, ScreenH, fx.preShader, op)
		for i := 0; i < 2; i++ {
			fx.q2.Clear()
			bo := &ebiten.DrawRectShaderOptions{}
			bo.Images[0] = fx.q1
			bo.Uniforms = map[string]any{"Dir": []float32{1, 0}}
			fx.q2.DrawRectShader(ScreenW/4, ScreenH/4, fx.blur, bo)
			fx.q1.Clear()
			bo = &ebiten.DrawRectShaderOptions{}
			bo.Images[0] = fx.q2
			bo.Uniforms = map[string]any{"Dir": []float32{0, 1}}
			fx.q1.DrawRectShader(ScreenW/4, ScreenH/4, fx.blur, bo)
		}
		uo := &ebiten.DrawImageOptions{Filter: ebiten.FilterLinear}
		uo.GeoM.Scale(4, 4)
		fx.bloomFull.DrawImage(fx.q1, uo)
	}

	op := &ebiten.DrawRectShaderOptions{}
	op.Images[0] = fx.frame
	op.Images[1] = fx.bloomFull
	op.Uniforms = map[string]any{
		"Pixel":   []float32{float32(p.pixSize), float32(p.pixRatio), float32(p.pixSX), float32(p.pixSY)},
		"Lens":    []float32{float32(p.lensTheta), float32(p.lensSigma), float32(p.lensIntensity), float32(p.caAmt)},
		"LensXY":  []float32{float32(p.lensIX), float32(p.lensIY)},
		"Vig":     []float32{float32(p.vigInt * 3), float32(p.vigSmooth * 5), float32((1-p.vigRound)*6 + p.vigRound), float32(p.vigRounded)},
		"VigCol":  []float32{float32(p.vigColor[0]), float32(p.vigColor[1]), float32(p.vigColor[2])},
		"VigCtr":  []float32{float32(p.vigX), float32(p.vigY)},
		"VigOn":   b32(p.vigOn),
		"GradeOn": b32(p.gradeOn),
		"Balance": []float32{float32(p.balR), float32(p.balG), float32(p.balB)},
		"Filter":  []float32{float32(p.filter[0]), float32(p.filter[1]), float32(p.filter[2])},
		"HSB":     []float32{float32(p.hue), float32(p.sat), float32(p.bright), float32(p.contra)},
		"Grain":   []float32{float32(p.grainInt), float32(p.grainSize), float32(p.grainColored), float32(math.Mod(t, 64))},
		"BloomIT": []float32{float32(p.bloomInt * p.bloomTint[0]), float32(p.bloomInt * p.bloomTint[1]), float32(p.bloomInt * p.bloomTint[2])},
	}
	dst.DrawRectShader(ScreenW, ScreenH, fx.uber, op)
}

func b32(b bool) float32 {
	if b {
		return 1
	}
	return 0
}
