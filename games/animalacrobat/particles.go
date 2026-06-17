package animalacrobat

import (
	"math"
	"math/rand"

	"hsdemo/kart"
)

const (
	acrobatStarSprite  = "blancoacrobat_61"
	acrobatRingSprite  = "blancoacrobat_62"
	acrobatSweatSprite = "blancoacrobat_23"

	// These values are from PlayerMonkey.prefab:
	// Sparkle/GreySparkle/Sweat/SparkleTrail ParticleSystem Initial,
	// Emission, Velocity, Size, UV, and Renderer modules. Unity's
	// ParticleSystem uses startSize as billboard world height, so queueParticle
	// converts it back from the extracted SpriteRenderer units each frame.
	acrobatSparkleBurstCount = 7
	acrobatRingBurstCount    = 1
	acrobatSweatBurstCount   = 3

	acrobatHoldSparkleSpeed       = 9
	acrobatHoldSparkleVelY        = 10
	acrobatHoldSparkleLifeMinSec  = 0.55
	acrobatHoldSparkleLifeMaxSec  = 1.1
	acrobatHoldSparkleSizeMin     = 1.7
	acrobatHoldSparkleSizeMax     = 2.1
	acrobatHoldSparkleSimSpeed    = 1.4
	acrobatReleaseSparkleSpeed    = 9
	acrobatReleaseSparkleVelY     = 8
	acrobatReleaseSparkleLifeMin  = 0.55
	acrobatReleaseSparkleLifeMax  = 0.9
	acrobatReleaseSparkleSizeMin  = 0.8
	acrobatReleaseSparkleSizeMax  = 0.9
	acrobatReleaseSparkleSimSpeed = 1.3
	acrobatSparkleConeDeg         = 25
	acrobatSparkleOrder           = 900

	acrobatRingLifeSec = 0.255 / 1.3
	acrobatRingSize    = 1
	acrobatRingOrder   = 1000

	acrobatSweatLifeSec      = 0.5 / 1.5
	acrobatSweatSize         = 0.18
	acrobatSweatSpeedA       = 15
	acrobatSweatSpeedB       = 13
	acrobatSweatVelX         = -3
	acrobatSweatOrder        = 900
	acrobatSweatClampSpeed   = 1
	acrobatSweatAngleADeg    = 102.952
	acrobatSweatAngleBDeg    = 175.985
	acrobatSweatStartDelayBS = 0.22

	acrobatTrailRateA       = 8
	acrobatTrailRateSmall   = 12
	acrobatTrailRateLate    = 7
	acrobatTrailLifeAMinSec = 1
	acrobatTrailLifeAMaxSec = 1.2
	acrobatTrailLifeSMinSec = 0.5
	acrobatTrailLifeSMaxSec = 0.7
	acrobatTrailSizeAMin    = 1.7
	acrobatTrailSizeAMax    = 2.1
	acrobatTrailSizeSMin    = 0.7
	acrobatTrailSizeSMax    = 1
	acrobatTrailVelA        = 2
	acrobatTrailVelSmall    = 3
	acrobatTrailOrder       = 1
	acrobatTrailLateOrder   = 999
)

func (m *Module) spawnHoldParticle(ob *acrobatObstacle, beat float64) {
	x, y := m.obstacleParticleAnchor(ob, ob.input.holdParticleRel, beat)
	m.spawnStarBurst(beat, m.ctx.BeatToTime(beat), x, y, acrobatParticleHold, 0x484f4c44)
}

func (m *Module) spawnSweatParticle(ob *acrobatObstacle, beat float64) {
	x, y := m.obstacleParticleAnchor(ob, ob.input.sweatParticleRel, beat)
	t := m.ctx.BeatToTime(beat)
	m.spawnSweatBurst(beat, t, x, y)
}

func (m *Module) spawnReleaseParticle(beat, x, y float64) {
	m.spawnStarBurst(beat, m.ctx.BeatToTime(beat), x, y, acrobatParticleRelease, 0x52454c53)
}

func (m *Module) obstacleParticleAnchor(ob *acrobatObstacle, rel string, beat float64) (float64, float64) {
	if ob == nil || ob.inst == nil || rel == "" {
		if ob == nil {
			return 0, 0
		}
		return ob.x + ob.gripX, ob.gripY
	}
	if world, ok := ob.inst.NodeWorldAt(rel, kart.Identity(), beat); ok {
		return world.Apply(0, 0)
	}
	return ob.x + ob.gripX, ob.gripY
}

type acrobatParticleKind int

const (
	acrobatParticleHold acrobatParticleKind = iota
	acrobatParticleRelease
)

func (m *Module) spawnStarBurst(beat, t, x, y float64, kind acrobatParticleKind, seedSalt int64) {
	rng := rand.New(rand.NewSource(int64(math.Round(beat*1000)) ^ seedSalt))
	speed, vy := float64(acrobatHoldSparkleSpeed), float64(acrobatHoldSparkleVelY)
	lifeMin, lifeMax := acrobatHoldSparkleLifeMinSec, acrobatHoldSparkleLifeMaxSec
	sizeMin, sizeMax := acrobatHoldSparkleSizeMin, acrobatHoldSparkleSizeMax
	simSpeed := acrobatHoldSparkleSimSpeed
	profile := sizeProfileSparkle
	tintMode := alphaProfileSparkle
	if kind == acrobatParticleRelease {
		speed, vy = acrobatReleaseSparkleSpeed, acrobatReleaseSparkleVelY
		lifeMin, lifeMax = acrobatReleaseSparkleLifeMin, acrobatReleaseSparkleLifeMax
		sizeMin, sizeMax = acrobatReleaseSparkleSizeMin, acrobatReleaseSparkleSizeMax
		simSpeed = acrobatReleaseSparkleSimSpeed
		profile = sizeProfileRelease
		tintMode = alphaProfileOpaque
	}
	for i := 0; i < acrobatSparkleBurstCount; i++ {
		ang := math.Pi/2 + degToRad((rng.Float64()-0.5)*acrobatSparkleConeDeg)
		life := lerp(lifeMin, lifeMax, rng.Float64()) / simSpeed
		size := lerp(sizeMin, sizeMax, rng.Float64())
		m.particles = append(m.particles, acrobatParticle{
			sprite: acrobatStarSprite,
			born:   t, life: life, x: x, y: y,
			vx:        math.Cos(ang) * speed,
			vy:        math.Sin(ang)*speed + vy,
			rot:       rng.Float64() * math.Pi * 2,
			spin:      (rng.Float64()*2 - 1) * math.Pi * 3,
			startSize: size, sizeProfile: profile, alpha: tintMode,
			tint: sparkleTint(rng, kind), order: acrobatSparkleOrder,
		})
	}
	for i := 0; i < acrobatRingBurstCount; i++ {
		m.particles = append(m.particles, acrobatParticle{
			sprite: acrobatRingSprite,
			born:   t, life: acrobatRingLifeSec, x: x, y: y,
			startSize: acrobatRingSize, sizeProfile: sizeProfileRing, alpha: alphaProfileRing,
			tint: ringTint(rng), order: acrobatRingOrder,
		})
	}
}

func (m *Module) spawnSweatBurst(beat, t, x, y float64) {
	rng := rand.New(rand.NewSource(int64(math.Round(beat*1000)) ^ 0x53574554))
	m.spawnSweatSide(rng, t, x, y, acrobatSweatSpeedA, acrobatSweatAngleADeg, 0)
	m.spawnSweatSide(rng, t+acrobatSweatStartDelayBS, x, y, acrobatSweatSpeedB, acrobatSweatAngleBDeg, 1)
}

func (m *Module) spawnSweatSide(rng *rand.Rand, t, x, y, speed, angleDeg float64, side int) {
	base := degToRad(angleDeg)
	for i := 0; i < acrobatSweatBurstCount; i++ {
		ang := base + degToRad((rng.Float64()-0.5)*acrobatSparkleConeDeg)
		vx := math.Cos(ang)*speed + acrobatSweatVelX
		vy := math.Sin(ang) * speed
		spd := math.Hypot(vx, vy)
		if spd > acrobatSweatClampSpeed {
			vx *= acrobatSweatClampSpeed / spd
			vy *= acrobatSweatClampSpeed / spd
		}
		m.particles = append(m.particles, acrobatParticle{
			sprite: acrobatSweatSprite,
			born:   t, life: acrobatSweatLifeSec, x: x, y: y,
			vx: vx, vy: vy,
			rot:       base + float64(side)*0.2,
			spin:      (rng.Float64()*2 - 1) * math.Pi,
			startSize: acrobatSweatSize, sizeProfile: sizeProfileSweat, alpha: alphaProfileSweat,
			tint: [4]float64{1, 1, 1, 1}, order: acrobatSweatOrder,
		})
	}
}

func (m *Module) startTrail(t, x, y float64) {
	m.stopTrail(t)
	m.trail.active = true
	m.trail.nextEmit = [3]float64{t, t, t}
	// SparkleTrail's root system is prewarmed in Unity. Seeding a short span of
	// already-born particles avoids a one-frame empty trail on long giraffe arcs.
	for _, dt := range []float64{-0.18, -0.09, 0} {
		m.emitTrailParticle(t+dt, x, y, 0)
		m.emitTrailParticle(t+dt, x, y, 1)
	}
}

func (m *Module) stopTrail(t float64) {
	if !m.trail.active && len(m.particles) == 0 {
		return
	}
	m.trail.active = false
	for i := range m.particles {
		if m.particles[i].alpha != alphaProfileTrail {
			continue
		}
		if remaining := m.particles[i].born + m.particles[i].life - t; remaining > 0.2 {
			m.particles[i].life = t + 0.2 - m.particles[i].born
		}
	}
}

func (m *Module) updateTrail(t float64) {
	if !m.trail.active {
		return
	}
	rates := [3]float64{acrobatTrailRateA, acrobatTrailRateSmall, acrobatTrailRateLate}
	for i, rate := range rates {
		step := 1 / rate
		if m.trail.nextEmit[i] <= 0 || t-m.trail.nextEmit[i] > 0.5 {
			m.trail.nextEmit[i] = t
		}
		for m.trail.nextEmit[i] <= t {
			m.emitTrailParticle(m.trail.nextEmit[i], m.playerX, m.playerY, i)
			m.trail.nextEmit[i] += step
		}
	}
}

func (m *Module) emitTrailParticle(born, x, y float64, system int) {
	rng := rand.New(rand.NewSource(int64(math.Round(born*10000)) ^ int64(0x74726100+system)))
	lifeMin, lifeMax := float64(acrobatTrailLifeAMinSec), float64(acrobatTrailLifeAMaxSec)
	sizeMin, sizeMax := acrobatTrailSizeAMin, acrobatTrailSizeAMax
	velY := float64(acrobatTrailVelA)
	profile := sizeProfileTrail
	order := acrobatTrailOrder
	if system == 1 {
		lifeMin, lifeMax = acrobatTrailLifeSMinSec, acrobatTrailLifeSMaxSec
		sizeMin, sizeMax = acrobatTrailSizeSMin, acrobatTrailSizeSMax
		velY = acrobatTrailVelSmall
	}
	if system == 2 {
		order = acrobatTrailLateOrder
		profile = sizeProfileTrailSmall
	}
	life := lerp(lifeMin, lifeMax, rng.Float64())
	size := lerp(sizeMin, sizeMax, rng.Float64())
	ang := math.Pi/2 + degToRad((rng.Float64()-0.5)*acrobatSparkleConeDeg)
	m.particles = append(m.particles, acrobatParticle{
		sprite: acrobatStarSprite,
		born:   born, life: life, x: x - 0.14800024, y: y + 0.407,
		vx:        math.Cos(ang) * 0.35,
		vy:        math.Sin(ang)*0.35 + velY,
		rot:       rng.Float64() * math.Pi * 2,
		spin:      (rng.Float64()*2 - 1) * math.Pi * 2,
		startSize: size, sizeProfile: profile, alpha: alphaProfileTrail,
		tint: trailTint(rng, system), order: order,
	})
}

func (m *Module) queueParticles(t float64) {
	m.updateTrail(t)
	alive := m.particles[:0]
	for _, p := range m.particles {
		if t < p.born {
			alive = append(alive, p)
			continue
		}
		age := t - p.born
		if age > p.life {
			continue
		}
		alive = append(alive, p)
		u := clamp01(age / p.life)
		size := p.startSize * particleSizeFactor(p.sizeProfile, u)
		sx, sy := m.particleSpriteScale(p.sprite, size)
		if p.sizeProfile == sizeProfileConfetti {
			xf, yf := confettiSizeFactors(u)
			sx, sy = p.startSize*xf, p.startSizeY*yf
		}
		if sx == 0 || sy == 0 {
			continue
		}
		tint := p.tint
		tint[3] *= particleAlpha(p.alpha, u)
		x := p.x + p.vx*age + 0.5*p.ax*age*age
		y := p.y + p.vy*age + 0.5*p.ay*age*age
		world := kart.TRS(x, y, p.rot+p.spin*age, sx, sy)
		if p.local {
			world = p.base.Mul(world)
		}
		m.ctx.Scene.Queue(kart.ExtraSprite{Sprite: p.sprite, World: world, Order: p.order, Tint: tint})
	}
	m.particles = alive
}

func (m *Module) particleSpriteScale(sprite string, particleSize float64) (float64, float64) {
	if m.ctx == nil || m.ctx.Assets == nil || particleSize <= 0 {
		return 0, 0
	}
	sp, ok := m.ctx.Assets.Sheet.Sprites[sprite]
	if !ok {
		return 0, 0
	}
	ppu := sp.PPU
	if ppu <= 0 {
		ppu = m.ctx.Assets.Sheet.PPU
	}
	if ppu <= 0 || sp.H == 0 {
		return 0, 0
	}
	unitH := float64(sp.H) / ppu
	scale := particleSize / unitH
	return scale, scale
}

func particleSizeFactor(profile particleSizeProfile, u float64) float64 {
	switch profile {
	case sizeProfileRelease:
		return keyCurve(u, []curveKey{
			{0.03431365, 0}, {0.20002776, 0.71580964}, {0.33333233, 0.5328404},
			{0.4657234, 0.63055277}, {0.67090195, 0.34615493}, {0.9254571, 0},
		})
	case sizeProfileSparkle:
		return keyCurve(u, []curveKey{
			{0, 1}, {0.20002776, 0.71580964}, {0.33333233, 0.5328404},
			{0.4657234, 0.63055277}, {0.67090195, 0.34615493}, {0.9254571, 0},
		})
	case sizeProfileRing:
		return u
	case sizeProfileSweat:
		return lerp(0.94117564, 0.18822926, u)
	case sizeProfileTrail:
		return keyCurve(u, []curveKey{
			{0, 1}, {0.20002776, 0.71580964}, {0.33333233, 0.5328404},
			{0.49023318, 0.7389009}, {0.67090195, 0.34615493}, {0.8666345, 0},
		})
	case sizeProfileTrailSmall:
		return keyCurve(u, []curveKey{
			{0.0068293437, 0.011764526}, {0.19319946, 0.5104187},
			{0.33333233, 0.36065638}, {0.4783008, 0.52614},
			{0.6708979, 0.18353784}, {0.8666345, 0},
		})
	case sizeProfileConfetti:
		x, _ := confettiSizeFactors(u)
		return x
	default:
		return 1
	}
}

func particleAlpha(profile particleAlphaProfile, u float64) float64 {
	switch profile {
	case alphaProfileRing:
		return 1 - u
	case alphaProfileSweat:
		if u <= 47993.0/65535.0 {
			return 1
		}
		return clamp01((58679.0/65535.0 - u) / ((58679.0 - 47993.0) / 65535.0))
	case alphaProfileSparkle, alphaProfileTrail:
		if u <= 40309.0/65535.0 {
			return u / (40309.0 / 65535.0)
		}
		return 1
	case alphaProfileConfetti:
		return confettiAlpha(u)
	default:
		return 1
	}
}

type curveKey struct {
	t, v float64
}

func keyCurve(u float64, keys []curveKey) float64 {
	u = clamp01(u)
	if len(keys) == 0 {
		return 1
	}
	if u <= keys[0].t {
		return keys[0].v
	}
	for i := 1; i < len(keys); i++ {
		if u <= keys[i].t {
			a, b := keys[i-1], keys[i]
			return lerp(a.v, b.v, (u-a.t)/(b.t-a.t))
		}
	}
	return keys[len(keys)-1].v
}

func sparkleTint(rng *rand.Rand, kind acrobatParticleKind) [4]float64 {
	if kind == acrobatParticleRelease {
		return [4]float64{1, 1, 1, 1}
	}
	switch rng.Intn(3) {
	case 0:
		return [4]float64{0, 1, 1, 1}
	case 1:
		return [4]float64{1, 0, 1, 1}
	default:
		return [4]float64{1, 1, 1, 1}
	}
}

func ringTint(rng *rand.Rand) [4]float64 {
	if rng.Intn(2) == 0 {
		return [4]float64{0, 1, 1, 1}
	}
	return [4]float64{1, 1, 0, 1}
}

func trailTint(rng *rand.Rand, system int) [4]float64 {
	if system == 2 {
		if rng.Intn(2) == 0 {
			return [4]float64{0.52, 1, 0.48, 1}
		}
		return [4]float64{1, 1, 0, 1}
	}
	if rng.Intn(2) == 0 {
		return [4]float64{1, 1, 0, 1}
	}
	return [4]float64{0, 1, 1, 1}
}

func degToRad(v float64) float64 { return v * math.Pi / 180 }
