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
		assets.borderTryAgain = unitySpriteSubImage(assets.borderSheet, 40, 1436, 576, 576)
		assets.borderOK = unitySpriteSubImage(assets.borderSheet, 736, 1436, 576, 576)
		assets.borderSuperb = unitySpriteSubImage(assets.borderSheet, 1432, 1436, 576, 576)
		assets.borderUnplayed = unitySpriteSubImage(assets.borderSheet, 40, 740, 576, 576)
		assets.borderPerfect = unitySpriteSubImage(assets.borderSheet, 1432, 740, 576, 576)
		assets.border = assets.borderUnplayed
	}
	return assets
}

func unitySpriteSubImage(sheet *ebiten.Image, x, y, w, h int) *ebiten.Image {
	if sheet == nil {
		return nil
	}
	sheetH := sheet.Bounds().Dy()
	rect := image.Rect(x, sheetH-y-h, x+w, sheetH-y)
	if sub, ok := sheet.SubImage(rect).(*ebiten.Image); ok {
		return sub
	}
	return nil
}
