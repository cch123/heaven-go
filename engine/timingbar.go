// timingbar.go：时机条 overlay（对应 HS TimingAccuracyDisplay 的
// "旋转 90°、半透明、置于底部中央"布局）：横向，中心 = 完美，
// 左 = 早（fast）、右 = 晚（slow）；三段色区比例 = 各判定窗口。
package engine

import (
	"image"
	"image/color"
	"math"
	"os"
	"path/filepath"
	"sync"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

const (
	timingBarHalfW          = float32(105) // screen-space equivalent of the prefab bar's +/-WinNG range.
	timingUnityBarHalfUnits = 1.4          // TimingAccuracy.prefab barTransform.localScale.y is the full 2.8-unit bar.
	timingBarAceNorm        = 0.111        // barJustTransform.localScale.y.
	timingBarOKNorm         = 0.5714286    // barOKTransform.localScale.y.
)

func (a *App) drawTimingBar(screen *ebiten.Image, t float64) {
	const (
		cx    = float32(ScreenW / 2)
		cy    = float32(508)
		halfW = timingBarHalfW
		bh    = float32(10)
	)
	justW := halfW * timingBarOKNorm
	aceW := halfW * timingBarAceNorm

	vector.DrawFilledRect(screen, cx-halfW-14, cy-bh/2-5, (halfW+14)*2, bh+10, color.RGBA{30, 30, 34, 110}, false)
	vector.DrawFilledRect(screen, cx-halfW, cy-bh/2, halfW*2, bh, color.RGBA{130, 60, 60, 150}, false)
	vector.DrawFilledRect(screen, cx-justW, cy-bh/2, justW*2, bh, color.RGBA{150, 130, 60, 170}, false)
	vector.DrawFilledRect(screen, cx-aceW, cy-bh/2, aceW*2+1, bh, color.RGBA{90, 200, 120, 220}, false)
	vector.StrokeLine(screen, cx, cy-bh/2-2, cx, cy+bh/2+2, 1, color.RGBA{255, 255, 255, 200}, false)

	dim := color.RGBA{230, 230, 235, 150}
	a.text(screen, "fast", a.faceSmall, float64(cx-halfW-14)-32, float64(cy)-9, dim, false)
	a.text(screen, "slow", a.faceSmall, float64(cx+halfW+14)+6, float64(cy)-9, dim, false)

	for i, h := range a.tdHits {
		age := t - h.t
		if age > 1.2 {
			continue
		}
		x := cx + float32(h.y)*halfW
		a.drawTimingHitStars(screen, x, cy, h, i, age)

		al := 1 - age/1.2
		var c color.RGBA
		switch h.rating {
		case JudgeAce:
			c = color.RGBA{140, 255, 170, uint8(255 * al)}
		case JudgeJust:
			c = color.RGBA{255, 230, 130, uint8(255 * al)}
		default:
			c = color.RGBA{255, 120, 120, uint8(255 * al)}
		}
		half := float32(2 + 4*al)
		vector.DrawFilledRect(screen, x-1.5, cy-bh/2-half, 3, bh+half*2, c, true)
		if h.rating == JudgeNG && age < 0.7 {
			label := "LATE"
			if h.y < 0 {
				label = "EARLY"
			}
			a.text(screen, label, a.faceSmall, float64(x)-18, float64(cy)-36, c, false)
		}
	}

	ax := cx + float32(a.tdArrow)*halfW
	drawTri(screen, ax, cy-bh/2-5, 5, true)
	drawTri(screen, ax, cy+bh/2+5, 5, false)
}

func (a *App) drawTimingHitStars(dst *ebiten.Image, x, y float32, h timingHit, idx int, age float64) {
	assets := timingAccuracyImages(a.assetsRoot)
	unitPx := float64(timingBarHalfW) / timingUnityBarHalfUnits
	scale := timingRatingBaseScale(h.rating)
	if h.rating == JudgeJust {
		scale *= timingOKScale(h)
	}
	for si, ps := range timingParticleSystems(h.rating) {
		drawTimingParticleSystem(dst, assets, ps, x, y, h, idx, si, age, unitPx, scale)
	}
}

func timingStarSeed(h timingHit, idx int) uint32 {
	bits := math.Float64bits(h.t + h.y*17)
	return uint32(bits) ^ uint32(bits>>32) ^ uint32(idx*0x9e3779b9) ^ uint32(h.rating)*0x85ebca6b
}

func timingRand(seed uint32, salt int) float64 {
	x := seed + uint32(salt)*0x9e3779b9
	x ^= x >> 16
	x *= 0x7feb352d
	x ^= x >> 15
	x *= 0x846ca68b
	x ^= x >> 16
	return float64(x&0xffffff) / float64(0x1000000)
}

type timingParticleMaterial int

const (
	timingMaterialStar timingParticleMaterial = iota
	timingMaterialAce
)

type timingParticleTexture int

const (
	timingTextureMain timingParticleTexture = iota
	timingTextureCircle
	timingTextureStar
)

type timingCurveKey struct {
	t, v float64
}

type timingParticleSystem struct {
	name            string
	seedSalt        int
	lifetime        float64
	burstTime       float64
	startSpeed      float64
	startSize       float64
	shapeRadius     float64
	count           int
	randomRotation  bool
	angularVelocity float64
	minColor        color.RGBA
	maxColor        color.RGBA
	randomColor     bool
	material        timingParticleMaterial
	texture         timingParticleTexture
	sizeCurve       []timingCurveKey
}

var (
	// Serialized from TimingAccuracy.prefab's Just/OK/NG ParticleSystems.
	// Keeping the prefab names here makes future audits against Unity YAML direct.
	timingParticleJust = []timingParticleSystem{
		{
			name: "Just00", seedSalt: 11,
			lifetime: 0.45, startSpeed: 5, startSize: 0.7, shapeRadius: 0.1, count: 10, randomRotation: true,
			angularVelocity: 0.43633232,
			minColor:        color.RGBA{255, 0, 255, 255}, maxColor: color.RGBA{255, 255, 255, 255}, randomColor: true,
			material: timingMaterialAce, texture: timingTextureMain,
			sizeCurve: []timingCurveKey{{0, 1}, {0.40679932, 1}, {0.9, 0.5}},
		},
		{
			name: "JustSub", seedSalt: 12,
			lifetime: 0.3, burstTime: 0.45, startSize: 0.6, count: 1,
			minColor: color.RGBA{255, 255, 255, 255}, maxColor: color.RGBA{255, 255, 255, 255},
			material: timingMaterialAce, texture: timingTextureMain,
			sizeCurve: []timingCurveKey{{0, 1}, {1, 0}},
		},
	}
	timingParticleOK = []timingParticleSystem{
		{
			name: "Just01", seedSalt: 21,
			lifetime: 0.4, startSpeed: 4, startSize: 0.7, shapeRadius: 0.25, count: 10, randomRotation: true,
			angularVelocity: 8.807386,
			minColor:        color.RGBA{255, 0, 255, 255}, maxColor: color.RGBA{0, 255, 255, 255}, randomColor: true,
			material: timingMaterialStar, texture: timingTextureMain,
			sizeCurve: []timingCurveKey{{0, 1}, {0.5, 1}, {0.9, 0.5}},
		},
		{
			name: "JustSub", seedSalt: 22,
			lifetime: 0.4, burstTime: 0.025, startSize: 0.45, count: 1,
			minColor: color.RGBA{255, 255, 255, 255}, maxColor: color.RGBA{255, 255, 255, 255},
			material: timingMaterialStar, texture: timingTextureMain,
			sizeCurve: []timingCurveKey{{0, 1}, {1, 0}},
		},
	}
	timingParticleNG = []timingParticleSystem{
		{
			name: "Miss00", seedSalt: 31,
			lifetime: 0.2, startSize: 3, count: 1,
			minColor: color.RGBA{255, 255, 255, 255}, maxColor: color.RGBA{255, 0, 0, 255}, randomColor: true,
			material: timingMaterialStar, texture: timingTextureCircle,
			sizeCurve: []timingCurveKey{{0, 0}, {1, 1}},
		},
		{
			name: "MissStar", seedSalt: 32,
			lifetime: 0.2, startSize: 2, count: 1,
			minColor: color.RGBA{255, 255, 255, 255}, maxColor: color.RGBA{255, 0, 0, 255}, randomColor: true,
			material: timingMaterialStar, texture: timingTextureStar,
			sizeCurve: []timingCurveKey{{0, 1}, {1, 0}},
		},
	}
)

func timingParticleSystems(j Judgment) []timingParticleSystem {
	switch j {
	case JudgeAce:
		return timingParticleJust
	case JudgeJust:
		return timingParticleOK
	default:
		return timingParticleNG
	}
}

func timingBarNorm(signed float64) float64 {
	d := math.Abs(signed)
	sign := 1.0
	if signed < 0 {
		sign = -1
	}
	switch {
	case d <= WinAce:
		return sign * (d / WinAce) * timingBarAceNorm
	case d <= WinJust:
		frac := (d - WinAce) / (WinJust - WinAce)
		return sign * (timingBarAceNorm + (timingBarOKNorm-timingBarAceNorm)*frac)
	default:
		frac := (d - WinJust) / (WinNG - WinJust)
		if frac > 1 {
			frac = 1
		}
		return sign * (timingBarOKNorm + (1-timingBarOKNorm)*frac)
	}
}

func timingRatingBaseScale(j Judgment) float64 {
	if j == JudgeNG {
		return 0.6320368
	}
	return 1
}

func timingOKScale(h timingHit) float64 {
	signed := h.signed
	if math.Abs(signed) <= WinAce {
		return 1
	}
	// TimingAccuracyDisplay.MakeAccuracyVfx scales the OK object down as the hit
	// moves through the OK band; keep the original early/late frac asymmetry.
	var frac float64
	if signed > 0 {
		frac = (signed - WinAce) / (WinJust - WinAce)
	} else {
		frac = (signed + WinJust) / (WinJust - WinAce)
	}
	if frac < 0 {
		frac = 0
	}
	if frac > 1 {
		frac = 1
	}
	return 1 - frac/2
}

func drawTimingParticleSystem(dst *ebiten.Image, assets *timingAccuracyAssets, ps timingParticleSystem, x, y float32, h timingHit, hitIdx, systemIdx int, age, unitPx, parentScale float64) {
	systemAge := age - ps.burstTime
	if systemAge < 0 || systemAge > ps.lifetime {
		return
	}
	p := systemAge / ps.lifetime
	sizeScale := evalTimingCurve(ps.sizeCurve, p)
	if sizeScale <= 0 {
		return
	}
	seed := timingStarSeed(h, hitIdx) ^ uint32(ps.seedSalt*0x45d9f3b)
	globalTime := h.t + age

	for i := 0; i < ps.count; i++ {
		particleSeed := seed ^ uint32(i*0x9e3779b9) ^ uint32(systemIdx*0x85ebca6b)
		angle := timingRand(particleSeed, 1) * math.Pi * 2
		radius := ps.shapeRadius * unitPx
		// TimingAccuracy uses ShapeModule type 10 with radiusThickness=0, so
		// particles start on the ring edge instead of filling the disk.
		dist := radius + ps.startSpeed*systemAge*unitPx
		px := x + float32(math.Cos(angle)*dist)
		py := y + float32(math.Sin(angle)*dist)
		rot := float32(0)
		if ps.randomRotation {
			rot = float32(timingRand(particleSeed, 3) * math.Pi * 2)
		}
		rot += float32(ps.angularVelocity * systemAge)
		size := float32(ps.startSize * sizeScale * parentScale * unitPx)
		if size < 0.5 {
			continue
		}
		c := timingParticleColor(assets, ps, particleSeed, p, globalTime)
		if img := assets.image(ps.texture); img != nil {
			drawTimingSprite(dst, img, px, py, size, rot, c)
		} else {
			drawTimingQuad(dst, px, py, size, rot, c)
		}
	}
}

func evalTimingCurve(keys []timingCurveKey, t float64) float64 {
	if len(keys) == 0 {
		return 1
	}
	if t <= keys[0].t {
		return keys[0].v
	}
	for i := 1; i < len(keys); i++ {
		prev, next := keys[i-1], keys[i]
		if t <= next.t {
			u := (t - prev.t) / (next.t - prev.t)
			return prev.v + (next.v-prev.v)*u
		}
	}
	return keys[len(keys)-1].v
}

func timingParticleColor(assets *timingAccuracyAssets, ps timingParticleSystem, seed uint32, lifeP, globalTime float64) color.RGBA {
	c := ps.minColor
	if ps.randomColor {
		c = lerpRGBA(ps.minColor, ps.maxColor, timingRand(seed, 4))
	}
	alpha := (float64(c.A) / 255) * (0.5 + 0.5*lifeP)
	if ps.material == timingMaterialAce {
		grayscale := (float64(c.R) + float64(c.G) + float64(c.B)) / (255 * 3)
		c = timingSampleAceColor(assets.aceColors, grayscale+2.5*globalTime)
	}
	c.A = uint8(math.Max(0, math.Min(255, alpha*255)))
	return c
}

func lerpRGBA(a, b color.RGBA, t float64) color.RGBA {
	return color.RGBA{
		R: uint8(float64(a.R) + (float64(b.R)-float64(a.R))*t),
		G: uint8(float64(a.G) + (float64(b.G)-float64(a.G))*t),
		B: uint8(float64(a.B) + (float64(b.B)-float64(a.B))*t),
		A: uint8(float64(a.A) + (float64(b.A)-float64(a.A))*t),
	}
}

func timingSampleAceColor(colors []color.RGBA, t float64) color.RGBA {
	if len(colors) == 0 {
		return color.RGBA{255, 245, 80, 255}
	}
	t -= math.Floor(t)
	pos := t * float64(len(colors)-1)
	i := int(pos)
	if i >= len(colors)-1 {
		return colors[len(colors)-1]
	}
	return lerpRGBA(colors[i], colors[i+1], pos-float64(i))
}

type timingAccuracyAssets struct {
	once      sync.Once
	main      *ebiten.Image
	circle    *ebiten.Image
	star1     *ebiten.Image
	aceColors []color.RGBA
}

var timingAccuracy timingAccuracyAssets

func timingAccuracyImages(assetsRoot string) *timingAccuracyAssets {
	timingAccuracy.once.Do(func() {
		dir := filepath.Join(assetsRoot, "common", "timing_accuracy")
		timingAccuracy.main = loadTimingParticleMask(filepath.Join(dir, "main.png"), false)
		timingAccuracy.circle = loadTimingParticleMask(filepath.Join(dir, "circle.png"), true)
		timingAccuracy.star1 = loadTimingParticleMask(filepath.Join(dir, "star1.png"), true)
		timingAccuracy.aceColors = loadTimingAceColors(filepath.Join(dir, "acecolors.png"))
	})
	return &timingAccuracy
}

func (a *timingAccuracyAssets) image(texture timingParticleTexture) *ebiten.Image {
	switch texture {
	case timingTextureMain:
		return a.main
	case timingTextureCircle:
		return a.circle
	case timingTextureStar:
		return a.star1
	default:
		return nil
	}
}

func loadTimingParticleMask(path string, alphaFromLuma bool) *ebiten.Image {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()
	src, _, err := image.Decode(f)
	if err != nil {
		return nil
	}
	b := src.Bounds()
	img := image.NewNRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	for y := 0; y < b.Dy(); y++ {
		for x := 0; x < b.Dx(); x++ {
			r, g, bl, a := src.At(b.Min.X+x, b.Min.Y+y).RGBA()
			alpha := uint8(a >> 8)
			if alphaFromLuma {
				// Unity imports circle.png/star1.png with alphaUsage=FromGrayScale.
				// OverlayStarShader ignores texture RGB and uses this alpha as the mask.
				alpha = uint8(((r*2126 + g*7152 + bl*722) / 10000) >> 8)
			}
			img.SetNRGBA(x, y, color.NRGBA{R: 255, G: 255, B: 255, A: alpha})
		}
	}
	return ebiten.NewImageFromImage(img)
}

func loadTimingAceColors(path string) []color.RGBA {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	if err != nil {
		return nil
	}
	b := img.Bounds()
	if b.Dx() == 0 || b.Dy() == 0 {
		return nil
	}
	colors := make([]color.RGBA, b.Dx())
	for x := 0; x < b.Dx(); x++ {
		n := color.NRGBAModel.Convert(img.At(b.Min.X+x, b.Min.Y)).(color.NRGBA)
		colors[x] = color.RGBA{n.R, n.G, n.B, n.A}
	}
	return colors
}

func drawTimingSprite(dst, img *ebiten.Image, x, y, height, rot float32, c color.RGBA) {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	if w == 0 || h == 0 {
		return
	}
	scale := float64(height) / float64(h)
	op := &ebiten.DrawImageOptions{Filter: ebiten.FilterLinear}
	op.GeoM.Translate(-float64(w)/2, -float64(h)/2)
	op.GeoM.Scale(scale, scale)
	op.GeoM.Rotate(float64(rot))
	op.GeoM.Translate(float64(x), float64(y))
	op.ColorScale.ScaleWithColor(c)
	dst.DrawImage(img, op)
}

func drawTimingQuad(dst *ebiten.Image, x, y, size, rot float32, c color.RGBA) {
	half := size / 2
	cosR, sinR := float32(math.Cos(float64(rot))), float32(math.Sin(float64(rot)))
	corners := [4][2]float32{{-half, -half}, {half, -half}, {half, half}, {-half, half}}
	var p vector.Path
	for i, corner := range corners {
		px := x + corner[0]*cosR - corner[1]*sinR
		py := y + corner[0]*sinR + corner[1]*cosR
		if i == 0 {
			p.MoveTo(px, py)
		} else {
			p.LineTo(px, py)
		}
	}
	p.Close()
	vs, is := p.AppendVerticesAndIndicesForFilling(nil, nil)
	cr, cg, cb, ca := float32(c.R)/255, float32(c.G)/255, float32(c.B)/255, float32(c.A)/255
	for i := range vs {
		vs[i].ColorR, vs[i].ColorG, vs[i].ColorB, vs[i].ColorA = cr, cg, cb, ca
	}
	dst.DrawTriangles(vs, is, whitePixel, &ebiten.DrawTrianglesOptions{AntiAlias: true})
}

// drawTri 画一个指向时机条的小三角（down=true 表示顶点朝下）。
func drawTri(dst *ebiten.Image, x, y, r float32, down bool) {
	dir := float32(1)
	if !down {
		dir = -1
	}
	var p vector.Path
	p.MoveTo(x-r, y-dir*r)
	p.LineTo(x+r, y-dir*r)
	p.LineTo(x, y+dir*r)
	p.Close()
	vs, is := p.AppendVerticesAndIndicesForFilling(nil, nil)
	for i := range vs {
		vs[i].ColorR, vs[i].ColorG, vs[i].ColorB, vs[i].ColorA = 1, 1, 1, 0.9
	}
	dst.DrawTriangles(vs, is, whitePixel, &ebiten.DrawTrianglesOptions{AntiAlias: true})
}

var whitePixel = func() *ebiten.Image {
	img := ebiten.NewImage(3, 3)
	img.Fill(color.White)
	return img
}()
