package tossboys

import (
	"math"

	"hsdemo/kmdata"
)

type pathValue struct {
	key   string
	value float64
}

type pathPoint struct {
	target      string
	pos         [3]float64
	duration    float64
	height      float64
	useLastReal bool
	values      []pathValue
}

type ballPath struct {
	name   string
	points []pathPoint
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
			point := pathPoint{
				target: pi.Refs["target"],
				pos: [3]float64{
					pi.Nums["pos.x"],
					pi.Nums["pos.y"],
					pi.Nums["pos.z"],
				},
				duration:    pi.Nums["duration"],
				height:      pi.Nums["height"],
				useLastReal: pi.Nums["useLastReal"] != 0,
			}
			for _, vi := range pi.Items["values"] {
				if key := vi.Strs["key"]; key != "" {
					point.values = append(point.values, pathValue{key: key, value: vi.Nums["value"]})
				}
			}
			p.points = append(p.points, point)
		}
		if len(p.points) >= 2 {
			paths[name] = p
		}
	}
	return paths
}

func (p ballPath) durationOverride(d float64) ballPath {
	q := p
	q.points = append([]pathPoint(nil), p.points...)
	if len(q.points) > 0 {
		q.points[0].duration = d
	}
	return q
}

func (p ballPath) posAt(beat, startBeat float64, lastReal [2]float64, targets map[string][3]float64) [3]float64 {
	if len(p.points) == 0 {
		return [3]float64{}
	}
	elapsed := math.Max(0, beat-startBeat)
	for i := 0; i+1 < len(p.points); i++ {
		a, b := p.points[i], p.points[i+1]
		from := pointWorld(a, lastReal, targets)
		to := pointWorld(b, lastReal, targets)
		d := a.duration
		if d <= 0 {
			continue
		}
		if elapsed <= d || i+2 == len(p.points) {
			u := math.Max(0, math.Min(1, elapsed/d))
			return [3]float64{
				from[0] + (to[0]-from[0])*u,
				from[1] + (to[1]-from[1])*u + math.Sin(u*math.Pi)*a.height,
				from[2] + (to[2]-from[2])*u,
			}
		}
		elapsed -= d
	}
	return pointWorld(p.points[len(p.points)-1], lastReal, targets)
}

func (p ballPath) rotAt() float64 {
	if len(p.points) == 0 {
		return 0
	}
	for _, v := range p.points[0].values {
		if v.key == "rot" {
			return v.value
		}
	}
	return 0
}

func pointWorld(p pathPoint, lastReal [2]float64, targets map[string][3]float64) [3]float64 {
	if p.useLastReal {
		return [3]float64{lastReal[0], lastReal[1], p.pos[2]}
	}
	if p.target != "" {
		if t, ok := targets[p.target]; ok {
			return [3]float64{t[0] + p.pos[0], t[1] + p.pos[1], t[2] + p.pos[2]}
		}
	}
	return p.pos
}
