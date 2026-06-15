package tossboys

import (
	"math"

	"hsdemo/engine"
	"hsdemo/kmdata"
	"hsdemo/riq"
)

const (
	kidAka = iota
	kidAo
	kidKii
	kidNone = -1

	actionAka = 0
	actionAo  = 3
	actionKii = 4
)

var defaultBG = [4]float64{0.38, 0.99, 0.73, 1}

func beatKey(beat float64) int64 { return int64(math.Round(beat * 10000)) }

func boolParamDefault(e *riq.Entity, key string, def bool) bool {
	if e.Data == nil {
		return def
	}
	if _, ok := e.Data[key]; !ok {
		return def
	}
	return e.Float(key, 0) != 0
}

func colorParam(e *riq.Entity, key string, def [4]float64) [4]float64 {
	if e.Data == nil {
		return def
	}
	raw, ok := e.Data[key]
	if !ok {
		return def
	}
	if arr, ok := raw.([]any); ok {
		out := def
		for i := 0; i < len(arr) && i < 4; i++ {
			if f, ok := arr[i].(float64); ok {
				out[i] = f
			}
		}
		return out
	}
	if m, ok := raw.(map[string]any); ok {
		out := def
		for i, k := range []string{"r", "g", "b", "a"} {
			if f, ok := m[k].(float64); ok {
				out[i] = f
			}
		}
		return out
	}
	return def
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

func kidColor(kid int, capital bool) string {
	switch kid {
	case kidAka:
		if capital {
			return "Red"
		}
		return "red"
	case kidAo:
		if capital {
			return "Blue"
		}
		return "blue"
	case kidKii:
		if capital {
			return "Yellow"
		}
		return "yellow"
	default:
		return ""
	}
}

func actionForKid(kid int) int {
	switch kid {
	case kidAo:
		return actionAo
	case kidKii:
		return actionKii
	default:
		return actionAka
	}
}

func isSpecialEvent(datamodel string) bool {
	switch datamodel {
	case "tossBoys/dual", "tossBoys/lightning", "tossBoys/blur":
		return true
	default:
		return false
	}
}

func roleOr(ctx *engine.Ctx, key, fallback string) string {
	if p := ctx.Role(key); p != "" {
		return p
	}
	if c := ctx.Assets.Extra.Components["game"]; c.Refs != nil {
		if p := c.Refs[key]; p != "" {
			return p
		}
	}
	return fallback
}

func componentByPath(components map[string]kmdata.Component, path string) (kmdata.Component, bool) {
	for _, c := range components {
		if c.Path == path {
			return c, true
		}
	}
	return kmdata.Component{}, false
}

func sceneTargets(as *engine.Ctx) map[string][3]float64 {
	targets := map[string][3]float64{}
	world := make([][3]float64, len(as.Assets.Rig.Nodes))
	for i, n := range as.Assets.Rig.Nodes {
		p := [3]float64{n.Pos[0], n.Pos[1], n.PosZ}
		if n.Parent >= 0 {
			parent := world[n.Parent]
			p[0] += parent[0]
			p[1] += parent[1]
			p[2] += parent[2]
		}
		world[i] = p
		targets[n.Path] = p
	}
	return targets
}

func deg(v float64) float64 { return v * math.Pi / 180 }
