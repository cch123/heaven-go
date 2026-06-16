package particlefx

import (
	"math"
	"sort"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"

	"hsdemo/kart"
	"hsdemo/kmdata"
)

type Effect struct {
	Beat, StartT float64
	Root, Anchor string
	Pos          [2]float64
	Life         float64
}

type Runtime struct {
	Assets       *kart.Assets
	Proj         kart.Aff
	FallbackLife float64
	Roots        map[string][]kmdata.ParticleSystem
	Worlds       map[string]kart.Aff
	Lifetimes    map[string]float64
}

func New(as *kart.Assets, proj kart.Aff, fallbackLife float64) *Runtime {
	roots := Roots(as)
	return &Runtime{
		Assets:       as,
		Proj:         proj,
		FallbackLife: fallbackLife,
		Roots:        roots,
		Worlds:       Worlds(as),
		Lifetimes:    Lifetimes(roots, fallbackLife),
	}
}

func (r *Runtime) NewEffect(root, anchor string, pos [2]float64, beat, startT float64) (Effect, bool) {
	if root == "" || len(r.Roots[root]) == 0 {
		return Effect{}, false
	}
	life := r.Lifetimes[root]
	if life <= 0 {
		life = r.FallbackLife
	}
	return Effect{Beat: beat, StartT: startT, Root: root, Anchor: firstNonEmpty(anchor, root), Pos: pos, Life: life}, true
}

func (r *Runtime) Draw(dst *ebiten.Image, fx Effect, now float64) {
	age := now - fx.StartT
	if age < 0 || age > fx.Life {
		return
	}
	systems := r.Roots[fx.Root]
	if len(systems) == 0 {
		return
	}
	base := kart.Translate(fx.Pos[0], fx.Pos[1])
	anchorWorld, ok := r.Worlds[firstNonEmpty(fx.Anchor, fx.Root)]
	if !ok {
		anchorWorld = kart.Identity()
	}
	invAnchor, ok := InvertAff(anchorWorld)
	if !ok {
		invAnchor = kart.Identity()
	}
	for si := range systems {
		ps := &systems[si]
		rel := kart.Identity()
		if world, ok := r.Worlds[ps.Path]; ok {
			rel = invAnchor.Mul(world)
		}
		r.drawSystem(dst, fx, *ps, base.Mul(rel), age)
	}
}

func Roots(as *kart.Assets) map[string][]kmdata.ParticleSystem {
	paths := make([]string, 0, len(as.Particles.Systems))
	for _, ps := range as.Particles.Systems {
		if ps.Active && ps.Enabled && ps.Renderer.Enabled {
			paths = append(paths, ps.Path)
		}
	}
	out := map[string][]kmdata.ParticleSystem{}
	for _, root := range paths {
		// Unity's ParticleSystem.Play defaults to withChildren=true. Building a
		// group for every valid prefix lets game code trigger the authored prefab
		// root without hard-coding its child emitters.
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

func Worlds(as *kart.Assets) map[string]kart.Aff {
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

func Lifetimes(roots map[string][]kmdata.ParticleSystem, fallback float64) map[string]float64 {
	out := map[string]float64{}
	for root, systems := range roots {
		life := 0.0
		for _, ps := range systems {
			simSpeed := ps.SimulationSpeed
			if simSpeed <= 0 {
				simSpeed = 1
			}
			delay := curveValue(ps.StartDelay, 0, 0)
			maxLife := curveMax(ps.StartLifetime)
			for _, burst := range ps.Emission.Bursts {
				life = math.Max(life, delay+burst.Time+maxLife/simSpeed)
			}
		}
		out[root] = math.Max(life, fallback)
	}
	return out
}

func (r *Runtime) drawSystem(dst *ebiten.Image, fx Effect, ps kmdata.ParticleSystem, world kart.Aff, age float64) {
	if !ps.Enabled || !ps.Active || !ps.Emission.Enabled || len(ps.TextureSheet.Sprites) == 0 {
		return
	}
	simSpeed := ps.SimulationSpeed
	if simSpeed <= 0 {
		simSpeed = 1
	}
	delay := curveValue(ps.StartDelay, 0, 0)
	for bi, burst := range ps.Emission.Bursts {
		sysAge := (age - delay - burst.Time) * simSpeed
		if sysAge < 0 {
			continue
		}
		count := int(math.Round(curveValue(burst.Count, 0, 0.5)))
		if count <= 0 {
			continue
		}
		if ps.MaxParticles > 0 && count > ps.MaxParticles {
			count = ps.MaxParticles
		}
		for i := 0; i < count; i++ {
			seed := particleSeed(fx, ps.Path, bi, i)
			life := curveValue(ps.StartLifetime, 0, particleRand(seed, 1))
			if life <= 0 || sysAge > life {
				continue
			}
			u := clamp01(sysAge / life)
			sprite := ps.TextureSheet.Sprites[int(math.Floor(particleRand(seed, 2)*float64(len(ps.TextureSheet.Sprites))))%len(ps.TextureSheet.Sprites)]
			x, y := startOffset(ps, particleRand(seed, 3), particleRand(seed, 4))
			vx, vy := startVelocity(ps, particleRand(seed, 5))
			if ps.VelocityOverLifetime.Enabled {
				vx += curveValue(ps.VelocityOverLifetime.X, u, particleRand(seed, 6))
				vy += curveValue(ps.VelocityOverLifetime.Y, u, particleRand(seed, 7))
			}
			ax, ay := 0.0, -math.Abs(curveValue(ps.GravityModifier, u, 0))
			if ps.ForceOverLifetime.Enabled {
				ax += curveValue(ps.ForceOverLifetime.X, u, particleRand(seed, 8))
				ay += curveValue(ps.ForceOverLifetime.Y, u, particleRand(seed, 9))
			}
			x += vx*sysAge + 0.5*ax*sysAge*sysAge
			y += vy*sysAge + 0.5*ay*sysAge*sysAge

			size := curveValue(ps.StartSize, u, particleRand(seed, 10))
			sx, sy := size, size
			if ps.SizeOverLifetime.Enabled {
				mul := curveValue(ps.SizeOverLifetime.Curve, u, particleRand(seed, 11))
				if mul != 0 {
					sx *= mul
					sy *= mul
				}
				if xmul := curveValue(ps.SizeOverLifetime.X, u, particleRand(seed, 12)); xmul != 0 {
					sx *= xmul
				}
				if ymul := curveValue(ps.SizeOverLifetime.Y, u, particleRand(seed, 13)); ymul != 0 {
					sy *= ymul
				}
			}
			if ps.Renderer.LengthScale > 0 {
				sy *= ps.Renderer.LengthScale
			}
			if sx == 0 || sy == 0 {
				continue
			}

			rot := curveValue(ps.StartRotation, u, particleRand(seed, 14))
			if ps.RotationOverLifetime.Enabled {
				rot += curveValue(ps.RotationOverLifetime.Curve, u, particleRand(seed, 15)) * sysAge
				rot += curveValue(ps.RotationOverLifetime.Z, u, particleRand(seed, 16)) * sysAge
			}
			tint := startColor(ps.StartColor, particleRand(seed, 17))
			if ps.ColorOverLifetime.Enabled {
				tint = mulColor(tint, gradientColor(ps.ColorOverLifetime.Color, u, particleRand(seed, 18)))
			}
			if tint[3] <= 0 {
				continue
			}
			opts := kart.SpriteOpts{Tint: tint}
			opts.FlipX = ps.Renderer.Flip[0] != 0
			opts.FlipY = ps.Renderer.Flip[1] != 0
			r.Assets.DrawSpriteOpts(dst, sprite, world.Mul(kart.TRS(x, y, rot, sx, sy)), r.Proj, opts)
		}
	}
}

func startOffset(ps kmdata.ParticleSystem, rx, ry float64) (float64, float64) {
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

func startVelocity(ps kmdata.ParticleSystem, r float64) (float64, float64) {
	speed := curveValue(ps.StartSpeed, 0, r)
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

func curveValue(c kmdata.ParticleCurve, u, r float64) float64 {
	switch c.Mode {
	case 1:
		return c.Scalar * keysValue(c.Max, u, 1)
	case 2:
		minv := c.MinScalar * keysValue(c.Min, u, 1)
		maxv := c.Scalar * keysValue(c.Max, u, 1)
		return lerp(minv, maxv, r)
	case 3:
		return lerp(c.MinScalar, c.Scalar, r)
	default:
		return c.Scalar
	}
}

func curveMax(c kmdata.ParticleCurve) float64 {
	switch c.Mode {
	case 1:
		return math.Abs(c.Scalar) * keysMax(c.Max, 1)
	case 2:
		return math.Max(math.Abs(c.MinScalar)*keysMax(c.Min, 1), math.Abs(c.Scalar)*keysMax(c.Max, 1))
	case 3:
		return math.Max(c.MinScalar, c.Scalar)
	default:
		return c.Scalar
	}
}

func keysValue(keys []kmdata.Key, t, fallback float64) float64 {
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

func keysMax(keys []kmdata.Key, fallback float64) float64 {
	if len(keys) == 0 {
		return fallback
	}
	maxv := math.Abs(keys[0].V)
	for _, k := range keys[1:] {
		maxv = math.Max(maxv, math.Abs(k.V))
	}
	return maxv
}

func startColor(g kmdata.ParticleGradient, r float64) [4]float64 {
	switch g.Mode {
	case 1:
		return gradKeysColor(g.MaxGradient, 0)
	case 3:
		return lerpColor(g.MinColor, g.MaxColor, r)
	default:
		if g.MaxColor != ([4]float64{}) {
			return g.MaxColor
		}
		return [4]float64{1, 1, 1, 1}
	}
}

func gradientColor(g kmdata.ParticleGradient, u, r float64) [4]float64 {
	switch g.Mode {
	case 1:
		return gradKeysColor(g.MaxGradient, u)
	case 3:
		return lerpColor(gradKeysColor(g.MinGradient, u), gradKeysColor(g.MaxGradient, u), r)
	default:
		return startColor(g, r)
	}
}

func gradKeysColor(g kmdata.ParticleGradKeys, u float64) [4]float64 {
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

func particleSeed(fx Effect, path string, burst, index int) uint64 {
	// Particle positions are reconstructed at draw time, so the seed must only
	// depend on effect identity and particle index. Including frame time here
	// makes authored bursts visibly jitter.
	h := hashString64(path)
	h ^= math.Float64bits(fx.Beat) + 0x9e3779b97f4a7c15 + (h << 6) + (h >> 2)
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

func InvertAff(m kart.Aff) (kart.Aff, bool) {
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

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

func lerp(a, b, u float64) float64 { return a + (b-a)*u }
