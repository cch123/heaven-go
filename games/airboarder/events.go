package airboarder

import (
	"hsdemo/engine"
	"hsdemo/kart"
)

func (m *Module) prepareJump(beat float64, readySound bool) {
	if readySound {
		m.ctx.SoundAt(beat, "ready", 1)
	}
}

func (m *Module) requestObstacle(appearBeat, targetBeat float64, kind int) {
	ob := &obstacle{kind: kind, appearBeat: appearBeat, targetBeat: targetBeat}
	ob.inst = m.newObstacleInstance(kind)
	m.obstacles = append(m.obstacles, ob)
	switch kind {
	case obstacleDuck:
		m.cueDuck(ob)
	case obstacleCrouch:
		m.cueCrouch(ob)
	case obstacleJump:
		m.cueJump(ob)
	}
}

func (m *Module) cueDuck(ob *obstacle) {
	b := ob.targetBeat
	m.ctx.At(b, func() {
		m.wantsCrouch = false
		m.cpu1.cantBop = true
		m.playBoarder(&m.cpu1, poseLetsGo, b, 1)
	})
	m.ctx.At(b+1, func() {
		m.cpu2.cantBop = true
		m.playBoarder(&m.cpu1, poseDuck, b+1, 1.5)
		m.playBoarder(&m.cpu2, poseLetsGo, b+1, 1)
		m.ctx.Sound("crouch")
		m.ctx.Sound("crouchvox")
	})
	m.ctx.At(b+2, func() {
		m.player.cantBop = true
		m.playBoarder(&m.cpu2, poseDuck, b+2, 1.5)
		m.playBoarder(&m.player, poseLetsGo, b+2, 1)
		m.ctx.Sound("crouch")
		m.ctx.Sound("crouchvox")
	})
	m.ctx.At(b+2.5, func() { m.cpu1.cantBop = false })
	m.ctx.At(b+3.5, func() { m.cpu2.cantBop = false })
	m.ctx.At(b+4.5, func() { m.player.cantBop = false })
	m.ctx.ScheduleInputAction(b, 0,
		func(state float64, _ engine.Judgment) { m.duckSuccess(ob, b, state) },
		func() { m.duckMiss(ob, b) })
}

func (m *Module) cueCrouch(ob *obstacle) {
	b := ob.targetBeat
	m.ctx.At(b, func() {
		m.wantsCrouch = true
		m.cpu1.cantBop = true
		m.playBoarder(&m.cpu1, poseLetsGo, b, 1)
	})
	m.ctx.At(b+1, func() {
		m.cpu2.cantBop = true
		m.ctx.Sound("crouch")
		m.ctx.Sound("crouchCharge")
		m.ctx.Sound("crouchvox")
		m.playBoarder(&m.cpu1, poseCharge, b+1, 99)
		m.playBoarder(&m.cpu2, poseLetsGo, b+1, 1)
	})
	m.ctx.At(b+2, func() {
		m.ctx.Sound("crouch")
		m.player.cantBop = true
		m.playBoarder(&m.cpu2, poseCharge, b+2, 99)
		m.playBoarder(&m.player, poseLetsGo, b+2, 1)
		m.ctx.Sound("crouchCharge")
		m.ctx.Sound("crouchvox")
	})
	m.ctx.ScheduleInputAction(b, 0,
		func(state float64, _ engine.Judgment) { m.crouchSuccess(ob, b, state) },
		func() { m.crouchMiss(ob, b) })
}

func (m *Module) cueJump(ob *obstacle) {
	b := ob.targetBeat
	m.ctx.At(b+1, func() {
		m.playBoarder(&m.cpu1, poseJump, b+1, 1.5)
		m.ctx.Sound("jump")
		m.ctx.Sound("jumpvox")
	})
	m.ctx.At(b+2, func() {
		m.playBoarder(&m.cpu2, poseJump, b+2, 1.5)
		m.ctx.Sound("jump")
		m.ctx.Sound("jumpvox")
	})
	m.ctx.At(b+2.5, func() { m.cpu1.cantBop = false })
	m.ctx.At(b+3.5, func() { m.cpu2.cantBop = false })
	m.ctx.ScheduleInputRelease(b,
		func(state float64, _ engine.Judgment) { m.jumpSuccess(ob, b, state) },
		func() { m.jumpMiss(ob, b) })
}

func (m *Module) duckSuccess(ob *obstacle, beat, state float64) {
	m.playBoarder(&m.player, poseDuck, beat, 1.5)
	m.player.cantBop = true
	m.player.cantUntil = beat + 1.5
	if barely(state) {
		ob.shake = true
		ob.effectBeat = beat
		m.playObstacleEffect(ob, "shake", beat)
		m.ctx.Sound("barely")
		m.ctx.Sound("barelyvox")
		return
	}
	m.ctx.Sound("crouch")
	m.ctx.Sound("crouchvox")
}

func (m *Module) duckMiss(ob *obstacle, beat float64) {
	m.playBoarder(&m.player, poseHit1, beat, 1.5)
	ob.broken = true
	ob.effectBeat = beat
	m.playObstacleEffect(ob, "break", beat)
	m.missSound(beat)
	m.player.cantBop = true
	m.player.cantUntil = beat + 1.5
}

func (m *Module) crouchSuccess(ob *obstacle, beat, state float64) {
	m.playBoarder(&m.player, poseCharge, beat, 99)
	m.player.cantBop = true
	if barely(state) {
		ob.shake = true
		ob.effectBeat = beat
		m.playObstacleEffect(ob, "shake", beat)
		m.ctx.Sound("barely")
		m.ctx.Sound("barelyvox")
		return
	}
	m.ctx.Sound("crouch")
	m.ctx.Sound("crouchCharge")
	m.ctx.Sound("crouchvox")
}

func (m *Module) crouchMiss(ob *obstacle, beat float64) {
	m.playBoarder(&m.player, poseHit1, beat, 1.5)
	ob.broken = true
	ob.effectBeat = beat
	m.playObstacleEffect(ob, "break", beat)
	m.missSound(beat)
	m.player.cantBop = true
	m.player.cantUntil = beat + 1.5
}

func (m *Module) jumpSuccess(ob *obstacle, beat, state float64) {
	m.playBoarder(&m.player, poseJump, beat, 1.5)
	if barely(state) {
		ob.shake = true
		ob.effectBeat = beat
		m.playObstacleEffect(ob, "shake", beat)
		m.ctx.Sound("barely")
		m.ctx.Sound("barelyvox")
	} else {
		m.ctx.Sound("jump")
		m.ctx.Sound("jumpvox")
	}
	m.player.cantBop = true
	m.player.cantUntil = beat + 1.5
	m.wantsCrouch = false
}

func (m *Module) jumpMiss(ob *obstacle, beat float64) {
	m.player.cantBop = true
	m.wantsCrouch = false
	m.playBoarder(&m.player, poseHit2, beat, 1.5)
	ob.broken = true
	ob.effectBeat = beat
	m.playObstacleEffect(ob, "break", beat)
	m.missSound(beat)
	m.player.cantUntil = beat + 1.5
}

func (m *Module) forceCharge(beat float64) {
	m.playBoarder(&m.cpu1, poseCharge, beat, 99)
	m.playBoarder(&m.cpu2, poseCharge, beat, 99)
	m.playBoarder(&m.player, poseCharge, beat, 99)
	m.cpu1.cantBop, m.cpu2.cantBop, m.player.cantBop = true, true, true
	m.wantsCrouch = true
}

func (m *Module) yeahLetsGo(beat float64, voiceOn bool) {
	if voiceOn {
		m.ctx.SoundAt(beat, "start1", 1)
		m.ctx.SoundAt(beat+6.5, "start2", 1)
		m.ctx.SoundAt(beat+7, "start3", 1)
	}
	m.ctx.At(beat, func() {
		m.cpu1.cantBop, m.cpu2.cantBop, m.player.cantBop = true, true, true
		m.playBoarder(&m.cpu1, poseLetsGo, beat, 7)
		m.playBoarder(&m.cpu2, poseLetsGo, beat, 7)
		m.playBoarder(&m.player, poseLetsGo, beat, 7)
	})
	m.ctx.At(beat+7, func() {
		m.cpu1.cantBop, m.cpu2.cantBop, m.player.cantBop = false, false, false
	})
}

func (m *Module) bop(beat float64) {
	if !m.player.cantBop {
		m.playBoarder(&m.player, poseBop, beat, 0.5)
	}
	if !m.cpu1.cantBop {
		m.playBoarder(&m.cpu1, poseBop, beat, 0.5)
	}
	if !m.cpu2.cantBop {
		m.playBoarder(&m.cpu2, poseBop, beat, 0.5)
	}
}

func (m *Module) playBoarder(b *boarder, pose string, beat, duration float64) {
	b.pose = pose
	b.poseBeat = beat
	if duration > 0 && duration < 90 {
		b.cantUntil = beat + duration
	}
	m.playOfficialBoarderPose(b, pose, beat, duration)
}

func (m *Module) playOfficialBoarderPose(b *boarder, pose string, beat, duration float64) {
	if m.ctx == nil || m.ctx.Scene == nil {
		return
	}
	role := m.boarderRole(b)
	if role == "" {
		return
	}
	root := m.ctx.Role(role)
	if root == "" {
		return
	}
	if pose == poseIdle {
		m.ctx.Scene.PlayDefaultState(root, beat, m.secPerBeat(beat))
		return
	}
	m.ctx.Scene.PlayState(root, pose, beat, officialBoarderTimeScale(pose, duration))
}

func (m *Module) boarderRole(b *boarder) string {
	switch b {
	case &m.cpu1:
		return "CPU1"
	case &m.cpu2:
		return "CPU2"
	case &m.player:
		return "Player"
	default:
		return ""
	}
}

func officialBoarderTimeScale(pose string, duration float64) float64 {
	switch pose {
	case poseBop:
		return 0.5
	case poseHit1:
		return 1.5
	case poseDuck, poseCharge, poseHold, poseJump, poseLetsGo, poseHit2:
		return 1
	}
	if duration > 0 && duration < 90 {
		return duration
	}
	return 1
}

func (m *Module) missSound(beat float64) {
	seq := []struct {
		name string
		off  float64
	}{
		{"miss1", 0}, {"missvox", 0}, {"miss2", 0.25}, {"miss3", 0.75},
		{"miss4", 0.875}, {"miss5", 1}, {"miss6", 1.125}, {"miss7", 1.25},
		{"miss8", 1.5}, {"miss9", 1.75}, {"miss10", 2}, {"miss11", 2.25},
		{"miss12", 2.5}, {"miss13", 2.75}, {"miss14", 3}, {"miss15", 3.25},
	}
	for _, s := range seq {
		m.ctx.SoundAt(beat+s.off, s.name, 1)
	}
}

func barely(state float64) bool { return state >= 1 || state <= -1 }

func (m *Module) newObstacleInstance(kind int) *kart.Instance {
	var tmpl *kart.Template
	switch kind {
	case obstacleJump:
		tmpl = m.wallT
	default:
		tmpl = m.archT
	}
	if tmpl == nil {
		return nil
	}
	inst := tmpl.NewInstance()
	inst.PlayDefaultState("", 0, m.secPerBeat(0))
	return inst
}

func (m *Module) playObstacleEffect(ob *obstacle, state string, beat float64) {
	if ob == nil || ob.inst == nil {
		return
	}
	// Arch.cs/Wall.cs play shake/break on animator layer 1 while layer 0 keeps
	// the approach movement frozen at GetPositionFromBeat(appearBeat, 40f).
	ob.inst.PlayStateLayer("effect", "", state, beat, 1)
}
