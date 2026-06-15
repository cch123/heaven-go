package lovelab

import (
	"math"

	"hsdemo/kmdata"
)

type pathPoint struct {
	x, y, duration, height float64
	rotPerBeat             float64
}

type flaskPath struct {
	name   string
	points []pathPoint
}

func loadFlaskPaths(items []kmdata.ComponentItem) map[string]flaskPath {
	out := map[string]flaskPath{}
	for _, item := range items {
		name := item.Strs["name"]
		if name == "" {
			continue
		}
		p := flaskPath{name: name}
		for _, pos := range item.Items["positions"] {
			pp := pathPoint{
				x:        pos.Nums["pos.x"],
				y:        pos.Nums["pos.y"],
				duration: pos.Nums["duration"],
				height:   pos.Nums["height"],
			}
			for _, v := range pos.Items["values"] {
				if v.Strs["key"] == "rot" {
					pp.rotPerBeat = v.Nums["value"]
					break
				}
			}
			p.points = append(p.points, pp)
		}
		if len(p.points) > 0 {
			out[name] = p
		}
	}
	return out
}

func (p flaskPath) eval(elapsed float64) (x, y, rot float64, done bool) {
	if len(p.points) == 0 {
		return 0, 0, 0, true
	}
	if len(p.points) == 1 {
		pp := p.points[0]
		return pp.x, pp.y, 0, true
	}
	if elapsed < 0 {
		pp := p.points[0]
		return pp.x, pp.y, 0, false
	}
	remain := elapsed
	for i := 0; i < len(p.points)-1; i++ {
		a, b := p.points[i], p.points[i+1]
		dur := math.Max(a.duration, 1e-6)
		if remain <= dur {
			u := clamp01(remain / dur)
			x = a.x + (b.x-a.x)*u
			y = a.y + (b.y-a.y)*u + math.Sin(math.Pi*u)*a.height
			rot -= remain * a.rotPerBeat * math.Pi / 180
			return x, y, rot, false
		}
		rot -= dur * a.rotPerBeat * math.Pi / 180
		remain -= dur
	}
	last := p.points[len(p.points)-1]
	return last.x, last.y, rot, true
}

func (p flaskPath) duration() float64 {
	if len(p.points) < 2 {
		return 0
	}
	var d float64
	for i := 0; i < len(p.points)-1; i++ {
		d += p.points[i].duration
	}
	return d
}
