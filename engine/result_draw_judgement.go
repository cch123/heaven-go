package engine

import (
	"image/color"
	"math"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

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
