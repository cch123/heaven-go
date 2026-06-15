package samuraislicervl

import (
	"strings"

	"hsdemo/kart"
)

type slicedConfig struct {
	root     string
	topRel   string
	botRel   string
	wait     float64
	rotSpeed float64
	topVel   [2]float64
	botVel   [2]float64
	gravity  float64
}

type slicedDemon struct {
	inst      *kart.Instance
	cfg       slicedConfig
	bornBeat  float64
	root      [2]float64
	scale     float64
	topBase   [2]float64
	botBase   [2]float64
	topRot    float64
	botRot    float64
	topOnly   bool
	dead      bool
	deathBeat float64
}

func (m *Module) loadSlicedConfigs() map[string]slicedConfig {
	out := map[string]slicedConfig{}
	for name, c := range m.ctx.Assets.Extra.Components {
		if !strings.HasPrefix(name, "sliced") || c.Path == "" {
			continue
		}
		out[c.Path] = slicedConfig{
			root:     c.Path,
			topRel:   rel(c.Path, c.Refs["topPart"]),
			botRel:   rel(c.Path, c.Refs["botPart"]),
			wait:     numOr(c, "waitTime", 0.08),
			rotSpeed: numOr(c, "rotationSpeed", -50),
			topVel:   [2]float64{numOr(c, "topVelocity.x", 0), numOr(c, "topVelocity.y", 2)},
			botVel:   [2]float64{numOr(c, "botVelocity.x", 0), numOr(c, "botVelocity.y", -2)},
			gravity:  numOr(c, "gravity", 25),
		}
	}
	return out
}

func (m *Module) spawnSlicedDemon(pos [2]float64, typ int, beat float64, horde bool, scale float64) {
	var tmpl *kart.Template
	if horde {
		tmpl = m.hordeSlicedT
	} else if typ >= 0 && typ < len(m.slicedT) {
		tmpl = m.slicedT[typ]
	}
	if tmpl == nil {
		return
	}
	cfg, ok := m.slicedCfg[tmpl.RootPath]
	if !ok {
		return
	}
	inst := tmpl.NewInstance()
	inst.Offset = pos
	if scale == 0 {
		scale = 1
	}
	inst.Scale = [2]float64{scale, scale}
	s := &slicedDemon{
		inst:      inst,
		cfg:       cfg,
		bornBeat:  beat,
		root:      pos,
		scale:     scale,
		topBase:   nodeRelPos(m.ctx.Assets, cfg.root, cfg.topRel),
		botBase:   nodeRelPos(m.ctx.Assets, cfg.root, cfg.botRel),
		topRot:    nodeRelRot(m.ctx.Assets, cfg.root, cfg.topRel),
		botRot:    nodeRelRot(m.ctx.Assets, cfg.root, cfg.botRel),
		deathBeat: beat + 8,
	}
	if horde && scale == 1 {
		// SamuraiSliceRvl.InitHorde(true) gives non-final horde slices a much
		// sharper upward-right impulse than the prefab's serialized defaults.
		s.cfg.topVel = [2]float64{2.5, 15}
		s.cfg.botVel = [2]float64{2.75, 14}
	}
	m.slices = append(m.slices, s)
}

func (s *slicedDemon) update(beat float64) {
	if s.dead {
		return
	}
	refT := (beat - s.bornBeat) * (60.0 / 130.0)
	if refT < s.cfg.wait {
		return
	}
	fallT := refT - s.cfg.wait
	top := ballistic(s.topBase, s.cfg.topVel, s.cfg.gravity, fallT)
	bot := ballistic(s.botBase, s.cfg.botVel, s.cfg.gravity, fallT)
	s.inst.SetPos(s.cfg.topRel, top[0], top[1])
	s.inst.SetPos(s.cfg.botRel, bot[0], bot[1])
	s.inst.SetRot(s.cfg.topRel, s.topRot+deg(s.cfg.rotSpeed)*fallT)
	if fallT > 0.1 {
		s.inst.SetRot(s.cfg.botRel, s.botRot+deg(s.cfg.rotSpeed*1.2)*(fallT-0.1))
	}
	if s.root[1]+top[1]*s.scale < -10 && s.root[1]+bot[1]*s.scale < -10 {
		s.dead = true
		s.deathBeat = beat
	}
}

func ballistic(base, vel [2]float64, gravity, t float64) [2]float64 {
	return [2]float64{
		base[0] + vel[0]*t,
		base[1] + vel[1]*t - 0.5*gravity*t*t,
	}
}

func (s *slicedDemon) queue(scene *kart.SceneInst, beat float64) {
	if !s.dead {
		s.inst.Queue(scene, beat, kart.Identity(), 0)
	}
}

func liveSlices(in []*slicedDemon, beat float64) []*slicedDemon {
	out := in[:0]
	for _, s := range in {
		if !s.dead && beat < s.deathBeat {
			out = append(out, s)
		}
	}
	return out
}

func nodeRelPos(as *kart.Assets, root, relPath string) [2]float64 {
	idx, ok := as.NodeIndex(root + "/" + relPath)
	if !ok {
		return [2]float64{}
	}
	return as.Rig.Nodes[idx].Pos
}

func nodeRelRot(as *kart.Assets, root, relPath string) float64 {
	idx, ok := as.NodeIndex(root + "/" + relPath)
	if !ok {
		return 0
	}
	return as.Rig.Nodes[idx].RotZ
}
