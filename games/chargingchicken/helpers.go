package chargingchicken

import (
	"math"

	"hsdemo/kart"
	"hsdemo/riq"
)

const (
	actionPress = 0
)

var (
	defaultBGTop     = hex(0x6e, 0xd6, 0xff)
	defaultBGBottom  = hex(0xff, 0xff, 0xff)
	defaultCar       = hex(0xf4, 0xdb, 0x2e)
	defaultCarCharge = hex(0xf4, 0x2e, 0x25)
	defaultCloud     = hex(0xff, 0xff, 0xff)
	defaultCloud2    = hex(0xc8, 0xf0, 0xf0)
	defaultLight     = [4]float64{1, 1, 1, 1}
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

func strDefault(e *riq.Entity, key, def string) string {
	if _, ok := e.Data[key]; !ok {
		return def
	}
	return e.Str(key, def)
}

func colorParam(e *riq.Entity, key string, def [4]float64) [4]float64 {
	v, ok := e.Data[key]
	if !ok {
		return def
	}
	switch c := v.(type) {
	case []any:
		if len(c) >= 4 {
			return [4]float64{num(c[0], def[0]), num(c[1], def[1]), num(c[2], def[2]), num(c[3], def[3])}
		}
	case []float64:
		if len(c) >= 4 {
			return [4]float64{c[0], c[1], c[2], c[3]}
		}
	case map[string]any:
		return [4]float64{num(c["r"], def[0]), num(c["g"], def[1]), num(c["b"], def[2]), num(c["a"], def[3])}
	}
	return def
}

func num(v any, def float64) float64 {
	switch n := v.(type) {
	case float64:
		return n
	case int:
		return float64(n)
	default:
		return def
	}
}

func hex(r, g, b byte) [4]float64 {
	return [4]float64{float64(r) / 255, float64(g) / 255, float64(b) / 255, 1}
}

func lerp(a, b, u float64) float64 { return a + (b-a)*u }

func lerpColor(a, b [4]float64, u float64) [4]float64 {
	u = clamp01(u)
	return [4]float64{lerp(a[0], b[0], u), lerp(a[1], b[1], u), lerp(a[2], b[2], u), lerp(a[3], b[3], u)}
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

func colorEaseAt(start, length float64, from, to [4]float64, beat float64) [4]float64 {
	if length <= 0 {
		if beat >= start {
			return to
		}
		return from
	}
	return lerpColor(from, to, clamp01((beat-start)/length))
}

func mod(v, width float64) float64 {
	if width <= 0 {
		return v
	}
	v = math.Mod(v, width)
	if v < -width {
		v += width
	}
	return v
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

func relIsland(path string) string {
	const root = "Island/"
	if len(path) >= len(root) && path[:len(root)] == root {
		return path[len(root):]
	}
	if path == "Island" {
		return ""
	}
	return path
}

func destinationText(dest int, custom string) string {
	switch dest {
	case 0:
		return custom
	case 1:
		return "You arrived in Seattle!"
	case 2:
		return "You arrived in Mexico!"
	case 3:
		return "You arrived in Brazil!"
	case 4:
		return "You arrived in France!"
	case 5:
		return "You arrived in England!"
	case 6:
		return "You arrived in Italy!"
	case 7:
		return "You arrived in Egypt!"
	case 8:
		return "You arrived in Turkey!"
	case 9:
		return "You arrived in Dubai!"
	case 10:
		return "You arrived in India!"
	case 11:
		return "You arrived in Thailand!"
	case 12:
		return "You arrived in China!"
	case 13:
		return "You arrived in Japan!"
	case 14:
		return "You arrived in Australia!"
	case 15:
		return "You arrived at The Moon!"
	case 16:
		return "You arrived at Mars!"
	case 17:
		return "You arrived at Jupiter!"
	case 18:
		return "You arrived at Uranus!"
	case 19:
		return "You arrived at The Edge of the Galaxy!"
	case 20:
		return "You arrived at The Future!"
	default:
		return custom
	}
}
