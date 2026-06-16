package engine

import (
	"image"
	"image/color"
	"log"
	"math"
	"os"
	"path/filepath"

	"github.com/hajimehoshi/ebiten/v2"
)

const vhsMaxIterations = 8

type vhsPostFX struct {
	noise, noiseTmp *ebiten.Image
	noiseFull       *ebiten.Image
	blur, blend     [vhsMaxIterations]*ebiten.Image
	upscale         [vhsMaxIterations]*ebiten.Image
	composite       *ebiten.Image
	slightFull      *ebiten.Image
	blurFull        *ebiten.Image

	noiseShader     *ebiten.Shader
	smearShader     *ebiten.Shader
	downShader      *ebiten.Shader
	upShader        *ebiten.Shader
	compositeShader *ebiten.Shader
	grainShader     *ebiten.Shader

	horizontalNoise *ebiten.Image
	speckNoise      *ebiten.Image
	grain           *ebiten.Image
	horizontalPad   *ebiten.Image
	speckPad        *ebiten.Image
	grainFull       *ebiten.Image
	assetsLoaded    bool
	fallbackLogged  bool

	timeReady bool
	lastTime  float64
	noisePos  float64
}

func (v *vhsPostFX) ensure(assetsRoot string) error {
	var err error
	if v.noiseShader == nil {
		if v.noiseShader, err = ebiten.NewShader([]byte(vhsNoiseGenKage)); err != nil {
			return err
		}
		if v.smearShader, err = ebiten.NewShader([]byte(vhsSmearKage)); err != nil {
			return err
		}
		if v.downShader, err = ebiten.NewShader([]byte(vhsDownsampleKage)); err != nil {
			return err
		}
		if v.upShader, err = ebiten.NewShader([]byte(vhsUpsampleKage)); err != nil {
			return err
		}
		if v.compositeShader, err = ebiten.NewShader([]byte(vhsCompositeKage)); err != nil {
			return err
		}
		if v.grainShader, err = ebiten.NewShader([]byte(vhsGrainKage)); err != nil {
			return err
		}
	}
	v.ensureBuffers()
	v.ensureAssets(assetsRoot)
	return nil
}

func (v *vhsPostFX) ensureBuffers() {
	nw := minInt(640, ScreenW/2)
	nh := minInt(480, ScreenH/2)
	if v.noise == nil || v.noise.Bounds().Dx() != nw || v.noise.Bounds().Dy() != nh {
		v.noise = ebiten.NewImage(nw, nh)
		v.noiseTmp = ebiten.NewImage(nw, nh)
		v.horizontalPad = nil
		v.speckPad = nil
	}
	if v.composite == nil {
		v.composite = ebiten.NewImage(ScreenW, ScreenH)
		v.noiseFull = ebiten.NewImage(ScreenW, ScreenH)
		v.slightFull = ebiten.NewImage(ScreenW, ScreenH)
		v.blurFull = ebiten.NewImage(ScreenW, ScreenH)
	}
	w, h := ScreenW/2, ScreenH/2
	for i := 0; i < vhsMaxIterations; i++ {
		w, h = maxInt(w/2, 1), maxInt(h/2, 1)
		if v.blur[i] == nil || v.blur[i].Bounds().Dx() != w || v.blur[i].Bounds().Dy() != h {
			v.blur[i] = ebiten.NewImage(w, h)
			v.blend[i] = ebiten.NewImage(w, h)
			v.upscale[i] = ebiten.NewImage(w, h)
		}
	}
}

func (v *vhsPostFX) ensureAssets(assetsRoot string) {
	if v.assetsLoaded {
		return
	}
	searchDirs := []string{
		filepath.Join(assetsRoot, "common", "vhs"),
		filepath.Join("/Users/xargin/Downloads/HeavenStudio-master", "Assets", "VHSEffect-main", "Assets", "VHSEffect", "Resources"),
	}
	for _, dir := range searchDirs {
		if v.horizontalNoise == nil {
			v.horizontalNoise = loadVHSImage(filepath.Join(dir, "horizontalNoise.png"))
		}
		if v.speckNoise == nil {
			v.speckNoise = loadVHSImage(filepath.Join(dir, "speckNoise.png"))
		}
		if v.grain == nil {
			v.grain = loadVHSImage(filepath.Join(dir, "vhsGrain.png"))
		}
	}
	if v.horizontalNoise == nil || v.speckNoise == nil || v.grain == nil {
		if !v.fallbackLogged {
			log.Printf("engine: VHS noise textures missing; using deterministic fallback textures (copy Heaven Studio VHSEffect Resources to %s for exact assets)", filepath.Join(assetsRoot, "common", "vhs"))
			v.fallbackLogged = true
		}
		if v.horizontalNoise == nil {
			v.horizontalNoise = generatedVHSNoise(256, 16, 11)
		}
		if v.speckNoise == nil {
			v.speckNoise = generatedVHSNoise(512, 512, 23)
		}
		if v.grain == nil {
			v.grain = generatedVHSGrain(256, 256)
		}
	}
	nw, nh := v.noise.Bounds().Dx(), v.noise.Bounds().Dy()
	if v.horizontalPad == nil {
		v.horizontalPad = tileImage(v.horizontalNoise, nw, nh)
	}
	if v.speckPad == nil {
		v.speckPad = tileImage(v.speckNoise, nw, nh)
	}
	if v.grainFull == nil {
		v.grainFull = tileImage(v.grain, ScreenW, ScreenH)
	}
	v.assetsLoaded = true
}

func loadVHSImage(path string) *ebiten.Image {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	if err != nil {
		log.Printf("engine: decode VHS asset %s: %v", path, err)
		return nil
	}
	return ebiten.NewImageFromImage(img)
}

func generatedVHSNoise(w, h int, salt int) *ebiten.Image {
	rgba := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			v := uint8(vhsHash(float64(x+salt), float64(y-salt)) * 255)
			rgba.SetRGBA(x, y, color.RGBA{v, v, v, 255})
		}
	}
	return ebiten.NewImageFromImage(rgba)
}

func generatedVHSGrain(w, h int) *ebiten.Image {
	rgba := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			r := uint8(vhsHash(float64(x), float64(y)) * 255)
			g := uint8(vhsHash(float64(x+31), float64(y-17)) * 255)
			b := uint8(vhsHash(float64(x-13), float64(y+47)) * 255)
			rgba.SetRGBA(x, y, color.RGBA{r, g, b, 255})
		}
	}
	return ebiten.NewImageFromImage(rgba)
}

func tileImage(src *ebiten.Image, w, h int) *ebiten.Image {
	dst := ebiten.NewImage(w, h)
	sw, sh := src.Bounds().Dx(), src.Bounds().Dy()
	for y := 0; y < h; y += sh {
		for x := 0; x < w; x += sw {
			op := &ebiten.DrawImageOptions{}
			op.GeoM.Translate(float64(x), float64(y))
			dst.DrawImage(src, op)
		}
	}
	return dst
}

func (v *vhsPostFX) apply(dst, src *ebiten.Image, assetsRoot string, p fxParams, t float64) bool {
	if err := v.ensure(assetsRoot); err != nil {
		log.Printf("engine: VHS postfx init failed: %v", err)
		return false
	}
	v.updateNoisePosition(t)
	nw, nh := v.noise.Bounds().Dx(), v.noise.Bounds().Dy()
	off1 := vhsHash(math.Floor(t*60)+3, 0)
	off2 := vhsHash(math.Floor(t*60)+17, 0)
	v.noise.Clear()
	op := &ebiten.DrawRectShaderOptions{}
	op.Images[0] = v.horizontalPad
	op.Images[1] = v.speckPad
	op.Uniforms = map[string]any{
		"DstSize":              []float32{float32(nw), float32(nh)},
		"HorizontalNoisePos":   float32(v.noisePos),
		"HorizontalNoisePower": float32(p.vhsStripeDensity * p.vhsStripeDensity),
		"SpeckScaleOffset":     []float32{float32(nw) / float32(v.speckNoise.Bounds().Dx()), float32(nh) / float32(v.speckNoise.Bounds().Dy()), float32(off1), float32(off2)},
	}
	v.noise.DrawRectShader(nw, nh, v.noiseShader, op)

	v.noiseTmp.Clear()
	op = &ebiten.DrawRectShaderOptions{}
	op.Images[0] = v.noise
	op.Uniforms = map[string]any{"DstSize": []float32{float32(nw), float32(nh)}, "Smear": []float32{1, 0.2}}
	v.noiseTmp.DrawRectShader(nw, nh, v.smearShader, op)
	v.noise.Clear()
	op = &ebiten.DrawRectShaderOptions{}
	op.Images[0] = v.noiseTmp
	op.Uniforms = map[string]any{"DstSize": []float32{float32(nw), float32(nh)}, "Smear": []float32{5, 0.8}}
	v.noise.DrawRectShader(nw, nh, v.smearShader, op)
	scaleImage(v.noiseFull, v.noise, ebiten.FilterLinear)

	iters := maxInt(2, minInt(vhsMaxIterations, p.vhsIterations))
	for i := 0; i < iters; i++ {
		target := v.blur[i]
		target.Clear()
		source := src
		if i > 0 {
			source = v.blur[i-1]
		}
		w, h := target.Bounds().Dx(), target.Bounds().Dy()
		sw, sh := source.Bounds().Dx(), source.Bounds().Dy()
		noiseOpacity := 0.0
		noiseSource := source
		if i == 0 {
			noiseOpacity = p.vhsStripeOpacity
			noiseSource = v.noiseFull
		}
		op = &ebiten.DrawRectShaderOptions{}
		op.GeoM.Scale(float64(w)/float64(sw), float64(h)/float64(sh))
		op.Images[0] = source
		op.Images[1] = noiseSource
		op.Uniforms = map[string]any{
			"SrcSize":      []float32{float32(sw), float32(sh)},
			"NoiseOpacity": float32(noiseOpacity),
		}
		target.DrawRectShader(sw, sh, v.downShader, op)
	}
	for i := iters - 1; i > 1; i-- {
		target := v.blur[i-1]
		tmp := v.blend[i-1]
		w, h := target.Bounds().Dx(), target.Bounds().Dy()
		scaleImage(v.upscale[i-1], v.blur[i], ebiten.FilterLinear)
		tmp.Clear()
		op = &ebiten.DrawRectShaderOptions{}
		op.Images[0] = target
		op.Images[1] = v.upscale[i-1]
		op.Uniforms = map[string]any{"DstSize": []float32{float32(w), float32(h)}, "Blend": float32(0.6)}
		tmp.DrawRectShader(w, h, v.upShader, op)
		target.Clear()
		target.DrawImage(tmp, nil)
	}
	scaleImage(v.slightFull, v.blur[0], ebiten.FilterLinear)
	scaleImage(v.blurFull, v.blur[1], ebiten.FilterLinear)

	v.composite.Clear()
	op = &ebiten.DrawRectShaderOptions{}
	op.Images[0] = src
	op.Images[1] = v.noiseFull
	op.Images[2] = v.slightFull
	op.Images[3] = v.blurFull
	op.Uniforms = map[string]any{
		"DstSize":             []float32{ScreenW, ScreenH},
		"NoiseOpacity":        float32(p.vhsStripeOpacity),
		"ColorBleedIntensity": float32(p.vhsBleed),
		"Edge":                []float32{float32(p.vhsEdgeIntensity), float32(p.vhsEdgeDistance)},
	}
	v.composite.DrawRectShader(ScreenW, ScreenH, v.compositeShader, op)

	dst.DrawRectShader(ScreenW, ScreenH, v.grainShader, &ebiten.DrawRectShaderOptions{
		Images: [4]*ebiten.Image{v.composite, v.grainFull},
		Uniforms: map[string]any{
			"DstSize": []float32{ScreenW, ScreenH},
			"Grain":   []float32{float32(p.vhsGrain), float32(p.vhsGrainScale), float32(vhsHash(math.Floor(t*60)+29, 0)), float32(vhsHash(math.Floor(t*60)+43, 0))},
		},
	})
	return true
}

func scaleImage(dst, src *ebiten.Image, filter ebiten.Filter) {
	dst.Clear()
	op := &ebiten.DrawImageOptions{Filter: filter}
	op.GeoM.Scale(
		float64(dst.Bounds().Dx())/float64(src.Bounds().Dx()),
		float64(dst.Bounds().Dy())/float64(src.Bounds().Dy()),
	)
	dst.DrawImage(src, op)
}

func (v *vhsPostFX) updateNoisePosition(t float64) {
	if !v.timeReady {
		v.lastTime = t
		v.timeReady = true
		return
	}
	dt := t - v.lastTime
	if dt < 0 || dt > 0.25 {
		dt = 1.0 / 60.0
	}
	v.lastTime = t
	v.noisePos += dt * 0.004
	frame := math.Floor(t * 60)
	if vhsHash(frame, 91) < 0.01 {
		v.noisePos += vhsHash(frame, 137)
	}
	v.noisePos = math.Mod(v.noisePos, 1)
	if v.noisePos < 0 {
		v.noisePos += 1
	}
}

func vhsHash(x, y float64) float64 {
	v := math.Sin(x*12.9898+y*78.233) * 43758.5453
	return v - math.Floor(v)
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
