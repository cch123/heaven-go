package animalacrobat

import (
	"image/color"
	"math"
	"math/rand"

	"github.com/hajimehoshi/ebiten/v2"

	"hsdemo/kart"
)

const (
	acrobatConfettiSprite = "__animal_acrobat_confetti"

	// PartyPoppers/Confetti*/Sprite/Confetti/stream ParticleSystem values.
	// The referenced shared mesh GUID is absent from the public source tree, so
	// these mesh bounds come from the sibling confetti.fbx used by the older
	// Confetti systems in the same prefab. Unity renders the white mesh with
	// vertex particle colors, which is equivalent to a white runtime sprite.
	acrobatConfettiBurstCount       = 6
	acrobatConfettiPopIntroSec      = 0.23333333
	acrobatConfettiLifeMinSec       = 0.4
	acrobatConfettiLifeMaxSec       = 0.6
	acrobatConfettiSimSpeed         = 0.9
	acrobatConfettiSpeedMin         = 66
	acrobatConfettiSpeedMax         = 77
	acrobatConfettiVelocityXMin     = 0
	acrobatConfettiVelocityXMax     = 1
	acrobatConfettiVelocityYMin     = -4
	acrobatConfettiVelocityYMax     = 0
	acrobatConfettiGravity          = -9.81 * 4
	acrobatConfettiShapeAngleDeg    = 25
	acrobatConfettiShapeArcDeg      = 80
	acrobatConfettiShapeRadius      = 0.8
	acrobatConfettiRandomPosition   = 0.1
	acrobatConfettiStartSizeX       = 0.6
	acrobatConfettiStartSizeY       = 17
	acrobatConfettiStartRotationRad = math.Pi
	acrobatConfettiMeshWidth        = 1.101369023323059
	acrobatConfettiMeshHeight       = 0.14055192470550537
	acrobatConfettiMeshPivotY       = 0.010995235928371916
	acrobatConfettiOrder            = 111
)

var confettiStreamPaths = [...]string{
	"PartyPoppers/ConfettiL/Sprite/Confetti/stream",
	"PartyPoppers/ConfettiR/Sprite/Confetti/stream",
}

var confettiStartColors = [...][4]float64{
	{0.9607844, 0.86274517, 0.003921569, 1},
	{0.17254902, 0.09411766, 0.9803922, 1},
	{1, 0.33333334, 0.2392157, 1},
	{0.9215687, 0.14901961, 0.70980394, 1},
	{1, 0.454902, 0.050980397, 1},
	{0, 0.8352942, 0.38823533, 1},
}

func registerConfettiSprite(as *kart.Assets) {
	if as == nil {
		return
	}
	if _, ok := as.Sheet.Sprites[acrobatConfettiSprite]; ok {
		return
	}
	img := ebiten.NewImage(1, 1)
	img.Fill(color.White)
	as.RegisterSprite(acrobatConfettiSprite, img, 1, 0.5, acrobatConfettiMeshPivotY)
}

func (m *Module) confettiPopDelayBeats(beat float64) float64 {
	dur := acrobatConfettiPopIntroSec
	secPerBeat := 1.0
	if m.ctx != nil && m.ctx.Assets != nil {
		if anim := m.ctx.Assets.Anims["Animations/PopIntro"]; anim != nil && anim.Duration > 0 {
			dur = anim.Duration
		}
	}
	if m.ctx != nil {
		secPerBeat = m.ctx.SecPerBeat(beat)
	}
	return dur / secPerBeat
}

func (m *Module) spawnConfettiFromScene(beat float64) {
	if m.ctx == nil || m.ctx.Scene == nil {
		return
	}
	m.ctx.SampleScene(beat)
	t := m.ctx.BeatToTime(beat)
	for i, p := range confettiStreamPaths {
		if world, ok := m.ctx.Scene.NodeWorld(p); ok {
			m.spawnConfettiBurst(beat, t, world, int64(0x434f4e46+i))
		}
	}
}

func (m *Module) spawnConfettiBurst(beat, t float64, base kart.Aff, seedSalt int64) {
	rng := rand.New(rand.NewSource(int64(math.Round(beat*1000)) ^ seedSalt))
	gx, gy := inverseVector(base, 0, acrobatConfettiGravity)
	for i := 0; i < acrobatConfettiBurstCount; i++ {
		dir := degToRad((rng.Float64() - 0.5) * acrobatConfettiShapeAngleDeg)
		speed := lerp(acrobatConfettiSpeedMin, acrobatConfettiSpeedMax, rng.Float64())
		vx := math.Cos(dir)*speed + lerp(acrobatConfettiVelocityXMin, acrobatConfettiVelocityXMax, rng.Float64())
		vy := math.Sin(dir)*speed + lerp(acrobatConfettiVelocityYMin, acrobatConfettiVelocityYMax, rng.Float64())
		radius := acrobatConfettiShapeRadius * (1 + (rng.Float64()-0.5)*acrobatConfettiRandomPosition)
		arc := degToRad((rng.Float64() - 0.5) * acrobatConfettiShapeArcDeg)
		x := math.Cos(arc) * radius
		y := math.Sin(arc) * radius
		life := lerp(acrobatConfettiLifeMinSec, acrobatConfettiLifeMaxSec, rng.Float64()) / acrobatConfettiSimSpeed
		rot := math.Atan2(vy, vx) - math.Pi/2 + acrobatConfettiStartRotationRad
		m.particles = append(m.particles, acrobatParticle{
			sprite:      acrobatConfettiSprite,
			born:        t,
			life:        life,
			x:           x,
			y:           y,
			vx:          vx,
			vy:          vy,
			ax:          gx,
			ay:          gy,
			rot:         rot,
			startSize:   acrobatConfettiMeshWidth * acrobatConfettiStartSizeX,
			startSizeY:  acrobatConfettiMeshHeight * acrobatConfettiStartSizeY,
			sizeProfile: sizeProfileConfetti,
			alpha:       alphaProfileConfetti,
			tint:        confettiStartColors[rng.Intn(len(confettiStartColors))],
			order:       acrobatConfettiOrder,
			base:        base,
			local:       true,
		})
	}
}

func inverseVector(m kart.Aff, x, y float64) (float64, float64) {
	det := m.A*m.D - m.B*m.C
	if math.Abs(det) < 1e-9 {
		return x, y
	}
	return (m.D*x - m.C*y) / det, (-m.B*x + m.A*y) / det
}

func confettiSizeFactors(u float64) (float64, float64) {
	return keyCurve(u, []curveKey{
			{0, 1},
			{0.94930977, 1},
			{1, 0},
		}),
		keyCurve(u, []curveKey{
			{0, 0},
			{0.39567584, 1},
			{0.6186864, 1},
			{1, 0},
		})
}

func confettiAlpha(u float64) float64 {
	if u <= 50773.0/65535.0 {
		return 1
	}
	return clamp01((64485.0/65535.0 - u) / ((64485.0 - 50773.0) / 65535.0))
}
