package rhythmrally

import (
	"image/color"
	"math"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"hsdemo/engine"
	"hsdemo/kart"
)

const (
	ballTrailLifetime         = 0.08
	ballTrailRateOverDistance = 8.0
	ballTrailSize             = 0.08
	ballTrailSamples          = 24
)

var ballTrailColor = color.RGBA{R: 0xff, G: 0xf5, B: 0x00, A: 0xff}

type trailParticle struct {
	pos [3]float64
	age float64
}

type trailSample struct {
	beat float64
	pos  [3]float64
	dist float64
}

func (m *Module) drawBallTrail(screen *ebiten.Image, beat float64) {
	if m.ctx == nil {
		return
	}
	for _, p := range m.ballTrailParticles(beat, m.ctx.SecPerBeat(beat)) {
		x, y, sc := projectRhythmScenePoint(p.pos, m.cameraFOV)
		r := ballTrailSize * 130 * sc / 2
		alpha := ballTrailAlpha(p.age)
		if r <= 0 || alpha <= 0 {
			continue
		}
		c := ballTrailColor
		c.A = uint8(math.Round(255 * alpha))
		vector.DrawFilledCircle(screen, float32(x), float32(y), float32(r), c, true)
	}
}

func (m *Module) ballTrailParticles(beat, secPerBeat float64) []trailParticle {
	if !m.ball.ballActive || !m.ball.started || m.ball.tossing || secPerBeat <= 0 {
		return nil
	}
	lifeBeats := ballTrailLifetime / secPerBeat
	startBeat := beat - lifeBeats
	if startBeat < m.ball.serveBeat {
		startBeat = m.ball.serveBeat
	}
	if m.ball.missed && !m.ball.tossing && startBeat < m.ball.missBeat {
		startBeat = m.ball.missBeat
	}
	if startBeat >= beat {
		return nil
	}

	samples := make([]trailSample, 0, ballTrailSamples+1)
	prev := m.ballPositionAt(startBeat, false)
	samples = append(samples, trailSample{beat: startBeat, pos: prev})
	total := 0.0
	for i := 1; i <= ballTrailSamples; i++ {
		b := startBeat + (beat-startBeat)*float64(i)/ballTrailSamples
		pos := m.ballPositionAt(b, false)
		total += dist3(prev, pos)
		samples = append(samples, trailSample{beat: b, pos: pos, dist: total})
		prev = pos
	}
	if total <= 0 {
		return nil
	}

	step := 1 / ballTrailRateOverDistance
	out := make([]trailParticle, 0, int(total/step)+1)
	for target := 0.0; target <= total+1e-9 && len(out) < 64; target += step {
		b, pos := interpolateTrailSample(samples, target)
		out = append(out, trailParticle{
			pos: pos,
			age: clamp01((beat - b) * secPerBeat / ballTrailLifetime),
		})
	}
	return out
}

func interpolateTrailSample(samples []trailSample, target float64) (float64, [3]float64) {
	if len(samples) == 0 {
		return 0, [3]float64{}
	}
	if target <= 0 {
		return samples[0].beat, samples[0].pos
	}
	for i := 1; i < len(samples); i++ {
		if target > samples[i].dist {
			continue
		}
		prev, cur := samples[i-1], samples[i]
		seg := cur.dist - prev.dist
		if seg <= 0 {
			return cur.beat, cur.pos
		}
		u := (target - prev.dist) / seg
		return prev.beat + (cur.beat-prev.beat)*u, lerp3(prev.pos, cur.pos, u)
	}
	last := samples[len(samples)-1]
	return last.beat, last.pos
}

func ballTrailAlpha(age float64) float64 {
	age = clamp01(age)
	// colorOverLifetime keeps full alpha until halfway through the 0.08 s
	// lifetime, then fades to transparent at death.
	if age <= 0.5 {
		return 1
	}
	return (1 - age) * 2
}

func projectRhythmScenePoint(p [3]float64, fov float64) (float64, float64, float64) {
	focal := kart.CameraFocalDistance(fov)
	ps := focal / (focal + p[2])
	if ps <= 0 {
		return 0, 0, 0
	}
	return float64(engine.ScreenW)/2 + p[0]*130*ps, float64(engine.ScreenH)/2 + 60 - p[1]*130*ps, ps
}

func dist3(a, b [3]float64) float64 {
	dx, dy, dz := a[0]-b[0], a[1]-b[1], a[2]-b[2]
	return math.Sqrt(dx*dx + dy*dy + dz*dz)
}

func lerp3(a, b [3]float64, u float64) [3]float64 {
	return [3]float64{
		a[0] + (b[0]-a[0])*u,
		a[1] + (b[1]-a[1])*u,
		a[2] + (b[2]-a[2])*u,
	}
}
