package spacesoccer

import (
	"image/color"
	"math"
	"math/rand"

	"hsdemo/kart"
	"hsdemo/riq"
)

const (
	animScale       = 0.5
	playerRootX     = 3.384
	bgSpriteOrder   = -2000
	kickerRootRel   = "Space Kicker"
	holderRel       = "Space Kicker/Holder"
	bodyRel         = "Space Kicker"
	flamesRel       = "Space Kicker/Holder/Platform/Flames"
	playerAction    = 0
	maxAutoDispense = 512
)

const (
	presetFive = iota
	presetDuo
	presetCustom
)

const (
	playerPresetLaunchStart = iota
	playerPresetLaunchEnd
	playerPresetCustom
)

const (
	launchSoundNone = iota
	launchSoundStart
	launchSoundEnd
)

const (
	animEnter = iota
	animExit
)

var (
	defaultBG    = [4]float64{1, 0.49, 0.153, 1}
	defaultDots  = [4]float64{248.0 / 255, 248.0 / 255, 248.0 / 255, 1}
	kickLavender = [4]float64{184.0 / 255, 136.0 / 255, 248.0 / 255, 1}
	kickPurple   = [4]float64{136.0 / 255, 64.0 / 255, 248.0 / 255, 1}
	platTop      = [4]float64{112.0 / 255, 248.0 / 255, 144.0 / 255, 1}
	platSide     = [4]float64{88.0 / 255, 168.0 / 255, 128.0 / 255, 1}
	platOutline  = [4]float64{24.0 / 255, 56.0 / 255, 40.0 / 255, 1}
	fireYellow   = [4]float64{248.0 / 255, 248.0 / 255, 88.0 / 255, 1}
	white        = [4]float64{1, 1, 1, 1}
)

func boolParam(e *riq.Entity, key string) bool {
	return e.Float(key, 0) != 0
}

func boolParamDefault(e *riq.Entity, key string, def bool) bool {
	if _, ok := e.Data[key]; !ok {
		return def
	}
	return boolParam(e, key)
}

func colorParam(e *riq.Entity, key string, def [4]float64) [4]float64 {
	v, ok := e.Data[key]
	if !ok {
		return def
	}
	m, ok := v.(map[string]any)
	if !ok {
		return def
	}
	c := def
	if r, ok := asFloat(m["r"]); ok {
		c[0] = r
	}
	if g, ok := asFloat(m["g"]); ok {
		c[1] = g
	}
	if b, ok := asFloat(m["b"]); ok {
		c[2] = b
	}
	if a, ok := asFloat(m["a"]); ok {
		c[3] = a
	}
	return c
}

func asFloat(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case int:
		return float64(n), true
	default:
		return 0, false
	}
}

func palette(alpha, fill, outline [4]float64) kart.Palette {
	return kart.Palette{Alpha: alpha, Fill: fill, Outline: outline}
}

func rgba(c [4]float64) color.NRGBA {
	return color.NRGBA{
		R: byte(clamp(c[0]) * 255),
		G: byte(clamp(c[1]) * 255),
		B: byte(clamp(c[2]) * 255),
		A: byte(clamp(c[3]) * 255),
	}
}

func clamp(v float64) float64 {
	return math.Max(0, math.Min(1, v))
}

func lerpColor(a, b [4]float64, u float64) [4]float64 {
	return [4]float64{
		a[0] + (b[0]-a[0])*u,
		a[1] + (b[1]-a[1])*u,
		a[2] + (b[2]-a[2])*u,
		a[3] + (b[3]-a[3])*u,
	}
}

func beatEq(a, b float64) bool {
	return math.Abs(a-b) < 1e-6
}

func beatIn(list []float64, beat float64) bool {
	for _, b := range list {
		if beatEq(b, beat) {
			return true
		}
	}
	return false
}

func randFloat() float64 { return rand.Float64() }
