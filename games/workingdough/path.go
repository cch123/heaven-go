package workingdough

import (
	"hsdemo/kart"
	"hsdemo/kmdata"
)

type bouncePath struct {
	name   string
	points []pathPoint
}

type pathPoint struct {
	pos         [2]float64
	height      float64
	duration    float64
	useLastReal bool
}

func loadBouncePaths(as *kart.Assets) map[string]bouncePath {
	out := map[string]bouncePath{}
	game, ok := as.Extra.Components["game"]
	if !ok {
		return out
	}
	for _, it := range game.Lists["ballBouncePaths"] {
		name := it.Strs["name"]
		if name == "" {
			continue
		}
		p := bouncePath{name: name}
		for _, step := range it.Items["positions"] {
			p.points = append(p.points, pathPointFromItem(as, step))
		}
		out[name] = p
	}
	return out
}

func pathPointFromItem(as *kart.Assets, it kmdata.ComponentItem) pathPoint {
	p := pathPoint{
		pos:         [2]float64{it.Nums["pos.x"], it.Nums["pos.y"]},
		height:      it.Nums["height"],
		duration:    it.Nums["duration"],
		useLastReal: it.Nums["useLastRealPos"] != 0,
	}
	if target := it.Refs["target"]; target != "" {
		p.pos = nodePos(as, target)
	}
	return p
}

func (p bouncePath) eval(elapsed float64) [2]float64 {
	if len(p.points) == 0 {
		return [2]float64{}
	}
	if elapsed <= 0 {
		return p.points[0].pos
	}
	lastReal := p.points[0].pos
	segStart := 0.0
	for i := 0; i < len(p.points)-1; i++ {
		cur, next := p.points[i], p.points[i+1]
		if cur.duration <= 0 {
			if !cur.useLastReal {
				lastReal = cur.pos
			}
			continue
		}
		// Unity's SuperCurveObject switches to the next segment on exact
		// boundaries. That matters for hit-frame values and for paths that use
		// useLastRealPos, so keep the strict comparison instead of clamping the
		// previous segment through its endpoint.
		if elapsed < segStart+cur.duration {
			u := clamp01((elapsed - segStart) / cur.duration)
			start := cur.pos
			if cur.useLastReal {
				start = lastReal
			}
			return [2]float64{
				lerp(start[0], next.pos[0], u),
				lerp(start[1], next.pos[1], u) + parabola(u)*cur.height,
			}
		}
		if !cur.useLastReal {
			lastReal = cur.pos
		}
		segStart += cur.duration
	}
	return p.points[len(p.points)-1].pos
}

func parabola(u float64) float64 {
	u = clamp01(u)
	return 4 * u * (1 - u)
}
