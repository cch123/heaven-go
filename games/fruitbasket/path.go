package fruitbasket

import (
	"math"

	"hsdemo/engine"
	"hsdemo/kart"
	"hsdemo/kmdata"
)

type curvePath struct {
	name   string
	points []pathPoint
}

type pathPoint struct {
	target string
	pos    [2]float64
	dur    float64
	height float64
	values map[string]float64
}

func (p pathPoint) value(key string) float64 {
	if p.values == nil {
		return 0
	}
	return p.values[key]
}

func readCurvePaths(ctx *engine.Ctx, items []kmdata.ComponentItem) map[string]curvePath {
	out := map[string]curvePath{}
	for _, it := range items {
		name := ""
		if it.Strs != nil {
			name = it.Strs["name"]
		}
		if name == "" {
			continue
		}
		cp := curvePath{name: name}
		for _, pit := range it.Items["positions"] {
			pp := pathPoint{
				dur:    numDefault(pit.Nums, "duration", 0),
				height: numDefault(pit.Nums, "height", 0),
				pos: [2]float64{
					numDefault(pit.Nums, "pos.x", 0),
					numDefault(pit.Nums, "pos.y", 0),
				},
				values: map[string]float64{},
			}
			if pit.Refs != nil && pit.Refs["target"] != "" {
				pp.target = pit.Refs["target"]
				pp.pos = nodeWorldPos(ctx, pp.target)
			}
			for _, vit := range pit.Items["values"] {
				if vit.Strs == nil {
					continue
				}
				key := vit.Strs["key"]
				if key == "" {
					continue
				}
				pp.values[key] = numDefault(vit.Nums, "value", 0)
			}
			cp.points = append(cp.points, pp)
		}
		out[name] = cp
	}
	return out
}

func (p curvePath) segmentAt(beat, startBeat float64) (int, float64) {
	if len(p.points) < 2 {
		return -1, 0
	}
	elapsed := beat - startBeat
	if elapsed < 0 {
		return 0, 0
	}
	acc := 0.0
	for i := 0; i < len(p.points)-1; i++ {
		dur := p.points[i].dur
		if dur <= 0 {
			if elapsed <= acc {
				return i, 1
			}
			continue
		}
		if elapsed <= acc+dur || i == len(p.points)-2 {
			return i, clamp01((elapsed - acc) / dur)
		}
		acc += dur
	}
	return len(p.points) - 2, 1
}

func (p curvePath) at(beat, startBeat float64) ([2]float64, int) {
	idx, u := p.segmentAt(beat, startBeat)
	if idx < 0 {
		return [2]float64{}, -1
	}
	a := p.points[idx]
	b := p.points[idx+1]
	yArc := (1 - math.Pow(2*u-1, 2)) * a.height
	return [2]float64{
		lerp(a.pos[0], b.pos[0], u),
		lerp(a.pos[1], b.pos[1], u) + yArc,
	}, idx
}

func nodeWorldPos(ctx *engine.Ctx, path string) [2]float64 {
	idx, ok := ctx.Assets.NodeIndex(path)
	if !ok {
		return [2]float64{}
	}
	chain := []int{}
	for i := idx; i >= 0; i = ctx.Assets.Rig.Nodes[i].Parent {
		chain = append(chain, i)
	}
	w := kart.Identity()
	for i := len(chain) - 1; i >= 0; i-- {
		n := ctx.Assets.Rig.Nodes[chain[i]]
		w = w.Mul(kart.TRS(n.Pos[0], n.Pos[1], n.RotZ, n.Scale[0], n.Scale[1]))
	}
	return [2]float64{w.Tx, w.Ty}
}

func numDefault(nums map[string]float64, key string, def float64) float64 {
	if nums == nil {
		return def
	}
	if v, ok := nums[key]; ok {
		return v
	}
	return def
}
