package engine

import (
	"fmt"
	"image/color"
	"math"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
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

func (a *App) drawTitle(screen *ebiten.Image, white, dim color.RGBA) {
	if a.bm == nil {
		a.drawLevelSelect(screen, white, dim)
		return
	} else {
		title := a.bm.Prop("remixtitle")
		if title == "" {
			title = "Untitled Remix"
		}
		a.text(screen, title, a.faceBig, ScreenW/2, 110, white, true)
		if author := a.bm.Prop("remixauthor"); author != "" {
			a.text(screen, "chart by "+author, a.faceMid, ScreenW/2, 170, dim, true)
		}
		a.text(screen, fmt.Sprintf("%d inputs | %.0f BPM | games: %s",
			len(a.inputs), a.bm.Tempos[0].BPM, strings.Join(keys(a.modules), ", ")),
			a.faceSmall, ScreenW/2, 208, dim, true)
		if len(a.unported) > 0 {
			a.text(screen, "Unported: "+strings.Join(a.unported, ", "), a.faceSmall, ScreenW/2, 232,
				color.RGBA{255, 170, 120, 255}, true)
		}
		a.text(screen, "Space / J / Click to play    (drop another .riq to switch)", a.faceMid, ScreenW/2, ScreenH-110, white, true)
		a.text(screen, "press to start", a.faceMid, ScreenW/2, ScreenH-72, white, true)
	}
	if a.loadErr != "" {
		a.text(screen, a.loadErr, a.faceSmall, ScreenW/2, ScreenH-36, color.RGBA{255, 120, 120, 255}, true)
	}
}

func (a *App) drawLevelSelect(screen *ebiten.Image, white, dim color.RGBA) {
	a.drawLibraryBackground(screen)
	vector.DrawFilledRect(screen, 0, 0, ScreenW, 78, color.RGBA{255, 250, 236, 220}, false)
	vector.DrawFilledRect(screen, 0, 77, ScreenW, 2, color.RGBA{118, 88, 148, 160}, false)
	vector.DrawFilledRect(screen, 0, ScreenH-54, ScreenW, 54, color.RGBA{255, 250, 236, 220}, false)

	ink := color.RGBA{66, 50, 88, 255}
	soft := color.RGBA{104, 92, 118, 255}
	a.text(screen, "Library", a.faceBig, 58, 20, ink, false)
	a.text(screen, "HEAVEN GO", a.faceSmall, 854, 30, color.RGBA{122, 105, 142, 255}, true)

	if len(a.levels) == 0 {
		vector.DrawFilledRect(screen, 248, 178, 464, 154, color.RGBA{255, 252, 242, 230}, false)
		vector.DrawFilledRect(screen, 248, 178, 464, 4, color.RGBA{118, 88, 148, 210}, false)
		a.text(screen, "No .riq levels found under levels/", a.faceMid, ScreenW/2, 222, ink, true)
		a.text(screen, "Drop a .riq file here to play", a.faceMid, ScreenW/2, 274, soft, true)
		if a.loadErr != "" {
			a.text(screen, a.fitText(a.loadErr, a.faceSmall, 760), a.faceSmall, ScreenW/2, ScreenH-36, color.RGBA{255, 120, 120, 255}, true)
		}
		return
	}

	a.keepMenuSelectionVisible()
	for slot := 0; slot < menuVisibleItems; slot++ {
		idx := a.menuScroll + slot
		if idx >= len(a.levels) {
			break
		}
		col := slot % menuGridCols
		row := slot / menuGridCols
		x := float64(menuGridX + col*(menuCardW+menuCardGapX))
		y := float64(menuGridY + row*(menuCardH+menuCardGapY))
		a.drawLevelCard(screen, a.levels[idx], idx, x, y, idx == a.menuSel)
	}

	first := a.menuScroll + 1
	last := a.menuScroll + menuVisibleItems
	if last > len(a.levels) {
		last = len(a.levels)
	}
	a.drawLibraryLevelInfo(screen, a.levels[a.menuSel], a.menuSel, ink, soft)
	a.text(screen, fmt.Sprintf("%d-%d / %d", first, last, len(a.levels)), a.faceSmall, 66, ScreenH-34, soft, false)
	a.text(screen, "Enter / Click    Arrows / WASD    Drop .riq", a.faceSmall, ScreenW/2, ScreenH-34, soft, true)
	if a.loadErr != "" {
		a.text(screen, a.fitText(a.loadErr, a.faceSmall, 840), a.faceSmall, ScreenW/2, ScreenH-18, color.RGBA{255, 120, 120, 255}, true)
	}
}

func (a *App) drawLibraryBackground(screen *ebiten.Image) {
	if a.libraryAssets.bgBase == nil {
		screen.Fill(color.RGBA{231, 226, 215, 255})
		return
	}
	drawImageCover(screen, a.libraryAssets.bgBase, 0, 0, ScreenW, ScreenH, 1)
	drawImageCover(screen, a.libraryAssets.bgGradient, 0, 0, ScreenW, ScreenH, 0.7)
	drawImageCover(screen, a.libraryAssets.bgStars, 0, 0, ScreenW, ScreenH, 0.28)
	drawImageCover(screen, a.libraryAssets.bgWaves, 0, 0, ScreenW, ScreenH, 0.24)
	vector.DrawFilledRect(screen, 0, 0, ScreenW, ScreenH, color.RGBA{255, 250, 238, 70}, false)
}

func (a *App) drawLevelCard(screen *ebiten.Image, level menuLevel, idx int, x, y float64, selected bool) {
	if selected {
		vector.DrawFilledRect(screen, float32(x-5), float32(y-5), menuCardW+10, menuCardH+10, color.RGBA{255, 227, 95, 235}, false)
	}
	vector.DrawFilledRect(screen, float32(x), float32(y), menuCardW, menuCardH, color.RGBA{255, 252, 242, 236}, false)
	vector.DrawFilledRect(screen, float32(x), float32(y+menuCardH-37), menuCardW, 37, color.RGBA{246, 239, 229, 245}, false)
	if selected {
		vector.DrawFilledRect(screen, float32(x), float32(y+menuCardH-4), menuCardW, 4, color.RGBA{118, 88, 148, 255}, false)
	}
	a.drawLevelThumbnail(screen, level, idx, x+11, y+9, 126)
	title := a.fitText(level.displayName(), a.faceSmall, menuCardW-20)
	a.text(screen, title, a.faceSmall, x+10, y+menuCardH-29, color.RGBA{68, 54, 82, 255}, false)
	meta := "RIQ"
	if len(level.games) > 0 {
		meta = fmt.Sprintf("%d games", len(level.games))
	}
	a.text(screen, meta, a.faceSmall, x+10, y+menuCardH-12, color.RGBA{120, 106, 133, 255}, false)
}

func (a *App) drawLevelThumbnail(screen *ebiten.Image, level menuLevel, idx int, x, y, size float64) {
	inner := size * 0.78
	innerX := x + (size-inner)/2
	innerY := y + (size-inner)/2
	vector.DrawFilledRect(screen, float32(innerX), float32(innerY), float32(inner), float32(inner), color.RGBA{240, 232, 220, 255}, false)
	if level.customIcon != nil {
		drawImageFit(screen, level.customIcon, innerX, innerY, inner, inner, 1)
	} else {
		a.drawFallbackLevelIcon(screen, level, idx, innerX, innerY, inner)
	}
	if a.libraryAssets.border != nil {
		drawImageFit(screen, a.libraryAssets.border, x, y, size, size, 1)
	} else {
		vector.DrawFilledRect(screen, float32(x), float32(y), float32(size), 5, color.RGBA{116, 94, 128, 255}, false)
		vector.DrawFilledRect(screen, float32(x), float32(y+size-5), float32(size), 5, color.RGBA{116, 94, 128, 255}, false)
		vector.DrawFilledRect(screen, float32(x), float32(y), 5, float32(size), color.RGBA{116, 94, 128, 255}, false)
		vector.DrawFilledRect(screen, float32(x+size-5), float32(y), 5, float32(size), color.RGBA{116, 94, 128, 255}, false)
	}
}

func (a *App) drawFallbackLevelIcon(screen *ebiten.Image, level menuLevel, idx int, x, y, size float64) {
	games := level.games
	if len(games) == 0 {
		games = []string{"RIQ"}
	}
	if len(games) == 1 {
		vector.DrawFilledRect(screen, float32(x), float32(y), float32(size), float32(size), menuAccent(idx), false)
		a.text(screen, a.fitText(menuGameLabel(games[0]), a.faceMid, size-18), a.faceMid, x+size/2, y+size/2-12, color.RGBA{255, 252, 242, 255}, true)
		return
	}
	tile := (size - 5) / 2
	for i := 0; i < 4; i++ {
		tx := x + float64(i%2)*(tile+5)
		ty := y + float64(i/2)*(tile+5)
		c := menuAccent(idx + i)
		vector.DrawFilledRect(screen, float32(tx), float32(ty), float32(tile), float32(tile), c, false)
		if i < len(games) {
			label := a.fitText(menuGameLabel(games[i]), a.faceSmall, tile-8)
			a.text(screen, label, a.faceSmall, tx+tile/2, ty+tile/2-8, color.RGBA{255, 252, 242, 255}, true)
		}
	}
}

func (a *App) drawLibraryLevelInfo(screen *ebiten.Image, level menuLevel, idx int, ink, soft color.RGBA) {
	x, y := 585.0, 116.0
	w, h := 326.0, 344.0
	vector.DrawFilledRect(screen, float32(x), float32(y), float32(w), float32(h), color.RGBA{255, 252, 242, 236}, false)
	vector.DrawFilledRect(screen, float32(x), float32(y), float32(w), 5, color.RGBA{118, 88, 148, 230}, false)
	a.drawLevelThumbnail(screen, level, idx, x+24, y+32, 112)
	a.text(screen, a.fitText(level.displayName(), a.faceMid, 172), a.faceMid, x+154, y+38, ink, false)
	author := level.author
	if author == "" {
		author = "Unknown author"
	}
	a.text(screen, a.fitText(author, a.faceSmall, 150), a.faceSmall, x+156, y+78, soft, false)
	a.text(screen, fmt.Sprintf("%.0f BPM", level.bpm), a.faceSmall, x+156, y+104, color.RGBA{118, 88, 148, 255}, false)

	baseY := y + 176
	games := level.games
	if len(games) == 0 {
		games = []string{"Unknown game"}
	}
	a.text(screen, "Games", a.faceSmall, x+24, baseY, soft, false)
	a.drawGameList(screen, games, x+24, baseY+26, w-48)
	desc := strings.TrimSpace(level.desc)
	if desc != "" {
		a.text(screen, "Description", a.faceSmall, x+24, y+276, soft, false)
		a.drawWrappedTextLimit(screen, desc, a.faceSmall, x+24, y+302, w-48, 20, 2, ink)
	} else {
		a.text(screen, a.fitText(level.path, a.faceSmall, w-48), a.faceSmall, x+24, y+h-46, soft, false)
	}
}

func (a *App) drawGameList(screen *ebiten.Image, games []string, x, y, maxW float64) {
	cx := x
	for i, game := range games {
		label := a.fitText(menuGameLabel(game), a.faceSmall, 100)
		tw, _ := text.Measure(label, a.faceSmall, 0)
		pw := math.Min(tw+22, 120)
		if cx+pw > x+maxW {
			break
		}
		vector.DrawFilledRect(screen, float32(cx), float32(y), float32(pw), 24, menuAccent(i), false)
		a.text(screen, label, a.faceSmall, cx+11, y+6, color.RGBA{255, 252, 242, 255}, false)
		cx += pw + 8
	}
}

func (a *App) drawWrappedTextLimit(screen *ebiten.Image, s string, face *text.GoTextFace, x, y, maxW, lineH float64, maxLines int, c color.Color) {
	words := strings.Fields(s)
	if len(words) == 0 || maxLines <= 0 {
		return
	}
	line := words[0]
	lines := 0
	for _, word := range words[1:] {
		next := line + " " + word
		if w, _ := text.Measure(next, face, 0); w <= maxW {
			line = next
			continue
		}
		lines++
		if lines >= maxLines {
			a.text(screen, a.fitText(line+"...", face, maxW), face, x, y, c, false)
			return
		}
		a.text(screen, line, face, x, y, c, false)
		y += lineH
		line = word
	}
	a.text(screen, line, face, x, y, c, false)
}

func menuGameLabel(game string) string {
	names := map[string]string{
		"blueBear":       "Blue Bear",
		"marchingOrders": "Marching Orders",
		"munchyMonk":     "Munchy Monk",
		"seeSaw":         "See-Saw",
		"somen":          "Somen",
		"totemClimb":     "Totem Climb",
		"trickClass":     "Trick Class",
	}
	if name, ok := names[game]; ok {
		return name
	}
	return humanizeGameID(game)
}

func humanizeGameID(id string) string {
	if id == "" {
		return "Unknown"
	}
	var b strings.Builder
	for i, r := range id {
		if i > 0 && r >= 'A' && r <= 'Z' {
			b.WriteByte(' ')
		}
		if i == 0 && r >= 'a' && r <= 'z' {
			r -= 'a' - 'A'
		}
		b.WriteRune(r)
	}
	return b.String()
}

func menuAccent(i int) color.RGBA {
	palette := []color.RGBA{
		{232, 184, 74, 255},
		{83, 189, 179, 255},
		{235, 111, 94, 255},
		{147, 194, 86, 255},
		{157, 139, 214, 255},
		{76, 151, 218, 255},
	}
	return palette[i%len(palette)]
}

func (a *App) fitText(s string, face *text.GoTextFace, maxW float64) string {
	if w, _ := text.Measure(s, face, 0); w <= maxW {
		return s
	}
	rs := []rune(s)
	for len(rs) > 0 {
		rs = rs[:len(rs)-1]
		candidate := string(rs) + "..."
		if w, _ := text.Measure(candidate, face, 0); w <= maxW {
			return candidate
		}
	}
	return "..."
}

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

func drawImageFit(dst, src *ebiten.Image, x, y, w, h float64, alpha float32) {
	if src == nil {
		return
	}
	sw, sh := src.Bounds().Dx(), src.Bounds().Dy()
	if sw == 0 || sh == 0 {
		return
	}
	s := math.Min(w/float64(sw), h/float64(sh))
	dw, dh := float64(sw)*s, float64(sh)*s
	op := &ebiten.DrawImageOptions{Filter: ebiten.FilterLinear}
	op.GeoM.Scale(s, s)
	op.GeoM.Translate(x+(w-dw)/2, y+(h-dh)/2)
	op.ColorScale.ScaleAlpha(alpha)
	dst.DrawImage(src, op)
}

func drawImageCover(dst, src *ebiten.Image, x, y, w, h float64, alpha float32) {
	if src == nil {
		return
	}
	sw, sh := src.Bounds().Dx(), src.Bounds().Dy()
	if sw == 0 || sh == 0 {
		return
	}
	s := math.Max(w/float64(sw), h/float64(sh))
	dw, dh := float64(sw)*s, float64(sh)*s
	op := &ebiten.DrawImageOptions{Filter: ebiten.FilterLinear}
	op.GeoM.Scale(s, s)
	op.GeoM.Translate(x+(w-dw)/2, y+(h-dh)/2)
	op.ColorScale.ScaleAlpha(alpha)
	dst.DrawImage(src, op)
}

func (a *App) drawWrappedText(screen *ebiten.Image, s string, face *text.GoTextFace, x, y, maxW, lineH float64, c color.Color) {
	words := strings.Fields(s)
	if len(words) == 0 {
		return
	}
	line := words[0]
	for _, word := range words[1:] {
		next := line + " " + word
		if w, _ := text.Measure(next, face, 0); w <= maxW {
			line = next
			continue
		}
		a.text(screen, line, face, x, y, c, false)
		y += lineH
		line = word
	}
	a.text(screen, line, face, x, y, c, false)
}

// viewScaleAt 折叠 vfx/scale view 事件得到画布缩放（StaticCamera.UpdateScale：
// 进行中的事件从上一事件终值缓动到自身目标）。
func (a *App) viewScaleAt(beat float64) (float64, float64) {
	sx, sy := 1.0, 1.0
	lx, ly := 1.0, 1.0
	for _, e := range a.viewScales {
		if beat < e.beat {
			continue
		}
		prog := 1.0
		if e.length > 0 {
			prog = math.Min((beat-e.beat)/e.length, 1)
		}
		switch e.axis {
		case 1:
			sx = Ease(e.ease, lx, e.x, prog)
		case 2:
			sy = Ease(e.ease, ly, e.y, prog)
		default:
			sx = Ease(e.ease, lx, e.x, prog)
			sy = Ease(e.ease, ly, e.y, prog)
		}
		if prog >= 1 {
			switch e.axis {
			case 1:
				lx = e.x
			case 2:
				ly = e.y
			default:
				lx, ly = e.x, e.y
			}
		}
	}
	return sx, sy
}

// drawFlash：vfx/flash 是单一覆盖层（HS Fade 语义）——按拍序折叠，
// 最后一个已开始的事件决定当前颜色（事件结束后停在其终色），
// 不能把多个事件叠画（先前事件的不透明终色会永久压住画面）。
func (a *App) drawFlash(screen *ebiten.Image, beat float64) {
	var c [4]float64
	hit := false
	for _, f := range a.flashes {
		if beat < f.beat || f.length <= 0 {
			continue
		}
		u := math.Min((beat-f.beat)/f.length, 1)
		for i := range c {
			c[i] = f.c0[i] + (f.c1[i]-f.c0[i])*u
		}
		hit = true
	}
	if hit && c[3] > 0 {
		vector.DrawFilledRect(screen, 0, 0, ScreenW, ScreenH, color.RGBA{
			uint8(c[0] * 255 * c[3]), uint8(c[1] * 255 * c[3]), uint8(c[2] * 255 * c[3]), uint8(c[3] * 255),
		}, false)
	}
}

func (a *App) drawPlaceholder(screen *ebiten.Image, id string) {
	screen.Fill(color.RGBA{40, 40, 52, 255})
	a.text(screen, id, a.faceBig, ScreenW/2, ScreenH/2-40, color.RGBA{210, 210, 225, 255}, true)
	a.text(screen, "This minigame is not ported yet; the song continues.", a.faceMid, ScreenW/2, ScreenH/2+20, color.RGBA{160, 160, 175, 255}, true)
}

func (a *App) drawDebug(screen *ebiten.Image, t, beat float64) {
	white := color.RGBA{235, 235, 245, 255}
	lines := []string{
		fmt.Sprintf("songPos %8.3fs  beat %7.3f", t, beat),
		fmt.Sprintf("tps %.0f fps %.0f", ebiten.ActualTPS(), ebiten.ActualFPS()),
	}
	if a.cond != nil {
		lines = append(lines, fmt.Sprintf("drift %+6.1fms", a.cond.Drift()*1000))
	}
	n := 0
	for _, in := range a.inputs {
		if !in.judged {
			n++
		}
	}
	lines = append(lines, fmt.Sprintf("actions %d/%d  inputs left %d", a.actIdx, len(a.actions), n))
	for i, s := range lines {
		a.text(screen, s, a.faceSmall, 20, 40+float64(i)*18, white, false)
	}
}

func (a *App) text(screen *ebiten.Image, s string, face *text.GoTextFace, x, y float64, c color.Color, center bool) {
	if center {
		w, _ := text.Measure(s, face, 0)
		x -= w / 2
	}
	op := &text.DrawOptions{}
	op.GeoM.Translate(x, y)
	op.ColorScale.ScaleWithColor(c)
	text.Draw(screen, s, face, op)
}

func (a *App) Layout(_, _ int) (int, int) { return ScreenW, ScreenH }

// Stats 是装载结果摘要（测试/诊断用）。
