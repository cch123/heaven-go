package engine

import (
	"image"
	_ "image/jpeg"
	_ "image/png"
	"log"
	"os"
	"path/filepath"

	"github.com/hajimehoshi/ebiten/v2"
)

func loadResultAssets(dir string) resultAssets {
	load := func(name string) *ebiten.Image {
		f, err := os.Open(filepath.Join(dir, name))
		if err != nil {
			return nil
		}
		defer f.Close()
		img, _, err := image.Decode(f)
		if err != nil {
			log.Printf("engine: decode result asset %s: %v", name, err)
			return nil
		}
		return ebiten.NewImageFromImage(img)
	}
	return resultAssets{
		bg:          load("judgementBg.png"),
		rankHi:      load(filepath.Join("Superb", "superbrating.png")),
		rankHiStar:  load(filepath.Join("Superb", "superbratingstar.png")),
		rankOk:      load(filepath.Join("OK", "okrating.png")),
		rankOkSweat: load(filepath.Join("OK", "okratingsweat.png")),
		rankNg: []*ebiten.Image{
			load(filepath.Join("TryAgain", "tryagainrating0001.png")),
			load(filepath.Join("TryAgain", "tryagainrating0002.png")),
			load(filepath.Join("TryAgain", "tryagainrating0003.png")),
		},
		epHi: load(filepath.Join("Epilogue", "superb.png")),
		epOk: load(filepath.Join("Epilogue", "ok.png")),
		epNg: load(filepath.Join("Epilogue", "tryagain.png")),
	}
}

func loadLibraryAssets(dir string) libraryAssets {
	load := func(name string) *ebiten.Image {
		f, err := os.Open(filepath.Join(dir, name))
		if err != nil {
			return nil
		}
		defer f.Close()
		img, _, err := image.Decode(f)
		if err != nil {
			log.Printf("engine: decode library asset %s: %v", name, err)
			return nil
		}
		return ebiten.NewImageFromImage(img)
	}
	assets := libraryAssets{
		bgBase:      load(filepath.Join("bg", "libBgBase.png")),
		bgGradient:  load(filepath.Join("bg", "libBgGradient.png")),
		bgStars:     load(filepath.Join("bg", "libBgStars.png")),
		bgWaves:     load(filepath.Join("bg", "libBgWaves.png")),
		borderSheet: load("levelBorders.png"),
	}
	// Unity's sprite atlas rects use a bottom-left origin. This is the original
	// unplayed level border slice from levelBorders.png.
	if assets.borderSheet != nil {
		sheetH := assets.borderSheet.Bounds().Dy()
		rect := image.Rect(40, sheetH-740-576, 40+576, sheetH-740)
		if sub, ok := assets.borderSheet.SubImage(rect).(*ebiten.Image); ok {
			assets.border = sub
		}
	}
	return assets
}
