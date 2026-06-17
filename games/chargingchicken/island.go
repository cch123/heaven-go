package chargingchicken

import (
	"math"

	"hsdemo/kart"
)

const (
	platformDistance           = 5.35 / 2
	platformsPerBeat           = 4
	platformOffsetUnderChicken = -6
)

type stonePlatform struct {
	inst    *kart.Instance
	number  int
	localX  float64
	offsetX float64
	fallen  bool
}

type island struct {
	inst *kart.Instance
	mod  *Module

	x      float64
	startX float64
	endX   float64
	moveA  float64
	moveB  float64
	moving bool

	stones                  []stonePlatform
	stonesExist             bool
	stonePlatformFallOffset float64
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
	i.inst.SetActive(relIsland(i.mod.platform), false)
	i.inst.SetActive(relIsland(i.mod.bigLandmass), false)
	i.inst.SetActive(relIsland(i.mod.smallLandmass), true)
	i.positionIsland(0)
	i.clearStones()
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
			i.x = 0
			// Unity subtracts stonePlatformFallOffset from existing stone clones
			// when the landmass resets; recomputing from state 0 preserves the
			// same final positions without mutating shared prefab data.
			i.positionIsland(0)
		} else {
			i.inst.SetActive(relIsland(i.mod.smallLandmass), false)
			i.playParticle(beat, i.mod.collapseNG)
		}
	})
}

func (i *island) spawnStones(beat, length float64, tooLate bool) {
	if i == nil || i.inst == nil || i.mod == nil || i.mod.platformT == nil {
		return
	}
	i.clearStones()
	count := stonePlatformCount(length)
	setupBeat := beat - length - 2
	if i.mod.ctx != nil {
		setupBeat = i.mod.ctx.Beat()
	}
	baseX, baseY := i.platformBasePos()
	for n := 0; n < count; n++ {
		inst := i.mod.platformT.NewInstance()
		offsetX := stonePlatformOffset(n, i.stonePlatformFallOffset)
		inst.Offset = [2]float64{baseX + offsetX, baseY}
		inst.SetActive("", true)
		switch stonePlatformVariant(n) {
		case 1:
			inst.PlayStateLayer("variant", "", "Plat1", setupBeat, 0.5)
		case 2:
			inst.PlayStateLayer("variant", "", "Plat2", setupBeat, 0.5)
		}
		if !tooLate {
			inst.PlayState("", "Set", setupBeat+float64(n)/64, 0.5)
		}
		i.stones = append(i.stones, stonePlatform{
			inst:    inst,
			number:  n,
			localX:  baseX + offsetX,
			offsetX: offsetX,
		})
	}
	if i.mod.ctx == nil {
		i.stonesExist = true
		return
	}
	i.mod.ctx.At(beat-length-1, func() { i.stonesExist = true })
}

func (i *island) beginMove(startBeat, endBeat, endX float64) {
	i.startX, i.endX = i.x, endX
	i.moveA, i.moveB = startBeat, endBeat
	i.moving = true
}

func (i *island) update(beat float64) {
	if i == nil {
		return
	}
	if i.stonesExist {
		i.stoneSplashCheck(beat, 0)
	}
	if !i.moving {
		return
	}
	if i.moveB <= i.moveA {
		i.x = i.endX
		i.moving = false
		return
	}
	u := easeOutCubic(clamp01((beat - i.moveA) / (i.moveB - i.moveA)))
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
	base := kart.Translate(i.x, 0)
	for si := range i.stones {
		if i.stones[si].inst != nil {
			i.stones[si].inst.Queue(scene, beat, base, 0)
		}
	}
}

func (i *island) playParticle(beat float64, root string) {
	if i == nil || i.mod == nil {
		return
	}
	i.mod.addParticleEffect(beat, root, "Island", [2]float64{i.x, 0})
}

func (i *island) positionIsland(state float64) {
	if i == nil || i.inst == nil || i.mod == nil {
		return
	}
	i.stonePlatformFallOffset = state
	i.inst.SetPos(relIsland(i.mod.collapsed), state, 0)
	baseX, baseY := i.platformBasePos()
	for si := range i.stones {
		offsetX := stonePlatformOffset(i.stones[si].number, state)
		i.stones[si].offsetX = offsetX
		i.stones[si].localX = baseX + offsetX
		if i.stones[si].inst != nil {
			i.stones[si].inst.Offset = [2]float64{i.stones[si].localX, baseY}
		}
	}
}

func (i *island) stoneSplashCheck(beat, offset float64) {
	if i == nil || i.mod == nil {
		return
	}
	for si := range i.stones {
		stone := &i.stones[si]
		if stone.fallen || stone.inst == nil {
			continue
		}
		worldX := i.x + stone.localX
		if worldX >= platformOffsetUnderChicken+offset {
			continue
		}
		stone.fallen = true
		stone.inst.PlayState("", "Fall", beat, 0.3)
		i.mod.playStoneFallSound(stone.number)
		splashBeat := beat + 0.5
		offsetX := stone.offsetX
		localX := stone.localX
		number := stone.number
		if i.mod.ctx == nil {
			break
		}
		i.mod.ctx.At(splashBeat, func() {
			if i.x+localX > -7.5+platformOffsetUnderChicken {
				i.mod.playStoneWaterSound(number)
			}
			i.mod.addParticleEffect(splashBeat, i.mod.stoneSplash, "Island", [2]float64{i.x + offsetX, 0})
		})
		break
	}
}

func (i *island) playChickenSplash(beat float64) {
	if i == nil || i.mod == nil {
		return
	}
	// Island.cs writes ChickenSplashEffect.localPosition.x = -IslandPos.x + 2.5.
	// The particle runtime anchors effects relative to Island, so compensate for
	// the authored root's local x and place the splash at the same world point.
	i.mod.addParticleEffect(beat, i.mod.chickenSplash, "Island", [2]float64{2.5, 0})
}

func (i *island) clearStones() {
	i.stones = nil
	i.stonesExist = false
}

func (i *island) platformBasePos() (float64, float64) {
	if i == nil || i.mod == nil || i.mod.ctx == nil {
		return 0, 0
	}
	return nodeLocalPos(i.mod.ctx.Assets, i.mod.platformBase)
}

func stonePlatformCount(length float64) int {
	if length <= 0 {
		return 0
	}
	return int(length * platformsPerBeat)
}

func stonePlatformOffset(number int, fallOffset float64) float64 {
	return float64(number)*platformDistance - platformDistance/2 + fallOffset
}

func stonePlatformVariant(number int) int {
	return number % 3
}

func easeOutCubic(u float64) float64 {
	v := 1 - clamp01(u)
	return 1 - v*v*v
}

func nodeLocalPos(as *kart.Assets, path string) (float64, float64) {
	if as == nil {
		return 0, 0
	}
	if i, ok := as.NodeIndex(path); ok {
		p := as.Rig.Nodes[i].Pos
		return p[0], p[1]
	}
	return 0, 0
}

func centsPitch(cents float64) float64 {
	return math.Pow(2, cents/1200)
}
