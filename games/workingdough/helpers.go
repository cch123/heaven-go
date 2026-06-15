package workingdough

import (
	"math"
	"strings"

	"hsdemo/engine"
	"hsdemo/kart"
	"hsdemo/riq"
)

const (
	actionBig = 1
	animBeat  = 0.5
)

var (
	white       = [4]float64{1, 1, 1, 1}
	transparent = [4]float64{1, 1, 1, 0}
	black       = [4]float64{0, 0, 0, 1}
)

func boolParam(e *riq.Entity, key string) bool { return e.Float(key, 0) != 0 }

func colorParam(e *riq.Entity, key string, def [4]float64) [4]float64 {
	if mm, ok := e.Data[key].(map[string]any); ok {
		return [4]float64{
			num(mm["r"], def[0]), num(mm["g"], def[1]),
			num(mm["b"], def[2]), num(mm["a"], def[3]),
		}
	}
	return def
}

func num(v any, def float64) float64 {
	switch x := v.(type) {
	case float64:
		return x
	case int:
		return float64(x)
	case bool:
		if x {
			return 1
		}
		return 0
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

func lerp(a, b, u float64) float64 { return a + (b-a)*u }

func lerpColor(a, b [4]float64, ease int, u float64) [4]float64 {
	u = clamp01(u)
	return [4]float64{
		engine.Ease(ease, a[0], b[0], u),
		engine.Ease(ease, a[1], b[1], u),
		engine.Ease(ease, a[2], b[2], u),
		engine.Ease(ease, a[3], b[3], u),
	}
}

func nodePos(as *kart.Assets, path string) [2]float64 {
	for i := range as.Rig.Nodes {
		if as.Rig.Nodes[i].Path == path {
			return as.Rig.Nodes[i].Pos
		}
	}
	return [2]float64{}
}

func role(ctx *engine.Ctx, key string) string {
	if ctx == nil || ctx.Assets == nil {
		return ""
	}
	return ctx.Assets.Roles[key]
}

func stateScale(as *kart.Assets, root, state string, length float64) float64 {
	if length <= 0 {
		return animBeat
	}
	ctrlName := as.Animators[root]
	ctrl, ok := as.Controllers[ctrlName]
	if !ok {
		return animBeat
	}
	st, ok := ctrl.States[state]
	if !ok || st.Clip == "" {
		return animBeat
	}
	anim := as.Anims[st.Clip]
	if anim == nil || anim.Duration <= 0 {
		return animBeat
	}
	speed := st.Speed
	if math.Abs(speed) < 1e-9 {
		speed = 1
	}
	return anim.Duration / (length * speed)
}

func isWorkingDoughSwitch(e riq.Entity) bool {
	return strings.HasPrefix(e.Datamodel, "gameManager/switchGame/workingDough")
}
