package engine

import (
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
)

func (a *App) drawGameCanvas(screen *ebiten.Image, t, beat float64) {
	// vfx/scale view：缩放生效时游戏画布整体渲到离屏帧再贴回
	//（StaticCamera 语义：画布外露出 letterbox 黑场；HUD 不参与缩放）。
	vsx, vsy := a.viewScaleAt(beat)
	canvas := screen
	if vsx != 1 || vsy != 1 {
		if a.viewBuf == nil {
			a.viewBuf = ebiten.NewImage(ScreenW, ScreenH)
		}
		a.viewBuf.Fill(color.RGBA{16, 16, 20, 255})
		canvas = a.viewBuf
	}

	a.drawActiveModule(canvas, t, beat)

	if canvas != screen {
		screen.Fill(color.RGBA{0, 0, 0, 255}) // letterbox 黑场
		op := &ebiten.DrawImageOptions{Filter: ebiten.FilterLinear}
		op.GeoM.Translate(-ScreenW/2, -ScreenH/2)
		op.GeoM.Scale(vsx, vsy)
		op.GeoM.Translate(ScreenW/2, ScreenH/2)
		screen.DrawImage(a.viewBuf, op)
	}
	// Flash is GameFlash, a UI Image in the game-view camera prefab. It sits
	// above camera image effects, so LUT/PPE and scale-view must not recolor or
	// resize the fade layer itself.
	a.drawFlash(screen, beat)
}

func (a *App) drawActiveModule(canvas *ebiten.Image, t, beat float64) {
	if a.active == nil {
		return
	}
	cam := a.CameraAt(beat)
	if a.fx.active() {
		// ppe：游戏画面渲到离屏帧，经后处理链上屏（flash/HUD 不参与，
		// 对应 HS 的编辑器叠层不过 PostProcessLayer）
		target := a.fx.Target()
		a.dcl.DrawBackground(target, a.customSprites, beat, cam)
		a.active.Draw(target, t, beat)
		a.dcl.DrawGame(target, a.customSprites, beat, cam)
		a.fx.Apply(canvas, a.assetsRoot, beat, t)
	} else {
		a.dcl.DrawBackground(canvas, a.customSprites, beat, cam)
		a.active.Draw(canvas, t, beat)
		a.dcl.DrawGame(canvas, a.customSprites, beat, cam)
	}
	a.flt.Apply(canvas, a.assetsRoot, beat)
	a.dcl.DrawOverlay(canvas, a.customSprites, beat, cam)
	a.tbx.Draw(canvas, a.assetsRoot, beat)
}
