package animalacrobat

import (
	"image/color"
	"math"

	"github.com/hajimehoshi/ebiten/v2"
)

func (m *Module) startInitialJump(beat float64, first *acrobatObstacle) {
	startX, startY := nodePos(m.ctx.Assets, m.player)
	m.playerX, m.playerY = startX, startY
	dist := num(m.playerNums, "_jumpDistanceStart", defaultJumpDistanceStart)
	m.jump = playerJump{
		start: beat, dur: 1, fromX: startX, toX: startX + dist,
		fromY: startY, toY: startY, height: num(m.playerNums, "_jumpHeightInitial", defaultJumpHeightInitial),
	}
	m.jumpActive = true
	m.playPlayer("PlayerJump", beat)
}

func (m *Module) startJumpFromObstacle(beat float64, ob *acrobatObstacle) {
	fromX := ob.x + ob.gripX
	fromY := ob.gripY
	dur := 2.0
	dist := num(m.playerNums, "_jumpDistance", defaultJumpDistance)
	height := num(m.playerNums, "_jumpHeight", defaultJumpHeight)
	if ob.kind == kindMonkeysLong {
		dist = 2.2
		height = 2.5
	}
	if ob.kind == kindGiraffe {
		dur = 4
		dist = num(m.playerNums, "_jumpDistanceGiraffe", defaultJumpDistanceGiraffe)
		height = num(m.playerNums, "_jumpHeightGiraffe", defaultJumpHeightGiraffe)
	}
	toX, toY := fromX+dist, 0.0
	if next := m.nextObstacleAfter(ob); next != nil {
		toX = next.x + next.gripX
		toY = next.gripY
		if next.kind == kindGorilla {
			toX = next.x + next.endX
			toY = next.endY
		}
	}
	m.jump = playerJump{start: beat, dur: dur, fromX: fromX, toX: toX, fromY: fromY, toY: toY, height: height, land: ob.end}
	m.jumpActive = true
	m.ctx.Scene.SetActive(m.player, true)
	m.playPlayer("PlayerJump", beat)
	if ob.end {
		m.ctx.SoundAt(beat+dur, "land", 1)
		m.ctx.At(beat+dur, func() { m.playPlayer("PlayerLand", beat+dur) })
	}
}

func (m *Module) nextObstacleAfter(ob *acrobatObstacle) *acrobatObstacle {
	for i, cand := range m.animals {
		if cand == ob && i+1 < len(m.animals) {
			return m.animals[i+1]
		}
	}
	return nil
}

func (m *Module) updatePlayer(beat float64) {
	if m.jumpActive {
		u := clamp01((beat - m.jump.start) / m.jump.dur)
		x := m.jump.fromX + (m.jump.toX-m.jump.fromX)*u
		// TODO(animalAcrobat): PlayerAcrobat uses its own jump coroutine curves.
		// This preserves the serialized distances/heights but still needs a
		// curve-by-curve pass before the README simplification can be removed.
		y := m.jump.fromY + (m.jump.toY-m.jump.fromY)*u + 4*m.jump.height*u*(1-u)
		m.playerX, m.playerY = x, y
		m.ctx.Scene.SetPosOver(m.player, x, y)
		if u >= 1 {
			m.jumpActive = false
			if !m.jump.land {
				m.playPlayer("PlayerAir", beat)
			}
		}
	} else if m.holding == nil {
		m.ctx.Scene.SetPosOver(m.player, m.playerX, m.playerY)
	}
}

func (m *Module) updateCamera(beat float64) {
	target := m.playerX - num(m.gameNums, "_jumpStartCameraDistance", defaultJumpStartCameraDelta)
	if len(m.animals) > 0 && beat < m.animals[0].beat-1 {
		target = 0
	}
	// TODO(animalAcrobat): this follows the player using AnimalAcrobat's
	// serialized smooth speed; the Unity BgTileManager recycling and camera
	// coroutine still need a dedicated parity pass.
	speed := num(m.gameNums, "_cameraSmoothSpeed", 10)
	if speed <= 0 {
		m.cameraX = target
		return
	}
	m.cameraX += (target - m.cameraX) * math.Min(1, speed/60)
}

func (m *Module) updateAutoBop(beat float64) {
	cur := int(math.Floor(beat))
	if cur == m.lastBop || m.holding != nil || m.jumpActive {
		return
	}
	m.lastBop = cur
	if m.ctx.PressingNow() {
		return
	}
	m.playPlayer("PlayerBop", float64(cur))
}

func (m *Module) playPlayer(state string, beat float64) {
	m.ctx.Scene.PlayState(m.player, state, beat, animScale)
}

func (m *Module) emitSparkle(beat, x, y float64, col color.NRGBA) {
	// Temporary ParticleSystem stand-in; README tracks this as a known
	// simplification until the serialized emission curves are ported.
	m.sparkles = append(m.sparkles, sparkle{beat: beat, x: x, y: y, col: col})
}

func (m *Module) drawSparkles(screen *ebiten.Image, beat float64) {
	alive := m.sparkles[:0]
	for _, sp := range m.sparkles {
		age := beat - sp.beat
		if age < 0 || age > 1.2 {
			if age <= 1.2 {
				alive = append(alive, sp)
			}
			continue
		}
		alpha := 1 - age/1.2
		col := sp.col
		col.A = byte(float64(col.A) * alpha)
		x, y := screenPoint(m.proj, sp.x-m.cameraWX, sp.y-m.cameraWY)
		drawSparkle(screen, x, y, float32(26*(1+age)), col)
		alive = append(alive, sp)
	}
	m.sparkles = alive
}
