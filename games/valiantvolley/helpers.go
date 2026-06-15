package valiantvolley

import (
	"math"

	"hsdemo/engine"
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

func refOr(ctx *engine.Ctx, c kmdata.Component, key, fallback string) string {
	if p := c.Refs[key]; p != "" {
		return p
	}
	if p := ctx.Role(key); p != "" {
		return p
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

func nearly(a, b float64) bool { return math.Abs(a-b) < 1e-6 }
