package fanclub

const (
	arisaFacePrefix  = "Animations/Arisa/FacePoser/"
	backupFacePrefix = "Animations/BackDancers/FacePoser/"
)

func (m *Module) resetFaceposers() {
	m.setFaceposerVisible(m.arisa, false)
	m.setFaceposerVisible(m.blue, false)
	m.setFaceposerVisible(m.orange, false)
}

func (m *Module) setFaceposerVisible(root string, enable bool) {
	face := root + "/idol_head/FacePoser"
	base := root + "/idol_head"
	m.ctx.Scene.SetActive(face, enable)
	m.ctx.Scene.SetRenderOver(base, !enable)
}

func (m *Module) setArisaFaceposer(enable, mouthOn, eyeOn bool, mouth, mouthEnd, eyeL, eyeR int, eyeX, eyeY, beat, length float64) {
	m.setFaceposerVisible(m.arisa, enable)
	if eyeOn {
		m.ctx.Scene.PlayLayer("arisaEyeTarget", m.arisa, arisaEyeTargetClip(eyeX, eyeY), beat, m.ctx.SecPerBeat(beat))
		m.ctx.Scene.PlayLayerNormalized("arisaEyeL", m.arisa, arisaFacePrefix+"EyeLeft", eyeNorm(eyeL, 6))
		m.ctx.Scene.PlayLayerNormalized("arisaEyeR", m.arisa, arisaFacePrefix+"EyeRight", eyeNorm(eyeR, 6))
	}
	if mouthOn {
		m.ctx.Scene.PlayLayer("arisaMouth", m.arisa, arisaMouthClip(mouth), beat, m.ctx.SecPerBeat(beat))
		m.ctx.At(beat+length, func() {
			m.ctx.Scene.PlayLayer("arisaMouth", m.arisa, arisaMouthClip(mouthEnd), beat+length, m.ctx.SecPerBeat(beat+length))
		})
	}
}

func arisaMouthClip(shape int) string {
	switch shape {
	case 1:
		return arisaFacePrefix + "MouthA"
	case 2:
		return arisaFacePrefix + "MouthE"
	case 3:
		return arisaFacePrefix + "MouthI"
	case 4:
		return arisaFacePrefix + "MouthO"
	case 5:
		return arisaFacePrefix + "MouthU"
	case 6:
		return arisaFacePrefix + "MouthFrown"
	default:
		return arisaFacePrefix + "MouthNormal"
	}
}

func backupMouthClip(shape int) string {
	switch shape {
	case 1:
		return backupFacePrefix + "MouthA"
	case 2:
		return backupFacePrefix + "MouthE"
	case 3:
		return backupFacePrefix + "MouthI"
	case 4:
		return backupFacePrefix + "MouthO"
	case 5:
		return backupFacePrefix + "MouthU"
	case 6:
		return backupFacePrefix + "MouthFrown"
	default:
		return backupFacePrefix + "MouthNormal"
	}
}

func backupEyeClip(right bool) string {
	if right {
		return backupFacePrefix + "EyeRight"
	}
	return backupFacePrefix + "EyeLeft"
}

func arisaEyeTargetClip(x, y float64) string {
	const dead = 0.05
	x, y = clamp01Signed(x), clamp01Signed(y)
	if abs(x) < dead && abs(y) < dead {
		return arisaFacePrefix + "EyeMiddle"
	}
	if abs(x) >= abs(y) {
		if x > 0 {
			return arisaFacePrefix + "EyeEast"
		}
		return arisaFacePrefix + "EyeWest"
	}
	if y > 0 {
		if y > 0.75 && abs(x) < 0.3 {
			return arisaFacePrefix + "EyeNorthRaised"
		}
		return arisaFacePrefix + "EyeNorth"
	}
	return arisaFacePrefix + "EyeSouth"
}

func eyeNorm(shape, maxShape int) float64 {
	if shape < 0 {
		shape = 0
	} else if shape > maxShape {
		shape = maxShape
	}
	return float64(shape) / float64(maxShape)
}
