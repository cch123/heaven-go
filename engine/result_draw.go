package engine

import (
	"fmt"
	"image/color"
	"math"

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
		if a.resultT >= resultMsgTime {
			a.drawWrappedText(screen, a.result.Message1, a.faceMid, 90, 155, 455, 30, ink)
		}
		if a.resultT >= resultMsg2Time {
			a.drawWrappedText(screen, a.result.Message2, a.faceMid, 90, 235, 455, 30, ink)
		}
	} else if a.resultT >= resultMsgTime {
		a.drawWrappedText(screen, a.result.Message0, a.faceMid, 90, 178, 455, 32, ink)
	}

	scoreShown := a.result.Score
	if a.resultT < resultBarStart {
		scoreShown = 0
	} else if a.resultT < resultBarStart+resultBarDur {
		scoreShown *= (a.resultT - resultBarStart) / resultBarDur
	}
	barColor := resultScoreColor(scoreShown)
	vector.DrawFilledRect(screen, 95, 388, 508, 32, color.RGBA{42, 38, 48, 220}, false)
	vector.DrawFilledRect(screen, 101, 394, 496, 20, color.RGBA{102, 90, 108, 255}, false)
	vector.DrawFilledRect(screen, 101, 394, float32(496*scoreShown), 20, barColor, false)
	vector.StrokeLine(screen, 101+float32(496*rankOkThreshold), 390, 101+float32(496*rankOkThreshold), 419, 2, color.RGBA{255, 255, 255, 170}, false)
	vector.StrokeLine(screen, 101+float32(496*rankHiThreshold), 390, 101+float32(496*rankHiThreshold), 419, 2, color.RGBA{255, 255, 255, 170}, false)
	a.text(screen, fmt.Sprintf("%d", int(scoreShown*100)), a.faceBig, 626, 379, barColor, false)

	if a.resultT >= resultRankTime {
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

func (a *App) drawJudgementBackground(screen *ebiten.Image) {
	if a.resultAssets.bg != nil {
		drawImageCover(screen, a.resultAssets.bg, 0, 0, ScreenW, ScreenH, 1)
	} else {
		screen.Fill(color.RGBA{41, 38, 58, 255})
	}
	vector.DrawFilledRect(screen, 0, 0, ScreenW, ScreenH, color.RGBA{20, 18, 28, 90}, false)
}

func (a *App) drawRankLogo(screen *ebiten.Image) {
	switch a.result.Rank {
	case resultRankHi:
		if a.resultAssets.rankHi != nil {
			drawImageFit(screen, a.resultAssets.rankHi, 620, 104, 300, 120, 1)
		} else {
			a.text(screen, "SUPERB", a.faceBig, 768, 150, resultRankColor(resultRankHi), true)
		}
		if a.resultAssets.rankHiStar != nil {
			s := 54 + 5*math.Sin(a.resultT*5)
			drawImageFit(screen, a.resultAssets.rankHiStar, 842, 82, s, s, 1)
		}
	case resultRankOk:
		if a.resultAssets.rankOk != nil {
			drawImageFit(screen, a.resultAssets.rankOk, 656, 118, 230, 132, 1)
		} else {
			a.text(screen, "OK", a.faceBig, 768, 150, resultRankColor(resultRankOk), true)
		}
		if a.resultAssets.rankOkSweat != nil {
			drawImageFit(screen, a.resultAssets.rankOkSweat, 826, 98, 58, 25, 1)
		}
	case resultRankNg:
		img := firstResultImage(a.resultAssets.rankNg)
		if len(a.resultAssets.rankNg) > 0 {
			if frame := int(a.resultT*8) % len(a.resultAssets.rankNg); a.resultAssets.rankNg[frame] != nil {
				img = a.resultAssets.rankNg[frame]
			}
		}
		if img != nil {
			drawImageFit(screen, img, 616, 120, 315, 118, 1)
		} else {
			a.text(screen, "TRY AGAIN", a.faceBig, 768, 150, resultRankColor(resultRankNg), true)
		}
	}
}

func firstResultImage(imgs []*ebiten.Image) *ebiten.Image {
	for _, img := range imgs {
		if img != nil {
			return img
		}
	}
	return nil
}

func (a *App) drawBadge(screen *ebiten.Image, x, y float32, label string, c color.RGBA) {
	vector.DrawFilledRect(screen, x, y, 112, 27, color.RGBA{35, 31, 42, 210}, false)
	vector.DrawFilledRect(screen, x, y, 6, 27, c, false)
	a.text(screen, label, a.faceSmall, float64(x+15), float64(y+6), color.RGBA{242, 240, 248, 255}, false)
}

func (a *App) drawResultEpilogue(screen *ebiten.Image, white color.RGBA) {
	img := a.resultAssets.epNg
	msg := a.resultProp("epilogue_ng")
	switch a.result.Rank {
	case resultRankOk:
		img, msg = a.resultAssets.epOk, a.resultProp("epilogue_ok")
	case resultRankHi:
		img, msg = a.resultAssets.epHi, a.resultProp("epilogue_hi")
	}
	if img != nil {
		drawImageCover(screen, img, 0, 0, ScreenW, ScreenH, 1)
	} else {
		screen.Fill(resultRankColor(a.result.Rank))
	}
	vector.DrawFilledRect(screen, 0, ScreenH-116, ScreenW, 116, color.RGBA{24, 20, 30, 218}, false)
	a.text(screen, msg, a.faceBig, 54, ScreenH-96, white, false)
	a.text(screen, fmt.Sprintf("Final score %d  |  ACE %d  OK %d  NG %d  MISS %d",
		int(a.result.Score*100), a.aces, a.justs, a.ngs, a.misses),
		a.faceSmall, 58, ScreenH-42, color.RGBA{218, 214, 226, 255}, false)
	a.text(screen, "Enter / Click - level select    R - replay    Esc - quit", a.faceSmall, ScreenW-326, ScreenH-42, white, false)
}

func resultRankColor(rank resultRank) color.RGBA {
	switch rank {
	case resultRankHi:
		return color.RGBA{252, 191, 54, 255}
	case resultRankOk:
		return color.RGBA{90, 196, 217, 255}
	default:
		return color.RGBA{238, 80, 93, 255}
	}
}

func resultScoreColor(score float64) color.RGBA {
	switch {
	case score >= rankHiThreshold:
		return resultRankColor(resultRankHi)
	case score >= rankOkThreshold:
		return resultRankColor(resultRankOk)
	default:
		return resultRankColor(resultRankNg)
	}
}
