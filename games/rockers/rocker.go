package rockers

import "hsdemo/engine"

type rocker struct {
	mod    *Module
	path   string
	fxPath string
	jj     bool

	loops          []*engine.SoundLoopHandle
	lastPitches    [6]int
	lastGleeClub   bool
	lastSample     noteSample
	lastSampleTone int
	lastBendPitch  int
	bendSemi       int

	muted    bool
	strum    bool
	bending  bool
	together bool
}

func (r *rocker) init(mod *Module, path, fxPath string, jj bool) {
	r.mod, r.path, r.fxPath, r.jj = mod, path, fxPath, jj
	for i := range r.lastPitches {
		r.lastPitches[i] = -1
	}
}

func (r *rocker) play(state string, beat float64) {
	if r.mod == nil || r.mod.ctx == nil || r.mod.ctx.Scene == nil {
		return
	}
	if r.jj {
		state = "JJ" + state
	}
	r.mod.ctx.Scene.PlayState(r.path, state, beat, 0.5)
}

func (r *rocker) playFX(state string, beat float64) {
	if r.mod == nil || r.mod.ctx == nil || r.mod.ctx.Scene == nil || r.fxPath == "" {
		return
	}
	r.mod.ctx.Scene.SetActive(r.fxPath, true)
	r.mod.ctx.Scene.PlayState(r.fxPath, state, beat, 0.5)
}

func (r *rocker) stopSounds() {
	for _, loop := range r.loops {
		if loop != nil {
			loop.Stop()
		}
	}
	r.loops = nil
}

func (r *rocker) startLoops() {
	r.stopSounds()
	if r.lastSample.key != "" {
		r.loops = append(r.loops, r.mod.ctx.SoundLoopPitchHandle(r.lastSample.key, semitonePitch(r.lastSampleTone+r.bendSemi), 1))
	} else {
		dir := "normal"
		if r.lastGleeClub {
			dir = "gleeClub"
		}
		for i, semi := range r.lastPitches {
			if semi < 0 {
				continue
			}
			name := "strings/" + dir + "/" + dir + itoa1(i+1)
			r.loops = append(r.loops, r.mod.ctx.SoundLoopPitchHandle(name, semitonePitch(semi+r.bendSemi), stringVolume(len(r.lastPitches))))
		}
	}
}

func (r *rocker) bendLoopsTo(semi int, seconds float64) bool {
	idx := 0
	if r.lastSample.key != "" {
		if len(r.loops) == 0 || r.loops[0] == nil {
			return false
		}
		r.loops[0].RampPitch(semitonePitch(r.lastSampleTone+semi), seconds)
		return true
	}
	ok := false
	for _, base := range r.lastPitches {
		if base < 0 {
			continue
		}
		if idx < len(r.loops) && r.loops[idx] != nil {
			r.loops[idx].RampPitch(semitonePitch(base+semi), seconds)
			ok = true
		}
		idx++
	}
	return ok
}

func (r *rocker) strumStrings(gleeClub bool, pitches [6]int, sample noteSample, sampleTone int, disableFX, jump bool) {
	if r.strum {
		return
	}
	r.lastGleeClub, r.lastPitches, r.lastSample, r.lastSampleTone = gleeClub, pitches, sample, sampleTone
	r.bendSemi = 0
	r.muted = false
	r.strum = true
	r.startLoops()

	beat := r.mod.ctx.Beat()
	if r.together {
		if jump {
			r.play("Jump", beat)
		} else {
			r.play("ComeOnStrum", beat)
		}
		if !disableFX {
			state := "StrumStartRIght"
			if r.jj && jump {
				state = "StrumStartLeft"
			}
			r.playFX(state, beat)
		}
		return
	}
	r.play("Strum", beat)
	if !disableFX {
		r.playFX("StrumStart", beat)
	}
}

func (r *rocker) strumLast(disableFX, jump bool) {
	r.strumStrings(r.lastGleeClub, r.lastPitches, r.lastSample, r.lastSampleTone, disableFX, jump)
}

func (r *rocker) bendUp(pitch int) {
	if r.bending || !r.strum {
		return
	}
	r.bending = true
	r.lastBendPitch = pitch
	r.bendSemi = pitch
	if !r.bendLoopsTo(pitch, 0.05) {
		r.startLoops()
	}
	r.mod.ctx.Sound("bendUp")
	r.play("Bend", r.mod.ctx.Beat())
}

func (r *rocker) bendDown() {
	if !r.bending {
		return
	}
	r.bending = false
	r.bendSemi = 0
	if !r.bendLoopsTo(0, 0.05) {
		r.startLoops()
	}
	r.mod.ctx.Sound("bendDown")
	r.play("Unbend", r.mod.ctx.Beat())
}

func (r *rocker) mute(sound, noAnim bool) {
	r.strum = false
	r.bending = false
	r.bendSemi = 0
	r.stopSounds()
	if r.mod != nil && r.mod.ctx != nil && r.mod.ctx.Scene != nil && r.fxPath != "" {
		r.mod.ctx.Scene.SetActive(r.fxPath, false)
	}
	if sound {
		r.mod.ctx.Sound("mute")
	}
	if !noAnim {
		if r.together {
			r.play("ComeOnMute", r.mod.ctx.Beat())
		} else {
			r.play("Crouch", r.mod.ctx.Beat())
		}
	}
	r.muted = true
}

func (r *rocker) unHold(force bool) {
	if !r.muted && !force {
		return
	}
	r.muted = false
	if !r.together {
		r.play("UnCrouch", r.mod.ctx.Beat())
	}
}

func (r *rocker) prepareTogether(forceMute bool) {
	r.together = true
	if forceMute {
		r.play("ComeOnPrepare", r.mod.ctx.Beat())
		r.mute(true, true)
		return
	}
	r.play("ComeOnPrepareNoMute", r.mod.ctx.Beat())
	if r.strum {
		r.playFX("StrumRight", r.mod.ctx.Beat())
	}
}

func (r *rocker) returnBack() {
	r.together = false
	if r.jj {
		r.muted = false
		r.play("Return", r.mod.ctx.Beat())
		return
	}
	if r.strum {
		r.playFX("StrumIdle", r.mod.ctx.Beat())
	}
	if r.mod.ctx.PressingNow() || (r.mod.ctx.App.Autoplay && r.muted) {
		r.play("Crouch", r.mod.ctx.Beat())
		return
	}
	r.muted = false
	r.play("Return", r.mod.ctx.Beat())
}

func (r *rocker) miss() {
	if r.strum {
		return
	}
	if r.together {
		r.play("Miss", r.mod.ctx.Beat())
	} else {
		r.play("MissComeOn", r.mod.ctx.Beat())
	}
}
