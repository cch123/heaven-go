package animalacrobat

import (
	"image/color"
	"math"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"hsdemo/engine"
	"hsdemo/kart"
	"hsdemo/kmdata"
	"hsdemo/riq"
)

func num(vals map[string]float64, key string, def float64) float64 {
	if vals == nil {
		return def
	}
	if v, ok := vals[key]; ok {
		return v
	}
	return def
}

func ref(vals map[string]string, key string) string {
	if vals == nil {
		return ""
	}
	return vals[key]
}

func obstacleComponent(comps map[string]kmdata.Component, root string) kmdata.Component {
	for key, c := range comps {
		if strings.HasPrefix(key, "obstacle") && c.Path == root {
			return c
		}
	}
	return kmdata.Component{}
}

func inputComponent(comps map[string]kmdata.Component, root string, giraffe bool) kmdata.Component {
	prefix := "obstacleInput"
	if giraffe {
		prefix = "giraffeInput"
	}
	for key, c := range comps {
		if strings.HasPrefix(key, prefix) && c.Path == root {
			return c
		}
	}
	return kmdata.Component{}
}

func relPath(root, path string) string {
	if path == "" || path == root {
		return ""
	}
	return strings.TrimPrefix(path, root+"/")
}

func nodePos(as *kart.Assets, path string) (float64, float64) {
	if i, ok := as.NodeIndex(path); ok {
		p := as.Rig.Nodes[i].Pos
		return p[0], p[1]
	}
	return 0, 0
}

func rotationHeight(as *kart.Assets, spec obstacleSpec) float64 {
	_, y := nodePos(as, spec.gripPoint)
	return math.Abs(y)
}

func rotationDistance(as *kart.Assets, spec obstacleSpec) float64 {
	rad := (spec.fullRotRange + 180) * math.Pi / 180
	return math.Abs(math.Cos(rad) * rotationHeight(as, spec) * 2)
}

func obstacleAngle(spec obstacleSpec, beat float64, startBeat float64) float64 {
	length := spec.holdLength + spec.holdPadding + spec.holdPaddingStart
	if length <= 0 {
		return 0
	}
	normalNoMod := math.Abs((beat - (startBeat - spec.holdPaddingStart)) / length)
	u := math.Mod(normalNoMod, 1)
	if int(math.Floor(normalNoMod))%2 == 1 {
		u = 1 - u
	}
	half := spec.fullRotRange / 2
	return engine.Ease(spec.ease, -half, half, u) * math.Pi / 180
}

func colorParam(e *riq.Entity, key string, def [4]float64) [4]float64 {
	v, ok := e.Data[key]
	if !ok {
		return def
	}
	switch c := v.(type) {
	case []any:
		out := def
		for i := 0; i < len(c) && i < 4; i++ {
			if f, ok := c[i].(float64); ok {
				out[i] = f
			}
		}
		return out
	case map[string]any:
		out := def
		for i, k := range []string{"r", "g", "b", "a"} {
			if f, ok := c[k].(float64); ok {
				out[i] = f
			}
		}
		return out
	}
	return def
}

func boolParam(e *riq.Entity, key string, def bool) bool {
	if _, ok := e.Data[key]; !ok {
		return def
	}
	return e.Float(key, 0) != 0
}

func hexColor(r, g, b byte) [4]float64 {
	return [4]float64{float64(r) / 255, float64(g) / 255, float64(b) / 255, 1}
}

func (e bgEase) colorsAt(beat float64) ([4]float64, [4]float64) {
	if e.length <= 0 {
		return e.toA, e.toB
	}
	u := (beat - e.beat) / e.length
	a, b := e.fromA, e.fromB
	for i := 0; i < 4; i++ {
		a[i] = engine.Ease(e.ease, e.fromA[i], e.toA[i], u)
		b[i] = engine.Ease(e.ease, e.fromB[i], e.toB[i], u)
	}
	return a, b
}

func toRGBA(c [4]float64) color.NRGBA {
	return color.NRGBA{
		R: byte(clamp01(c[0]) * 255),
		G: byte(clamp01(c[1]) * 255),
		B: byte(clamp01(c[2]) * 255),
		A: byte(clamp01(c[3]) * 255),
	}
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

func drawSparkle(dst *ebiten.Image, x, y, size float32, c color.NRGBA) {
	vector.DrawFilledCircle(dst, x, y, size*0.45, c, true)
	vector.StrokeLine(dst, x-size, y, x+size, y, size*0.12, c, true)
	vector.StrokeLine(dst, x, y-size, x, y+size, size*0.12, c, true)
	vector.StrokeLine(dst, x-size*0.65, y-size*0.65, x+size*0.65, y+size*0.65, size*0.1, c, true)
	vector.StrokeLine(dst, x-size*0.65, y+size*0.65, x+size*0.65, y-size*0.65, size*0.1, c, true)
}

func screenPoint(proj kart.Aff, x, y float64) (float32, float32) {
	px, py := proj.Apply(x, y)
	return float32(px), float32(py)
}
