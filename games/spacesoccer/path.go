package spacesoccer

import (
	"math"

	"hsdemo/kmdata"
)

type curvePoint struct {
	pos         [3]float64
	duration    float64
	height      float64
	useLastReal bool
}

type ballPath struct {
	name   string
	points []curvePoint
}

func loadBallPaths(items []kmdata.ComponentItem) map[string]ballPath {
	paths := map[string]ballPath{}
	for _, it := range items {
		name := it.Strs["name"]
		if name == "" {
			continue
		}
		p := ballPath{name: name}
		for _, pi := range it.Items["positions"] {
			p.points = append(p.points, curvePoint{
				pos: [3]float64{
					pi.Nums["pos.x"],
					pi.Nums["pos.y"],
					pi.Nums["pos.z"],
				},
				duration:    pi.Nums["duration"],
				height:      pi.Nums["height"],
				useLastReal: pi.Nums["useLastReal"] != 0,
			})
		}
		if len(p.points) >= 2 {
			paths[name] = p
		}
	}
	return paths
}

func (p ballPath) durationOverride(d float64) ballPath {
	q := p
	q.points = append([]curvePoint(nil), p.points...)
	if len(q.points) > 0 {
		q.points[0].duration = d
	}
	return q
}

func (p ballPath) endOverride(x, y, z float64) ballPath {
	q := p
	q.points = append([]curvePoint(nil), p.points...)
	if len(q.points) > 1 {
		q.points[1].pos = [3]float64{x, y, z}
	}
	return q
}

func (p ballPath) posAt(beat, startBeat float64, lastReal [2]float64) [3]float64 {
	if len(p.points) == 0 {
		return [3]float64{}
	}
	elapsed := math.Max(0, beat-startBeat)
	curStart := p.points[0].pos
	for i := 0; i+1 < len(p.points); i++ {
		a, b := p.points[i], p.points[i+1]
		from := a.pos
		if a.useLastReal {
			from[0], from[1] = lastReal[0], lastReal[1]
		}
		d := a.duration
		if d <= 0 {
			curStart = b.pos
			continue
		}
		if elapsed <= d || i+2 == len(p.points) {
			u := math.Max(0, math.Min(1, elapsed/d))
			x := from[0] + (b.pos[0]-from[0])*u
			y := from[1] + (b.pos[1]-from[1])*u
			z := from[2] + (b.pos[2]-from[2])*u
			// SuperCurveObject uses a simple arcing offset on each segment; the
			// extracted height field is not an AnimationClip curve, so it has to be
			// re-applied here for the ball to follow the original lob.
			y += math.Sin(u*math.Pi) * a.height
			return [3]float64{x, y, z}
		}
		elapsed -= d
		curStart = b.pos
	}
	return curStart
}
