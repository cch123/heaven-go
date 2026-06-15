package nightwalkagb

import (
	"math"

	"hsdemo/kart"
)

type starInst struct {
	inst     *kart.Instance
	originX  float64
	originY  float64
	x        float64
	y        float64
	evoStage int
	devolved bool
}

type starField struct {
	mod         *Module
	stars       []starInst
	collective  int
	lastBlink   float64
	normalizedX float64
	normalizedY float64
}

func newStarField(m *Module) starField {
	sf := starField{mod: m, collective: 1, lastBlink: math.Inf(-1)}
	if m.starT == nil {
		return sf
	}
	for i := 0; i < m.starCfg.starCount; i++ {
		inst := m.starT.NewInstance()
		inst.PlayDefaultState("", 0, m.ctx.SecPerBeat(0))
		sf.stars = append(sf.stars, starInst{
			inst:     inst,
			originX:  m.rng.Float64()*m.starCfg.boundaryX*2 - m.starCfg.boundaryX,
			originY:  m.rng.Float64()*m.starCfg.boundaryY*2 - m.starCfg.boundaryY,
			evoStage: 1,
		})
	}
	return sf
}

func (s *starField) reset(beat float64) {
	s.collective = 1
	s.lastBlink = beat
	for i := range s.stars {
		st := &s.stars[i]
		st.evoStage = 1
		st.devolved = false
		st.inst.PlayState("", "Small", beat, s.mod.ctx.SecPerBeat(beat))
	}
}

func (s *starField) update(beat float64) {
	m := s.mod
	if !m.stopStars {
		if m.cfg.starLength > 0 {
			s.normalizedX = -beat / m.cfg.starLength
		}
	}
	if s.lastBlink == math.Inf(-1) {
		s.lastBlink = beat
	}
	for beat >= s.lastBlink+m.starCfg.blinkFrequency {
		s.lastBlink += m.starCfg.blinkFrequency
		s.blink()
	}
	for i := range s.stars {
		st := &s.stars[i]
		st.x, st.y = s.relative(st.originX, st.originY)
	}
}

func (s *starField) relative(ogX, ogY float64) (float64, float64) {
	m := s.mod
	x := m.starCfg.boundaryX*s.normalizedX + ogX
	for x > m.starCfg.boundaryX {
		ogX -= m.starCfg.boundaryX * 2
		x = m.starCfg.boundaryX*s.normalizedX + ogX
	}
	for x < -m.starCfg.boundaryX {
		ogX += m.starCfg.boundaryX * 2
		x = m.starCfg.boundaryX*s.normalizedX + ogX
	}
	y := m.starCfg.boundaryY*s.normalizedY + ogY
	for y > m.starCfg.boundaryY {
		ogY -= m.starCfg.boundaryY * 2
		y = m.starCfg.boundaryY*s.normalizedY + ogY
	}
	for y < -m.starCfg.boundaryY {
		ogY += m.starCfg.boundaryY * 2
		y = m.starCfg.boundaryY*s.normalizedY + ogY
	}
	return x, y
}

func (s *starField) blink() {
	if len(s.stars) == 0 {
		return
	}
	seen := map[int]bool{}
	for i := 0; i < s.mod.starCfg.blinkAmount; i++ {
		idx := s.mod.rng.Intn(len(s.stars))
		if seen[idx] || s.stars[idx].devolved {
			continue
		}
		seen[idx] = true
		stage := s.stars[idx].evoStage
		s.stars[idx].inst.PlayState("", "Blink"+string(rune('0'+stage)), s.mod.ctx.Beat(), s.mod.ctx.SecPerBeat(s.mod.ctx.Beat()))
	}
}

func (s *starField) evolve(amount int) {
	for i := 0; i < amount; i++ {
		idx := s.evolveTarget()
		if idx < 0 {
			return
		}
		st := &s.stars[idx]
		if st.evoStage >= 5 || st.devolved {
			continue
		}
		st.inst.PlayState("", "Evolve"+string(rune('0'+st.evoStage)), s.mod.ctx.Beat(), s.mod.ctx.SecPerBeat(s.mod.ctx.Beat()))
		st.evoStage++
	}
}

func (s *starField) evolveTarget() int {
	var candidates []int
	for i := range s.stars {
		st := &s.stars[i]
		if st.evoStage != s.collective || st.devolved {
			continue
		}
		if st.x >= -17.77695/2 && st.x <= 17.77695/2 && st.y >= -5 && st.y <= 5 {
			candidates = append(candidates, i)
		}
	}
	if len(candidates) > 0 {
		return candidates[s.mod.rng.Intn(len(candidates))]
	}
	candidates = candidates[:0]
	for i := range s.stars {
		st := &s.stars[i]
		if st.evoStage == s.collective && !st.devolved {
			candidates = append(candidates, i)
		}
	}
	if len(candidates) > 0 {
		return candidates[s.mod.rng.Intn(len(candidates))]
	}
	s.collective++
	if s.collective > 5 {
		s.collective = 5
	}
	if len(s.stars) == 0 {
		return -1
	}
	return s.mod.rng.Intn(len(s.stars))
}

func (s *starField) devolve() {
	for i := range s.stars {
		st := &s.stars[i]
		if st.devolved {
			continue
		}
		st.inst.PlayState("", "Devolve"+string(rune('0'+st.evoStage)), s.mod.ctx.Beat(), s.mod.ctx.SecPerBeat(s.mod.ctx.Beat()))
		st.devolved = true
	}
}

func (s *starField) queue(sc *kart.SceneInst, beat float64) {
	for i := range s.stars {
		st := &s.stars[i]
		st.inst.Offset = [2]float64{st.x, st.y}
		st.inst.Queue(sc, beat, kart.Identity(), 0)
	}
}
