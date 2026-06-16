package chargingchicken

import (
	"hsdemo/kart"
)

const (
	platformDistance = 5.35 / 2
	platformsPerBeat = 4
)

type island struct {
	inst *kart.Instance
	mod  *Module

	x      float64
	startX float64
	endX   float64
	moveA  float64
	moveB  float64
	moving bool
}

func newIsland(mod *Module, x float64) *island {
	in := mod.islandT.NewInstance()
	isl := &island{mod: mod, inst: in, x: x, startX: x, endX: x}
	isl.setDefaults(0)
	return isl
}

func (i *island) setDefaults(beat float64) {
	if i == nil || i.inst == nil {
		return
	}
	i.inst.PlayDefaultState(relIsland(i.mod.charger), beat, i.mod.ctx.SecPerBeat(beat))
	i.inst.PlayDefaultState(relIsland(i.mod.fakeChicken), beat, i.mod.ctx.SecPerBeat(beat))
	i.inst.PlayDefaultState(relIsland(i.mod.platform), beat, i.mod.ctx.SecPerBeat(beat))
	i.inst.SetActive(relIsland(i.mod.bigLandmass), false)
	i.inst.SetActive(relIsland(i.mod.smallLandmass), true)
}

func (i *island) prep(beat, lateness float64) {
	for step := 4; step >= 1; step-- {
		step := step
		i.mod.ctx.At(beat-float64(step), func() {
			if lateness > float64(step-1) {
				i.inst.PlayState(relIsland(i.mod.charger), "Prep"+itoa(5-step), beat-float64(step), 0.5)
			}
		})
	}
}

func (i *island) charge(beat float64) {
	i.inst.PlayState(relIsland(i.mod.charger), "Pump", beat, 0.5)
}

func (i *island) idle(beat float64) {
	i.inst.PlayState(relIsland(i.mod.charger), "Idle", beat, 0.5)
}

func (i *island) collapse(beat float64, success bool) {
	i.mod.ctx.SoundAt(beat, "SE_CHIKEN_LAND_RESET", 0.7)
	i.mod.ctx.At(beat, func() {
		if success {
			i.inst.SetActive(relIsland(i.mod.bigLandmass), false)
			i.inst.SetActive(relIsland(i.mod.smallLandmass), true)
			i.playParticle(beat, i.mod.collapseOK)
			i.playParticle(beat, i.mod.grassL)
			i.playParticle(beat, i.mod.grassR)
		} else {
			i.inst.SetActive(relIsland(i.mod.smallLandmass), false)
			i.playParticle(beat, i.mod.collapseNG)
		}
	})
}

func (i *island) spawnStones(beat, length float64, tooLate bool) {
	if i == nil || i.inst == nil {
		return
	}
	i.inst.SetActive(relIsland(i.mod.platform), true)
	if !tooLate {
		i.inst.PlayState(relIsland(i.mod.platform), "Set", beat-length-1, 0.5)
		i.mod.ctx.SoundAt(beat-length-1, "SE_CHIKEN_BLOCK_SET", 1)
	}
	i.mod.ctx.At(beat, func() {
		i.playParticle(beat, i.mod.stoneSplash)
	})
}

func (i *island) beginMove(startBeat, endBeat, endX float64) {
	i.startX, i.endX = i.x, endX
	i.moveA, i.moveB = startBeat, endBeat
	i.moving = true
}

func (i *island) update(beat float64) {
	if i == nil || !i.moving {
		return
	}
	if i.moveB <= i.moveA {
		i.x = i.endX
		i.moving = false
		return
	}
	u := clamp01((beat - i.moveA) / (i.moveB - i.moveA))
	i.x = lerp(i.startX, i.endX, u)
	if u >= 1 {
		i.moving = false
	}
}

func (i *island) queue(scene *kart.SceneInst, beat float64) {
	if i == nil || i.inst == nil {
		return
	}
	i.inst.Offset = [2]float64{i.x, 0}
	i.inst.Queue(scene, beat, kart.Identity(), 0)
}

func (i *island) playParticle(beat float64, root string) {
	if i == nil || i.mod == nil {
		return
	}
	i.mod.addParticleEffect(beat, root, "Island", [2]float64{i.x, 0})
}
