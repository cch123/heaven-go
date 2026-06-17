package engine

import (
	"bytes"
	"image"
	"log"
	"os"
	"path/filepath"

	"github.com/hajimehoshi/ebiten/v2"
)

// filterNames 对应 Filter.FilterType 枚举顺序（按枚举值索引文件名）。
var filterNames = []string{
	"accent", "air", "atri", "bleach", "bleak", "blockbuster", "cinecold", "cinewarm",
	"colorshift", "dawn", "deepfry", "deuteranopia", "exposed", "friend", "friend_diffusion",
	"gamebob", "gamebob_2", "gameboy", "gameboy_color", "glare", "grayscale",
	"grayscale_invert", "invert", "iso_blue", "iso_cyan", "iso_green", "iso_highlights",
	"iso_magenta", "iso_mid", "iso_red", "iso_shadows", "iso_yellow", "maritime",
	"moonlight", "nightfall", "polar", "poster", "protanopia", "redder", "sanic",
	"sepia", "sepier", "sepiest", "shareware", "shift_behind", "shift_left", "shift_right",
	"tina", "tiny_palette", "toxic", "tritanopia", "vibrance", "winter", "blackwhite",
	"blackwhite_2",
}

// LUT 条带 1024×32 与屏幕 960×540 尺寸不同，统一垫到能容纳两者的画布。
const (
	fxPadW = 1024
	fxPadH = 544
)

func (f *filterFX) lut(assetsRoot, name string) *ebiten.Image {
	if f.luts == nil {
		f.luts = map[string]*ebiten.Image{}
	}
	if img, ok := f.luts[name]; ok {
		return img
	}
	raw, err := os.ReadFile(filepath.Join(assetsRoot, "common", "filters", name+".png"))
	if err != nil {
		log.Printf("engine: filter LUT %s 缺失（运行 extract -game common）", name)
		f.luts[name] = nil
		return nil
	}
	img, _, err := image.Decode(bytes.NewReader(raw))
	if err != nil {
		f.luts[name] = nil
		return nil
	}
	pad := ebiten.NewImage(fxPadW, fxPadH)
	pad.DrawImage(ebiten.NewImageFromImage(img), nil)
	f.luts[name] = pad
	return f.luts[name]
}
