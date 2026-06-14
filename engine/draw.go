package engine

import (
	"image/color"
	"math"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

// ---------- Draw ----------

func (a *App) Draw(screen *ebiten.Image) {
	screen.Fill(color.RGBA{16, 16, 20, 255})
	t, beat := 0.0, 0.0
	if a.cond != nil {
		t, beat = a.cond.Time(), a.cond.Beat()
	}

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

	if a.active != nil {
		if a.fx.active() {
			// ppe：游戏画面渲到离屏帧，经后处理链上屏（flash/HUD 不参与，
			// 对应 HS 的编辑器叠层不过 PostProcessLayer）
			a.active.Draw(a.fx.Target(), t, beat)
			a.fx.Apply(canvas, beat, t)
		} else {
			a.active.Draw(canvas, t, beat)
		}
		a.flt.Apply(canvas, a.assetsRoot, beat)
		a.tbx.Draw(canvas, a.assetsRoot, beat)
	}

	a.drawFlash(canvas, beat)

	if canvas != screen {
		screen.Fill(color.RGBA{0, 0, 0, 255}) // letterbox 黑场
		op := &ebiten.DrawImageOptions{Filter: ebiten.FilterLinear}
		op.GeoM.Translate(-ScreenW/2, -ScreenH/2)
		op.GeoM.Scale(vsx, vsy)
		op.GeoM.Translate(ScreenW/2, ScreenH/2)
		screen.DrawImage(a.viewBuf, op)
	}

	white := color.RGBA{245, 245, 250, 255}
	dim := color.RGBA{200, 200, 210, 200}
	switch a.state {
	case stateTitle:
		a.drawTitle(screen, white, dim)
	case statePlay:
		if a.lastMsg != "" && t-a.msgT < 0.6 && !isTimingFeedbackMsg(a.lastMsg) {
			a.text(screen, a.lastMsg, a.faceBig, ScreenW/2, 90, white, true)
		}
		if a.starGot {
			a.text(screen, "* SKILL STAR", a.faceSmall, ScreenW-130, 20, color.RGBA{255, 230, 90, 255}, false)
		}
		if sec := a.bm.SectionAt(beat); sec != "" {
			a.text(screen, "- "+sec+" -", a.faceSmall, ScreenW-130, 40, color.RGBA{210, 210, 225, 200}, false)
		}
		if a.endBeat > 0 {
			prog := math.Min(beat/a.endBeat, 1)
			vector.DrawFilledRect(screen, 0, 0, float32(ScreenW*prog), 4, white, false)
		}
		a.drawTimingBar(screen, t)
	case stateResult:
		a.drawResult(screen, white)
	}

	if a.debug {
		a.drawDebug(screen, t, beat)
	}
}

func isTimingFeedbackMsg(s string) bool {
	switch s {
	case "ACE!!", "OK!", "NG", "MISS...", "...", "SKILL STAR!":
		return true
	default:
		return false
	}
}
