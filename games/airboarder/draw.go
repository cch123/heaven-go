package airboarder

import (
	"image/color"
	"math"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"hsdemo/engine"
)

func (m *Module) drawTemporary2D(screen *ebiten.Image, beat float64) {
	sky, cloud := m.bgAt(beat)
	floor, stripe := m.floorAt(beat)
	screen.Fill(rgba(sky))
	m.drawTemporaryForeground(screen, beat, rgba(cloud), rgba(floor), rgba(stripe))
}

func (m *Module) drawTemporaryFallbackBG(screen *ebiten.Image, beat float64) {
	sky, _ := m.bgAt(beat)
	screen.Fill(rgba(sky))
}

func (m *Module) drawTemporaryForeground(screen *ebiten.Image, beat float64, cloud, floor, stripe color.RGBA) {
	m.drawClouds(screen, beat, cloud)
	m.drawFloor(screen, beat, floor, stripe)
	m.drawDog(screen, beat)
	for _, ob := range m.obstacles {
		m.drawObstacle(screen, ob, beat)
	}
	m.drawBoarder(screen, &m.cpu1, beat, color.RGBA{R: 235, G: 70, B: 120, A: 255})
	m.drawBoarder(screen, &m.cpu2, beat, color.RGBA{R: 80, G: 170, B: 245, A: 255})
	m.drawBoarder(screen, &m.player, beat, color.RGBA{R: 245, G: 215, B: 80, A: 255})
}

func (m *Module) drawClouds(screen *ebiten.Image, beat float64, c color.RGBA) {
	for i := 0; i < 7; i++ {
		x := float32(math.Mod(float64(i)*190-beat*20, engine.ScreenW+260) - 130)
		y := float32(70 + i%3*45)
		vector.DrawFilledCircle(screen, x, y, 34, c, true)
		vector.DrawFilledCircle(screen, x+38, y-10, 44, c, true)
		vector.DrawFilledCircle(screen, x+88, y, 36, c, true)
		vector.DrawFilledRect(screen, x, y, 92, 30, c, true)
	}
}

func (m *Module) drawFloor(screen *ebiten.Image, beat float64, floor, stripe color.RGBA) {
	y := float32(engine.ScreenH - 215)
	vector.DrawFilledRect(screen, 0, y, engine.ScreenW, 215, floor, true)
	const stripeSpacing = 128
	off := m.floorStripeOffset(beat, stripeSpacing)
	for x := -160.0 - off; x < engine.ScreenW+180; x += stripeSpacing {
		vector.StrokeLine(screen, float32(x), float32(engine.ScreenH), float32(x+180), y, 18, stripe, true)
		vector.StrokeLine(screen, float32(x+40), float32(engine.ScreenH), float32(x+220), y, 5, stripe, true)
	}
	vector.StrokeLine(screen, 0, y, engine.ScreenW, y, 5, color.RGBA{R: 255, G: 255, B: 255, A: 160}, true)
}

func (m *Module) drawDog(screen *ebiten.Image, beat float64) {
	x := float32(585 + 10*math.Sin(beat*math.Pi*2))
	y := float32(250 + 7*math.Sin(beat*math.Pi*4))
	body := color.RGBA{R: 110, G: 75, B: 45, A: 255}
	dark := color.RGBA{R: 55, G: 35, B: 25, A: 255}
	vector.DrawFilledCircle(screen, x, y, 28, body, true)
	vector.DrawFilledCircle(screen, x+27, y-16, 18, body, true)
	vector.DrawFilledCircle(screen, x+33, y-24, 6, dark, true)
	vector.StrokeLine(screen, x-28, y-2, x-54, y-22+float32(8*math.Sin(beat*math.Pi*5)), 8, dark, true)
}

func (m *Module) drawObstacle(screen *ebiten.Image, ob *obstacle, beat float64) {
	u := (beat - ob.appearBeat) / 40
	if u < -0.05 || u > 1.05 {
		return
	}
	x := float32(lerp(engine.ScreenW+180, -260, clamp01(u)))
	if ob.shake && beat < ob.effectBeat+0.5 {
		x += float32(math.Sin((beat-ob.effectBeat)*80) * 8)
	}
	y := float32(engine.ScreenH - 280)
	col := color.RGBA{R: 210, G: 42, B: 75, A: 255}
	if ob.broken {
		col = color.RGBA{R: 80, G: 80, B: 100, A: 230}
	}
	switch ob.kind {
	case obstacleJump:
		vector.DrawFilledRect(screen, x-58, y-18, 116, 36, col, true)
		vector.DrawFilledRect(screen, x-45, y-88, 22, 72, col, true)
		vector.DrawFilledRect(screen, x+22, y-88, 22, 72, col, true)
		if ob.broken {
			vector.StrokeLine(screen, x-50, y-76, x+48, y-18, 5, color.Black, true)
		}
	default:
		h := float32(128)
		if ob.kind == obstacleCrouch {
			h = 88
		}
		vector.DrawFilledRect(screen, x-74, y-h, 22, h, col, true)
		vector.DrawFilledRect(screen, x+52, y-h, 22, h, col, true)
		vector.DrawFilledRect(screen, x-74, y-h, 148, 24, col, true)
		if ob.broken {
			vector.StrokeLine(screen, x-68, y-h+8, x+70, y-8, 5, color.Black, true)
		}
	}
}

func (m *Module) drawBoarder(screen *ebiten.Image, b *boarder, beat float64, suit color.RGBA) {
	x := float32(engine.ScreenW/2 + b.x*64)
	y := float32(engine.ScreenH - 245 - b.y*35)
	age := beat - b.poseBeat
	scale := float32(1)
	switch b.pose {
	case poseBop:
		y -= float32(math.Sin(clamp01(age/0.5)*math.Pi) * 18)
	case poseDuck:
		scale = 0.78
		y += 18
	case poseCharge, poseHold:
		scale = 0.7
		y += 24
	case poseJump:
		y -= float32(100 * math.Sin(clamp01(age/1.2)*math.Pi))
	case poseLetsGo:
		y -= float32(22 * math.Sin(clamp01(age/1.0)*math.Pi))
	case poseHit1, poseHit2:
		x += float32(math.Sin(age*35) * 10)
		y += 18
		scale = 0.72
	}
	board := color.RGBA{R: 30, G: 35, B: 55, A: 255}
	vector.StrokeLine(screen, x-58*scale, y+54*scale, x+66*scale, y+40*scale, 13*scale, board, true)
	flame := color.RGBA{R: 255, G: 96, B: 35, A: 220}
	vector.DrawFilledCircle(screen, x-72*scale, y+58*scale, 13*scale, flame, true)
	vector.DrawFilledCircle(screen, x-88*scale, y+60*scale, 8*scale, color.RGBA{R: 255, G: 225, B: 60, A: 210}, true)

	vector.DrawFilledRect(screen, x-12*scale, y-6*scale, 34*scale, 56*scale, suit, true)
	vector.DrawFilledCircle(screen, x+5*scale, y-34*scale, 23*scale, suit, true)
	vector.DrawFilledCircle(screen, x+12*scale, y-38*scale, 4*scale, color.Black, true)
	vector.DrawFilledCircle(screen, x-10*scale, y-38*scale, 4*scale, color.Black, true)
	vector.StrokeLine(screen, x-5*scale, y+10*scale, x-35*scale, y+38*scale, 7*scale, suit, true)
	vector.StrokeLine(screen, x+18*scale, y+12*scale, x+50*scale, y+34*scale, 7*scale, suit, true)
}
