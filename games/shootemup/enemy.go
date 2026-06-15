package shootemup

import (
	"fmt"
	"math"

	"hsdemo/engine"
	"hsdemo/kart"
)

const (
	enemyBasic = iota
	enemyPractice
	enemyEndless
)

const (
	enemyArrange  = 6
	enemyRemix9   = 7
	enemyLockstep = 100
)

var white = [4]float64{1, 1, 1, 1}

type vec2 struct {
	x, y float64
}

type enemy struct {
	m          *Module
	inst       *kart.Instance
	createBeat float64
	interval   float64
	pos        vec2
	base       vec2
	enemyType  int
	colorA     [4]float64
	colorB     [4]float64

	spawned bool
	judged  bool
	deadAt  float64
}

func newEnemy(m *Module, ev spawnEvt, pos vec2, interval float64) *enemy {
	en := &enemy{
		m: m, inst: m.enemyT.NewInstance(),
		createBeat: ev.beat, interval: interval,
		pos: pos, base: enemyPosition(pos),
		enemyType: ev.enemyType, colorA: ev.colorA, colorB: ev.colorB,
	}
	en.inst.Offset = [2]float64{en.base.x, en.base.y}
	en.applyAppearance()
	return en
}

func enemyPosition(pos vec2) vec2 {
	return vec2{x: 5.05 / 3 * pos.x, y: 2.5/3*pos.y + 1.25}
}

func (e *enemy) activate(beat float64, spawnAnim bool) {
	if e.spawned {
		return
	}
	e.spawned = true
	e.applyAppearance()
	if spawnAnim {
		e.inst.Play("", "Animations/enemy/enemySpawn", beat, animScale)
	}
}

func (e *enemy) applyAppearance() {
	e.inst.SetSprite("sprite", enemySprite(e.enemyType))
	e.inst.SetActive("sprite/far", enemyShowsFar(e.enemyType))
	pal := kart.DefaultPalette()
	pal.Alpha = e.colorA
	pal.Outline = e.colorB
	e.inst.SetPalette("sprite", pal)
}

func (e *enemy) startInput(passBeat, relativeBeat float64) {
	target := passBeat + relativeBeat
	e.m.ctx.ScheduleInputAny(target, func(_ float64, j engine.Judgment) {
		e.m.ctx.Sound("shoot")
		now := e.m.ctx.Beat()
		e.m.shoot(now)
		if j == engine.JudgeNG {
			e.judge("miss", now)
			e.m.spawnSmoke(e.currentPos(), now)
			return
		}
		e.judge("just", now)
		e.m.spawnHitParticles(vec2{0, 0.29}, now)
	}, func() {
		now := e.m.ctx.Beat()
		e.m.damageShip(now)
		e.judge("attack", now)
	})
}

func (e *enemy) judge(kind string, beat float64) {
	if e.judged {
		return
	}
	e.judged = true
	current := e.currentPos()
	next := vec2{0, 0.29}
	switch kind {
	case "attack":
		switch {
		case e.pos.x > 0:
			next = vec2{-5, -3}
		case e.pos.x < 0:
			next = vec2{5, -3}
		default:
			next = vec2{0, -1.25}
		}
	}

	e.m.spawnEffect(e.m.originT, "origin", current, 0, vec2{1, 1}, beat)
	e.m.spawnTrajectoryDamage(current, next, beat)
	e.inst.Offset = [2]float64{next.x, next.y}

	switch kind {
	case "just", "attack":
		e.inst.Scale = [2]float64{1.25, 1.25}
		e.inst.Play("", "Animations/enemy/enemyAttack", beat, animScale)
		e.deadAt = beat + animDuration(e.m.ctx, "Animations/enemy/enemyAttack")/animScale
		e.m.spawnEffect(e.m.impactT, "impact", next, 0, vec2{1, 1}, beat)
	case "miss":
		state := "enemyMissRight"
		clip := "Animations/enemy/enemyMissR"
		if e.pos.x > 0 {
			state = "enemyMissLeft"
			clip = "Animations/enemy/enemyMissL"
		}
		e.inst.PlayState("", state, beat, animScale)
		e.deadAt = beat + animDuration(e.m.ctx, clip)/animScale
		e.m.spawnEffect(e.m.missImpactT, "missimpact", current, 0, vec2{1, 1}, beat)
	}
}

func (e *enemy) currentPos() vec2 {
	if e.judged {
		return vec2{e.inst.Offset[0], e.inst.Offset[1]}
	}
	return e.base
}

func (e *enemy) queue(beat float64) bool {
	if !e.spawned {
		return true
	}
	if e.deadAt > 0 && beat > e.deadAt {
		return false
	}
	if !e.judged {
		denom := math.Max(e.interval, 0.001) * 2
		scale := 1 + math.Max(0, beat-e.createBeat)*0.16/denom
		e.inst.Scale = [2]float64{scale, scale}
	}
	e.inst.Queue(e.m.ctx.Scene, beat, kart.Identity(), 0)
	return true
}

type effectInst struct {
	inst       *kart.Instance
	start, end float64
}

func (fx *effectInst) queue(scene *kart.SceneInst, beat float64) bool {
	if beat > fx.end {
		return false
	}
	fx.inst.Queue(scene, beat, kart.Identity(), 0)
	return true
}

func (m *Module) spawnTrajectory(en *enemy, beat float64) {
	rot := 0.0
	switch {
	case en.pos.x > 0 && en.pos.y >= 0:
		rot = deg(-70)
	case en.pos.x < 0 && en.pos.y >= 0:
		rot = deg(70)
	case en.pos.x > 0 && en.pos.y <= 0:
		rot = deg(-110)
	case en.pos.x < 0 && en.pos.y <= 0:
		rot = deg(110)
	}
	m.spawnEffect(m.trajectoryT, "trajectory", en.base, rot, vec2{1, 1}, beat)
}

func (m *Module) spawnTrajectoryDamage(from, to vec2, beat float64) {
	dx, dy := to.x-from.x, to.y-from.y
	angle := math.Pi - math.Atan2(dx, dy)
	dist := math.Hypot(dx, dy)
	m.spawnEffect(m.trajectoryT, "trajectory_damage", to, angle, vec2{1, dist * 0.16}, beat)
}

func (m *Module) spawnEffect(t *kart.Template, state string, pos vec2, rot float64, scale vec2, beat float64) {
	if t == nil {
		return
	}
	inst := t.NewInstance()
	inst.Offset = [2]float64{pos.x, pos.y}
	inst.Rot = rot
	inst.Scale = [2]float64{scale.x, scale.y}
	inst.PlayState("", state, beat, animScale)
	clip := effectClip(state)
	m.effects = append(m.effects, &effectInst{
		inst:  inst,
		start: beat,
		end:   beat + animDuration(m.ctx, clip)/animScale,
	})
}

func effectClip(state string) string {
	switch state {
	case "trajectory", "trajectory_damage":
		return "Animations/Effect/" + state
	case "origin", "impact", "missimpact":
		return "Animations/Effect/" + state
	}
	return state
}

func enemySprite(t int) string {
	switch t {
	case enemyPractice:
		return "shoot_enemy_4"
	case enemyEndless:
		return "shoot_enemy_3"
	case enemyArrange:
		return "shoot_enemy_8"
	case enemyRemix9:
		return "shoot_enemy_7"
	case enemyLockstep:
		return "shoot_enemy_1"
	default:
		return "shoot_enemy_0"
	}
}

func enemyShowsFar(t int) bool {
	return t == enemyBasic
}

func deg(v float64) float64 { return v * math.Pi / 180 }

func _debugEnemyType(t int) string {
	switch t {
	case enemyBasic:
		return "Basic"
	case enemyPractice:
		return "Practice"
	case enemyEndless:
		return "Endless"
	case enemyArrange:
		return "Arrange"
	case enemyRemix9:
		return "Remix9"
	case enemyLockstep:
		return "Lockstep"
	default:
		return fmt.Sprintf("Enemy(%d)", t)
	}
}
