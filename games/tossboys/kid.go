package tossboys

import "hsdemo/engine"

type kidState struct {
	ctx       *engine.Ctx
	path      string
	prefix    string
	action    int
	crouch    bool
	preparing bool
}

func newKid(ctx *engine.Ctx, path, prefix string, action int) *kidState {
	return &kidState{ctx: ctx, path: path, prefix: prefix, action: action}
}

func (k *kidState) state(name string, beat float64) {
	if k == nil || k.path == "" {
		return
	}
	k.ctx.Scene.PlayState(k.path, k.prefix+name, beat, 0.5)
}

func (k *kidState) hitBall(beat float64, hit bool) {
	if k == nil {
		return
	}
	if hit {
		if k.crouch {
			k.state("CrouchHit", beat)
		} else {
			k.state("Hit", beat)
		}
	} else {
		state, playing := k.ctx.Scene.StateInfo(k.path, beat)
		if playing && (state == k.prefix+"Whiff" || state == k.prefix+"Miss") {
			return
		}
		k.state("Whiff", beat)
		k.ctx.Sound("whiff")
	}
	k.preparing = false
}

func (k *kidState) bop(beat float64) {
	if k == nil || k.crouch || k.preparing {
		return
	}
	k.state("Bop", beat)
}

func (k *kidState) crouchPrepare(beat float64) {
	if k == nil {
		return
	}
	k.state("Crouch", beat)
	k.crouch = true
}

func (k *kidState) popBall(beat float64) {
	if k == nil {
		return
	}
	k.state("Slap", beat)
	k.preparing = false
}

func (k *kidState) popBallPrepare(beat float64) {
	if k == nil || k.preparing {
		return
	}
	k.state("PrepareHand", beat)
	k.preparing = true
}

func (k *kidState) miss(beat float64) {
	if k != nil {
		k.state("Miss", beat)
	}
}

func (k *kidState) barely(beat float64) {
	if k != nil {
		k.state("Barely", beat)
	}
}
