package engine

import "github.com/hajimehoshi/ebiten/v2/text/v2"

type fontState struct {
	faceBig, faceMid, faceSmall *text.GoTextFace
}
