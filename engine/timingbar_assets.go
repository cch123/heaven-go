package engine

import (
	"image"
	"image/color"
	"os"
	"path/filepath"
	"sync"

	"github.com/hajimehoshi/ebiten/v2"
)

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
