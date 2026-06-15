package supersamuraislice

import (
	"hsdemo/kart"
	"hsdemo/kmdata"
)

type curvePath struct {
	name   string
	points []pathPoint
}

type pathPoint struct {
	pos         [2]float64
	height      float64
	duration    float64
	useLastReal bool
}

func loadCurvePaths(as *kart.Assets) map[string]curvePath {
	out := map[string]curvePath{}
	game, ok := as.Extra.Components["game"]
	if !ok {
		return out
	}
	for _, it := range game.Lists["demonPaths"] {
		name := it.Strs["name"]
		if name == "" {
			continue
		}
		cp := curvePath{name: name}
		for _, pos := range it.Items["positions"] {
			cp.points = append(cp.points, pathPointFromItem(as, pos))
		}
		out[name] = cp
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

func (p curvePath) eval(elapsed float64) [2]float64 {
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
		if elapsed <= segStart+cur.duration {
			u := clamp01((elapsed - segStart) / cur.duration)
			start := cur.pos
			if cur.useLastReal {
				start = lastReal
			}
			return [2]float64{
				start[0] + (next.pos[0]-start[0])*u,
				start[1] + (next.pos[1]-start[1])*u + parabola(u)*cur.height,
			}
		}
		if !cur.useLastReal {
			lastReal = cur.pos
		}
		segStart += cur.duration
	}
	return p.points[len(p.points)-1].pos
}
