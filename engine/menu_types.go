package engine

import "github.com/hajimehoshi/ebiten/v2"

type menuLevel struct {
	path       string
	fileName   string
	title      string
	author     string
	desc       string
	games      []string
	bpm        float64
	customIcon *ebiten.Image
}

type libraryAssets struct {
	bgBase      *ebiten.Image
	bgGradient  *ebiten.Image
	bgStars     *ebiten.Image
	bgWaves     *ebiten.Image
	borderSheet *ebiten.Image
	border      *ebiten.Image
}
