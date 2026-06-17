package fanclub

import (
	"math"

	"hsdemo/engine"
	"hsdemo/kart"
)

const dancerAnimCount = 16

type dancer struct {
	path, root, shadow string
	clapEffect         string
	winkEffect         string
	stepDistance       float64
	startPos           float64
	rootY              float64
	startStepBeat      float64
	stepLength         float64
	exiting            bool
	active             bool
	jumpStart          float64
}

func newDancer(ctx *engine.Ctx, path, root, shadow string) dancer {
	d := dancer{
		path: path, root: root, shadow: shadow,
		clapEffect:   path + "/Effect_IdolCrap",
		winkEffect:   path + "/Effect_IdolWinkArr",
		stepDistance: 1, startStepBeat: math.Inf(1), stepLength: 16,
		jumpStart: math.Inf(-1),
	}
	for _, c := range ctx.Assets.Extra.Components {
		if c.Path != path {
			continue
		}
		d.stepDistance = numDefault(c.Nums, "stepDistance", d.stepDistance)
		d.startPos = numDefault(c.Nums, "startPostion", d.startPos)
		d.rootY = numDefault(c.Nums, "rootYPos", d.rootY)
		if p := c.Refs["rootTransform"]; p != "" {
			d.root = p
		}
		if p := c.Refs["shadow"]; p != "" {
			d.shadow = p
		}
		if p := c.Refs["clapEffect"]; p != "" {
			d.clapEffect = p
		}
		if p := c.Refs["winkEffect"]; p != "" {
			d.winkEffect = p
		}
	}
	return d
}

func (d *dancer) applyActive(sc *kart.SceneInst) {
	sc.SetActive(d.path, d.active)
	sc.SetActive(d.root, d.active)
	sc.SetActive(d.shadow, d.active)
}

func (d *dancer) startEntrance(sc *kart.SceneInst, beat, length float64, exit bool) {
	d.active = true
	d.applyActive(sc)
	d.startStepBeat, d.stepLength, d.exiting = beat, length, exit
	if exit {
		sc.SetPosOver(d.root, d.startPos+d.stepDistance*dancerAnimCount, d.rootY)
	} else {
		sc.SetPosOver(d.root, d.startPos, d.rootY)
	}
}

func (d *dancer) finishEntrance(sc *kart.SceneInst, exit bool) {
	d.exiting = exit
	if exit {
		sc.SetPosOver(d.root, d.startPos, d.rootY)
		d.active = false
	} else {
		sc.SetPosOver(d.root, d.startPos+d.stepDistance*dancerAnimCount, d.rootY)
		d.active = true
	}
	d.applyActive(sc)
	d.startStepBeat = math.Inf(1)
}

func (d *dancer) update(m *Module, beat float64) {
	sc := m.ctx.Scene
	if beat >= d.startStepBeat+d.stepLength {
		d.finishEntrance(sc, d.exiting)
	}
	if beat >= d.startStepBeat && beat < d.startStepBeat+d.stepLength {
		seg := d.stepLength / dancerAnimCount
		cur := int((beat - d.startStepBeat) / seg)
		start := d.startStepBeat + seg*float64(cur)
		prog := (beat - start - 0.75) / seg
		if d.exiting {
			cur = dancerAnimCount - cur
			prog = (beat - start) / seg
		}
		prog = clamp01(prog * 4)
		clipStart := start
		if d.exiting {
			clipStart -= 0.75
		}
		state := "WalkB"
		if cur%2 != 0 {
			state = "WalkA"
		}
		d.playState(sc, state, clipStart, m.ctx.SecPerBeat(clipStart))
		x := d.startPos + d.stepDistance*float64(cur)
		if d.exiting {
			x -= d.stepDistance * prog
		} else {
			x += d.stepDistance * prog
		}
		sc.SetPosOver(d.root, x, d.rootY)
		return
	}
	if !d.active {
		return
	}
	if beat >= d.jumpStart && beat < d.jumpStart+1 {
		yw := parabola01(beat - d.jumpStart)
		sc.SetPosOver(d.root, d.startPos+d.stepDistance*dancerAnimCount, d.rootY+2*yw+0.25)
		s := (1 - yw*0.8) * 1.18
		sc.SetScaleOver(d.shadow, s, s)
		d.playState(sc, "Jump", d.jumpStart, m.ctx.SecPerBeat(d.jumpStart))
	} else {
		sc.SetPosOver(d.root, d.startPos+d.stepDistance*dancerAnimCount, d.rootY)
		sc.SetScaleOver(d.shadow, 1.18, 1.18)
	}
}

func (d *dancer) playState(sc *kart.SceneInst, state string, beat, sec float64) {
	if d.active {
		sc.PlayState(d.path, state, beat, sec)
	}
}

func (d *dancer) doJump(beat float64) {
	if !d.active || !math.IsInf(d.startStepBeat, 1) {
		return
	}
	d.jumpStart = beat
}

func (d *dancer) playAnim(m *Module, beat, length float64, typ int) {
	if !d.active || !math.IsInf(d.startStepBeat, 1) {
		return
	}
	switch typ {
	case idolAnimBop:
		d.playState(m.ctx.Scene, "Beat", beat, m.ctx.SecPerBeat(beat))
	case idolAnimPeaceVocal, idolAnimPeace:
		d.playState(m.ctx.Scene, "Peace", beat, m.ctx.SecPerBeat(beat))
	case idolAnimClap:
		d.playState(m.ctx.Scene, "Crap", beat, m.ctx.SecPerBeat(beat))
		m.effects.spawnDancerClap(m, d, beat)
	case idolAnimJump:
		d.doJump(beat)
	case idolAnimSquat:
		d.playState(m.ctx.Scene, "Squat0", beat, m.ctx.SecPerBeat(beat))
		m.ctx.At(beat+length, func() { d.playState(m.ctx.Scene, "Squat1", beat+length, m.ctx.SecPerBeat(beat+length)) })
	case idolAnimWink:
		d.playState(m.ctx.Scene, "Wink0", beat, m.ctx.SecPerBeat(beat))
		winkBeat := m.winkEffectBeat(beat + length)
		m.ctx.At(beat+length, func() { d.playState(m.ctx.Scene, "Wink1", beat+length, m.ctx.SecPerBeat(beat+length)) })
		m.effects.spawnDancerWink(m, d, winkBeat)
	case idolAnimDab:
		d.playState(m.ctx.Scene, "Dab", beat, m.ctx.SecPerBeat(beat))
	}
}

func (d *dancer) setFaceposer(m *Module, enable, mouthOn, eyeOn bool, mouth, mouthEnd, eyeL, eyeR int, beat, length float64) {
	m.setFaceposerVisible(d.path, enable)
	if eyeOn {
		m.ctx.Scene.PlayLayerNormalized(d.path+":eyeL", d.path, backupEyeClip(false), eyeNorm(eyeL, 3))
		m.ctx.Scene.PlayLayerNormalized(d.path+":eyeR", d.path, backupEyeClip(true), eyeNorm(eyeR, 3))
	}
	if mouthOn {
		m.ctx.Scene.PlayLayer(d.path+":mouth", d.path, backupMouthClip(mouth), beat, m.ctx.SecPerBeat(beat))
		m.ctx.At(beat+length, func() {
			m.ctx.Scene.PlayLayer(d.path+":mouth", d.path, backupMouthClip(mouthEnd), beat+length, m.ctx.SecPerBeat(beat+length))
		})
	}
}
