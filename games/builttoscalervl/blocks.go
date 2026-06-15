package builttoscalervl

import "hsdemo/engine"

type blockState struct {
	path        string
	base        [2]float64
	slideOffset float64
	isOpen      bool
	isPrepare   bool
	closeBeat   float64
	shootBeat   float64
}

type presenceEvent struct {
	beat, length float64
	in           bool
	ease         int
	blocks       [4]bool
}

func (m *Module) playBlockBounce(pos int, closeBeat float64) {
	b := m.block(pos)
	if b == nil {
		return
	}
	m.playBlockState(pos, "bounce", m.ctx.Beat())
	if b.closeBeat < closeBeat {
		b.closeBeat = closeBeat
	}
}

func (m *Module) playBlockBounceNearlyMiss(pos int) {
	if m.block(pos) == nil {
		return
	}
	m.ctx.Sound("barely")
	m.playBlockState(pos, "open", m.ctx.Beat())
}

func (m *Module) playBlockBounceMiss(pos int) {
	b := m.block(pos)
	if b == nil {
		return
	}
	state := "miss"
	if b.isOpen {
		state = "miss_open"
	}
	m.playBlockState(pos, state, m.ctx.Beat())
}

func (m *Module) playBlockPrepare(pos int, shootBeat float64) {
	b := m.block(pos)
	if b == nil {
		return
	}
	m.ctx.Sound("playerRetract")
	// The Go input layer maps the alternate shoot action to a pad-style button,
	// so the Wii "B" prompt is the closest runtime equivalent.
	m.playBlockState(pos, "prepare B", m.ctx.Beat())
	b.isOpen = false
	b.isPrepare = true
	b.shootBeat = shootBeat
}

func (m *Module) playBlockShoot(pos int) {
	b := m.block(pos)
	if b == nil {
		return
	}
	m.ctx.Sound("shoot")
	m.playBlockState(pos, "shoot", m.ctx.Beat())
	b.isPrepare = false
}

func (m *Module) playBlockShootMiss(pos int) {
	b := m.block(pos)
	if b == nil || !b.isPrepare {
		return
	}
	m.playBlockState(pos, "shoot miss B", m.ctx.Beat())
	b.isPrepare = false
}

func (m *Module) playBlockOpen(pos int) {
	b := m.block(pos)
	if b == nil || b.isPrepare {
		return
	}
	m.playBlockState(pos, "open", m.ctx.Beat())
	b.isOpen = true
}

func (m *Module) playBlockIdle(pos int, beat float64) {
	b := m.block(pos)
	if b == nil {
		return
	}
	if b.closeBeat > beat || b.shootBeat >= beat {
		return
	}
	m.playBlockState(pos, "idle", beat)
	b.isOpen = false
}

func (m *Module) block(pos int) *blockState {
	if !inRange(pos) {
		return nil
	}
	return m.blocks[pos]
}

func (m *Module) playBlockState(pos int, state string, beat float64) {
	if b := m.block(pos); b != nil {
		m.ctx.Scene.PlayState(b.path, state, beat, m.ctx.SecPerBeat(beat))
	}
}

func (m *Module) applyPresence(beat float64) {
	for i, b := range m.blocks {
		if b == nil {
			continue
		}
		y := b.base[1]
		for _, ev := range m.presence {
			if !ev.blocks[i] {
				continue
			}
			if beat < ev.beat {
				break
			}
			from := b.base[1]
			to := b.base[1] + b.slideOffset
			if ev.in {
				from, to = to, from
			}
			if ev.length <= 0 || beat >= ev.beat+ev.length {
				y = to
				continue
			}
			u := (beat - ev.beat) / ev.length
			y = engine.Ease(ev.ease, from, to, u)
			break
		}
		m.ctx.Scene.SetPosOver(b.path, b.base[0], y)
	}
}
