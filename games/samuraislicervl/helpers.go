package samuraislicervl

import (
	"math"
	"strconv"
	"strings"

	"hsdemo/engine"
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

func numOr(c kmdata.Component, key string, def float64) float64 {
	if v, ok := c.Nums[key]; ok {
		return v
	}
	return def
}

func refOr(ctx *engine.Ctx, c kmdata.Component, key, fallback string) string {
	if v := c.Refs[key]; v != "" {
		return v
	}
	if v := ctx.Role(key); v != "" {
		return v
	}
	return fallback
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

func sign(v bool) float64 {
	if v {
		return 1
	}
	return -1
}

func smoothStep(v float64) float64 {
	v = clamp01(v)
	return v * v * (3 - 2*v)
}

func deg(v float64) float64 { return v * math.Pi / 180 }

func itoa(v int) string { return strconv.Itoa(v) }

func rel(root, path string) string {
	if path == root {
		return ""
	}
	return strings.TrimPrefix(strings.TrimPrefix(path, root), "/")
}

func componentByPath(as *kart.Assets, path string) kmdata.Component {
	for _, c := range as.Extra.Components {
		if c.Path == path {
			return c
		}
	}
	return kmdata.Component{}
}

func vec2List(items []kmdata.ComponentItem) [][2]float64 {
	out := make([][2]float64, 0, len(items))
	for _, it := range items {
		out = append(out, [2]float64{it.Nums["x"], it.Nums["y"]})
	}
	return out
}
