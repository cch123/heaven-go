package fruitbasket

import (
	"math"
	"math/bits"

	"hsdemo/engine"
	"hsdemo/kmdata"
	"hsdemo/riq"
)

func boolParam(e *riq.Entity, key string) bool {
	if v, ok := e.Data[key]; ok {
		if b, ok := v.(bool); ok {
			return b
		}
		return e.Float(key, 0) != 0
	}
	return false
}

func boolDefault(e *riq.Entity, key string, def bool) bool {
	if _, ok := e.Data[key]; !ok {
		return def
	}
	return boolParam(e, key)
}

func colorParam(e *riq.Entity, key string, def [4]float64) [4]float64 {
	m, ok := e.Data[key].(map[string]any)
	if !ok {
		return def
	}
	get := func(name string, fallback float64) float64 {
		if v, ok := m[name].(float64); ok {
			return v
		}
		return fallback
	}
	return [4]float64{
		get("r", def[0]),
		get("g", def[1]),
		get("b", def[2]),
		get("a", def[3]),
	}
}

func refOr(ctx *engine.Ctx, c kmdata.Component, field, fallback string) string {
	if c.Refs != nil && c.Refs[field] != "" {
		return c.Refs[field]
	}
	if p := ctx.Role(field); p != "" {
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

func lerp(a, b, u float64) float64 { return a + (b-a)*u }

func deg(v float64) float64 { return v * math.Pi / 180 }

func pick[T any](cond bool, a, b T) T {
	if cond {
		return a
	}
	return b
}

func pickFloat(cond bool, a, b float64) float64 {
	if cond {
		return a
	}
	return b
}

func quadEase(u float64, mode int) float64 {
	u = clamp01(u)
	switch mode {
	case 0:
		return u * u
	case 1:
		return 1 - (1-u)*(1-u)
	default:
		if u < 0.5 {
			return 2 * u * u
		}
		return 1 - math.Pow(-2*u+2, 2)/2
	}
}

func easeColor(ease int, a, b [4]float64, u float64) [4]float64 {
	return [4]float64{
		engine.Ease(ease, a[0], b[0], u),
		engine.Ease(ease, a[1], b[1], u),
		engine.Ease(ease, a[2], b[2], u),
		engine.Ease(ease, a[3], b[3], u),
	}
}

func expressionName(v int) string {
	switch v {
	case exprHappy:
		return "Happy"
	case exprCry:
		return "Cry"
	default:
		return "None"
	}
}

func expressionNameAll(v int) string {
	switch v {
	case 0:
		return "Happy"
	case 1:
		return "Cry"
	case 2:
		return "DaydreamNone"
	case 3:
		return "DaydreamKiss"
	case 4:
		return "DaydreamTongue"
	case 5:
		return "DaydreamWorried"
	default:
		return "None"
	}
}

func daydreamExpressionName(v int) string {
	switch v {
	case 1:
		return "Kiss"
	case 2:
		return "Tongue"
	case 3:
		return "Worried"
	default:
		return "None"
	}
}

func eventRand(beat float64, salt int) float64 {
	x := uint64(math.Float64bits(beat)) + uint64(salt)*0x9e3779b97f4a7c15
	x ^= bits.RotateLeft64(x, 17)
	x *= 0xbf58476d1ce4e5b9
	x ^= x >> 31
	x *= 0x94d049bb133111eb
	x ^= x >> 33
	return float64(x&((1<<53)-1)) / float64(1<<53)
}
