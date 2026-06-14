package kart

import (
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
)

func TestCellAnimeShaderCompiles(t *testing.T) {
	if _, err := ebiten.NewShader([]byte(cellAnimeKage)); err != nil {
		t.Fatalf("CellAnime shader compile failed: %v", err)
	}
}
