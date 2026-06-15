package bossanova

import (
	"math"
	"math/rand"
	"strings"

	"hsdemo/engine"
	"hsdemo/kart"
	"hsdemo/kmdata"
)

const floorHeight = -3

type shapeSpec struct {
	path       string
	shapeRel   string
	shadowPath string
	isCube     bool
	enter      kmdata.Curve
	hit        kmdata.Curve
	miss       kmdata.Curve
	template   *kart.Template
	shadowT    *kart.Template
}

type shape struct {
	mod          *Module
	spec         *shapeSpec
	inst, shadow *kart.Instance
	startBeat    float64
	voiceline    int
	voiceVariant int
	spin         bool
	forBossa     bool
	isEntering   bool
	isHit        bool
	isFalling    bool
	dead         bool
}

func loadShapeSpec(ctx *engine.Ctx, key string, isCube bool) shapeSpec {
	comp := ctx.Assets.Extra.Components[key]
	path := comp.Path
	if path == "" {
		path = ctx.Role(key)
	}
	shapePath := comp.Refs["shapeTransform"]
	shapeRel := ""
	if strings.HasPrefix(shapePath, path+"/") {
		shapeRel = strings.TrimPrefix(shapePath, path+"/")
	}
	shadowPath := comp.Refs["Shadow"]
	if shadowPath == "" {
		shadowPath = "ShapeShadow"
	}
	return shapeSpec{
		path:       path,
		shapeRel:   shapeRel,
		shadowPath: shadowPath,
		isCube:     isCube,
		enter:      ctx.Assets.Extra.Curves[key+".enterCurve"],
		hit:        ctx.Assets.Extra.Curves[key+".hitCurve"],
		miss:       ctx.Assets.Extra.Curves[key+".missCurve"],
		template:   kart.NewTemplate(ctx.Assets, path),
		shadowT:    kart.NewTemplate(ctx.Assets, shadowPath),
	}
}

func newShape(mod *Module, spec *shapeSpec, beat float64, voice, variant int, spin, forBossa bool) *shape {
	s := &shape{
		mod: mod, spec: spec, startBeat: beat,
		voiceline: voice, voiceVariant: variant, spin: spin, forBossa: forBossa,
		isEntering: true,
		inst:       spec.template.NewInstance(),
	}
	if spec.shadowT != nil {
		s.shadow = spec.shadowT.NewInstance()
	}
	return s
}

func (s *shape) schedule() {
	target := s.startBeat + 1
	if s.forBossa {
		s.mod.ctx.ScheduleInput(target, s.hitInput, s.missInput)
		return
	}
	in := s.mod.ctx.ScheduleInputNoScore(target, s.headBumpInput, nil)
	// Shape.Start uses ScheduleUserInput here: it is a player-only wrong-action
	// window. Autoplay must not trigger a head bump while verification is running.
	in.NoAutoplay = true
	if s.spec.isCube {
		s.mod.ctx.SoundAt(target, "SE_BOSSA_EN_NUT_OTHER", 1)
	} else {
		s.mod.ctx.SoundAt(target, "SE_BOSSA_EN_BALL_OTHER", 1)
	}
	s.mod.ctx.At(target, func() {
		s.isEntering = false
		s.isHit = true
		state, playing := s.mod.ctx.Scene.StateInfo(s.mod.novaAnim, s.mod.ctx.Beat())
		if !(state == "Head Bump" && playing) {
			s.mod.ctx.Scene.PlayState(s.mod.novaAnim, "Hit", target, animScale)
		}
	})
}

func (s *shape) hitInput(state float64, _ engine.Judgment) {
	beat := s.mod.ctx.Beat()
	if state >= 1 || state <= -1 {
		s.mod.ctx.Scene.PlayState(s.mod.bossaAnim, "Barely", beat, animScale)
		s.mod.ctx.PlayCommon("miss")
		s.isEntering = false
		s.isFalling = true
		s.shadow = nil
		return
	}

	s.isEntering = false
	s.isHit = true
	if s.spec.isCube {
		s.mod.ctx.Sound("SE_BOSSA_EN_NUT")
	} else {
		s.mod.ctx.Sound("SE_BOSSA_EN_BALL")
	}
	if s.mod.angerLevel <= 5 {
		if s.mod.angerLevel > 0 {
			s.voiceAngry()
		} else {
			switch s.mod.emotion {
			case 0:
				s.voiceAngry()
			case 1:
				s.voiceNormal()
			case 2:
				s.voicePlayful()
			}
		}
	}
	s.mod.ctx.Scene.PlayState(s.mod.bossaAnim, "Hit", beat, animScale)
	if s.mod.angerLevel > 0 && s.mod.angerLevel <= 5 {
		s.mod.angerLevel--
	}
	s.mod.playRing(s.spec.isCube, beat)
}

func (s *shape) headBumpInput(state float64, _ engine.Judgment) {
	beat := s.mod.ctx.Beat()
	if state >= 1 || state <= -1 {
		s.mod.ctx.Scene.PlayState(s.mod.bossaAnim, "Whiff", beat, animScale)
		if s.spec.isCube {
			s.mod.ctx.Sound("SE_BOSSA_EN_SWING_BALL")
		} else {
			s.mod.ctx.Sound("SE_BOSSA_EN_SWING_NUT")
		}
		return
	}
	s.mod.ctx.Sound("SE_BOSSA_EN_MISS_HEAD")
	s.mod.ctx.ScoreMiss()
	s.mod.angerLevel = 5
	s.mod.ctx.Scene.PlayState(s.mod.bossaAnim, "Head Bump", beat, animScale)
	s.mod.ctx.Scene.PlayState(s.mod.novaAnim, "Head Bump", beat, animScale)
}

func (s *shape) missInput() {
	beat := s.mod.ctx.Beat()
	if s.spec.isCube {
		s.mod.ctx.Sound("SE_BOSSA_EN_NUT_MISS_" + itoa(rand.Intn(8)+1))
		s.mod.ctx.SoundVol("Nova/SE_BOSSA_EN_71", 0.62)
	} else {
		s.mod.ctx.Sound("SE_BOSSA_EN_BALL_MISS")
		s.mod.ctx.SoundVol("Bossa/SE_BOSSA_EN_69", 1.4)
	}
	s.mod.ctx.Scene.PlayState(s.mod.bossaAnim, "Miss", beat, animScale)
	s.isEntering = false
	s.isHit = false
	s.isFalling = true
	s.shadow = nil
	s.mod.angerLevel = 6
}

func (s *shape) queue(sc *kart.SceneInst, t, beat float64) {
	if s.dead || s.inst == nil {
		return
	}
	pos := s.positionAt(beat)
	if s.dead {
		return
	}
	s.inst.Offset = [2]float64{pos[0], pos[1]}
	if s.isEntering {
		s.inst.Rot = -2 * math.Pi * math.Max(0, t-s.mod.ctx.BeatToTime(s.startBeat))
	} else {
		s.inst.Rot = 2 * math.Pi * math.Max(0, t-s.mod.ctx.BeatToTime(s.startBeat+1))
	}
	s.inst.Queue(sc, beat, kart.Identity(), pos[2])
	if s.shadow != nil && !s.isFalling {
		s.shadow.Offset = [2]float64{pos[0], floorHeight - 0.5}
		s.shadow.Rot = 0
		s.shadow.Queue(sc, beat, kart.Identity(), pos[2])
	}
}

func (s *shape) positionAt(beat float64) [3]float64 {
	switch {
	case s.isEntering:
		return kart.EvalBezier(s.spec.enter, (beat-s.startBeat)/1.15)
	case s.isHit:
		u := (beat - (s.startBeat + 1)) / 1.75
		if u > 1 {
			s.dead = true
		}
		return kart.EvalBezier(s.spec.hit, u)
	case s.isFalling:
		u := beat - (s.startBeat + 1)
		if u > 1 {
			s.dead = true
		}
		return kart.EvalBezier(s.spec.miss, u)
	default:
		return kart.EvalBezier(s.spec.enter, 1)
	}
}

func (s *shape) atOrNow(beat float64, name string, vol float64) {
	if beat <= s.mod.ctx.Beat()+1e-6 {
		s.mod.ctx.SoundVol(name, vol)
		return
	}
	s.mod.ctx.SoundAt(beat, name, vol)
}

func itoa(n int) string {
	if n < 10 {
		return string(byte('0' + n))
	}
	return ""
}
