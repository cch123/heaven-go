package engine

import (
	"fmt"
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

func (a *App) drawResult(screen *ebiten.Image, white color.RGBA) {
	if a.resultEpilogue {
		a.drawResultEpilogue(screen, white)
		return
	}
	a.drawJudgementBackground(screen)

	panel := color.RGBA{245, 239, 224, 235}
	ink := color.RGBA{58, 46, 64, 255}
	dim := color.RGBA{112, 98, 118, 255}
	vector.DrawFilledRect(screen, 60, 82, 530, 255, panel, false)
	vector.DrawFilledRect(screen, 60, 82, 530, 42, resultRankColor(a.result.Rank), false)
	vector.DrawFilledRect(screen, 60, 333, 530, 4, color.RGBA{58, 46, 64, 180}, false)
	a.text(screen, a.result.Header, a.faceMid, 82, 94, color.RGBA{255, 252, 242, 255}, false)

	if a.result.TwoMessage {
		if a.resultT >= resultMessage1Time {
			a.drawWrappedText(screen, a.result.Message1, a.faceMid, 90, 155, 455, 30, ink)
		}
		if a.resultT >= resultMessage2Time {
			a.drawWrappedText(screen, a.result.Message2, a.faceMid, 90, 235, 455, 30, ink)
		}
	} else if a.resultT >= resultMessage0Time {
		a.drawWrappedText(screen, a.result.Message0, a.faceMid, 90, 178, 455, 32, ink)
	}

	scoreShown := clamp01(a.result.Score)
	barDone := a.resultBarDoneTime()
	if a.resultT < resultBarStart {
		scoreShown = 0
	} else if a.resultT < barDone {
		scoreShown *= (a.resultT - resultBarStart) / (barDone - resultBarStart)
	}
	barColor := resultScoreColor(scoreShown)
	vector.DrawFilledRect(screen, 95, 388, 508, 32, color.RGBA{42, 38, 48, 220}, false)
	vector.DrawFilledRect(screen, 101, 394, 496, 20, color.RGBA{102, 90, 108, 255}, false)
	vector.DrawFilledRect(screen, 101, 394, float32(496*scoreShown), 20, barColor, false)
	vector.StrokeLine(screen, 101+float32(496*rankOkThreshold), 390, 101+float32(496*rankOkThreshold), 419, 2, color.RGBA{255, 255, 255, 170}, false)
	vector.StrokeLine(screen, 101+float32(496*rankHiThreshold), 390, 101+float32(496*rankHiThreshold), 419, 2, color.RGBA{255, 255, 255, 170}, false)
	a.text(screen, fmt.Sprintf("%d", int(scoreShown*100)), a.faceBig, 626, 379, barColor, false)

	if a.resultT >= a.resultRankTime() {
		a.drawRankLogo(screen)
		if a.result.SubRank {
			a.text(screen, "...but, just", a.faceMid, 760, 306, dim, true)
		}
		if a.result.Star {
			a.drawBadge(screen, 86, 446, "SKILL STAR", color.RGBA{255, 220, 82, 255})
		}
		if a.result.NoMiss {
			a.drawBadge(screen, 236, 446, "NO MISS", color.RGBA{92, 205, 236, 255})
		}
		if a.result.Perfect {
			a.drawBadge(screen, 366, 446, "PERFECT", color.RGBA{255, 140, 210, 255})
		}
		a.text(screen, "Enter / Click - epilogue    R - replay    Esc - quit", a.faceSmall, ScreenW/2, ScreenH-34, white, true)
	} else {
		a.text(screen, "Enter / Click - skip    R - replay    Esc - quit", a.faceSmall, ScreenW/2, ScreenH-34, dim, true)
	}
}
