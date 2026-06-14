package engine

import "github.com/hajimehoshi/ebiten/v2"

type resultRank int

const (
	resultRankNg resultRank = iota
	resultRankOk
	resultRankHi
)

type resultScoreInput struct {
	Beat     float64
	Accuracy float64
	Weight   float64
	Category int
}

type resultSummary struct {
	Score      float64
	Rank       resultRank
	Header     string
	Message0   string
	Message1   string
	Message2   string
	TwoMessage bool
	SubRank    bool
	NoMiss     bool
	Perfect    bool
	Star       bool
}

type resultAssets struct {
	bg          *ebiten.Image
	rankHi      *ebiten.Image
	rankHiStar  *ebiten.Image
	rankOk      *ebiten.Image
	rankOkSweat *ebiten.Image
	rankNg      []*ebiten.Image
	epHi        *ebiten.Image
	epOk        *ebiten.Image
	epNg        *ebiten.Image
}
