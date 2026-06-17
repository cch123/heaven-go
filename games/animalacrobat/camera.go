package animalacrobat

import (
	"math"

	"hsdemo/engine"
	"hsdemo/kart"
)

type cameraTarget struct {
	x, y, z float64
}

func (m *Module) resetCamera(beat float64) {
	m.cameraX, m.cameraY, m.cameraZ = 0, 0, 0
	if target, ok := m.cameraTargetAt(beat); ok {
		m.cameraX, m.cameraY, m.cameraZ = target.x, target.y, target.z
	}
	m.cameraTSet = false
}

func (m *Module) updateCamera(t, beat float64) {
	target, ok := m.cameraTargetAt(beat)
	if !ok {
		target = cameraTarget{}
	}

	dt := 1.0 / 60.0
	if m.cameraTSet {
		if d := t - m.cameraT; d > 0 && d < 0.25 {
			dt = d
		}
	}
	m.cameraT, m.cameraTSet = t, true

	speed := num(m.gameNums, "_cameraSmoothSpeed", 10)
	if speed <= 0 {
		m.cameraX, m.cameraY, m.cameraZ = target.x, target.y, target.z
		return
	}
	alpha := clamp01(dt * speed)
	m.cameraX = lerp(m.cameraX, target.x, alpha)
	m.cameraY = lerp(m.cameraY, target.y, alpha)
	m.cameraZ = lerp(m.cameraZ, target.z, alpha)
}

func (m *Module) updateStickySpotlight(camZ float64) {
	if m.ctx == nil || m.ctx.Scene == nil || m.spotlight == "" {
		return
	}
	// SpotlightMain has Heaven Studio's StickyCanvas component. Unity moves it
	// to GameCamera.position + camera.forward*10 in LateUpdate, so it must cancel
	// camera x/y and keep a constant depth during AnimalAcrobat.CameraUpdate zooms.
	m.ctx.Scene.SetPosOver(m.spotlight, m.cameraWX, m.cameraWY)
	m.ctx.Scene.SetZOver(m.spotlight, camZ+kart.CamDist)
}

func (m *Module) cameraTargetAt(beat float64) (cameraTarget, bool) {
	if len(m.animals) == 0 {
		return cameraTarget{}, false
	}
	first := m.animals[0]
	if beat < first.beat-1-first.spec.holdPaddingStart {
		return cameraTarget{}, false
	}

	lastX := 0.0
	cameraHoldTime := 1.0
	lastWasGiraffe := false

	for idx, ob := range m.animals {
		distance := m.cameraDistance(idx, lastWasGiraffe)
		if ob.kind != kindGorilla {
			releaseBeat := ob.beat + holdLengthForKind(ob.kind)
			arrivalBeat := releaseBeat + 2
			if idx+1 < len(m.animals) {
				arrivalBeat = m.animals[idx+1].beat - m.animals[idx+1].spec.holdPaddingStart
			}
			if beat >= arrivalBeat {
				lastX += distance + cameraRotationDistance(ob)
				cameraHoldTime = 2
				if ob.kind == kindGiraffe {
					cameraHoldTime = 4
				}
				lastWasGiraffe = ob.kind == kindGiraffe
				continue
			}
		}
		return m.cameraTargetForAnimal(beat, idx, ob, lastX, distance, cameraHoldTime), true
	}
	return cameraTarget{x: lastX}, true
}

func (m *Module) cameraTargetForAnimal(beat float64, idx int, ob *acrobatObstacle, lastX, distance, cameraHoldTime float64) cameraTarget {
	holdLen := holdLengthForKind(ob.kind)
	releaseBeat := ob.beat + holdLen
	fullMovementDuration := holdLen + ob.spec.holdPadding + ob.spec.holdPaddingStart
	movementStartBeat := ob.beat - ob.spec.holdPaddingStart

	if ob.kind == kindGorilla {
		u := beatProgress(beat, movementStartBeat-cameraHoldTime, cameraHoldTime)
		return cameraTarget{x: lerpClamp(lastX, lastX+distance, u)}
	}

	normalizedHold := beatProgress(beat, movementStartBeat, fullMovementDuration)
	if normalizedHold < 0 {
		u := beatProgress(beat, movementStartBeat-cameraHoldTime, cameraHoldTime)
		return cameraTarget{x: lerpClamp(lastX, lastX+distance, u)}
	}

	rotDist := cameraRotationDistance(ob)
	startRotX := lastX + distance
	endRotX := startRotX + rotDist
	if beat >= releaseBeat {
		releaseProgress := (releaseBeat - movementStartBeat) / fullMovementDuration
		cutX := engine.Ease(ob.spec.ease, startRotX, endRotX, releaseProgress)
		targetX := endRotX + m.cameraGapAfter(ob.kind)
		arrivalBeat := releaseBeat + 2
		if idx+1 < len(m.animals) {
			arrivalBeat = m.animals[idx+1].beat - m.animals[idx+1].spec.holdPaddingStart
		}
		jumpProgress := clamp01(beatProgress(beat, releaseBeat, arrivalBeat-releaseBeat))
		camZ := 0.0
		if ob.kind == kindGiraffe && !m.monkeyMissed {
			camZ = m.giraffeZoomAt(jumpProgress)
		}
		return cameraTarget{x: lerp(cutX, targetX, jumpProgress), z: -camZ}
	}

	angle := engine.Ease(ob.spec.ease, 0, 180, normalizedHold) * math.Pi / 180
	return cameraTarget{
		x: engine.Ease(ob.spec.ease, startRotX, endRotX, normalizedHold),
		y: math.Sin(angle) * ob.gripY,
	}
}

func (m *Module) cameraDistance(idx int, lastWasGiraffe bool) float64 {
	if idx == 0 {
		return num(m.gameNums, "_jumpStartCameraDistance", defaultJumpStartCameraDelta)
	}
	if lastWasGiraffe {
		return num(m.gameNums, "_jumpDistanceGiraffe", defaultCameraJumpGiraffe)
	}
	return num(m.gameNums, "_jumpDistance", defaultCameraJumpDistance)
}

func (m *Module) cameraGapAfter(kind animalKind) float64 {
	if kind == kindGiraffe {
		return num(m.gameNums, "_jumpDistanceGiraffe", defaultCameraJumpGiraffe)
	}
	return num(m.gameNums, "_jumpDistance", defaultCameraJumpDistance)
}

func (m *Module) giraffeZoomAt(u float64) float64 {
	zoom := num(m.gameNums, "_giraffeCameraZoom", defaultGiraffeCameraZoom)
	switch {
	case u < 0.2:
		t := u / 0.2
		return zoom * t * t
	case u < 0.85:
		return zoom
	default:
		t := (u - 0.85) / 0.15
		return lerp(zoom, 0, t*t*t)
	}
}

func holdLengthForKind(kind animalKind) float64 {
	switch kind {
	case kindElephant:
		return 2
	case kindGiraffe:
		return 4
	case kindMonkeysLong:
		return 3
	case kindMonkeyShort:
		return 1
	case kindGorilla:
		return 4
	default:
		return 1
	}
}

func cameraRotationDistance(ob *acrobatObstacle) float64 {
	if ob == nil {
		return 0
	}
	rad := (ob.spec.fullRotRange + 180) * math.Pi / 180
	return math.Abs(math.Cos(rad) * ob.gripY * 2)
}

func beatProgress(beat, start, length float64) float64 {
	if length == 0 {
		if beat >= start {
			return 1
		}
		return 0
	}
	return (beat - start) / length
}

func lerp(start, end, t float64) float64 {
	return start + (end-start)*t
}

func lerpClamp(start, end, t float64) float64 {
	return lerp(start, end, clamp01(t))
}
