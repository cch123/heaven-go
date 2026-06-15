package fanclub

import (
	"math"
	"strconv"
	"strings"

	"hsdemo/engine"
	"hsdemo/riq"
)

func roleOr(ctx *engine.Ctx, key, fallback string) string {
	if p := ctx.Role(key); p != "" {
		return p
	}
	return fallback
}

func boolParam(e *riq.Entity, key string) bool { return e.Float(key, 0) != 0 }

func boolDefault(e *riq.Entity, key string, def bool) bool {
	if _, ok := e.Data[key]; !ok {
		return def
	}
	return boolParam(e, key)
}

func numDefault(m map[string]float64, key string, def float64) float64 {
	if v, ok := m[key]; ok {
		return v
	}
	return def
}

func nodePos(ctx *engine.Ctx, path string) [2]float64 {
	for _, n := range ctx.Assets.Rig.Nodes {
		if n.Path == path {
			return n.Pos
		}
	}
	return [2]float64{}
}

func parabola01(u float64) float64 {
	u = clamp01(u)
	x := u*2 - 1
	return -(x * x) + 1
}

func clamp01(v float64) float64 {
	return math.Max(0, math.Min(1, v))
}

func clamp01Signed(v float64) float64 {
	return math.Max(-1, math.Min(1, v))
}

func abs(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}

func itoa(v int) string { return strconv.Itoa(v) }

func hasPrefix(s, p string) bool { return strings.HasPrefix(s, p) }
