package supersamuraislice

import (
	"math"
	"math/rand"
	"strconv"

	"hsdemo/kart"
	"hsdemo/kmdata"
	"hsdemo/riq"
)

func boolParam(e *riq.Entity, key string) bool { return e.Float(key, 0) != 0 }

func boolDefault(e *riq.Entity, key string, def bool) bool {
	if _, ok := e.Data[key]; !ok {
		return def
	}
	return boolParam(e, key)
}

func intDefault(e *riq.Entity, key string, def int) int {
	if _, ok := e.Data[key]; !ok {
		return def
	}
	return int(e.Float(key, float64(def)))
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

func parabola(u float64) float64 {
	u = clamp01(u)
	return 4 * u * (1 - u)
}

func itoa(v int) string { return strconv.Itoa(v) }

func nodePos(as *kart.Assets, path string) [2]float64 {
	for i := range as.Rig.Nodes {
		n := &as.Rig.Nodes[i]
		if n.Path == path {
			return n.Pos
		}
	}
	return [2]float64{}
}

func roleOr(as *kart.Assets, role, fallback string) string {
	if p := as.Roles[role]; p != "" {
		return p
	}
	if game, ok := as.Extra.Components["game"]; ok {
		if p := game.Refs[role]; p != "" {
			return p
		}
	}
	return fallback
}

func refOr(c kmdata.Component, key, fallback string) string {
	if p := c.Refs[key]; p != "" {
		return p
	}
	return fallback
}

func randomPan(r *rand.Rand) float64 { return r.Float64()*2 - 1 }

func modOffset(v, width float64) float64 {
	if width <= 0 {
		return v
	}
	v = math.Mod(v, width)
	if v < -width {
		v += width
	}
	return v
}
