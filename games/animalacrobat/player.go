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
		shadowMul: 1,
	}
	m.jumpActive = true
	m.playPlayer("PlayerJump", beat)
}

func (m *Module) startJumpFromObstacle(beat float64, ob *acrobatObstacle) {
	fromX := ob.x + ob.gripX
	fromY := ob.gripY
	muted := m.muteRelease
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
			height -= 0.2
		}
	}
	rot := playerRotateJump
	shadowMul := 1.4
	if ob.kind == kindGiraffe {
		shadowMul = 2.2
	}
	if next := m.nextObstacleAfter(ob); next != nil && next.kind == kindGorilla {
		rot = playerRotateArc
		shadowMul = 1.4
	}
	if ob.kind == kindGiraffe && !muted {
		m.startTrail(m.ctx.BeatToTime(beat), fromX, fromY)
	}
	m.jump = playerJump{
		start: beat, dur: dur, fromX: fromX, toX: toX,
		fromY: fromY, toY: toY, height: height, land: ob.end,
		rotate: rot, shadowMul: shadowMul,
	}
	m.jumpActive = true
	m.ctx.Scene.SetActive(m.player, true)
	m.playPlayer("PlayerJump", beat)
	if muted {
		m.muteRelease = false
	} else {
		m.spawnReleaseParticle(beat, fromX, fromY)
	}
	if ob.end {
		m.ctx.SoundAt(beat+dur, "land", 1)
		m.ctx.At(beat+dur, func() {
			m.stopTrail(m.ctx.BeatToTime(beat + dur))
			m.playPlayer("PlayerLand", beat+dur)
		})
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
		// SuperCurveObject.GetPathPositionFromBeat uses this exact parabola:
		// LerpUnclamped(start,end,t) plus (-(2t-1)^2+1)*height on Y.
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
	m.updatePlayerVisuals(beat)
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

func (m *Module) resetPlayerVisuals() {
	m.ctx.Scene.SetSpinOver(m.player, 0)
	if m.playerShadow != "" {
		m.ctx.Scene.SetActive(m.playerShadow, true)
		m.ctx.Scene.SetScaleOver(m.playerShadow, m.shadowScale[0], m.shadowScale[1])
	}
}

func (m *Module) updatePlayerVisuals(beat float64) {
	startAngle := num(m.playerNums, "_jumpStartAngle", 120)
	m.ctx.Scene.SetSpinOver(m.player, playerRotationAt(m.jump, beat, startAngle)*math.Pi/180)
	if m.playerShadow == "" {
		return
	}
	sx, sy := playerShadowScaleAt(m.jump, beat, m.shadowScale[0], m.shadowScale[1], m.landingShadowBeats())
	m.ctx.Scene.SetActive(m.playerShadow, true)
	m.ctx.Scene.SetScaleOver(m.playerShadow, sx, sy)
}

func (m *Module) landingShadowBeats() float64 {
	end := m.jump.start + m.jump.dur
	secPerBeat := m.ctx.SecPerBeat(end)
	if secPerBeat <= 0 {
		return 0.5
	}
	return 0.5 / secPerBeat
}

func playerRotationAt(j playerJump, beat, startAngle float64) float64 {
	if j.dur <= 0 {
		return 0
	}
	switch j.rotate {
	case playerRotateArc:
		if beat >= j.start+j.dur {
			return 0
		}
		return lerp(startAngle, 360, clamp01((beat-j.start)/j.dur))
	case playerRotateJump:
		spinStart := j.start + j.dur - 1
		spinEnd := spinStart + 0.5
		if beat < spinStart {
			length := j.dur - 1
			if length <= 0 {
				return 360
			}
			return lerp(startAngle, 360, clamp01((beat-j.start)/length))
		}
		if beat < spinEnd {
			return lerp(0, 720, clamp01((beat-spinStart)/0.5))
		}
	}
	return 0
}

func playerShadowScaleAt(j playerJump, beat, defaultX, defaultY, landingBeats float64) (float64, float64) {
	if j.dur <= 0 || j.shadowMul <= 0 {
		return defaultX, defaultY
	}
	end := j.start + j.dur
	if j.land && beat >= end {
		if landingBeats <= 0 {
			return defaultX * 12, defaultY * 12
		}
		u := clamp01((beat - end) / landingBeats)
		e := 1 - (1-u)*(1-u)
		return lerp(defaultX, defaultX*12, e), lerp(defaultY, defaultY*12, e)
	}
	spinStart := end - 1
	if beat < spinStart {
		return defaultX * j.shadowMul, defaultY * j.shadowMul
	}
	if beat < end {
		u := clamp01((beat - spinStart) / (end - spinStart))
		e := 1 - (1-u)*(1-u)
		return lerp(defaultX*j.shadowMul, defaultX, e), lerp(defaultY*j.shadowMul, defaultY, e)
	}
	return defaultX, defaultY
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
