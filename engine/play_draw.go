package engine

import (
	"image/color"
	"math"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

func (a *App) drawPlayHUD(screen *ebiten.Image, t, beat float64, white color.RGBA) {
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
	a.timingDisplayState.draw(screen, t, a.assetsRoot, a.faceSmall, a.text)
}

func isTimingFeedbackMsg(s string) bool {
	switch s {
	case "ACE!!", "OK!", "NG", "MISS...", "...", "SKILL STAR!":
		return true
	default:
		return false
	}
}
