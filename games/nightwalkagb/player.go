package nightwalkagb

import (
	"math"

	"hsdemo/engine"
	"hsdemo/kmdata"
)

const (
	playerFlying = iota
	playerWalking
	playerJumping
	playerShocked
	playerFalling
	playerWhiffing
	playerFloating
	playerRolling
	playerHighJumping
	playerJumpingFall
	playerHighJumpingFall
)

type playYan struct {
	mod *Module

	root     string
	sprite   string
	star     string
	balloons []string

	jumpPath  jumpPath
	whiffPath jumpPath
	highPath  jumpPath

	state       int
	jumpBeat    float64
	walkBeat    float64
	fallBeat    float64
	fallStartY  float64
	hasFallen   bool
	expectUntil float64
}

func newPlayYan(m *Module, game kmdata.Component) playYan {
	py := playYan{
		mod: m, root: m.playerRoot, sprite: m.playerRoot + "/Sprite", star: m.playerRoot + "/Star",
		state: playerFlying,
		balloons: []string{
			m.playerRoot + "/Balloons/1Hold/1",
			m.playerRoot + "/Balloons/2Hold/2",
			m.playerRoot + "/Balloons/3Hold/3",
			m.playerRoot + "/Balloons/4Hold/4",
			m.playerRoot + "/Balloons/5Hold/5",
			m.playerRoot + "/Balloons/6Hold/6",
			m.playerRoot + "/Balloons/7Hold/7",
		},
	}
	for _, item := range game.Lists["jumpPaths"] {
		p := readPath(item)
		switch p.name {
		case "Jump":
			py.jumpPath = p
		case "Whiff":
			py.whiffPath = p
		case "highJump":
			py.highPath = p
		}
	}
	return py
}

func (p *playYan) reset(beat float64) {
	p.state = playerFlying
	p.jumpBeat, p.walkBeat, p.fallBeat = beat, beat, beat
	p.fallStartY, p.hasFallen = 0, false
	p.expectUntil = math.Inf(-1)
	p.mod.ctx.Scene.SetActive(p.star, false)
	p.mod.ctx.Scene.SetSpinOver(p.sprite, 0)
	p.mod.ctx.Scene.SetPosOver(p.root, 0, 0)
	p.mod.ctx.Scene.PlayDefaultState(p.root, beat, p.mod.ctx.SecPerBeat(beat))
	for _, b := range p.balloons {
		p.mod.ctx.Scene.PlayDefaultState(b, beat, p.mod.ctx.SecPerBeat(beat))
	}
	p.updateBalloons(beat)
}

func (p *playYan) update(beat float64) {
	switch p.state {
	case playerJumping:
		x, y := samplePath(p.jumpPath, math.Min(p.jumpBeat+p.jumpPath.duration, beat), p.jumpBeat)
		p.mod.ctx.Scene.SetPosOver(p.root, x, y)
		if beat >= p.jumpBeat+p.jumpPath.duration {
			p.walk()
		}
	case playerJumpingFall:
		x, y := samplePath(p.jumpPath, beat, p.jumpBeat)
		p.mod.ctx.Scene.SetPosOver(p.root, x, y)
		if beat >= p.jumpBeat+p.jumpPath.duration && !p.hasFallen {
			p.hasFallen = true
			p.mod.ctx.Sound("fall")
		}
	case playerWalking:
		p.mod.ctx.Scene.SetPosOver(p.root, 0, 0)
		p.mod.ctx.Scene.PlayState(p.root, "Walk", p.walkBeat, 0.5)
	case playerFlying:
		p.mod.ctx.Scene.SetPosOver(p.root, 0, 0)
	case playerFalling:
		y := engine.Ease(2, p.fallStartY, -12, norm(beat, p.fallBeat, 2))
		p.mod.ctx.Scene.SetPosOver(p.root, 0, y)
	case playerWhiffing:
		x, y := samplePath(p.whiffPath, math.Min(p.jumpBeat+0.5, beat), p.jumpBeat)
		p.mod.ctx.Scene.SetPosOver(p.root, x, y)
		if beat >= p.jumpBeat+0.5 {
			p.walk()
		}
	case playerFloating:
		y := engine.Ease(0, p.fallStartY, 12, norm(beat, p.fallBeat, 10))
		p.mod.ctx.Scene.SetPosOver(p.root, 0, y)
	case playerRolling:
		u := norm(beat, p.jumpBeat, 0.5)
		p.mod.ctx.Scene.SetSpinOver(p.sprite, -2*math.Pi*u)
	case playerHighJumping:
		x, y := samplePath(p.highPath, math.Min(p.jumpBeat+p.highPath.duration, beat), p.jumpBeat)
		p.mod.ctx.Scene.SetPosOver(p.root, x, y)
		if beat >= p.jumpBeat+p.highPath.duration {
			p.walk()
		}
	case playerHighJumpingFall:
		x, y := samplePath(p.highPath, beat, p.jumpBeat)
		p.mod.ctx.Scene.SetPosOver(p.root, x, y)
		if beat >= p.jumpBeat+p.highPath.duration && !p.hasFallen {
			p.hasFallen = true
			p.mod.ctx.Sound("fall")
		}
	}
}

func (p *playYan) expectingPrimary(beat float64) bool {
	return beat <= p.expectUntil
}

func (p *playYan) jump(beat float64, fall bool) {
	p.state = playerJumping
	if fall {
		p.state = playerJumpingFall
	}
	p.jumpBeat = beat
	p.hasFallen = false
	p.jumpPath.duration = 1 - engine.WinJust/p.mod.ctx.SecPerBeat(beat)
	p.mod.ctx.Scene.SetSpinOver(p.sprite, 0)
	p.mod.ctx.Scene.PlayState(p.root, "Jump", beat, 0.5)
	p.update(beat)
}

func (p *playYan) highJump(beat float64, fall, barely bool) {
	p.state = playerHighJumping
	if fall {
		p.state = playerHighJumpingFall
	}
	p.jumpBeat = beat
	p.hasFallen = false
	p.highPath.duration = 1.5 - engine.WinJust/p.mod.ctx.SecPerBeat(beat)
	if barely {
		p.highPath.height = 3.5
	} else {
		p.highPath.height = 4.5
	}
	p.mod.ctx.Scene.SetSpinOver(p.sprite, 0)
	p.mod.ctx.Scene.PlayState(p.root, "HighJump", beat, 0.5)
	p.update(beat)
}

func (p *playYan) roll(beat float64) {
	p.state = playerRolling
	p.jumpBeat = beat
	p.mod.ctx.Scene.PlayState(p.root, "Roll", beat, 0.5)
	p.update(beat)
}

func (p *playYan) whiff(beat float64) {
	p.state = playerWhiffing
	p.jumpBeat = beat
	p.mod.ctx.Scene.SetSpinOver(p.sprite, 0)
	p.mod.ctx.Scene.PlayState(p.root, "Jump", beat, 0.5)
	p.mod.ctx.Sound("whiff")
	p.update(beat)
}

func (p *playYan) walk() {
	if p.state == playerWalking {
		return
	}
	p.state = playerWalking
	p.walkBeat = p.mod.ctx.Beat()
	p.mod.ctx.Scene.SetSpinOver(p.sprite, 0)
}

func (p *playYan) shock(roll bool) {
	p.state = playerShocked
	state := "Shock"
	if roll {
		state = "RollShock"
	}
	p.mod.ctx.Scene.PlayState(p.root, state, p.mod.ctx.Beat(), 0.5)
	p.mod.ctx.Sound("shock")
	p.mod.ctx.Scene.SetSpinOver(p.sprite, 0)
}

func (p *playYan) fall(beat float64) {
	p.state = playerFalling
	p.fallBeat = beat
	p.fallStartY = p.currentY()
	p.mod.ctx.Scene.PlayState(p.root, "Jump", beat, 0.5)
	p.mod.ctx.Sound("fall")
	p.mod.ctx.Scene.SetSpinOver(p.sprite, 0)
	p.update(beat)
}

func (p *playYan) floatUp(beat float64) {
	p.state = playerFloating
	p.fallBeat = beat
	p.fallStartY = p.currentY()
	p.mod.ctx.Scene.PlayState(p.root, "Jump", beat, 0.5)
	p.mod.ctx.Scene.SetActive(p.star, true)
	p.mod.ctx.Scene.PlayState(p.star, "Blink", beat, 0.5)
	p.mod.ctx.Scene.SetSpinOver(p.sprite, 0)
	p.update(beat)
}

func (p *playYan) hide() {
	p.mod.ctx.Scene.SetActive(p.root, false)
}

func (p *playYan) currentY() float64 {
	beat := p.mod.ctx.Beat()
	switch p.state {
	case playerJumping, playerJumpingFall:
		_, y := samplePath(p.jumpPath, beat, p.jumpBeat)
		return y
	case playerHighJumping, playerHighJumpingFall:
		_, y := samplePath(p.highPath, beat, p.jumpBeat)
		return y
	}
	return 0
}

func (p *playYan) updateBalloons(beat float64) {
	if math.IsInf(p.mod.countInBeat, -1) {
		p.popAll()
		return
	}
	if p.mod.countInLength == 8 {
		for i, off := range []float64{0, 2, 4, 5, 6, 7, 8} {
			idx, popBeat := i, p.mod.countInBeat+off
			p.mod.ctx.At(popBeat, func() { p.popBalloon(idx, beat > popBeat) })
		}
		return
	}
	p.popBalloon(0, true)
	p.popBalloon(1, true)
	for i, off := range []float64{0, 1, 2, 3, 4} {
		idx, popBeat := i+2, p.mod.countInBeat+off
		p.mod.ctx.At(popBeat, func() { p.popBalloon(idx, beat > popBeat) })
	}
}

func (p *playYan) popBalloon(index int, instant bool) {
	if index < 0 || index >= len(p.balloons) {
		return
	}
	if instant {
		p.mod.ctx.Scene.PlayFrozen(p.balloons[index], "Pop", 1)
		return
	}
	p.mod.ctx.Scene.PlayState(p.balloons[index], "Pop", p.mod.ctx.Beat(), 0.5)
}

func (p *playYan) popAll() {
	for i := range p.balloons {
		p.popBalloon(i, true)
	}
}
