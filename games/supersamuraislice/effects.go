package supersamuraislice

import (
	"math"
	"sort"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"

	"hsdemo/kart"
	"hsdemo/kmdata"
)

const superSliceEffectFallbackLife = 1.15

func (m *Module) addParticleEffect(beat float64, root, anchor string, pos [2]float64) {
	if root == "" {
		return
	}
	startT := 0.0
	if m.ctx != nil {
		startT = m.ctx.Time()
	}
	life := m.particleLife[root]
	if life <= 0 {
		life = superSliceEffectFallbackLife
	}
	m.effects = append(m.effects, effect{
		beat: beat, startT: startT, root: root, anchor: firstNonEmpty(anchor, root), pos: pos, life: life,
	})
}

func (m *Module) smallExplosionParticleRoot(typ int) (root, anchor string) {
	switch typ {
	case effectExplode2:
		return m.smallExplodes[1], m.smallRoot
	case effectExplode3:
		return m.smallExplodes[2], m.smallRoot
	default:
		return m.smallExplodes[0], m.smallRoot
	}
}

func (m *Module) effectNodePos(path string) [2]float64 {
	if m.ctx != nil && m.ctx.Scene != nil {
		if world, ok := m.ctx.Scene.NodeWorld(path); ok {
			return [2]float64{world.Tx, world.Ty}
		}
	}
	if m.ctx != nil && m.ctx.Assets != nil {
		return nodePos(m.ctx.Assets, path)
	}
	return [2]float64{}
}

func (m *Module) drawEffect(dst *ebiten.Image, fx effect, t float64) {
	age := t - fx.startT
	if age < 0 || age > fx.life {
		return
	}
	systems := m.particleRoots[fx.root]
	if len(systems) == 0 {
		return
	}
	base := kart.Translate(fx.pos[0], fx.pos[1])
	anchorWorld, ok := m.particleWorld[firstNonEmpty(fx.anchor, fx.root)]
	if !ok {
		anchorWorld = kart.Identity()
	}
	invAnchor, ok := invertAff(anchorWorld)
	if !ok {
		invAnchor = kart.Identity()
	}
	for si := range systems {
		ps := &systems[si]
		rel := kart.Identity()
		if world, ok := m.particleWorld[ps.Path]; ok {
			rel = invAnchor.Mul(world)
		}
		m.drawParticleSystem(dst, fx, *ps, base.Mul(rel), age)
	}
}

func (m *Module) drawParticleSystem(dst *ebiten.Image, fx effect, ps kmdata.ParticleSystem, world kart.Aff, age float64) {
	if !ps.Enabled || !ps.Active || !ps.Emission.Enabled || len(ps.TextureSheet.Sprites) == 0 {
		return
	}
	simSpeed := ps.SimulationSpeed
	if simSpeed <= 0 {
		simSpeed = 1
	}
	delay := particleCurveValue(ps.StartDelay, 0, 0)
	for bi, burst := range ps.Emission.Bursts {
		sysAge := (age - delay - burst.Time) * simSpeed
		if sysAge < 0 {
			continue
		}
		count := int(math.Round(particleCurveValue(burst.Count, 0, 0.5)))
		if count <= 0 {
			continue
		}
		if ps.MaxParticles > 0 && count > ps.MaxParticles {
			count = ps.MaxParticles
		}
		for i := 0; i < count; i++ {
			seed := particleSeed(fx, ps.Path, bi, i)
			life := particleCurveValue(ps.StartLifetime, 0, particleRand(seed, 1))
			if life <= 0 || sysAge > life {
				continue
			}
			u := clamp01(sysAge / life)
			sprite := ps.TextureSheet.Sprites[int(math.Floor(particleRand(seed, 2)*float64(len(ps.TextureSheet.Sprites))))%len(ps.TextureSheet.Sprites)]
			x, y := particleStartOffset(ps, particleRand(seed, 3), particleRand(seed, 4))
			vx, vy := particleStartVelocity(ps, particleRand(seed, 5))
			if ps.VelocityOverLifetime.Enabled {
				vx += particleCurveValue(ps.VelocityOverLifetime.X, u, particleRand(seed, 6))
				vy += particleCurveValue(ps.VelocityOverLifetime.Y, u, particleRand(seed, 7))
			}
			ax, ay := 0.0, -math.Abs(particleCurveValue(ps.GravityModifier, u, 0))
			if ps.ForceOverLifetime.Enabled {
				ax += particleCurveValue(ps.ForceOverLifetime.X, u, particleRand(seed, 8))
				ay += particleCurveValue(ps.ForceOverLifetime.Y, u, particleRand(seed, 9))
			}
			x += vx*sysAge + 0.5*ax*sysAge*sysAge
			y += vy*sysAge + 0.5*ay*sysAge*sysAge

			size := particleCurveValue(ps.StartSize, u, particleRand(seed, 10))
			sx, sy := size, size
			if ps.SizeOverLifetime.Enabled {
				mul := particleCurveValue(ps.SizeOverLifetime.Curve, u, particleRand(seed, 11))
				if mul != 0 {
					sx *= mul
					sy *= mul
				}
				if xmul := particleCurveValue(ps.SizeOverLifetime.X, u, particleRand(seed, 12)); xmul != 0 {
					sx *= xmul
				}
				if ymul := particleCurveValue(ps.SizeOverLifetime.Y, u, particleRand(seed, 13)); ymul != 0 {
					sy *= ymul
				}
			}
			if ps.Renderer.LengthScale > 0 {
				sy *= ps.Renderer.LengthScale
			}
			if sx == 0 || sy == 0 {
				continue
			}

			rot := particleCurveValue(ps.StartRotation, u, particleRand(seed, 14))
			if ps.RotationOverLifetime.Enabled {
				rot += particleCurveValue(ps.RotationOverLifetime.Curve, u, particleRand(seed, 15)) * sysAge
				rot += particleCurveValue(ps.RotationOverLifetime.Z, u, particleRand(seed, 16)) * sysAge
			}
			tint := particleStartColor(ps.StartColor, particleRand(seed, 17))
			if ps.ColorOverLifetime.Enabled {
				tint = mulColor(tint, particleGradientColor(ps.ColorOverLifetime.Color, u, particleRand(seed, 18)))
			}
			if tint[3] <= 0 {
				continue
			}
			opts := kart.SpriteOpts{Tint: tint}
			opts.FlipX = ps.Renderer.Flip[0] != 0
			opts.FlipY = ps.Renderer.Flip[1] != 0
			m.ctx.Assets.DrawSpriteOpts(dst, sprite, world.Mul(kart.TRS(x, y, rot, sx, sy)), m.proj, opts)
		}
	}
}

func superSliceParticleRoots(as *kart.Assets) map[string][]kmdata.ParticleSystem {
	paths := make([]string, 0, len(as.Particles.Systems))
	for _, ps := range as.Particles.Systems {
		if ps.Active && ps.Enabled && ps.Renderer.Enabled {
			paths = append(paths, ps.Path)
		}
	}
	out := map[string][]kmdata.ParticleSystem{}
	for _, root := range paths {
		// Explosion and lightning prefabs trigger a parent ParticleSystem plus all
		// child systems beneath it. Group every valid prefix so runtime code can
		// fire "demon/Holder/blowup" without hard-coding its child list.
		for _, ps := range as.Particles.Systems {
			if !ps.Active || !ps.Enabled || !ps.Renderer.Enabled {
				continue
			}
			if ps.Path == root || strings.HasPrefix(ps.Path, root+"/") {
				out[root] = append(out[root], ps)
			}
		}
		sort.Slice(out[root], func(i, j int) bool {
			a, b := out[root][i], out[root][j]
			if a.Renderer.SortingOrder != b.Renderer.SortingOrder {
				return a.Renderer.SortingOrder < b.Renderer.SortingOrder
			}
			return a.Path < b.Path
		})
	}
	return out
}

func superSliceParticleLifetimes(roots map[string][]kmdata.ParticleSystem) map[string]float64 {
	out := map[string]float64{}
	for root, systems := range roots {
		life := 0.0
		for _, ps := range systems {
			simSpeed := ps.SimulationSpeed
			if simSpeed <= 0 {
				simSpeed = 1
			}
			delay := particleCurveValue(ps.StartDelay, 0, 0)
			maxLife := particleCurveMax(ps.StartLifetime)
			for _, burst := range ps.Emission.Bursts {
				life = math.Max(life, delay+burst.Time+maxLife/simSpeed)
			}
		}
		out[root] = math.Max(life, superSliceEffectFallbackLife)
	}
	return out
}

func superSliceParticleWorlds(as *kart.Assets) map[string]kart.Aff {
	out := map[string]kart.Aff{}
	for i, n := range as.Rig.Nodes {
		local := kart.TRS(n.Pos[0], n.Pos[1], n.RotZ, nonZero(n.Scale[0], 1), nonZero(n.Scale[1], 1))
		if n.Parent >= 0 && n.Parent < i {
			out[n.Path] = out[as.Rig.Nodes[n.Parent].Path].Mul(local)
		} else {
			out[n.Path] = local
		}
	}
	return out
}

func particleStartOffset(ps kmdata.ParticleSystem, rx, ry float64) (float64, float64) {
	x := ps.Shape.Position[0]
	y := ps.Shape.Position[1]
	sx, sy := ps.Shape.Scale[0], ps.Shape.Scale[1]
	if sx == 0 && sy == 0 && ps.Shape.Radius > 0 {
		a := rx * 2 * math.Pi
		r := math.Sqrt(ry) * ps.Shape.Radius
		return x + math.Cos(a)*r, y + math.Sin(a)*r
	}
	x += (rx - 0.5) * sx
	y += (ry - 0.5) * sy
	return x, y
}

func particleStartVelocity(ps kmdata.ParticleSystem, r float64) (float64, float64) {
	speed := particleCurveValue(ps.StartSpeed, 0, r)
	if speed == 0 {
		return 0, 0
	}
	angle := math.Pi/2 + ps.Shape.Rotation[2]*math.Pi/180
	spread := ps.Shape.Angle * math.Pi / 180
	if ps.Shape.Type == 10 {
		angle = r * 2 * math.Pi
	} else {
		angle += (r - 0.5) * spread * 2
	}
	return math.Cos(angle) * speed, math.Sin(angle) * speed
}

func particleCurveValue(c kmdata.ParticleCurve, u, r float64) float64 {
	switch c.Mode {
	case 1:
		return c.Scalar * particleKeysValue(c.Max, u, 1)
	case 2:
		minv := c.MinScalar * particleKeysValue(c.Min, u, 1)
		maxv := c.Scalar * particleKeysValue(c.Max, u, 1)
		return lerp(minv, maxv, r)
	case 3:
		return lerp(c.MinScalar, c.Scalar, r)
	default:
		return c.Scalar
	}
}

func particleCurveMax(c kmdata.ParticleCurve) float64 {
	switch c.Mode {
	case 1:
		return math.Abs(c.Scalar) * particleKeysMax(c.Max, 1)
	case 2:
		return math.Max(math.Abs(c.MinScalar)*particleKeysMax(c.Min, 1), math.Abs(c.Scalar)*particleKeysMax(c.Max, 1))
	case 3:
		return math.Max(c.MinScalar, c.Scalar)
	default:
		return c.Scalar
	}
}

func particleKeysValue(keys []kmdata.Key, t, fallback float64) float64 {
	if len(keys) == 0 {
		return fallback
	}
	if t <= keys[0].T {
		return keys[0].V
	}
	last := keys[len(keys)-1]
	if t >= last.T {
		return last.V
	}
	i := 0
	for i+1 < len(keys) && keys[i+1].T <= t {
		i++
	}
	k0, k1 := keys[i], keys[i+1]
	if math.Abs(k0.O) >= kmdata.StepSlope || math.Abs(k1.I) >= kmdata.StepSlope || k1.T <= k0.T {
		return k0.V
	}
	u := (t - k0.T) / (k1.T - k0.T)
	h00 := 2*u*u*u - 3*u*u + 1
	h10 := u*u*u - 2*u*u + u
	h01 := -2*u*u*u + 3*u*u
	h11 := u*u*u - u*u
	dt := k1.T - k0.T
	return h00*k0.V + h10*dt*k0.O + h01*k1.V + h11*dt*k1.I
}

func particleKeysMax(keys []kmdata.Key, fallback float64) float64 {
	if len(keys) == 0 {
		return fallback
	}
	maxv := math.Abs(keys[0].V)
	for _, k := range keys[1:] {
		maxv = math.Max(maxv, math.Abs(k.V))
	}
	return maxv
}

func particleStartColor(g kmdata.ParticleGradient, r float64) [4]float64 {
	switch g.Mode {
	case 1:
		return particleGradientKeysColor(g.MaxGradient, 0)
	case 3:
		return lerpColor(g.MinColor, g.MaxColor, r)
	default:
		if g.MaxColor != ([4]float64{}) {
			return g.MaxColor
		}
		return [4]float64{1, 1, 1, 1}
	}
}

func particleGradientColor(g kmdata.ParticleGradient, u, r float64) [4]float64 {
	switch g.Mode {
	case 1:
		return particleGradientKeysColor(g.MaxGradient, u)
	case 3:
		return lerpColor(particleGradientKeysColor(g.MinGradient, u), particleGradientKeysColor(g.MaxGradient, u), r)
	default:
		return particleStartColor(g, r)
	}
}

func particleGradientKeysColor(g kmdata.ParticleGradKeys, u float64) [4]float64 {
	c := [4]float64{1, 1, 1, 1}
	if len(g.ColorKeys) > 0 {
		if u <= g.ColorKeys[0].T {
			c = g.ColorKeys[0].Color
		} else {
			c = g.ColorKeys[len(g.ColorKeys)-1].Color
			for i := 1; i < len(g.ColorKeys); i++ {
				if u <= g.ColorKeys[i].T {
					a, b := g.ColorKeys[i-1], g.ColorKeys[i]
					span := b.T - a.T
					if span <= 0 {
						c = b.Color
					} else {
						c = lerpColor(a.Color, b.Color, (u-a.T)/span)
					}
					break
				}
			}
		}
	}
	if len(g.AlphaKeys) > 0 {
		if u <= g.AlphaKeys[0].T {
			c[3] = g.AlphaKeys[0].A
		} else {
			c[3] = g.AlphaKeys[len(g.AlphaKeys)-1].A
			for i := 1; i < len(g.AlphaKeys); i++ {
				if u <= g.AlphaKeys[i].T {
					a, b := g.AlphaKeys[i-1], g.AlphaKeys[i]
					span := b.T - a.T
					if span <= 0 {
						c[3] = b.A
					} else {
						c[3] = lerp(a.A, b.A, (u-a.T)/span)
					}
					break
				}
			}
		}
	}
	return c
}

func mulColor(a, b [4]float64) [4]float64 {
	return [4]float64{a[0] * b[0], a[1] * b[1], a[2] * b[2], a[3] * b[3]}
}

func lerpColor(a, b [4]float64, u float64) [4]float64 {
	return [4]float64{lerp(a[0], b[0], u), lerp(a[1], b[1], u), lerp(a[2], b[2], u), lerp(a[3], b[3], u)}
}

func particleSeed(fx effect, path string, burst, index int) uint64 {
	// Particle positions are reconstructed at draw time, so the seed must only
	// depend on effect identity and particle index. Including frame time here
	// would recreate the old visible jitter bug.
	h := hashString64(path)
	h ^= math.Float64bits(fx.beat) + 0x9e3779b97f4a7c15 + (h << 6) + (h >> 2)
	h ^= uint64(burst+1)*0xbf58476d1ce4e5b9 + uint64(index+1)*0x94d049bb133111eb
	return h
}

func particleRand(seed uint64, salt int) float64 {
	x := seed + uint64(salt)*0x9e3779b97f4a7c15
	x = (x ^ (x >> 30)) * 0xbf58476d1ce4e5b9
	x = (x ^ (x >> 27)) * 0x94d049bb133111eb
	x ^= x >> 31
	return float64(x>>11) * (1.0 / (1 << 53))
}

func hashString64(s string) uint64 {
	h := uint64(1469598103934665603)
	for i := 0; i < len(s); i++ {
		h ^= uint64(s[i])
		h *= 1099511628211
	}
	return h
}

func invertAff(m kart.Aff) (kart.Aff, bool) {
	det := m.A*m.D - m.B*m.C
	if math.Abs(det) < 1e-9 {
		return kart.Identity(), false
	}
	inv := 1 / det
	return kart.Aff{
		A: m.D * inv, B: -m.B * inv, C: -m.C * inv, D: m.A * inv,
		Tx: (m.C*m.Ty - m.D*m.Tx) * inv,
		Ty: (m.B*m.Tx - m.A*m.Ty) * inv,
	}, true
}

func firstNonEmpty(v, fallback string) string {
	if v != "" {
		return v
	}
	return fallback
}

func nonZero(v, fallback float64) float64 {
	if v != 0 {
		return v
	}
	return fallback
}
