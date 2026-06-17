package engine

import "github.com/hajimehoshi/ebiten/v2"

type menuRank int

const (
	menuRankUnplayed menuRank = iota
	menuRankTryAgain
	menuRankOK
	menuRankSuperb
)

type menuSortMode int

const (
	menuSortTitle menuSortMode = iota
	menuSortBPM
	menuSortRank
)

type menuLevel struct {
	path       string
	key        string
	fileName   string
	title      string
	author     string
	desc       string
	games      []string
	bpm        float64
	customIcon *ebiten.Image
	rank       menuRank
	perfect    bool
	favorite   bool
}

type libraryAssets struct {
	bgBase         *ebiten.Image
	bgGradient     *ebiten.Image
	bgStars        *ebiten.Image
	bgWaves        *ebiten.Image
	borderSheet    *ebiten.Image
	border         *ebiten.Image
	borderUnplayed *ebiten.Image
	borderTryAgain *ebiten.Image
	borderOK       *ebiten.Image
	borderSuperb   *ebiten.Image
	borderPerfect  *ebiten.Image
}
