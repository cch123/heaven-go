package builttoscalervl

import "hsdemo/kart"

type square struct {
	inst                  *kart.Instance
	startBeat, targetBeat float64
	lengthBeat            float64
	firstBeat             float64
	endTime               int
	base                  [2]float64
	correction            [2]float64
	anim                  string
	lastStep              int
	dead                  bool
}

type assembled struct {
	inst *kart.Instance
	beat float64
	dead bool
}

func (m *Module) spawnSquares(targetBeat float64) []*square {
	return []*square{
		m.newSquare(m.leftSquareT, m.leftSquareAnim, m.leftSquareCorrection, targetBeat),
		m.newSquare(m.rightSquareT, m.rightSquareAnim, m.rightSquareCorrection, targetBeat),
	}
}

func (m *Module) newSquare(t *kart.Template, anim string, corr [2]float64, targetBeat float64) *square {
	endTime := intCeil((targetBeat - m.gameStartBeat) / 1)
	firstBeat := targetBeat - float64(endTime)
	inst := t.NewInstance()
	s := &square{
		inst: inst, startBeat: m.gameStartBeat, targetBeat: targetBeat,
		lengthBeat: 1, firstBeat: firstBeat, endTime: endTime,
		base: inst.Offset, correction: corr, anim: anim, lastStep: -1,
	}
	s.inst.Offset[0] -= float64(endTime) * corr[0]
	s.inst.Offset[1] -= float64(endTime) * corr[1]
	m.squares = append(m.squares, s)
	return s
}

func (m *Module) spawnAssembled(beat float64) {
	a := &assembled{inst: m.assembledT.NewInstance(), beat: beat}
	a.inst.PlayDefaultState("", beat, m.ctx.SecPerBeat(beat))
	m.assembled = append(m.assembled, a)
}

func (s *square) update(m *Module, beat float64) {
	if s.dead || beat < s.firstBeat {
		return
	}
	step := intFloor((beat - s.firstBeat) / s.lengthBeat)
	if step > s.endTime+10 {
		s.dead = true
		return
	}
	if step == s.lastStep {
		return
	}
	s.lastStep = step
	s.inst.Offset[0] = s.base[0] - float64(s.endTime-step)*s.correction[0]
	s.inst.Offset[1] = s.base[1] - float64(s.endTime-step)*s.correction[1]
	if step == 0 && s.firstBeat != 0 {
		s.inst.PlayFrozen("", s.anim, 1)
		return
	}
	s.inst.PlayState("", s.anim, s.firstBeat+float64(step)*s.lengthBeat, m.ctx.SecPerBeat(beat))
}

func (a *assembled) update(beat float64) {
	if beat >= a.beat+4 {
		a.dead = true
	}
}

func intCeil(v float64) int {
	n := int(v)
	if float64(n) < v {
		n++
	}
	return n
}

func intFloor(v float64) int {
	n := int(v)
	if float64(n) > v {
		n--
	}
	return n
}
